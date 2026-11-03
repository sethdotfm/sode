package app

import (
	"os"
	"strings"
	"testing"
	"time"
)

// An admin browser can lock itself: its token rests on that connection only.
func TestSuspendToken(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.5"}
	tok, err := a.AdminUnlock(c, "2468")
	if err != nil || !a.pageAllowed(c, gateMain) {
		t.Fatalf("admin unlock: %v", err)
	}
	other := &client{ip: "10.0.0.5"}
	a.Hi(other, "main", tok)
	a.UserLock(c, false, true)
	if a.pageAllowed(c, gateMain) || a.pageAllowed(c, gateLog) {
		t.Fatal("suspended connection still open")
	}
	a.mu.Lock()
	st := a.lockStateLocked(c)
	a.mu.Unlock()
	if !st.Locked || st.Exempt || !st.Token {
		t.Fatalf("state %+v", st)
	}
	if !a.pageAllowed(other, gateMain) {
		t.Fatal("suspending leaked to another connection")
	}
	if !a.tokens.ok(tok, a.r().cfg.SettingsPin) {
		t.Fatal("suspend revoked the token")
	}
	// A user PIN opens it, and locking again leaves it suspended.
	if err := a.UserUnlock(c, "1234"); err != nil || !a.pageAllowed(c, gateMain) {
		t.Fatalf("user unlock after suspend: %v", err)
	}
	a.UserLock(c, false, false)
	if a.pageAllowed(c, gateMain) {
		t.Fatal("token came back on a plain lock")
	}
	// The admin password restores the exemption.
	a.mu.Lock()
	a.pin.until = time.Time{}
	a.mu.Unlock()
	tok, err = a.AdminUnlock(c, "2468")
	if err != nil || !a.pageAllowed(c, gateMain) {
		t.Fatalf("admin unlock after suspend: %v", err)
	}
	// Forget revokes it for good, suspended or not.
	a.UserLock(c, true, true)
	if a.tokens.ok(tok, a.r().cfg.SettingsPin) {
		t.Fatal("forget kept the token")
	}
}

func TestUserSetPin(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserSetPin(c, "1234", "4321"); err == nil {
		t.Fatal("changed a PIN while locked")
	}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	other := &client{ip: "10.0.0.2"}
	if err := a.UserUnlock(other, "1234"); err != nil {
		t.Fatal(err)
	}
	for name, pins := range map[string][2]string{
		"short":     {"1234", "123"},
		"letters":   {"1234", "12a4"},
		"too long":  {"1234", "123456789"},
		"another's": {"1234", "567890"},
		"same":      {"1234", "1234"},
	} {
		if err := a.UserSetPin(c, pins[0], pins[1]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	// A wrong old PIN counts as a try and changes nothing.
	n := a.userPin.n
	if err := a.UserSetPin(c, "0000", "4321"); err == nil || a.userPin.n != n+1 {
		t.Fatalf("wrong old PIN: %v n=%d", err, a.userPin.n)
	}
	if err := a.UserSetPin(c, "1234", "43210"); err != nil {
		t.Fatalf("change: %v", err)
	}
	if !a.pageAllowed(c, gateMain) || c.who() != "Ana" {
		t.Fatal("the changing connection was locked")
	}
	if a.pageAllowed(other, gateMain) {
		t.Fatal("the user's other connection stayed unlocked")
	}
	fresh := &client{ip: "10.0.0.3"}
	if err := a.UserUnlock(fresh, "1234"); err == nil {
		t.Fatal("old PIN still works")
	}
	a.mu.Lock()
	a.userPin.until = time.Time{}
	a.mu.Unlock()
	if err := a.UserUnlock(fresh, "43210"); err != nil {
		t.Fatalf("new PIN: %v", err)
	}
	b, _ := os.ReadFile(a.cfgPath)
	if strings.Contains(string(b), "43210") {
		t.Fatal("PIN in the config file in plain")
	}
}

func TestMaxPinLen(t *testing.T) {
	a := userApp(t, true)
	if a.r().cfg.MaxPinLen != 8 {
		t.Fatalf("default %d", a.r().cfg.MaxPinLen)
	}
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	locks := a.r().cfg.Locks
	long := []userEdit{{Name: "Ana"}, {Name: "Bo", Pin: "123456789"}}
	if err := a.SaveUsers(admin, usersReq{Users: long, Locks: locks}); err == nil {
		t.Fatal("a 9-digit PIN under maxPinLen 8")
	}
	if err := a.SaveUsers(admin, usersReq{Users: long, Locks: locks, MaxPin: 9}); err != nil {
		t.Fatalf("raised: %v", err)
	}
	keep := []userEdit{{Name: "Ana"}, {Name: "Bo"}}
	if err := a.SaveUsers(admin, usersReq{Users: keep, Locks: locks, MaxPin: 6}); err == nil {
		t.Fatal("lowered below an existing PIN")
	}
	if err := a.SaveUsers(admin, usersReq{Users: keep, Locks: locks, MaxPin: 13}); err == nil {
		t.Fatal("13 accepted")
	}
}
