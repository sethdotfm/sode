package app

import (
	"encoding/json"
	"errors"
	"testing"
	"time"
)

// sessionToken drains c's queue and returns the token of a `session` message, if any.
func sessionToken(c *client) string {
	tok := ""
	for {
		select {
		case b := <-c.send:
			var m struct{ T, Token string }
			json.Unmarshal(b, &m)
			if m.T == "session" {
				tok = m.Token
			}
		default:
			return tok
		}
	}
}

// A non-persistent user has no session token to hold the Lock, so the Lock hands them a
// tab token that a reload presents (ADR 0001: every signed-in user's Lock is held).
func TestLockOfNonPersistentUserSurvivesReload(t *testing.T) {
	for _, enforced := range []bool{false, true} {
		a := userApp(t, enforced)
		setFlags(t, a, "Ana", false, false)
		c := &client{ip: "10.0.0.1"}
		register(a, c)
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		a.UserLock(c)
		tok := sessionToken(c)
		if tok == "" {
			t.Fatalf("enforced=%v: the Lock gave no token to carry it across a reload", enforced)
		}
		dropClient(a, c)
		fresh := &client{ip: "10.0.0.2"}
		read := register(a, fresh)
		a.Hi(fresh, "main", tok)
		if err := canTake(a, fresh); !errors.Is(err, errPageLocked) {
			t.Fatalf("enforced=%v: reload of a locked session: %v", enforced, err)
		}
		if st := lastLockState(t, read); st["held"] != true || st["user"] != nil {
			t.Fatalf("enforced=%v: lockState %v", enforced, st)
		}
		if err := a.UserUnlock(fresh, "567890"); err == nil {
			t.Fatalf("enforced=%v: Bo's PIN resumed Ana's Lock", enforced)
		}
		if err := a.UserUnlock(fresh, "1234"); err != nil {
			t.Fatal(err)
		}
		if err := canTake(a, fresh); err != nil || fresh.who() != "Ana" {
			t.Fatalf("enforced=%v: after the PIN: %v, %q", enforced, err, fresh.who())
		}
		// Under a PIN lock an unlocked non-persistent session never survives a reload.
		again := &client{ip: "10.0.0.3"}
		register(a, again)
		a.Hi(again, "main", tok)
		if enforced && a.pageAllowed(again, gateMain) {
			t.Fatal("a resumed session's token signed a reload in without a PIN")
		}
	}
}

func TestSignOutFromNonPersistentLockRevokesToken(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", false, false)
	c := &client{ip: "10.0.0.1"}
	register(a, c)
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	a.UserLock(c)
	tok := sessionToken(c)
	if tok == "" {
		t.Fatal("no token")
	}
	a.SignOut(c)
	if tokenAlive(a, tok) {
		t.Fatal("sign out left the token alive")
	}
}

// A rename ends the user's persistent sessions (ADR 0001): tokens are looked up by name.
func TestRenameEndsPersistentSession(t *testing.T) {
	a, c, tok := persistentApp(t)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Anna", Was: "Ana", Persistent: true}, {Name: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	if tokenAlive(a, tok) {
		t.Fatal("the token outlived a rename")
	}
	fresh := &client{ip: "10.0.0.2"}
	register(a, fresh)
	a.Hi(fresh, "main", tok)
	if err := canTake(a, fresh); !errors.Is(err, errPageLocked) {
		t.Fatalf("a reload with the old token: %v", err)
	}
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("the open connection stayed signed in as the old name: %v", err)
	}
}

// A Lock raised while a token is being issued must end up on that token, or a reload
// loses it.
func TestLockRacingTokenIssueReachesTheToken(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", false, true)
	ana, _ := a.r().cfg.FindUser("Ana")
	c := &client{ip: "10.0.0.1"}
	register(a, c)
	a.mu.Lock()
	c.u.user.Store(&ana)
	a.mu.Unlock()
	a.tokens.mu.Lock() // holds issue() between its checks and the re-lock
	done := make(chan string)
	go func() { done <- a.issueSession(c) }()
	time.Sleep(50 * time.Millisecond)
	a.UserLock(c)
	a.tokens.mu.Unlock()
	tok := <-done
	a.mu.Lock()
	held := c.u.tokenMu()
	a.mu.Unlock()
	if held != "" && !a.tokens.locked(held) {
		t.Fatalf("the Lock is on but token %q (issued %q) is not marked locked", held, tok)
	}
}
