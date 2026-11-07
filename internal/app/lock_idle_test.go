package app

import (
	"errors"
	"testing"
)

// Auto-lock sends the same userLock as the lock button, for persistent users too. The
// page does the timing, so these tests drive the message its idle timer sends.

func idleLock(t *testing.T, a *App, c *client) {
	t.Helper()
	if err := admitMsg(a, c, inMsg{T: "userLock"}); err != nil {
		t.Fatalf("userLock refused: %v", err)
	}
	a.UserLock(c)
}

func dropClient(a *App, c *client) {
	a.mu.Lock()
	delete(a.clients, c)
	a.mu.Unlock()
}

func TestIdleLockOfPersistentUserAcrossReloadAndReconnect(t *testing.T) {
	a, c, tok := persistentApp(t)
	idleLock(t, a, c)
	if !tokenAlive(a, tok) {
		t.Fatal("the idle Lock signed the persistent user out")
	}
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("after idle: %v", err)
	}
	dropClient(a, c)
	// A reload, then a later reconnect: each new socket says hi with the token.
	for _, ip := range []string{"10.0.0.2", "10.0.0.3"} {
		fresh := &client{ip: ip}
		read := register(a, fresh)
		a.Hi(fresh, "main", tok)
		if err := canTake(a, fresh); !errors.Is(err, errPageLocked) {
			t.Fatalf("%s: lost the Lock: %v", ip, err)
		}
		if st := lastLockState(t, read); st["held"] != true || st["user"] != nil {
			t.Fatalf("%s: lockState %v", ip, st)
		}
		if err := a.UserUnlock(fresh, "1234"); err != nil {
			t.Fatal(err)
		}
		if err := canTake(a, fresh); err != nil || fresh.who() != "Ana" {
			t.Fatalf("%s: after the PIN: %v, %q", ip, err, fresh.who())
		}
		idleLock(t, a, fresh)
		dropClient(a, fresh)
	}
}

func TestIdleLockOfNonPersistentUserResumesWithPin(t *testing.T) {
	for _, enforced := range []bool{false, true} {
		a := userApp(t, enforced)
		setFlags(t, a, "Ana", false, false)
		c := &client{ip: "10.0.0.1"}
		register(a, c)
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		idleLock(t, a, c)
		if err := canTake(a, c); !errors.Is(err, errPageLocked) {
			t.Fatalf("enforced=%v: after idle: %v", enforced, err)
		}
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		if err := canTake(a, c); err != nil || c.who() != "Ana" {
			t.Fatalf("enforced=%v: after the PIN: %v, %q", enforced, err, c.who())
		}
	}
}
