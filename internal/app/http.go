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
	"strings"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
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
	return mux
}

// clientIP is the peer address, or, when the peer is a trusted proxy, the
// nearest untrusted hop in X-Forwarded-For. Untrusted peers can't spoof it.
func clientIP(r *http.Request, trusted []netip.Prefix) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	isTrusted := func(s string) bool {
		a, err := netip.ParseAddr(strings.TrimSpace(s))
		if err != nil {
			return false
		}
		for _, p := range trusted {
			if p.Contains(a.Unmap()) {
				return true
			}
		}
		return false
	}
	if !isTrusted(host) {
		return host
	}
	hops := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(hops) - 1; i >= 0; i-- {
		if h := strings.TrimSpace(hops[i]); h != "" && !isTrusted(h) {
			return h
		}
	}
	return host
}

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
		for {
			select {
			case <-ctx.Done():
				return
			case b, ok := <-c.send:
				if !ok {
					conn.Close(websocket.StatusPolicyViolation, "too slow")
					return
				}
				wctx, wcancel := context.WithTimeout(ctx, 5*time.Second)
				err := conn.Write(wctx, websocket.MessageText, b)
				wcancel()
				if err != nil {
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
