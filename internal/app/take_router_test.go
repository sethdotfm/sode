package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// secondRouter gives a with a second router unit, B, and a fake router on its link. The first router keeps its
// default timers, so a take that waits on them would take seconds.
func secondRouter(t *testing.T) (a *App, first, second *fakeRouter, u *routerUnit) {
	t.Helper()
	a = newTestApp(t, func(c *config.Config) {
		c.MoreRouters = []config.Router{{Address: "10.0.0.9:8910", Levels: 3, Prefix: "B", Title: "Second"}}
	})
	first = newFakeRouter(t, a)
	a.checkTimeout, a.routeTimeout, a.routeGrace = defaultCheckTimeout, defaultRouteTimeout, defaultRouteGrace
	second, u = addFakeUnit(t, a, a.r().cfg.MoreRouters[0])
	return a, first, second, u
}

// addFakeUnit adds a unit for router rc to a, with short timers and a fake router on its link.
func addFakeUnit(t *testing.T, a *App, rc config.Router) (*fakeRouter, *routerUnit) {
	t.Helper()
	u := newRouterUnit(rc)
	u.checkTimeout, u.routeTimeout, u.routeGrace, u.queueMax = 40*time.Millisecond, 40*time.Millisecond, 40*time.Millisecond, 500*time.Millisecond
	a.routers[u.uid] = u
	f := &fakeRouter{a: a, tally: map[[2]int]int{}, ignore: map[int]bool{}, deaf: map[[2]int]bool{}, lose: map[int]bool{}, mute: map[int]bool{}, out: make(chan []byte, 256)}
	f.prot = fakeProtect{holds: map[[2]int]protHold{}, deafSet: map[int]bool{}, deafClear: map[int]bool{}}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for m := range f.out {
			a.onUnitMsg(u, m)
		}
	}()
	t.Cleanup(func() { close(f.out); <-done })
	u.sendMsg = f.send
	return f, u
}

// A take names its router: the check, the connects, the waiter, the own keys and the timers are that router's.
func TestTakeUsesItsRouter(t *testing.T) {
	a, first, second, u := secondRouter(t)
	wire := a.r().byUI[1].Wire
	a.takeAllowed(&client{ip: "10.0.0.1"}, []routeReq{{ID: 1, Src: 4, Dst: 2, Levels: []int{1}, Router: u.uid}})
	if n := first.sends(); n != 0 {
		t.Fatalf("the first router got %d messages", n)
	}
	second.mu.Lock()
	got := second.tally[[2]int{2, wire}]
	second.mu.Unlock()
	if got != 4 {
		t.Fatalf("second router tally %v", second.tally)
	}
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || !es[0].OK || es[0].Router != u.uid || es[0].RouterTitle != "Second" {
		t.Fatalf("entry %+v", es)
	}
	a.mu.Lock()
	// The Connected used up the own key in the second router's tracker, so it is the take's, not an External.
	tally, own, causes, firstTally := u.router.tally[2][1], len(u.ext.own), len(u.ext.causes), len(a.router.tally)
	waiting := len(u.waiters) + len(a.waiters)
	a.mu.Unlock()
	if tally != 4 || own != 0 || causes != 1 || firstTally != 0 || waiting != 0 {
		t.Errorf("second tally %d, own keys %d, causes %d, first tally %d, waiters %d", tally, own, causes, firstTally, waiting)
	}

	// The second router never answers: its short timers end the take, not the first router's 3 s.
	second.mu.Lock()
	second.ignore[3] = true
	second.mu.Unlock()
	start := time.Now()
	a.takeAllowed(&client{ip: "10.0.0.1"}, []routeReq{{ID: 2, Src: 4, Dst: 3, Levels: []int{1}, Router: u.uid}})
	if d := time.Since(start); d > time.Second {
		t.Errorf("the take waited %v: not the second router's timers", d)
	}
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || es[0].OK {
		t.Errorf("silent router: entry %+v", es)
	}
}

// A route on a router that Sode does not have is refused and sends nothing.
func TestTakeOnUnknownRouterRefused(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	if unread := a.takeAllowed(&client{ip: "10.0.0.1"}, []routeReq{{ID: 1, Src: 4, Dst: 2, Levels: []int{1}, Router: "b0b0b0b0"}}); unread {
		t.Error("unread with nothing read")
	}
	if n := f.sends(); n != 0 {
		t.Errorf("sent %d messages", n)
	}
	if es, _ := a.hist.Page(-1, 1); len(es) != 0 {
		t.Errorf("entries %+v", es)
	}
}

// A Communication Log row for a message names the router it went to, with that router's names.
func TestLogRowUsesItsRouter(t *testing.T) {
	a, _, _, u := secondRouter(t)
	a.mu.Lock()
	u.router.dstNames[2] = "B MON 2"
	a.router.dstNames[2] = "A MON 2"
	a.mu.Unlock()
	e := a.describe(swp08.TxEvent{Msg: swp08.ExtConnect(0, 0, 2, 4), Why: swp08.Why{Reason: swp08.ReasonTake, Router: u.uid}})
	if e.Router != u.uid || e.DstName != "B MON 2" {
		t.Errorf("row router %q dst name %q", e.Router, e.DstName)
	}
}
