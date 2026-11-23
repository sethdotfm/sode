package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
)

func clearWaits(a *App) {
	a.mu.Lock()
	clear(a.userPins)
	a.mu.Unlock()
}

// Wrong user PINs wait longer and longer, to five minutes, and never lock
// anyone out for good.
func TestUserPinBackoffCapped(t *testing.T) {
	if got := userPinWait(pinFirstWait, pinFreeTries); got != 0 {
		t.Fatalf("free try waits %v", got)
	}
	if got := userPinWait(pinFirstWait, pinFreeTries+1); got != pinFirstWait {
		t.Fatalf("first wait %v", got)
	}
	if got := userPinWait(pinFirstWait, 1000); got != userPinMaxWait || userPinMaxWait != 5*time.Minute {
		t.Fatalf("capped wait %v", got)
	}
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	ana := idOf(a, "Ana")
	for range 30 {
		a.mu.Lock()
		if p := a.userPins[ana]; p != nil {
			p.until = time.Time{}
		}
		a.mu.Unlock()
		if err := a.UserUnlockAs(c, "Ana", "0000"); err == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	a.mu.Lock()
	wait := time.Until(a.userPins[ana].until)
	a.mu.Unlock()
	if wait <= 0 || wait > userPinMaxWait {
		t.Fatalf("wait %v", wait)
	}
	clearWaits(a)
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatalf("locked out for good: %v", err)
	}
}

func TestLockGatesMoreOverWebSocket(t *testing.T) {
	a := userApp(t, true)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 24)
	for _, m := range []map[string]any{
		{"t": "uptime", "id": 1},
		{"t": "rereadNames"},
	} {
		b, _ := json.Marshal(m)
		ws.Write(ctx, websocket.MessageText, b)
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				t.Fatalf("%v: no refusal: %v", m["t"], err)
			}
			var r map[string]any
			json.Unmarshal(b, &r)
			if r["t"] == "uptimeResult" {
				t.Fatal("uptime answered on a locked connection")
			}
			if r["t"] == "lockState" && r["locked"] == true {
				break
			}
		}
	}
}

func TestUserPinForgetsAfterQuiet(t *testing.T) {
	a := userApp(t, true)
	wrongAs(t, a, "Ana", pinFreeTries+1)
	a.mu.Lock()
	p := a.userPins[idOf(a, "Ana")]
	p.until = time.Time{}
	p.last = time.Now().Add(-pinMaxWait - time.Minute)
	a.mu.Unlock()
	if err := wrongAs(t, a, "Ana", 1); waits(err) || userPinCount(a, "Ana") != 1 {
		t.Fatalf("still counting after a quiet hour: %v", err)
	}
}
