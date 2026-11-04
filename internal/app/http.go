package app

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

type client struct {
	ip   string
	send chan []byte

	// pin is the settings PIN hash this connection unlocked, guarded by
	// App.mu; a changed or removed PIN no longer matches it.
	pin string

	// sent: this is a /log page, sent new sent log entries. Guarded by App.mu.
	sent bool

	u connUser // page type, unlocked user, session token (users.go)
}

// wsReadLimit is the most one browser message may be. The largest ones are
// a Settings save and a presets save; see TestMessageSizes.
const wsReadLimit = 64 << 10

// Handler serves the embedded UI, the WebSocket and a health check.
func (a *App) Handler(static fs.FS) http.Handler {
	mux := http.NewServeMux()
	files := http.FileServerFS(static)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
	mux.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, static, "log.html")
	})
	mux.HandleFunc("/icon.png", serveIcon)
	mux.HandleFunc("/manifest.webmanifest", a.serveManifest)
	mux.HandleFunc("/ws", a.serveWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		st := a.status
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
	})
	mux.HandleFunc("/api/health", a.serveHealth)
	mux.HandleFunc("/api/yaml", a.serveYAML)
	return secureHeaders(mux)
}

// csp is the one Content-Security-Policy for every response. Scripts are
// same-origin files only (no inline bodies, no on*= handlers: a test over the
// embedded pages keeps it so); inline style attributes are used, hence
// 'unsafe-inline' for styles only. The socket is same-origin ws(s).
const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"img-src 'self' data:; connect-src 'self' ws: wss:; font-src 'self' https://fonts.gstatic.com; object-src 'none'; " +
	"base-uri 'self'; frame-ancestors 'none'"

// secureHeaders sets the browser-hardening headers in one place.
func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

