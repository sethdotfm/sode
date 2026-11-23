package app

import (
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
)

type client struct {
	ip   string
	send chan []byte

	// secure: the upgrade request came through a trusted proxy marked HTTPS (ADR 0005).
	secure bool

	// sent: a /log page that gets Communication Log rows. Guarded by App.mu.
	sent bool

	// api: set for a caller that is not a browser session; its Actor and Rights replace
	// the lookup callerOf does for a browser.
	api *apiCtx
	// forPage: the page connection that this in-process caller stands for (apiCtx.page). Who and the Rights are its own,
	// and its entries carry no API actor.
	forPage *client

	u connUser // page type, signed-in user, session token (users.go)
}

// wsReadLimit: see TestMessageSizes (largest are a Settings save and a presets save).
const wsReadLimit = 64 << 10

// A page sends one message per action. Past this a socket is closed, since each refusal writes a log row.
const wsMsgsPerSec = 100

func (a *App) Handler(static fs.FS) http.Handler {
	mux := http.NewServeMux()
	files := http.FileServerFS(static)
	cached := &staticFiles{fsys: static}
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name == "" {
			name = "index.html"
		}
		// FileServer keeps its redirects (/index.html to /) and folder answers.
		if r.URL.Path == "/index.html" || !fs.ValidPath(name) || !cached.serve(w, r, name) {
			w.Header().Set("Cache-Control", "no-cache")
			files.ServeHTTP(w, r)
		}
	})
	page := func(name string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if !cached.serve(w, r, name) {
				http.NotFound(w, r)
			}
		}
	}
	mux.HandleFunc("/log", page("log.html"))
	// {id} is any segment; the editor's pattern is more specific, so it wins.
	mux.HandleFunc("/panels", page("panels.html"))
	mux.HandleFunc("/panels/{$}", page("panels.html"))
	mux.HandleFunc("/panels/editor", page("paneleditor.html"))
	// Only a segment that could be an id or a short name reaches the page. Whether one exists is
	// the panel read route's answer, which needs a key, so this route says nothing about panels.
	panelPage := page("panel.html")
	mux.HandleFunc("/panels/{id}", func(w http.ResponseWriter, r *http.Request) {
		if id := r.PathValue("id"); !validPanelID(id) && !validPanelSlug(id) {
			http.NotFound(w, r)
			return
		}
		panelPage(w, r)
	})
	worker := &shellWorker{fsys: static}
	mux.HandleFunc("GET /sw.js", worker.serve)
	mux.HandleFunc("/icon.png", serveIcon)
	mux.HandleFunc("/manifest.webmanifest", a.serveManifest)
	mux.HandleFunc("/ws", a.serveWS)
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		a.mu.Lock()
		st := a.firstUnit().status
		a.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(st)
	})
	mux.HandleFunc("/api/health", a.serveHealth)
	mux.HandleFunc("POST /api/log/download", a.serveLogDownload)
	mux.HandleFunc("POST /api/export", a.serveExport)
	mux.HandleFunc("POST /api/export/size", a.serveExportSize)
	mux.HandleFunc("GET /api/export/items", a.serveExportItems)
	mux.HandleFunc("POST /api/import", a.serveImportCheck)
	mux.HandleFunc("POST /api/import/apply", a.serveImportApply)
	mux.HandleFunc("POST /api/own/export", a.serveOwnExport)
	mux.HandleFunc("POST /api/own/export/size", a.serveOwnExportSize)
	mux.HandleFunc("GET /api/own/export/items", a.serveOwnExportItems)
	mux.HandleFunc("POST /api/own/import", a.serveOwnImportCheck)
	mux.HandleFunc("POST /api/own/import/apply", a.serveOwnImportApply)
	mux.HandleFunc("POST /api/setup/restore", a.serveSetupRestore)
	a.mountAPI(mux)
	return a.httpsGate(secureHeaders(mux))
}

// csp: same-origin scripts only (no inline bodies or on*= handlers; a test keeps it so);
// 'unsafe-inline' for styles only.
const csp = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; " +
	"img-src 'self' data: blob:;connect-src 'self' ws: wss:; font-src 'self'; object-src 'none'; " +
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

