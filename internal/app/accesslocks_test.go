package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestLocksAndNewUserDefaultsSaveAndAct(t *testing.T) {
	a := namedApp(t)
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	guest := dialPeer(t, a)
	refusedForRole(t, "Guest presets before", guest.ask("presets", 1, map[string]any{"presets": []any{}}), config.RoleTrusted)

	r := p.saveFields(1, map[string]any{
		"guestRole":       config.RoleTrusted,
		"pinWaits":        false,
		"newUserDefaults": map[string]any{"persistent": true, "noAutoLock": true},
	})
	for _, k := range []string{"guestRole", "pinWaits", "newUserDefaults"} {
		wantSaved(t, r, k)
	}

	if r := guest.ask("presets", 2, map[string]any{"presets": []any{}}); r["ok"] != true {
		t.Fatalf("Guest with Guest role Trusted user: %v", r)
	}
	if err := wrongAs(t, a, "Ana", 3*pinFreeTries); waits(err) {
		t.Fatalf("Ana's name waits with Wait after wrong PINs off: %v", err)
	}

	allSaved(t, "add Cy and Di", p.saveLive(a, 3, map[string]any{
		"user/new:0/add": map[string]any{"name": "Cy", "pin": "1357"},
		"user/new:1/add": map[string]any{"name": "Di", "pin": "8642", "persistent": false, "noAutoLock": false}}))
	cfg := a.r().cfg
	if cy, _ := cfg.FindUserByName("Cy"); !cy.Persistent || !cy.NoAutoLock {
		t.Errorf("Cy, sent without either: Persistent %v, No auto-lock %v; want the New user defaults", cy.Persistent, cy.NoAutoLock)
	}
	if di, _ := cfg.FindUserByName("Di"); di.Persistent || di.NoAutoLock {
		t.Errorf("Di, sent with both off: Persistent %v, No auto-lock %v", di.Persistent, di.NoAutoLock)
	}
	setFlags(t, a, "Ana", false, true)
	allSaved(t, "save again", p.saveLive(a, 4, map[string]any{ukey(a, "Ana", "name"): "Ana", ukey(a, "Ana", "role"): config.RoleStandard}))
	if ana, _ := a.r().cfg.FindUserByName("Ana"); !ana.Persistent {
		t.Error("a saved user sent without Persistent lost it")
	}
}

func TestPageLockOnWithUsersWithoutPinSaves(t *testing.T) {
	a := newTestApp(t, withAdministrator, func(c *config.Config) {
		c.Users = append(c.Users, config.User{Name: "Cy", Role: config.RoleStandard})
	})
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	wantSaved(t, p.saveFields(1, map[string]any{"locks": map[string]any{"main": true, "log": true, "panel": true}}), "locks")
	if l := a.r().cfg.Locks; !l.Main || !l.Log || !l.Panel {
		t.Fatalf("locks %+v", l)
	}
}
