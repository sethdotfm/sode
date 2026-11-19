package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Guest role and Wait after wrong PINs save in Locks, New user defaults in Users.
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

	cfg := a.r().cfg
	rows := []any{}
	for _, u := range cfg.Users {
		rows = append(rows, map[string]any{"id": u.ID, "name": u.Name, "role": u.Role})
	}
	rows = append(rows,
		map[string]any{"name": "Cy", "pin": "1357"},
		map[string]any{"name": "Di", "pin": "8642", "persistent": false, "noAutoLock": false})
	passed(t, "add Cy and Di", p.askUsers(a, 3, map[string]any{"users": rows}))
	cfg = a.r().cfg
	if cy, _ := cfg.FindUserByName("Cy"); !cy.Persistent || !cy.NoAutoLock {
		t.Errorf("Cy, sent without either: Persistent %v, No auto-lock %v; want the New user defaults", cy.Persistent, cy.NoAutoLock)
	}
	if di, _ := cfg.FindUserByName("Di"); di.Persistent || di.NoAutoLock {
		t.Errorf("Di, sent with both off: Persistent %v, No auto-lock %v", di.Persistent, di.NoAutoLock)
	}
	// A saved user sent without them keeps theirs.
	setFlags(t, a, "Ana", false, true)
	rows = rows[:0]
	for _, u := range a.r().cfg.Users {
		rows = append(rows, map[string]any{"id": u.ID, "name": u.Name, "role": u.Role})
	}
	passed(t, "save again", p.askUsers(a, 4, map[string]any{"users": rows}))
	if ana, _ := a.r().cfg.FindUserByName("Ana"); !ana.Persistent {
		t.Error("a saved user sent without Persistent lost it")
	}
}

// Locks owns Guest role: Sode tells a stale Manage users dialog that sends it to reload.
func TestUsersSaveRefusesGuestRole(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signInRechecked("Ada", "2468")
	rows := []any{}
	for _, u := range a.r().cfg.Users {
		rows = append(rows, map[string]any{"id": u.ID, "name": u.Name, "role": u.Role})
	}
	r := p.askUsers(a, 1, map[string]any{"users": rows, "guestRole": config.RoleTrusted})
	if e, _ := r["err"].(string); r["ok"] == true || !strings.Contains(e, "Locks") {
		t.Fatalf("want a refusal that names Locks: %v", r)
	}
	if a.r().cfg.GuestRole != config.RoleStandard {
		t.Fatal("Guest role changed")
	}
}

// Turning a page lock on is never refused, even while users without a PIN exist.
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
