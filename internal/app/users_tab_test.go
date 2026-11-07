package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestTabSession(t *testing.T) {
	a := userApp(t, false)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if tok := a.issueSessionTab(c, false); tok != "" {
		t.Fatal("token without tab")
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" || !tokenAlive(a, tok) {
		t.Fatal("no tab token")
	}
	fresh := &client{ip: "10.0.0.2"}
	a.Hi(fresh, "main", tok)
	if fresh.who() != "Ana" {
		t.Fatal("tab token not honored")
	}
	a.UserLock(fresh)
	if tokenAlive(a, tok) {
		t.Fatal("lock did not revoke")
	}
	setFlags(t, a, "Ana", false, true)
	d := &client{ip: "10.0.0.3"}
	if err := a.UserUnlock(d, "1234"); err != nil {
		t.Fatal(err)
	}
	if a.issueSessionTab(d, true) == "" {
		t.Fatal("persistent user got no token")
	}
	for _, r := range a.tokens.toks {
		if r.Tab {
			t.Fatal("persistent token marked tab")
		}
	}
}

// With a lock on, a tab token is refused (the idle lock must stay in force).
func TestTabSessionRefusedWhenLocked(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if a.issueSessionTab(c, true) != "" {
		t.Fatal("tab token issued for a locked page")
	}
}

// A lock switched on after a no-lock login: the tab token no longer brings a
// reload back in, so the PIN pad (and the idle lock) apply.
func TestTabTokenIgnoredOnceLocked(t *testing.T) {
	a := userApp(t, false)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" {
		t.Fatal("no tab token")
	}
	a.cfgMu.Lock()
	_, nr, err := a.update(func(cfg *config.Config) error {
		cfg.Locks.Main = true
		return nil
	})
	if err != nil {
		a.cfgMu.Unlock()
		t.Fatal(err)
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.mu.Unlock()
	a.cfgMu.Unlock()
	fresh := &client{ip: "10.0.0.2"}
	a.Hi(fresh, "main", tok)
	if fresh.u.user.Load() != nil {
		t.Fatal("tab token honored for a locked page")
	}
}

// A tab token is not a persistent one: ticking Never log out afterwards must
// not turn it into a token that skips a lock; it is pruned.
func TestTabTokenDiesWhenUserBecomesPersistent(t *testing.T) {
	a := userApp(t, false)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" || !tokenAlive(a, tok) {
		t.Fatal("no tab token")
	}
	setFlags(t, a, "Ana", false, true)
	if tokenAlive(a, tok) {
		t.Fatal("tab token survived the user becoming persistent")
	}
}
