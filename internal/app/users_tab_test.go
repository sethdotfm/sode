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
	a.SignOut(fresh)
	if tokenAlive(a, tok) {
		t.Fatal("sign out did not revoke")
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

// With a lock on, a tab token is refused (the Auto-lock must stay in force).
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

// A lock switched on after a sign-in with no lock: the tab token does not bring a
// reload back in. So the PIN pad and the Auto-lock apply.
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

// A tab token that no open page holds must not become one that skips a lock, so prune deletes it.
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

// The page type is the browser's own word. Here a connection says it is the Communication Log page,
// and its lock is off. Its tab token must not bring a reload back in past the main page lock.
func TestTabSessionRefusedOnLogPageUnderMainLock(t *testing.T) {
	a := userApp(t, false)
	cfg := a.r().cfg.Clone()
	cfg.Locks.Main, cfg.Locks.Log = true, false
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.1"}
	a.Hi(c, "log", "")
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if tok := a.issueSessionTab(c, true); tok != "" {
		fresh := &client{ip: "10.0.0.2"}
		a.Hi(fresh, "log", tok)
		if fresh.who() == "Ana" {
			t.Fatal("a tab token from the Log page signed in a reload under the main page lock")
		}
	}
}
