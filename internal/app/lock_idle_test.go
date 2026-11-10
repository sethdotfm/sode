package app

import (
	"errors"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Auto-lock: the page's timer sends the same userLock as the lock button (idleLock), and
// the server's idle sweep locks a session whose browser is gone or asleep (fake clock).

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

type fakeClock struct{ t time.Time }

func (f *fakeClock) now() time.Time       { return f.t }
func (f *fakeClock) pass(d time.Duration) { f.t = f.t.Add(d) }

func useFakeClock(a *App) *fakeClock {
	f := &fakeClock{t: time.Now()}
	a.now = f.now
	return f
}

// setAutoLock sets the live minutes and, when noAuto names a user, their No auto-lock.
func setAutoLock(t *testing.T, a *App, minutes int, noAuto string) {
	t.Helper()
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	_, nr, err := a.update(func(cfg *config.Config) error {
		cfg.AutoLockMinutes = minutes
		for i := range cfg.Users {
			cfg.Users[i].NoAutoLock = cfg.Users[i].Name == noAuto
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.mu.Unlock()
}

func ping(t *testing.T, a *App, c *client) {
	t.Helper()
	m := inMsg{T: "activity"}
	if err := admitMsg(a, c, m); err != nil {
		t.Fatalf("activity refused: %v", err)
	}
	policy["activity"].handle(a, c, &m, nil)
}

func TestServerIdleLocksClosedPersistentSession(t *testing.T) {
	for _, sweep := range []bool{true, false} {
		a, c, tok := persistentApp(t)
		clk := useFakeClock(a)
		setAutoLock(t, a, 5, "")
		dropClient(a, c)
		clk.pass(6 * time.Minute)
		if sweep {
			a.idleSweep()
		}
		fresh := &client{ip: "10.0.0.2"}
		read := register(a, fresh)
		a.Hi(fresh, "main", tok)
		if err := canTake(a, fresh); !errors.Is(err, errPageLocked) {
			t.Fatalf("sweep=%v: the reconnect is not locked: %v", sweep, err)
		}
		if st := lastLockState(t, read); st["held"] != true {
			t.Fatalf("sweep=%v: lockState %v", sweep, st)
		}
		if !tokenAlive(a, tok) {
			t.Fatalf("sweep=%v: the idle Lock signed the user out", sweep)
		}
		if err := a.UserUnlock(fresh, "1234"); err != nil {
			t.Fatal(err)
		}
		a.idleSweep() // the resume is activity
		if err := canTake(a, fresh); err != nil || fresh.who() != "Ana" {
			t.Fatalf("sweep=%v: after the PIN: %v, %q", sweep, err, fresh.who())
		}
	}
}

func TestServerIdleLockPushesAndPingsKeepSiblings(t *testing.T) {
	a, c, tok := persistentApp(t)
	clk := useFakeClock(a)
	setAutoLock(t, a, 5, "")
	sib := &client{ip: "10.0.0.2"}
	readSib := register(a, sib)
	a.Hi(sib, "panel", tok)
	for range 3 {
		clk.pass(4 * time.Minute)
		ping(t, a, c)
		a.idleSweep()
	}
	for _, cl := range []*client{c, sib} {
		if err := canTake(a, cl); err != nil {
			t.Fatalf("%s locked although the session was active: %v", cl.ip, err)
		}
	}
	readSib()
	clk.pass(5 * time.Minute)
	a.idleSweep()
	for _, cl := range []*client{c, sib} {
		if err := canTake(a, cl); !errors.Is(err, errPageLocked) {
			t.Fatalf("%s not locked after idle: %v", cl.ip, err)
		}
	}
	if st := lastLockState(t, readSib); st["held"] != true || st["locked"] != true {
		t.Fatalf("panel tab got %v", st)
	}
	// A ping under the Lock does not count: only the resume does.
	ping(t, a, sib)
	if err := canTake(a, sib); !errors.Is(err, errPageLocked) {
		t.Fatalf("a ping lifted the Lock: %v", err)
	}
}

// periodic is what runs the sweep: due once idleEvery has passed since the last one.
func TestPeriodicRunsIdleSweepWhenDue(t *testing.T) {
	a, c, _ := persistentApp(t)
	clk := useFakeClock(a)
	setAutoLock(t, a, 5, "")
	ping(t, a, c)
	a.idleEvery = time.Minute
	a.periodic() // first sweep, nothing idle yet
	clk.pass(6 * time.Minute)
	a.mu.Lock()
	a.idleAt = clk.now().Add(-30 * time.Second) // swept 30 s ago: not due
	a.mu.Unlock()
	a.periodic()
	if err := canTake(a, c); err != nil {
		t.Fatalf("periodic swept before idleEvery: %v", err)
	}
	clk.pass(30 * time.Second)
	a.periodic()
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("periodic did not run the idle sweep: %v", err)
	}
}

func TestServerIdleLockTokenlessSession(t *testing.T) {
	a := userApp(t, false)
	clk := useFakeClock(a)
	setAutoLock(t, a, 5, "")
	c := &client{ip: "10.0.0.1"}
	read := register(a, c)
	a.Hi(c, "panel", "")
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	a.idleSweep()
	clk.pass(6 * time.Minute)
	a.idleSweep()
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("not locked after idle: %v", err)
	}
	if st := lastLockState(t, read); st["held"] != true {
		t.Fatalf("lockState %v", st)
	}
}

func TestServerIdleLockMinutesAreLive(t *testing.T) {
	a, c, _ := persistentApp(t)
	clk := useFakeClock(a)
	setAutoLock(t, a, 0, "")
	clk.pass(100 * time.Hour)
	a.idleSweep()
	if err := canTake(a, c); err != nil {
		t.Fatalf("0 minutes locked: %v", err)
	}
	ping(t, a, c)
	setAutoLock(t, a, 30, "")
	clk.pass(10 * time.Minute)
	a.idleSweep()
	if err := canTake(a, c); err != nil {
		t.Fatalf("locked before 30 minutes: %v", err)
	}
	setAutoLock(t, a, 5, "")
	a.idleSweep()
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("lowering the minutes did not apply: %v", err)
	}
}

func TestServerIdleLockSkipsNoAutoLock(t *testing.T) {
	a, c, tok := persistentApp(t)
	clk := useFakeClock(a)
	setAutoLock(t, a, 5, "Ana")
	clk.pass(time.Hour)
	a.idleSweep()
	if err := canTake(a, c); err != nil {
		t.Fatalf("No auto-lock user locked: %v", err)
	}
	dropClient(a, c)
	a.idleSweep()
	fresh := &client{ip: "10.0.0.2"}
	register(a, fresh)
	a.Hi(fresh, "main", tok)
	if err := canTake(a, fresh); err != nil {
		t.Fatalf("No auto-lock user's token locked: %v", err)
	}
}

// A signed-out connection's ping does nothing and is never refused.
func TestActivityPingSignedOut(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	register(a, c)
	ping(t, a, c)
	if c.who() != "10.0.0.1" {
		t.Fatalf("ping signed in %q", c.who())
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