// clientIP: a hop that is not an address ends the search at the peer. Sode does not trust what lies beyond it.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, ok := trustedPeer(r, trusted)
	if !ok {
		return host
	}
	parse := func(s string) (netip.Addr, bool) {
		a, err := netip.ParseAddr(strings.TrimSpace(s))
		return a.Unmap(), err == nil
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
		if !inPrefixes(a, trusted) {
			return a.String()
		}
	}
	return host
}

// wsHeartbeat: browsers cannot see ping frames, so a silently dropped socket would stay
// OPEN on the page. A var so tests can shorten it.
var wsHeartbeat = 5 * time.Second

var pingMsg = []byte(`{"t":"ping"}`)

func (a *App) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(wsReadLimit)

	c := &client{ip: clientIP(r, a.r().proxies), secure: secureRequest(r, a.r().proxies), send: make(chan []byte, 256)}
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
		if c.u.step != nil {
			a.stepOutRowMu(c, "closed")
		}
		a.cardsSyncMu() // a page close cancels its request
		a.mu.Unlock()
		a.setupLeft(c)
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

	var win time.Time
	n := 0
	for {
		_, data, err := conn.Read(ctx)
		if err != nil {
			gone = err
			return
		}
		if now := time.Now(); now.Sub(win) >= time.Second {
			win, n = now, 0
		}
		if n++; n > wsMsgsPerSec {
			conn.Close(websocket.StatusPolicyViolation, "too many messages")
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
		// In line, like stepIn: the approver's PIN is checked before this message is admitted.
		if m.Approval != nil {
			if err := a.approve(c, rl, &m); err != nil {
				a.refuse(c, m.T, rl.forMsg(&m), &m, err)
				continue
			}
		}
		rl = rl.forMsg(&m)
		if err := a.admit(c, rl); err != nil {
			a.refuse(c, m.T, rl, &m, err)
			continue
		}
		if rl.write {
			done, ok := a.writes.start(m.T)
			if !ok {
				a.refuse(c, m.T, rl, &m, errShuttingDown)
				continue
			}
			m.done = done
		}
		rl.handle(a, c, &m)
	}
}

func (a *App) serveTake(c *client, m *inMsg) {
	if len(m.Items) > 0 {
		n := len(m.Items)
		a.command(c, takeWhy(m.Items), "sent", fmt.Sprintf("%d %s", n, plural(n, "take", "takes")), nil)
	}
	name, id := cleanPresetName(m.Preset), cleanPresetName(m.PresetID)
	if name != "" || id != "" {
		for i := range m.Items {
			m.Items[i].Preset, m.Items[i].PresetID = name, id
		}
	}
	go func() {
		defer m.finish()
		a.take(c, m.Items)
	}()
}

func (a *App) serveConfig(c *client, m *inMsg) {
	if !present(m.Config) {
		m.finish()
		return
	}
	// Who saved, not what (settings hold the PIN hash).
	go func(cfg json.RawMessage) {
		defer m.finish()
		a.replyOp(c, "config", -1, 0, "", a.saveSettings(c, cfg))
	}(m.Config)
}

func present(r json.RawMessage) bool { return r != nil && string(r) != "null" }

func outcome(err error) string {
	switch {
	case err == nil:
		return "ok"
	case errors.Is(err, errPageLocked):
		return "locked"
	}
	return "error"
}

