package app

import (
	"context"
	"encoding/json"
	"errors"
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
}

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
	return mux
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
	conn.SetReadLimit(64 << 10)

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

	defer func() {
		a.mu.Lock()
		if _, ok := a.clients[c]; ok {
			delete(a.clients, c)
			close(c.send)
		}
		a.mu.Unlock()
		log.Printf("ws %s: disconnected", c.ip)
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
			return
		}
		var m struct {
			T        string             `json:"t"`
			Before   int                `json:"before"`
			N        int                `json:"n"` // history page size; 0 = the default
			Config   *config.Config     `json:"config"`
			Pin      string             `json:"pin"`
			ClearPin bool               `json:"clearPin"`
			ID       int                `json:"id"` // echoed by unlock and filterSets replies
			Items    []takeReq          `json:"items"`
			Sets     []config.FilterSet `json:"sets"`
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		switch m.T {
		case "batch":
			go a.batch(c, m.Items)
		case "history":
			go a.history(c, m.Before, m.N)
		case "sent":
			go a.sentHistory(c, int64(m.Before), m.N)
		case "refreshNames":
			go a.RefreshNames()
		case "unlock":
			// Checked in line, so a lock sent after it is handled after it
			// and one connection checks one PIN at a time.
			res := opResult{T: "unlockResult", OK: true, ID: &m.ID}
			if err := a.Unlock(c, m.Pin); err != nil {
				res.OK, res.Err = false, err.Error()
			}
			a.reply(c, res)
		case "lock":
			a.Lock(c)
		case "clearHistory":
			if a.locked(c) {
				a.reply(c, simpleMsg{T: "locked"})
				continue
			}
			go a.ClearHistory(c.ip)
		case "filterSets":
			go func(sets []config.FilterSet, id int) {
				res := opResult{T: "filterSetsResult", OK: true, ID: &id}
				if err := a.SaveFilterSets(c, sets); err != nil {
					res.OK, res.Err = false, err.Error()
					res.Locked = locked(err)
				} else {
					log.Printf("config: filter sets saved by %s", c.ip)
				}
				a.reply(c, res)
			}(m.Sets, m.ID)
		case "config":
			if m.Config == nil {
				continue
			}
			// A page from before deviceDefaults existed sends none, and its
			// zero values would turn the "on" defaults off for everyone, so
			// the current ones are kept.
			var has struct {
				Config struct {
					DeviceDefaults json.RawMessage `json:"deviceDefaults"`
				} `json:"config"`
			}
			json.Unmarshal(data, &has)
			keepDefaults := has.Config.DeviceDefaults == nil
			go func(cfg config.Config, pin string, clearPin bool) {
				res := opResult{T: "configResult", OK: true}
				if err := a.saveSettings(c, cfg, pin, clearPin, keepDefaults); err != nil {
					res.OK, res.Err = false, err.Error()
					res.Locked = locked(err)
				} else {
					log.Printf("config: saved by %s", c.ip)
				}
				a.reply(c, res)
			}(*m.Config, m.Pin, m.ClearPin)
		}
	}
}

// locked is opResult.Locked for a failed save: whether the PIN was why.
func locked(err error) *bool {
	l := errors.Is(err, errLocked)
	return &l
}