// healthMsg is the /api/health body.
type healthMsg struct {
	Server        string `json:"server"`
	Router        string `json:"router"`
	Addr          string `json:"targetAddress"`
	Err           string `json:"err,omitempty"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	Browsers      int    `json:"browsers"`
}

// serveHealth reports server and router health apart for uptime monitors:
// 200 while the router is up, 503 otherwise. No PIN, no config.
func (a *App) serveHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	a.mu.Lock()
	st := a.status
	browsers := len(a.clients)
	a.mu.Unlock()
	code := http.StatusOK
	if st.Router != string(swp08.StateUp) {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(healthMsg{"up", st.Router, st.Addr, st.Err, VersionLabel(), int64(time.Since(a.started).Seconds()), browsers})
}

// clientIP is the peer address, or, when the peer is a trusted proxy, the
// nearest untrusted hop in X-Forwarded-For (X-Real-IP when there is none).
// Untrusted peers can't spoof it, and a hop that isn't an address ends the
// search at the peer rather than trusting what lies beyond it.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	parse := func(s string) (netip.Addr, bool) {
		a, err := netip.ParseAddr(strings.TrimSpace(s))
		return a.Unmap(), err == nil
	}
	isTrusted := func(a netip.Addr) bool {
		return slices.ContainsFunc(trusted, func(p netip.Prefix) bool { return p.Contains(a) })
	}
	if a, ok := parse(host); !ok || !isTrusted(a) {
		return host
	}
	// Several header lines are one list, in order.
	lines := r.Header.Values("X-Forwarded-For")
	if len(lines) == 0 {
		lines = r.Header.Values("X-Real-IP") // nginx's usual recipe
	}
	for _, h := range slices.Backward(strings.Split(strings.Join(lines, ","), ",")) {
		if strings.TrimSpace(h) == "" {
			continue
		}
		a, ok := parse(h)
		if !ok {
			return host
		}
		if !isTrusted(a) {
			return a.String()
		}
	}
	return host
}

// How often each browser gets a {"t":"ping"} message. Browsers can't see
// WebSocket ping frames, so without this a socket a proxy or the network
// dropped silently stays OPEN on the page (#142). A var so a test can shorten it.
var wsHeartbeat = 5 * time.Second

var pingMsg = []byte(`{"t":"ping"}`)

func (a *App) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	c := &client{ip: clientIP(r, a.r().proxies), send: make(chan []byte, 256)}
	if r.Header.Get("X-Forwarded-For") != "" && len(a.r().proxies) == 0 {
		a.xffWarn.Do(func() {
			log.Printf("ws: behind a proxy but trustedProxies is empty; set it to the proxy's address so history shows real client IPs")
		})
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Register and queue the snapshot under one lock so no delta can
	// arrive ahead of it.
	page := a.readHello()
	a.mu.Lock()
	a.clients[c] = struct{}{}
	c.send <- a.hello(page)
	a.mu.Unlock()
	log.Printf("ws %s: connected", c.ip)
	connected := time.Now()
	a.logEvent(event(kindBrowser, "connect", c.ip, "connected"))

	// Why the read loop ended, for the log.
	var gone error
	defer func() {
		a.mu.Lock()
		if _, ok := a.clients[c]; ok {
			delete(a.clients, c)
			close(c.send)
		}
		a.mu.Unlock()
		log.Printf("ws %s: disconnected", c.ip)
		e := event(kindBrowser, "disconnect", c.ip, "disconnected")
		e.Note = "connected " + time.Since(connected).Round(time.Second).String()
		switch websocket.CloseStatus(gone) {
		case websocket.StatusNormalClosure, websocket.StatusGoingAway:
		default:
			if gone != nil && !errors.Is(gone, context.Canceled) {
				e.Err = gone.Error()
			}
		}
		a.logEvent(e)
	}()

	go func() {
		defer cancel()
		ping := time.NewTicker(20 * time.Second)
		defer ping.Stop()
		beat := time.NewTicker(wsHeartbeat)
		defer beat.Stop()
		write := func(b []byte) error {
			wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
			defer wcancel()
			return conn.Write(wctx, websocket.MessageText, b)
		}
		for {
			select {
			case <-ctx.Done():
				return
			case b, ok := <-c.send:
				if !ok {
					conn.Close(websocket.StatusPolicyViolation, "too slow")
					return
				}
				if write(b) != nil {
					return
				}
			case <-beat.C:
				if write(pingMsg) != nil {
					return
				}
			case <-ping.C:
				pctx, pcancel := context.WithTimeout(ctx, 10*time.Second)
				err := conn.Ping(pctx)
				pcancel()
				if err != nil {
					return
				}
			}
		}
	}()

	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			gone = err
			return
		}
		var m inMsg
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		rl, known := policy[m.T]
		if !known {
			continue // closed: an unlisted type is never dispatched
		}
		rl = rl.forMsg(&m)
		if err := a.admit(c, rl); err != nil {
			a.refuse(c, m.T, rl, &m, err)
			continue
		}
		switch m.T {
		case "hi", "userUnlock", "userLock", "userSetPin", "users", "usersList", "userSignOutAll":
			a.serveUsers(c, &m)
		case "batch":
			a.serveBatch(c, &m)
		case "history":
			go a.history(c, m.Before, m.N)
		case "sent":
			go a.sentHistory(c, int64(m.Before), m.N)
		case "uptime":
			a.reply(c, a.uptimeReply(m.ID, m.From, m.To, m.Slot))
		case "refreshNames":
			a.command(c, "refreshNames", "sent", "", nil)
			go a.RefreshNames()
		case "unlock":
			// Checked in line, so a lock sent after it is handled after it
			// and one connection checks one PIN at a time.
			res := opResult{T: "unlockResult", OK: true, ID: &m.ID}
			err := a.Unlock(c, m.Pin)
			if err != nil {
				res.OK, res.Err = false, err.Error()
			}
			// Never the PIN, only whether it was right.
			a.command(c, "unlock", map[bool]string{true: "ok", false: "refused"}[err == nil], "", err)
			a.reply(c, res)
		case "lock":
			a.Lock(c)
			a.command(c, "lock", "ok", "", nil)
		case "clearHistory":
			a.command(c, "clearHistory", "ok", "", nil)
			go a.ClearHistory(c.who())
		case "filterSets":
			go func(sets []config.FilterSet, id int) {
				a.replyOp(c, "filterSets", id, len(sets), "set", a.SaveFilterSets(c, sets))
			}(m.Sets, m.ID)
		case "presets":
			go func(presets []config.Preset, id int) {
				a.replyOp(c, "presets", id, len(presets), "preset", a.SavePresets(c, presets))
			}(m.Presets, m.ID)
		case "config":
			a.serveConfig(c, &m)
		case "profileSave":
			go a.serveProfileSave(c, &m)
		case "profileGet":
			a.serveProfileGet(c, &m)
		}
	}
}

// serveBatch handles a batch of takes: one command-log row for the lot, the
// preset's name (if any) stamped on every take, then the takes themselves.
func (a *App) serveBatch(c *client, m *inMsg) {
	if len(m.Items) > 0 {
		why, n := "take", len(m.Items)
		for _, it := range m.Items {
			if it.Undo != nil {
				why = "undo"
			} else if it.Retry != nil && why == "take" {
				why = "retry"
			}
		}
		a.command(c, why, "sent", fmt.Sprintf("%d %s", n, plural(n, "take", "takes")), nil)
	}
	if name := cleanPresetName(cmp.Or(m.Preset, m.Salvo)); name != "" {
		for i := range m.Items {
			m.Items[i].Preset = name
		}
	}
	go a.batch(c, m.Items)
}

// serveConfig saves the Settings form in the background and answers when done.
func (a *App) serveConfig(c *client, m *inMsg) {
	if m.Config == nil {
		return
	}
	// Who saved, not what: the settings hold the PIN's hash. A page from
	// before deviceDefaults existed sends none, and its zero values would turn
	// the "on" defaults off for everyone, so the current ones are kept.
	go func(cfg configMsg, pin string, clearPin bool) {
		a.replyOp(c, "config", -1, 0, "", a.saveSettings(c, cfg.Config, pin, clearPin, !cfg.hasDefaults))
	}(*m.Config, m.Pin, m.ClearPin)
}

// configMsg is the Settings form's config, and whether it carried
// deviceDefaults at all.
type configMsg struct {
	config.Config
	hasDefaults bool
}

func (m *configMsg) UnmarshalJSON(b []byte) error {
	var probe struct {
		DeviceDefaults json.RawMessage `json:"deviceDefaults"`
	}
	if err := json.Unmarshal(b, &probe); err != nil {
		return err
	}
	m.hasDefaults = probe.DeviceDefaults != nil
	return json.Unmarshal(b, &m.Config)
}

// outcome is a command's result: ok, locked (a PIN or a lock was why) or error.
func outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errLocked), errors.Is(err, errPageLocked):
		return "locked"
	}
	return "error"
}

// inMsg is any browser-to-server message; T says which fields are used. Every
// type read here must be in policy (policy.go).
type inMsg struct {
	T        string             `json:"t"`
	Before   int                `json:"before"`
	N        int                `json:"n"`    // history page size; 0 = the default
	From     int64              `json:"from"` // uptime range, unix ms
	To       int64              `json:"to"`
	Slot     int64              `json:"slot"` // ...and its slot length, ms
	Config   *configMsg         `json:"config"`
	Pin      string             `json:"pin"`
	ClearPin bool               `json:"clearPin"`
	ID       int                `json:"id"` // echoed by unlock and save replies
	Items    []takeReq          `json:"items"`
	Preset   string             `json:"preset"` // batch: the preset that was fired, if any
	Salvo    string             `json:"salvo"`  // the same, from a page loaded before presets were renamed
	Sets     []config.FilterSet `json:"sets"`
	// profileSave: the parts to replace; nil = keep (an empty list or {} clears)
	Prefs      map[string]string  `json:"prefs"`
	FilterSets []config.FilterSet `json:"filterSets"`
	Presets    []config.Preset    `json:"presets"`
	Page       string             `json:"page"`  // hi: main | log
	Token      string             `json:"token"` // hi: session token of a persistent user or a tab login
	Tab        bool               `json:"tab"`   // userUnlock: no lock on, wants a tab session token
	Name       string             `json:"name"`  // userSignOutAll, profileSave, profileGet
	Old        string             `json:"old"`
	New        string             `json:"new"`
	UsersReq   *usersReq          `json:"usersReq"`
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// uptimeReply is the link's record for an uptime request. A range with no
// end runs to now; one with no start, a week back.
func (a *App) uptimeReply(id int, from, to, slot int64) uptimeMsg {
	now := time.Now().UnixMilli()
	if to <= 0 || to > now {
		to = now
	}
	if from <= 0 {
		from = to - 7*24*3600_000
	}
	// Cut here, not only in Range, so the slots are counted from the start the
	// reply says.
	if oldest := time.UnixMilli(now).Add(-uptimeKeep).UnixMilli(); from < oldest {
		from = oldest
	}
	r := a.up.Range(from, to, slot)
	m := uptimeMsg{T: "uptimeResult", ID: id, From: from, To: to, Slot: max(slot, 60_000), Now: now, Start: r.Start, Trans: r.Trans, Counts: r.Counts}
	if m.Trans == nil {
		m.Trans = []UptimeTrans{}
	}
	if m.Counts == nil {
		m.Counts = []UptimeSlot{}
	}
	return m
}

// locked is opResult.Locked for a failed save: whether the PIN was why.
func locked(err error) *bool {
	l := errors.Is(err, errLocked)
	return &l
}
