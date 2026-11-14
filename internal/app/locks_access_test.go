package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A Settings save whose message has no locks / autoLockMinutes key (a page
// from before they were form fields) must keep the live values, not decode to
// "every lock off".
func TestSaveWithoutLocksKeepsThem(t *testing.T) {
	a := userApp(t, true)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	w := a.r().cfg
	w.AutoLockMinutes = 7
	if err := a.SaveSettings(admin, w, "", false); err != nil {
		t.Fatal(err)
	}
	setFlags(t, a, "Ana", true, false)
	ana := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(ana, "1234"); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"admin", "trusted"} {
		c := map[string]*client{"admin": admin, "trusted": ana}[who]
		if who == "trusted" {
			a.Lock(admin) // only the trusted flag from here on
		}
		if err := pageSave(t, a, c, map[string]any{"levelColors": []string{"#abcdef"}}, "locks", "autoLockMinutes"); err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		if got := a.r().cfg; !got.Locks.Main || !got.Locks.Log || got.AutoLockMinutes != 7 {
			t.Fatalf("%s: a save without locks turned them off: %+v", who, got.Locks)
		}
	}
}

// Changing the locks or the idle time needs the Advanced Settings PIN; a trusted user
// is refused with guardedErr (which counts as errLocked), the PIN holder saves.
func TestLocksNeedSettingsPin(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, false)
	ana := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(ana, "1234"); err != nil {
		t.Fatal(err)
	}
	for name, mut := range map[string]func(*config.Config){
		"main off":  func(c *config.Config) { c.Locks.Main = false },
		"log off":   func(c *config.Config) { c.Locks.Log = false },
		"auto-lock": func(c *config.Config) { c.AutoLockMinutes = 3 },
		"kill on":   func(c *config.Config) { c.KillEnabled = true },
	} {
		cfg := a.r().cfg
		mut(&cfg)
		err := a.SaveSettings(ana, cfg, "", false)
		var g guardedErr
		if !errors.As(err, &g) || !errors.Is(err, errLocked) {
			t.Errorf("%s: trusted user got %v, want guardedErr", name, err)
		}
	}
	if got := a.r().cfg; !got.Locks.Main || !got.Locks.Log || got.AutoLockMinutes != 0 || got.KillEnabled {
		t.Fatalf("a refused save changed the locks or Kill: %+v", got)
	}
	if err := a.SaveSettings(ana, a.r().cfg, "", false); err != nil {
		t.Fatalf("unchanged locks: %v", err)
	}
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.Locks.Log, cfg.AutoLockMinutes = false, 3
	if err := a.SaveSettings(admin, cfg, "", false); err != nil {
		t.Fatalf("the PIN holder: %v", err)
	}
	if got := a.r().cfg; got.Locks.Log || !got.Locks.Main || got.AutoLockMinutes != 3 {
		t.Fatalf("not saved: %+v", got)
	}
}

func TestLockNeedsUserAndPin(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.Locks.Main = true
	if err := a.SaveSettings(c, cfg, "", false); err == nil || !strings.Contains(err.Error(), "user") {
		t.Fatalf("lock without users: %v", err)
	}
	a2 := userApp(t, false)
	cfg = a2.r().cfg
	cfg.Locks.Main = true
	if err := a2.SaveSettings(&client{ip: "10.0.0.9"}, cfg, "", true); err == nil || !strings.Contains(err.Error(), "PIN") {
		t.Fatalf("lock without an Advanced Settings PIN: %v", err)
	}
}

// Removing the last user while a lock is on says what to do (the locks are
// Settings > Access fields, not part of the users save).
func TestRemoveLastUserWhileLockedSaysWhy(t *testing.T) {
	a := userApp(t, true)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	err := a.SaveUsers(admin, usersReq{Users: nil})
	if err == nil || !strings.Contains(err.Error(), "Settings > Access") {
		t.Fatalf("want a message pointing at Settings > Access, got %v", err)
	}
}

// The guard leaves the users alone when it refuses, and does not fire when no
// lock is on (the last user may go then).
func TestRemoveLastUserGuardScope(t *testing.T) {
	a := userApp(t, true)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	before := len(a.r().cfg.Users)
	if err := a.SaveUsers(admin, usersReq{Users: nil}); err == nil {
		t.Fatal("last user removed while a lock is on")
	}
	if got := len(a.r().cfg.Users); got != before || before == 0 {
		t.Fatalf("users changed by a refused save: %d -> %d", before, got)
	}
	b := userApp(t, false)
	adm := &client{ip: "10.0.0.9"}
	if err := b.Unlock(adm, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := b.SaveUsers(adm, usersReq{Users: nil}); err != nil {
		t.Fatalf("removing every user with no lock on: %v", err)
	}
	if len(b.r().cfg.Users) != 0 {
		t.Fatal("users not removed")
	}
}

// A users message from a page loaded before the locks moved is refused out
// loud, not saved with its lock edits quietly dropped.
func TestSaveUsersRefusesOldLockKeys(t *testing.T) {
	a := userApp(t, false)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	var req usersReq
	if err := json.Unmarshal([]byte(`{"users":[{"name":"Ana","was":"Ana"}],"locks":{"main":true,"log":false},"autoLockMinutes":3}`), &req); err != nil {
		t.Fatal(err)
	}
	err := a.SaveUsers(admin, req)
	if err == nil || !strings.Contains(err.Error(), "Settings > Access") {
		t.Fatalf("want a refusal pointing at Settings > Access, got %v", err)
	}
}
