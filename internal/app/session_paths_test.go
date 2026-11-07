package app

import (
	"errors"
	"testing"
)

// These pin the session paths (sign in, Lock, resume, sign out, sign out all, token
// file) through the same calls the policy handlers make, so the session struct refactor
// can move code without changing what a page sees.

func heldState(t *testing.T, read func() []map[string]any) bool {
	t.Helper()
	return lastLockState(t, read)["held"] == true
}

func TestSharedSessionLocksResumesAndSignsOutTogether(t *testing.T) {
	a, c, tok := persistentApp(t)
	readC := register(a, c)
	twin := &client{ip: "10.0.0.2"}
	readTwin := register(a, twin)
	a.Hi(twin, "main", tok)
	readC()
	readTwin()

	a.UserLock(twin)
	if !heldState(t, readC) || !heldState(t, readTwin) {
		t.Fatal("the Lock did not reach both connections")
	}
	if !loadTokens(a.tokens.path).locked(tok) {
		t.Fatal("the Lock is not in tokens.json")
	}

	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if heldState(t, readC) || heldState(t, readTwin) {
		t.Fatal("the resume did not reach both connections")
	}
	if err := canTake(a, twin); err != nil {
		t.Fatalf("twin after the resume: %v", err)
	}
	if loadTokens(a.tokens.path).locked(tok) || !tokenAlive(a, tok) {
		t.Fatal("a resume must clear the flag in tokens.json and keep the token")
	}

	a.UserLock(c)
	a.SignOut(twin)
	if tokenAlive(a, tok) || loadTokens(a.tokens.path).locked(tok) {
		t.Fatal("sign out left the token")
	}
	for _, cl := range []*client{c, twin} {
		if cl.who() != cl.ip {
			t.Fatalf("%s still signed in as %s", cl.ip, cl.who())
		}
		if err := canTake(a, cl); !errors.Is(err, errPageLocked) {
			t.Fatalf("%s after sign out: %v", cl.ip, err)
		}
	}
}

func TestSwitchingUserLeavesTheSharedSession(t *testing.T) {
	a, c, tok := persistentApp(t)
	twin := &client{ip: "10.0.0.2"}
	register(a, twin)
	a.Hi(twin, "main", tok)
	if err := a.UserUnlock(c, "567890"); err != nil { // Bo takes over c
		t.Fatal(err)
	}
	a.UserLock(c)
	if err := a.UserUnlock(c, "567890"); err != nil {
		t.Fatal(err)
	}
	a.SignOut(c)
	if twin.who() != "Ana" || !tokenAlive(a, tok) {
		t.Fatal("Bo's Lock, resume and sign out touched Ana's session")
	}
	if err := canTake(a, twin); err != nil {
		t.Fatalf("Ana's twin: %v", err)
	}
}

func TestHeldTabTokenIsSharedAndDroppedOnResume(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", false, false)
	c := &client{ip: "10.0.0.1"}
	register(a, c)
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	a.UserLock(c)
	tok := sessionToken(c)
	if tok == "" || !loadTokens(a.tokens.path).locked(tok) {
		t.Fatalf("held tab token %q missing or not locked in the file", tok)
	}
	one, two := &client{ip: "10.0.0.2"}, &client{ip: "10.0.0.3"}
	readTwo := register(a, two)
	register(a, one)
	a.Hi(one, "main", tok)
	a.Hi(two, "main", tok)
	if !heldState(t, readTwo) {
		t.Fatal("a reload with a held tab token did not come back locked")
	}
	if err := a.UserUnlock(one, "1234"); err != nil {
		t.Fatal(err)
	}
	if err := canTake(a, two); err != nil {
		t.Fatalf("the other tab after the resume: %v", err)
	}
	if tokenAlive(a, tok) {
		t.Fatal("a resume under a PIN lock left the tab token")
	}
	late := &client{ip: "10.0.0.4"}
	register(a, late)
	a.Hi(late, "main", tok)
	if a.pageAllowed(late, gateMain) {
		t.Fatal("the dropped token signed a reload in")
	}
}

func TestResumedTabsShareNoLockOrSignOut(t *testing.T) {
	setup := func(t *testing.T) (a *App, c, one, two *client) {
		a = userApp(t, true)
		setFlags(t, a, "Ana", false, false)
		c = &client{ip: "10.0.0.1"}
		register(a, c)
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		a.UserLock(c)
		tok := sessionToken(c)
		one, two = &client{ip: "10.0.0.2"}, &client{ip: "10.0.0.3"}
		register(a, one)
		register(a, two)
		a.Hi(one, "main", tok)
		a.Hi(two, "main", tok)
		if err := a.UserUnlock(one, "1234"); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Run("Lock of one tab", func(t *testing.T) {
		a, c, one, two := setup(t)
		a.UserLock(one)
		for name, cl := range map[string]*client{"two": two, "c": c} {
			if err := canTake(a, cl); err != nil {
				t.Fatalf("%s locked by another tab's Lock: %v", name, err)
			}
		}
	})
	t.Run("sign out of one tab", func(t *testing.T) {
		a, c, one, two := setup(t)
		a.SignOut(one)
		for name, cl := range map[string]*client{"two": two, "c": c} {
			if err := canTake(a, cl); err != nil {
				t.Fatalf("%s signed out by another tab: %v", name, err)
			}
		}
	})
}

func TestSignOutAllEndsHeldSessions(t *testing.T) {
	a, c, tok := persistentApp(t)
	read := register(a, c)
	twin := &client{ip: "10.0.0.2"}
	register(a, twin)
	a.Hi(twin, "main", tok)
	a.UserLock(c)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SignOutAll(admin, "Ana"); err != nil {
		t.Fatal(err)
	}
	if tokenAlive(a, tok) {
		t.Fatal("token survived")
	}
	if heldState(t, read) || c.who() != c.ip || twin.who() != twin.ip {
		t.Fatal("a held session survived")
	}
	if err := a.UserUnlock(c, "1234"); err != nil || c.who() != "Ana" {
		t.Fatalf("sign in after sign out all: %v", err)
	}
}

func TestHeldLockEndsWhenTheHolderChangesPin(t *testing.T) {
	a, c, _ := persistentApp(t)
	read := register(a, c)
	a.UserLock(c)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	req := usersReq{Users: []userEdit{{Name: "Ana", Was: "Ana", Pin: "4321", Persistent: true}, {Name: "Bo", Was: "Bo"}}}
	if err := a.SaveUsers(admin, req); err != nil {
		t.Fatal(err)
	}
	a.syncAccess(c)
	st := lastLockState(t, read)
	if st["held"] != nil || st["user"] != nil || c.who() != c.ip {
		t.Fatalf("a Lock whose PIN changed: %v", st)
	}
}

func TestLockWithoutTokenThenSignInAgain(t *testing.T) {
	a := userApp(t, false)
	setFlags(t, a, "Ana", false, false)
	c := &client{ip: "10.0.0.1"}
	register(a, c)
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if tok := a.issueSessionTab(c, true); tok == "" || !tokenAlive(a, tok) {
		t.Fatalf("tab sign-in token %q", tok)
	}
	a.UserLock(c)
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	// No PIN lock here: the tab token stays through a resume, as it carries the sign-in.
	if len(a.tokens.toks) != 1 {
		t.Fatalf("tokens after Lock and resume: %d", len(a.tokens.toks))
	}
	a.SignOut(c)
	if len(a.tokens.toks) != 0 {
		t.Fatal("sign out left the tab token")
	}
}
