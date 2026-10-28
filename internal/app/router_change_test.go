package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// changeRouter applies the current settings with another router address.
func changeRouter(t *testing.T, a *App) {
	t.Helper()
	cfg := a.r().cfg
	cfg.Router = "127.0.0.1:1"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

// waitFor polls cond until it holds, failing after a few seconds.
func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// runTake starts a take in the background; the returned channel closes when
// it has finished.
func runTake(a *App, req takeReq) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.take(&client{ip: "10.0.0.1"}, req)
	}()
	return done
}

func waitDone(t *testing.T, done <-chan struct{}, within time.Duration) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(within):
		t.Fatalf("take still running after %v", within)
	}
}

// A take waiting for its confirms when the router changes ends at once and
// leaves nothing in the new router's tracker.
func TestRouterChangeCutsTake(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.ignore[5] = true // the connects go out and nobody answers
	done := runTake(a, takeReq{ID: 1, Src: 7, Dst: 5, Levels: []int{1, 2}, NoCheck: true})
	waitFor(t, "the take to register", func() bool {
		a.mu.Lock()
		registered := len(a.waiters) == 1
		a.mu.Unlock()
		return registered && f.sends() >= 2 // both connects are out
	})
	sent := f.sends()
	changeRouter(t, a)
	waitDone(t, done, takeTimeout/2)
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].OK || !strings.Contains(es[0].Err, "level 1 no answer before the router changed") ||
		!strings.Contains(es[0].Err, "level 2 no answer before the router changed") || !equalInts(es[0].Failed, []int{1, 2}) {
		t.Fatalf("take %+v", es)
	}
	if n := f.sends(); n != sent {
		t.Fatalf("%d messages sent after the router changed", n-sent)
	}
	a.mu.Lock()
	if len(a.waiters) != 0 || len(a.ext.own) != 0 || len(a.ext.late) != 0 || len(a.ext.lateTake) != 0 || len(a.ext.causes) != 0 {
		t.Fatalf("take left state in the new tracker: %d waiters, %+v", len(a.waiters), a.ext)
	}
	a.mu.Unlock()
	// The route made on the new router is a change made elsewhere.
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	es, _ = a.hist.Page(-1, 2)
	if len(es) != 2 || !es[0].Ext || es[0].Src != 7 {
		t.Fatalf("Connected after the change %+v", es)
	}
}

// A take still reading dst's routes sends no connect once the router changed.
func TestRouterChangeDuringCheck(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.mute[5] = true // the check goes unanswered
	done := runTake(a, takeReq{ID: 1, Src: 7, Dst: 5, Levels: []int{1, 2, 3}})
	waitFor(t, "the check to start", func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return len(a.checks) == 1
	})
	changeRouter(t, a)
	waitDone(t, done, checkTimeout/2)
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].OK || !strings.HasPrefix(es[0].Err, "levels 1, 2, 3 not sent: the router changed") || !equalInts(es[0].Failed, []int{1, 2, 3}) {
		t.Fatalf("take %+v", es)
	}
	if n := f.sentFor("take"); n != 0 {
		t.Fatalf("%d connects sent", n)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.waiters) != 0 || len(a.checks) != 0 || len(a.ext.own) != 0 || len(a.ext.causes) != 0 {
		t.Fatalf("take left state in the new tracker: %+v", a.ext)
	}
}

// A route an old take gave up on is no longer ours after the router changes:
// the new router's tally of it is a change made elsewhere.
func TestRouterChangeClearsLate(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.ignore[5] = true
	takeOne(t, a, 7, 5, 1)
	a.mu.Lock()
	if len(a.ext.late) != 1 {
		t.Fatalf("late %v", a.ext.late)
	}
	a.mu.Unlock()
	changeRouter(t, a)
	a.mu.Lock()
	if len(a.ext.late) != 0 || len(a.ext.lateTake) != 0 {
		t.Fatalf("late kept: %v %v", a.ext.late, a.ext.lateTake)
	}
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtTally(0, 0, 5, 3)) // the new router's cache learns dst 5
	a.onRouterMsg(swp08.ExtTally(0, 0, 5, 7)) // then it reports the old take's route unannounced
	endBurst(a)
	es, _ := a.hist.Page(-1, 2)
	if len(es) != 2 || !es[0].Ext || es[0].Src != 7 || es[1].Ext || es[1].OK {
		t.Fatalf("history %+v", es)
	}
}
