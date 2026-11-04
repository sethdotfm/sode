package app

import (
	"os"
	"strings"
	"testing"
	"time"
)

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
	long := []userEdit{{Name: "Ana"}, {Name: "Bo", Pin: "123456789"}}
	if err := a.SaveUsers(admin, usersReq{Users: long}); err == nil {
		t.Fatal("a 9-digit PIN under maxPinLen 8")
	}
	if err := a.SaveUsers(admin, usersReq{Users: long, MaxPin: 9}); err != nil {
		t.Fatalf("raised: %v", err)
	}
	keep := []userEdit{{Name: "Ana"}, {Name: "Bo"}}
	if err := a.SaveUsers(admin, usersReq{Users: keep, MaxPin: 6}); err == nil {
		t.Fatal("lowered below an existing PIN")
	}
	if err := a.SaveUsers(admin, usersReq{Users: keep, MaxPin: 13}); err == nil {
		t.Fatal("13 accepted")
	}
}
