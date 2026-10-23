package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestSettingsPin(t *testing.T) {
	cfg := config.Defaults()
	cfg.Normalize()
	hist, _ := OpenHistory("")
	a, err := New(cfg, "", hist, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.9"}
	if a.locked(c) || a.Unlock(c.ip, "") != nil {
		t.Fatal("no PIN set: should be open")
	}
	if err := a.SaveSettings(a.r().cfg, "12", false); err == nil {
		t.Fatal("short PIN accepted")
	}
	if err := a.SaveSettings(a.r().cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	if !a.locked(c) {
		t.Fatal("PIN set: should be locked")
	}
	// A save from the browser (no hash in its JSON) keeps the PIN.
	if err := a.SaveSettings(a.r().cfg, "", false); err != nil || a.r().cfg.SettingsPin == "" {
		t.Fatalf("PIN lost on save: %v", err)
	}
	if a.Unlock(c.ip, "2468") != nil {
		t.Fatal("right PIN refused")
	}
	for range pinMaxFails {
		if a.Unlock(c.ip, "0000") == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	if a.Unlock(c.ip, "2468") == nil {
		t.Fatal("right PIN accepted during lockout")
	}
	if a.Unlock("10.0.0.10", "2468") != nil {
		t.Fatal("lockout should be per IP")
	}
	if err := a.SaveSettings(a.r().cfg, "", true); err != nil || a.locked(c) {
		t.Fatalf("clearing the PIN: %v", err)
	}
}
