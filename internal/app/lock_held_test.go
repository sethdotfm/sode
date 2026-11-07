package app

import (
	"errors"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Lock of a signed-in user is state of their session: it survives a reload, keeps
// the session, and only that user's PIN resumes it (docs/adr/0001-server-held-lock.md).

func canTake(a *App, c *client) error { return admitMsg(a, c, inMsg{T: "batch"}) }

func lastLockState(t *testing.T, read func() []map[string]any) map[string]any {
	t.Helper()
	got := read()
	if len(got) == 0 {
		t.Fatal("no lockState sent")
	}
	return got[len(got)-1]
}

func TestLockKeepsSessionAndRefusesActions(t *testing.T) {
	for _, enforced := range []bool{false, true} {
		a := userApp(t, enforced)
		setFlags(t, a, "Ana", true, true)
		c := &client{ip: "10.0.0.1"}
		read := register(a, c)
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		tok := a.issueSession(c)
		if err := canTake(a, c); err != nil {
			t.Fatalf("enforced=%v: before the Lock: %v", enforced, err)
		}
		read()
		a.UserLock(c)
		if !tokenAlive(a, tok) {
			t.Fatalf("enforced=%v: the Lock ended the session", enforced)
		}
		for _, m := range []inMsg{{T: "batch"}, {T: "refreshNames"}, {T: "sent"}, {T: "uptime"}} {
			if err := admitMsg(a, c, m); !errors.Is(err, errPageLocked) {
				t.Errorf("enforced=%v: %s from a locked session: %v", enforced, m.T, err)
			}
		}
		if err := admitMsg(a, c, inMsg{T: "config"}); err == nil {
			t.Errorf("enforced=%v: a locked session saved settings as a trusted user", enforced)
		}
		st := lastLockState(t, read)
		if st["locked"] != true || st["held"] != true || st["user"] != nil || st["names"] != nil || st["trusted"] != nil {
			t.Errorf("enforced=%v: lockState %v", enforced, st)
		}
	}
}

func TestLockedSessionSurvivesReloadAndRestart(t *testing.T) {
	a, c, tok := persistentApp(t)
	a.UserLock(c)
	fresh := &client{ip: "10.0.0.2"}
	read := register(a, fresh)
	a.Hi(fresh, "main", tok)
	if err := canTake(a, fresh); !errors.Is(err, errPageLocked) {
		t.Fatalf("reload of a locked session: %v", err)
	}
	if st := lastLockState(t, read); st["locked"] != true || st["held"] != true || st["hi"] != true || st["user"] != nil {
		t.Fatalf("lockState after reload: %v", st)
	}
	if !loadTokens(a.tokens.path).locked(tok) {
		t.Fatal("the locked flag did not reach tokens.json")
	}
}

func TestLockLocksOtherTabs(t *testing.T) {
	a, c, tok := persistentApp(t)
	twin := &client{ip: "10.0.0.2"}
	register(a, twin)
	a.Hi(twin, "main", tok)
	a.UserLock(c)
	if err := canTake(a, twin); !errors.Is(err, errPageLocked) {
		t.Fatalf("another tab of the session: %v", err)
	}
}

func TestOwnPinResumesLockedSession(t *testing.T) {
	a, c, tok := persistentApp(t)
	twin := &client{ip: "10.0.0.2"}
	register(a, twin)
	a.Hi(twin, "main", tok)
	a.UserLock(c)
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	for _, x := range []*client{c, twin} {
		if err := canTake(a, x); err != nil {
			t.Fatalf("after the resume: %v", err)
		}
	}
	if loadTokens(a.tokens.path).locked(tok) {
		t.Fatal("tokens.json still says locked")
	}
	fresh := &client{ip: "10.0.0.3"}
	a.Hi(fresh, "main", tok)
	if err := canTake(a, fresh); err != nil {
		t.Fatalf("reload after the resume: %v", err)
	}
}

func TestOtherUsersPinDoesNotResume(t *testing.T) {
	a, c, _ := persistentApp(t)
	a.UserLock(c)
	if err := a.UserUnlock(c, "567890"); err == nil {
		t.Fatal("Bo's PIN resumed Ana's Lock")
	}
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("still locked expected: %v", err)
	}
	if c.who() == "Bo" {
		t.Fatal("the session switched user")
	}
	// A wrong try: after the free tries even Ana's own PIN waits.
	for range pinFreeTries {
		a.UserUnlock(c, "567890")
	}
	if err := a.UserUnlock(c, "1234"); err == nil {
		t.Fatal("another user's PIN was not counted as a wrong try")
	}
}

