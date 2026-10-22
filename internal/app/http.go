package app

import (
	"context"
	"encoding/json"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
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

func clientIP(r *http.Request) string {
	if f := r.Header.Get("X-Forwarded-For"); f != "" {
		return strings.TrimSpace(strings.Split(f, ",")[0])
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

func (a *App) serveWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, nil)
	if err != nil {
		return
	}
	defer conn.CloseNow()
	conn.SetReadLimit(64 << 10)

	c := &client{ip: clientIP(r), send: make(chan []byte, 256)}
	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	// Register and queue the snapshot under one lock so no delta can
	// arrive ahead of it.
	a.mu.Lock()
	hello, _ := json.Marshal(a.hello())
	a.clients[c] = struct{}{}
	c.send <- hello
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
			T      string `json:"t"`
			Before int    `json:"before"`
			takeReq
		}
		if err := json.Unmarshal(data, &m); err != nil {
			continue
		}
		switch m.T {
		case "take":
			go a.take(c, m.takeReq)
		case "history":
			go a.history(c, m.Before)
		}
	}
}
