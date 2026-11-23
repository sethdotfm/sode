package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

func changeRouter(t *testing.T, a *App) {
	t.Helper()
	cfg := a.r().cfg
	cfg.Address = "127.0.0.1:1"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

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
func runTake(a *App, req routeReq) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		a.takeAllowed(&client{ip: "10.0.0.1"}, []routeReq{req})
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
	done := runTake(a, routeReq{ID: 1, Src: 7, Dst: 5, Levels: []int{1, 2}})
	waitFor(t, "the take to register", func() bool {
		a.mu.Lock()
		registered := len(a.first.waiters) == 1
		a.mu.Unlock()
		return registered && f.cmds(swp08.CmdExtConnect) >= 2 // both connects are out, not only the reads
	})
	sent := f.sends()
	changeRouter(t, a)
	waitDone(t, done, a.first.routeTimeout/2)
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].OK || !strings.Contains(es[0].Err, "level 1 no answer before the router changed") ||
		!strings.Contains(es[0].Err, "level 2 no answer before the router changed") || !equalInts(es[0].Failed, []int{1, 2}) {
		t.Fatalf("take %+v", es)
	}
	if n := f.sends(); n != sent {
		t.Fatalf("%d messages sent after the router changed", n-sent)
	}
	a.mu.Lock()
	if len(a.first.waiters) != 0 || len(a.first.ext.own) != 0 || len(a.first.ext.late) != 0 || len(a.first.ext.lateRoute) != 0 || len(a.first.ext.causes) != 0 {
		t.Fatalf("take left state in the new tracker: %d waiters, %+v", len(a.first.waiters), a.first.ext)
	}
	a.mu.Unlock()
	a.onUnitMsg(a.first, swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	es, _ = a.hist.Page(-1, 2)
	if len(es) != 2 || !es[0].Ext || es[0].Src != 7 {
		t.Fatalf("Connected after the change %+v", es)
	}
}

func TestRouterChangeDuringCheck(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.mute[5] = true // the check goes unanswered
	done := runTake(a, routeReq{ID: 1, Src: 7, Dst: 5, Levels: []int{1, 2, 3}})
	waitFor(t, "the check to start", func() bool {
		a.mu.Lock()
		defer a.mu.Unlock()
		return len(a.first.checks) == 1
	})
	changeRouter(t, a)
	waitDone(t, done, a.first.checkTimeout/2)
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].OK || !strings.HasPrefix(es[0].Err, "levels 1, 2, 3 not sent: the router changed") || !equalInts(es[0].Failed, []int{1, 2, 3}) {
		t.Fatalf("take %+v", es)
	}
	if n := f.sentFor("take"); n != 0 {
		t.Fatalf("%d connects sent", n)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.first.waiters) != 0 || len(a.first.checks) != 0 || len(a.first.ext.own) != 0 || len(a.first.ext.causes) != 0 {
		t.Fatalf("take left state in the new tracker: %+v", a.first.ext)
	}
}

// A route that an old take gave up on is no longer ours after the router changes.
// The new router's tally of it is a change made elsewhere.
func TestRouterChangeClearsLate(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.ignore[5] = true
	routeOne(t, a, 7, 5, 1)
	a.mu.Lock()
	if len(a.first.ext.late) != 1 {
		t.Fatalf("late %v", a.first.ext.late)
	}
	a.mu.Unlock()
	changeRouter(t, a)
	a.mu.Lock()
	if len(a.first.ext.late) != 0 || len(a.first.ext.lateRoute) != 0 {
		t.Fatalf("late kept: %v %v", a.first.ext.late, a.first.ext.lateRoute)
	}
	a.mu.Unlock()
	a.onUnitMsg(a.first, swp08.ExtTally(0, 0, 5, 3)) // the new router's cache learns dst 5
	a.onUnitMsg(a.first, swp08.ExtTally(0, 0, 5, 7)) // then it reports the old take's route unannounced
	endBurst(a)
	es, _ := a.hist.Page(-1, 2)
	if len(es) != 2 || !es[0].Ext || es[0].Src != 7 || es[1].Ext || es[1].OK {
		t.Fatalf("history %+v", es)
	}
}