// inMsg: every type read here must be in policy.go.
type inMsg struct {
	T      string               `json:"t"`
	done   func()               // ends the shutdown count for a write (rule.write); see finish
	Before int                  `json:"before"`
	N      int                  `json:"n"`    // history page size; 0 = the default
	From   int64                `json:"from"` // uptime range, unix ms
	To     int64                `json:"to"`
	Slot   int64                `json:"slot"`   // ...and its slot length, ms
	Config json.RawMessage      `json:"config"` // decoded in saveSettings, over the live config
	Fields map[string]fieldEdit `json:"fields"` // settingsSave, settingsReview: changed fields only
	// AccessPreset: settingsSave, the Access preset staged into it, for the Settings saves row.
	AccessPreset string             `json:"accessPreset"`
	Pin          string             `json:"pin"`
	User         string             `json:"user"` // userSignIn: the user's name (name and PIN mode)
	ID           int                `json:"id"`   // echoed by save replies
	Items        []routeReq         `json:"items"`
	Routes       json.RawMessage    `json:"routes"`   // tieTake: the routes of POST /takes
	Preset       string             `json:"preset"`   // take: the preset that was fired, if any
	PresetID     string             `json:"presetId"` // ...and its hidden id
	Sets         []config.FilterSet `json:"sets"`
	Prefs        map[string]string  `json:"prefs"`
	FilterSets   []config.FilterSet `json:"filterSets"`
	Presets      []config.Preset    `json:"presets"`
	Bookmarks    []config.Bookmark  `json:"bookmarks"`    // bookmarks (Everyone), profileSave (Me)
	TieLines     []config.TieLine   `json:"tieLines"`     // tieLines: the whole list
	PalettePicks []string           `json:"palettePicks"` // profileSave: the Palette's last picks
	Page         string             `json:"page"`         // hi: main | log
	Token        string             `json:"token"`        // hi: session token of a persistent user or a tab sign-in
	Tab          bool               `json:"tab"`          // userSignIn: no lock on, wants a tab session token
	Name         string             `json:"name"`         // userSignOutAll, profileSave, profileGet
	Old          string             `json:"old"`
	New          string             `json:"new"`
	Remove       bool               `json:"remove"`   // userSetPin: remove my PIN
	KeyName      string             `json:"keyName"`  // keyCreate, keyCreateFor
	KeyID        string             `json:"keyId"`    // keyRevoke, keyRevokeAny
	Expires      int64              `json:"expires"`  // key expiry, unix ms; 0 = never
	Owner        *keyOwner          `json:"owner"`    // keyCreateFor, keyRevokeAll
	Panel        string             `json:"panel"`    // panelAlarm: the panel id
	Action       string             `json:"action"`   // panelAlarm: clear | stop
	Instance     string             `json:"instance"` // panelAlarm: the watched device, empty for a whole panel
	MyClient     *myClientReq       `json:"client"`   // myClientSave
	Approval     *approvalReq       `json:"approval"` // an Approval in person for this one message
	approver     *config.User       // approve sets it after it checks the Approval's PIN
	Router       string             `json:"router"`  // setupRouter: the address typed; routerUses and uptime: the router UID
	Setup        *setupReq          `json:"setup"`   // setupFinish
	TakeOver     bool               `json:"take"`    // setupClaim: Take control from the browser that runs setup
	For          *inMsg             `json:"for"`     // approvalAsk: the action to approve
	Request      string             `json:"request"` // approvalAnswer
	Approve      bool               `json:"approve"` // approvalAnswer: false denies
	On           bool               `json:"on"`      // approvalCards
	asRole       string             // Request approval: the requester's Role when sent (approvalrequest.go)
	Inbox        *inboxReq          `json:"inbox"`   // the inbox messages (inboxsend.go)
	Swap         *swapReq           `json:"swap"`    // swapList, swapMine (swap.go)
	Mine         bool               `json:"mine"`    // brokenCount: only the user's own items
	Protect      *protectReq        `json:"protect"` // protectSet, protectClear
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// A range with no end runs to now; no start, a week back. router is the router UID; blank is the first router. A UID
// with no router gets a range with nothing recorded, and the reply echoes it.
func (a *App) uptimeReply(id int, router string, from, to, slot int64) uptimeMsg {
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
	var r UptimeRange
	u, ok := a.unit(router)
	if ok {
		r = u.up.Range(from, to, slot)
		router = u.uid
	}
	m := uptimeMsg{T: "uptimeResult", ID: id, Router: router, From: from, To: to, Slot: max(slot, 60_000), Now: now, Start: r.Start, Trans: r.Trans, Counts: r.Counts}
	if m.Trans == nil {
		m.Trans = []UptimeTrans{}
	}
	if m.Counts == nil {
		m.Counts = []UptimeSlot{}
	}
	return m
}

// KeepAliveListener probes an idle connection after 2 s, every 1 s, and drops it after 3 misses. So a
// peer that vanished without a FIN (power cut, lost Wi-Fi) ends its stream in about 5 s. The kernel
// default is 2 hours. A proxy in front only gets this on its own leg of the connection.
func KeepAliveListener(ln net.Listener) net.Listener { return keepAliveLn{ln} }

type keepAliveLn struct{ net.Listener }

func (l keepAliveLn) Accept() (net.Conn, error) {
	c, err := l.Listener.Accept()
	if tc, ok := c.(*net.TCPConn); ok && err == nil {
		_ = tc.SetKeepAliveConfig(net.KeepAliveConfig{Enable: true, Idle: 2 * time.Second, Interval: time.Second, Count: 3})
	}
	return c, err
}
