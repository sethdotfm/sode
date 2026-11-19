package app

import (
	"os"
	"strings"
	"testing"
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
		"short":    {"1234", "123"},
		"letters":  {"1234", "12a4"},
		"too long": {"1234", "123456789"},
		"same":     {"1234", "1234"},
	} {
		if err := a.UserSetPin(c, pins[0], pins[1]); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	n := userPinCount(a, "Ana")
	if err := a.UserSetPin(c, "0000", "4321"); err == nil || userPinCount(a, "Ana") != n+1 {
		t.Fatalf("wrong old PIN: %v n=%d", err, userPinCount(a, "Ana"))
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
	clear(a.userPins)
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
	if err := a.UserUnlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	long := ukey(a, "Bo", "pin")
	mustRefuse(t, a, admin, map[string]any{long: "123456789"})
	mustSave(t, a, admin, map[string]any{long: "123456789", "maxPinLen": 9})
	mustRefuse(t, a, admin, map[string]any{"maxPinLen": 6})
	mustRefuse(t, a, admin, map[string]any{"maxPinLen": 13})
}
