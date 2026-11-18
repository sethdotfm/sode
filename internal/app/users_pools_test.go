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
	a.pin.until = time.Time{}
	clear(a.userPins)
	a.mu.Unlock()
}

// A user's success never resets the Advanced Settings PIN's count, and neither
// pool's failures reach the other.
func TestPinClassesSeparate(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range 3 {
		a.mu.Lock()
		a.pin.until = time.Time{}
		a.mu.Unlock()
		if a.Unlock(c, "0000") == nil {
			t.Fatal("wrong Advanced Settings PIN accepted")
		}
	}
	if a.pin.n != 3 || userPinCount(a, "Ana") != 0 {
		t.Fatalf("counts settings %d, Ana %d", a.pin.n, userPinCount(a, "Ana"))
	}
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatal(err)
	}
	if a.pin.n != 3 {
		t.Fatalf("a user's success reset the settings count to %d", a.pin.n)
	}
	wrongAs(t, a, "Ana", 1)
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if userPinCount(a, "Ana") != 1 || a.pin.n != 0 {
		t.Fatalf("after a settings success: settings %d, Ana %d", a.pin.n, userPinCount(a, "Ana"))
	}
}

// Wrong user PINs wait longer and longer, to five minutes, and never lock
// anyone out for good or touch the Advanced Settings PIN.
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
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatalf("Advanced Settings PIN blocked by user failures: %v", err)
	}
	clearWaits(a)
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatalf("locked out for good: %v", err)
	}
}

// uptime and refreshNames are refused over a locked connection, too.
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
		{"t": "refreshNames"},
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

func TestSettingsPinSuccessResetsCount(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range pinFreeTries {
		a.Unlock(c, "0000")
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	n := a.pin.n
	a.mu.Unlock()
	if n != 0 {
		t.Fatalf("count after a right Advanced Settings PIN: %d", n)
	}
}
