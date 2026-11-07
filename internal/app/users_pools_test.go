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
	a.userPin.until = time.Time{}
	a.mu.Unlock()
}

// A user's own success never resets the Advanced Settings PIN's count, and neither
// class's failures reach the other.
func TestPinClassesSeparate(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range 3 {
		clearWaits(a)
		if a.Unlock(c, "0000") == nil {
			t.Fatal("wrong Advanced Settings PIN accepted")
		}
	}
	if a.pin.n != 3 || a.userPin.n != 0 {
		t.Fatalf("counts settings %d, users %d", a.pin.n, a.userPin.n)
	}
	clearWaits(a)
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if a.pin.n != 3 {
		t.Fatalf("a user's success reset the settings count to %d", a.pin.n)
	}
	if err := a.UserUnlock(c, "0000"); err == nil {
		t.Fatal("wrong user PIN accepted")
	}
	clearWaits(a)
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if a.userPin.n != 1 || a.pin.n != 0 {
		t.Fatalf("after a settings success: settings %d, users %d", a.pin.n, a.userPin.n)
	}
}

// Wrong user PINs wait longer and longer, to five minutes, and never lock
// anyone out for good or touch the Advanced Settings PIN.
func TestUserPinBackoffCapped(t *testing.T) {
	waitFor := userPool().wait
	if got := waitFor(pinFreeTries); got != 0 {
		t.Fatalf("free try waits %v", got)
	}
	if got := waitFor(pinFreeTries + 1); got != pinFirstWait {
		t.Fatalf("first wait %v", got)
	}
	if got := waitFor(1000); got != userPinMaxWait || userPinMaxWait != 5*time.Minute {
		t.Fatalf("capped wait %v", got)
	}
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range 30 { // far past pinMaxTries (10)
		clearWaits(a)
		if err := a.UserUnlock(c, "0000"); err == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	a.mu.Lock()
	wait := time.Until(a.userPin.until)
	a.mu.Unlock()
	if wait <= 0 || wait > userPinMaxWait {
		t.Fatalf("wait %v", wait)
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatalf("Advanced Settings PIN blocked by user failures: %v", err)
	}
	clearWaits(a)
	if err := a.UserUnlock(c, "1234"); err != nil {
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
	if err := a.UserUnlock(c, "1234"); err != nil {
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

// A valid user PIN must not hand out fresh guesses at everyone else's: the
// count survives it, so the waits still arrive.
func TestUserPinSuccessDoesNotResetCount(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range pinFreeTries { // all the free tries
		if err := a.UserUnlock(c, "0000"); err == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	if err := a.UserUnlock(c, "1234"); err != nil { // a valid PIN, not blocked yet
		t.Fatalf("valid PIN: %v", err)
	}
	err := a.UserUnlock(c, "0000")
	if err == nil || !strings.Contains(err.Error(), "try again in") {
		t.Fatalf("the fifth wrong PIN should start the wait, got: %v", err)
	}
}

func TestUserPinForgetsAfterQuiet(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range pinFreeTries + 1 {
		a.UserUnlock(c, "0000")
	}
	a.mu.Lock()
	a.userPin.until = time.Time{}
	a.userPin.last = time.Now().Add(-pinMaxWait - time.Minute)
	a.mu.Unlock()
	if err := a.UserUnlock(c, "1234"); err != nil {
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
