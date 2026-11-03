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
	a.usr.pin.until = time.Time{}
	a.mu.Unlock()
}

// A user's own success never resets the settings PIN's count, and neither
// class's failures reach the other.
func TestPinClassesSeparate(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range 3 {
		clearWaits(a)
		if a.Unlock(c, "0000") == nil {
			t.Fatal("wrong settings PIN accepted")
		}
	}
	for range 3 {
		clearWaits(a)
		if _, err := a.AdminUnlock(c, "0000"); err == nil {
			t.Fatal("wrong admin password accepted")
		}
	}
	// The admin password is the settings PIN's class: one count.
	if a.pin.n != 6 || a.usr.pin.n != 0 {
		t.Fatalf("counts settings %d, users %d", a.pin.n, a.usr.pin.n)
	}
	clearWaits(a)
	if _, err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if a.pin.n != 6 {
		t.Fatalf("a user's success reset the settings count to %d", a.pin.n)
	}
	// And a settings success doesn't reset the user count.
	if _, err := a.UserUnlock(c, "0000"); err == nil {
		t.Fatal("wrong user PIN accepted")
	}
	clearWaits(a)
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if a.usr.pin.n != 1 || a.pin.n != 0 {
		t.Fatalf("after a settings success: settings %d, users %d", a.pin.n, a.usr.pin.n)
	}
}

// Wrong user PINs wait longer and longer, to five minutes, and never lock
// anyone out for good or touch the settings PIN.
func TestUserPinBackoffCapped(t *testing.T) {
	if got := userPinWait(pinFreeTries); got != 0 {
		t.Fatalf("free try waits %v", got)
	}
	if got := userPinWait(pinFreeTries + 1); got != pinFirstWait {
		t.Fatalf("first wait %v", got)
	}
	if got := userPinWait(1000); got != userPinMaxWait || userPinMaxWait != 5*time.Minute {
		t.Fatalf("capped wait %v", got)
	}
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range 30 { // far past pinMaxTries (10)
		clearWaits(a)
		if _, err := a.UserUnlock(c, "0000"); err == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	a.mu.Lock()
	wait := time.Until(a.usr.pin.until)
	a.mu.Unlock()
	if wait <= 0 || wait > userPinMaxWait {
		t.Fatalf("wait %v", wait)
	}
	// Mid-wait the settings and admin class still work.
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatalf("settings PIN blocked by user failures: %v", err)
	}
	if _, err := a.AdminUnlock(c, "2468"); err != nil {
		t.Fatalf("admin password blocked by user failures: %v", err)
	}
	// And the right user PIN works once the wait is over.
	clearWaits(a)
	if _, err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatalf("locked out for good: %v", err)
	}
}

// The settings class still locks for good at pinMaxTries, without taking
// users down with it.
func TestSettingsLockoutLeavesUsers(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range a.r().cfg.PinMaxTries {
		clearWaits(a)
		a.Unlock(c, "0000")
	}
	clearWaits(a)
	if err := a.Unlock(c, "2468"); err == nil || !strings.Contains(err.Error(), "-reset-pin") {
		t.Fatalf("settings not locked out: %v", err)
	}
	if _, err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatalf("users locked out by settings failures: %v", err)
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
