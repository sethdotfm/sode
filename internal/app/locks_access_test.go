package app

import (
	"errors"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A Settings save whose message has no locks / autoLockMinutes key must keep
// the live values, not decode to "every lock off". Such a message comes from a
// page from before they were form fields.
func TestSaveWithoutLocksKeepsThem(t *testing.T) {
	a := userApp(t, true)
	admin := &client{ip: "10.0.0.9"}
	if err := a.UserUnlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	w := a.r().cfg
	w.AutoLockMinutes = 7
	if err := a.SaveSettings(admin, w); err != nil {
		t.Fatal(err)
	}
	setFlags(t, a, "Ana", true, false)
	ana := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(ana, "1234"); err != nil {
		t.Fatal(err)
	}
	for _, who := range []string{"admin", "trusted"} {
		c := map[string]*client{"admin": admin, "trusted": ana}[who]
		if err := pageSave(t, a, c, map[string]any{"levelColors": []string{"#abcdef"}}, "locks", "autoLockMinutes"); err != nil {
			t.Fatalf("%s: %v", who, err)
		}
		if got := a.r().cfg; !got.Locks.Main || !got.Locks.Log || got.AutoLockMinutes != 7 {
			t.Fatalf("%s: a save without locks turned them off: %+v", who, got.Locks)
		}
	}
}

func TestLocksNeedAnAdministrator(t *testing.T) {
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
	} {
		cfg := a.r().cfg
		mut(&cfg)
		err := a.SaveSettings(ana, cfg)
		var g guardedErr
		if !errors.As(err, &g) || !isRoleErr(err) {
			t.Errorf("%s: trusted user got %v, want guardedErr", name, err)
		}
	}
	if got := a.r().cfg; !got.Locks.Main || !got.Locks.Log || got.AutoLockMinutes != 0 {
		t.Fatalf("a refused save changed the locks: %+v", got)
	}
	if err := a.SaveSettings(ana, a.r().cfg); err != nil {
		t.Fatalf("unchanged locks: %v", err)
	}
	admin := &client{ip: "10.0.0.9"}
	if err := a.UserUnlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.Locks.Log, cfg.AutoLockMinutes = false, 3
	if err := a.SaveSettings(admin, cfg); err != nil {
		t.Fatalf("the PIN holder: %v", err)
	}
	if got := a.r().cfg; got.Locks.Log || !got.Locks.Main || got.AutoLockMinutes != 3 {
		t.Fatalf("not saved: %+v", got)
	}
}

func TestLockNeedsAUser(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	cfg := a.r().cfg
	cfg.Locks.Main = true
	if err := a.SaveSettings(c, cfg); err == nil || !strings.Contains(err.Error(), "user") {
		t.Fatalf("lock without users: %v", err)
	}
}

// The locks are Settings > Access fields, not part of the users save, so the refusal says what to do.
func TestRemoveLastUserWhileLockedSaysWhy(t *testing.T) {
	a := adaOnly(t, true)
	if r := mustRefuse(t, a, adminClient(t, a), map[string]any{ukey(a, "Ada", "remove"): true}); !strings.Contains(r.Err, "Settings > Access") {
		t.Fatalf("want a message pointing at Settings > Access, got %+v", r)
	}
	if len(a.r().cfg.Users) != 1 {
		t.Fatal("last user removed while a lock is on")
	}
}

// With no lock on, removing the last user is still refused, since it leaves no Administrator.
func TestRemoveLastUserGuardScope(t *testing.T) {
	a := adaOnly(t, false)
	if r := mustRefuse(t, a, adminClient(t, a), map[string]any{ukey(a, "Ada", "remove"): true}); r.Err != errNoAdministrator.Error() {
		t.Fatalf("removing every user with no lock on: %+v", r)
	}
}

func adaOnly(t *testing.T, locks bool) *App {
	return newTestApp(t, withAdministrator, func(c *config.Config) { c.Locks.Main = locks })
}
