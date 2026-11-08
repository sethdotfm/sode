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

	// pin: Advanced Settings PIN hash this connection unlocked, guarded by App.mu; a changed or
	// removed PIN stops matching.
	pin string

	// sent: a /log page receiving sent-log entries. Guarded by App.mu.
	sent bool

	u connUser // page type, unlocked user, session token (users.go)
}

// wsReadLimit: see TestMessageSizes (largest are a Settings save and a presets save).
const wsReadLimit = 64 << 10

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

// csp: same-origin scripts only (no inline bodies or on*= handlers; a test keeps it so);
// 'unsafe-inline' for styles only.
const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline' https://fonts.googleapis.com; " +
	"img-src 'self' data:; connect-src 'self' ws: wss:; font-src 'self' https://fonts.gstatic.com; object-src 'none'; " +
	"base-uri 'self'; frame-ancestors 'none'"

func secureHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", csp)
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "same-origin")
		next.ServeHTTP(w, r)
	})
}

type healthMsg struct {
	Server        string `json:"server"`
	Router        string `json:"router"`
	Addr          string `json:"targetAddress"`
	Err           string `json:"err,omitempty"`
	Version       string `json:"version"`
	UptimeSeconds int64  `json:"uptimeSeconds"`
	Browsers      int    `json:"browsers"`
}

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

// clientIP: a hop that isn't an address ends the search at the peer rather than trusting
// what lies beyond.
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

// wsHeartbeat: browsers can't see ping frames, so a silently dropped socket would stay
// OPEN on the page. A var so tests can shorten it.
var wsHeartbeat = 5 * time.Second

var pingMsg = []byte(`{"t":"ping"}`)

func (a *App) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	budget := newReadBudget(conn.SetReadLimit)
	cc := &connCtx{budget: budget}

	c := &client{ip: clientIP(r, a.r().proxies), send: make(chan []byte, 256)}
	if r.Header.Get("X-Forwarded-For") != "" && len(a.r().proxies) == 0 {
		a.xffWarn.Do(func() {
			log.Printf("ws: behind a proxy but trustedProxies is empty; set it to the proxy's address so history shows real client IPs")
		})
	}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Register and queue the snapshot under one lock so no delta arrives ahead of it.
	page := a.readHello()
	a.mu.Lock()
	a.clients[c] = struct{}{}
	c.send <- a.hello(page)
	a.mu.Unlock()
	log.Printf("ws %s: connected", c.ip)
	connected := time.Now()
	a.logEvent(event(kindBrowser, "connect", c.ip, "connected"))

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
		budget.Reset()
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
		rl.handle(a, c, &m, cc)
	}
}

func (a *App) serveTake(c *client, m *inMsg) {
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
	go a.take(c, m.Items)
}

func (a *App) serveConfig(c *client, m *inMsg) {
	if !present(m.Config) {
		return
	}
	// Who saved, not what (settings hold the PIN hash).
	go func(cfg json.RawMessage, pin string, clearPin bool) {
		a.replyOp(c, "config", -1, 0, "", a.saveSettings(c, cfg, pin, clearPin))
	}(m.Config, m.Pin, m.ClearPin)
}

func present(r json.RawMessage) bool { return r != nil && string(r) != "null" }

func outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errLocked), errors.Is(err, errPageLocked):
		return "locked"
	}
	return "error"
}

// inMsg: every type read here must be in policy.go.
type inMsg struct {
	T          string             `json:"t"`
	Before     int                `json:"before"`
	N          int                `json:"n"`    // history page size; 0 = the default
	From       int64              `json:"from"` // uptime range, unix ms
	To         int64              `json:"to"`
	Slot       int64              `json:"slot"`   // ...and its slot length, ms
	Config     json.RawMessage    `json:"config"` // decoded in saveSettings, over the live config
	Pin        string             `json:"pin"`
	User       string             `json:"user"` // userUnlock: the username (Username and PIN mode)
	ClearPin   bool               `json:"clearPin"`
	ID         int                `json:"id"` // echoed by unlock and save replies
	Items      []routeReq         `json:"items"`
	Preset     string             `json:"preset"` // take: the preset that was fired, if any
	Salvo      string             `json:"salvo"`  // the same, from a page loaded before presets were renamed
	Sets       []config.FilterSet `json:"sets"`
	Prefs      map[string]string  `json:"prefs"`
	FilterSets []config.FilterSet `json:"filterSets"`
	Presets    []config.Preset    `json:"presets"`
	PinOrder   []string           `json:"pinOrder"`
	Page       string             `json:"page"`  // hi: main | log
	Token      string             `json:"token"` // hi: session token of a persistent user or a tab login
	Tab        bool               `json:"tab"`   // userUnlock: no lock on, wants a tab session token
	Name       string             `json:"name"`  // userSignOutAll, profileSave, profileGet
	Old        string             `json:"old"`
	New        string             `json:"new"`
	UsersReq   *usersReq          `json:"usersReq"`
	Pins       bool               `json:"pins"`    // configExport: include the PIN hashes
	YAML       string             `json:"yaml"`    // configImport: the file
	Size       int                `json:"size"`    // configImportBegin: the file's size in bytes
	KeyName    string             `json:"keyName"` // keyCreate, keyCreateFor
	KeyID      string             `json:"keyId"`   // keyRevoke, keyRevokeAny
	Expires    int64              `json:"expires"` // key expiry, unix ms; 0 = never
	Owner      *keyOwner          `json:"owner"`   // keyCreateFor, keyRevokeAll
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// A range with no end runs to now; no start, a week back.
func (a *App) uptimeReply(id int, from, to, slot int64) uptimeMsg {
	now := time.Now().UnixMilli()
	if to <= 0 || to > now {
		to = now
	}
	if from <= 0 {
		from = to - 7*24*3600_000
	}
	// Cut here too so slots count from the start the reply says.
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

func locked(err error) *bool {
	l := errors.Is(err, errLocked)
	return &l
}