func TestSignOutFromLockEndsSession(t *testing.T) {
	a, c, tok := persistentApp(t)
	twin := &client{ip: "10.0.0.2"}
	register(a, twin)
	a.Hi(twin, "main", tok)
	a.UserLock(c)
	a.SignOut(c)
	if tokenAlive(a, tok) {
		t.Fatal("Sign out left the token alive")
	}
	if c.who() != "10.0.0.1" || twin.who() != "10.0.0.2" {
		t.Fatalf("still signed in: %s, %s", c.who(), twin.who())
	}
	fresh := &client{ip: "10.0.0.3"}
	read := register(a, fresh)
	a.Hi(fresh, "main", tok)
	if st := lastLockState(t, read); st["held"] != nil || st["user"] != nil {
		t.Fatalf("a reload after Sign out: %v", st)
	}
}

func TestLockWithNobodySignedInStaysPageOnly(t *testing.T) {
	a := userApp(t, false)
	c := &client{ip: "10.0.0.1"}
	read := register(a, c)
	a.UserLock(c)
	if err := canTake(a, c); err != nil {
		t.Fatalf("a page Lock reached the server: %v", err)
	}
	a.syncAccess(c)
	if st := lastLockState(t, read); st["locked"] != false || st["held"] != nil {
		t.Fatalf("lockState %v", st)
	}
}

func TestPolicyListsSignOut(t *testing.T) {
	r, ok := policy["userSignOut"]
	if !ok || r.need != 0 {
		t.Fatal("userSignOut must be open: signing out is never refused")
	}
}

func TestLockedSessionOfRemovedUserIsGone(t *testing.T) {
	a, c, _ := persistentApp(t)
	read := register(a, c)
	a.UserLock(c)
	cfg := a.r().cfg.Clone()
	cfg.Users = cfg.Users[1:]
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.syncAccess(c)
	if st := lastLockState(t, read); st["held"] != nil {
		t.Fatalf("a Lock with nobody to resume it: %v", st)
	}
}

func TestHeldLockBlocksAdvancedSettingsPinPaths(t *testing.T) {
	for _, enforced := range []bool{false, true} {
		a := userApp(t, enforced)
		setFlags(t, a, "Ana", true, true)
		c := &client{ip: "10.0.0.1"}
		register(a, c)
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		if err := a.Unlock(c, "2468"); err != nil {
			t.Fatal(err)
		}
		if err := admitMsg(a, c, inMsg{T: "users"}); err != nil {
			t.Fatalf("enforced=%v: before the Lock: %v", enforced, err)
		}
		a.UserLock(c)
		for _, m := range []inMsg{{T: "users"}, {T: "configExport"}, {T: "configImportBegin"}, {T: "userSignOutAll"}} {
			if err := admitMsg(a, c, m); !errors.Is(err, errPageLocked) {
				t.Errorf("enforced=%v: %s from a locked session with the Advanced Settings PIN: %v", enforced, m.T, err)
			}
		}
	}
}

func TestHeldLockBlocksTrustedSavesEvenWithAdvancedSettingsPin(t *testing.T) {
	for _, enforced := range []bool{false, true} {
		a := userApp(t, enforced)
		setFlags(t, a, "Ana", true, true)
		c := &client{ip: "10.0.0.1"}
		register(a, c)
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		if err := a.Unlock(c, "2468"); err != nil {
			t.Fatal(err)
		}
		a.UserLock(c)
		for _, m := range []inMsg{{T: "config"}, {T: "filterSets"}, {T: "presets"}, {T: "profileGet", Name: "Ana"}, {T: "profileSave", Name: "Ana"}} {
			if err := admitMsg(a, c, m); err == nil {
				t.Errorf("enforced=%v: %s admitted from a locked session with the Advanced Settings PIN", enforced, m.T)
			}
		}
		if err := a.SaveFilterSets(c, []config.FilterSet{{Name: "X", Sode: true, Ext: true}}); err == nil {
			t.Errorf("enforced=%v: filter sets saved from a locked session", enforced)
		}
	}
}
