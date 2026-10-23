package app

import (
	"context"
	"encoding/json"
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
}

// Handler serves the embedded UI, the WebSocket and a health check.
func (a *App) Handler(static fs.FS) http.Handler {
	mux := http.NewServeMux()
	files := http.FileServerFS(static)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
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
			T      string         `json:"t"`
			Before int            `json:"before"`
			Config *config.Config `json:"config"`
			Items  []takeReq      `json:"items"`
			takeReq
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		switch m.T {
		case "batch":
			go a.batch(c, m.Items)
		case "take": // pages loaded before batches existed
			go a.batch(c, []takeReq{m.takeReq})
		case "history":
			go a.history(c, m.Before)
		case "refreshNames":
			go a.RefreshNames()
		case "clearHistory":
			go a.ClearHistory(c.ip)
		case "config":
			if m.Config == nil {
				continue
			}
			go func(cfg config.Config) {
				res := map[string]any{"t": "configResult", "ok": true}
				if err := a.ApplyConfig(cfg); err != nil {
					res["ok"], res["err"] = false, err.Error()
				} else {
					log.Printf("config: saved by %s", c.ip)
				}
				a.reply(c, res)
			}(*m.Config)
		}
	}
}
