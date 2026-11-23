package app

import (
	"errors"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"testing"

	"github.com/sethdotfm/sode/internal/swp08"
)

func TestAppHoldsOneRouterUnitByUID(t *testing.T) {
	a := newTestApp(t)
	uid := a.r().cfg.UID
	if len(a.units()) != 1 || a.units()[uid] != a.first || a.first.uid != uid {
		t.Fatalf("routers %v, first %p, uid %q", a.units(), a.first, uid)
	}
	for _, k := range []string{"", uid} {
		if u, ok := a.unit(k); !ok || u != a.first {
			t.Errorf("unit(%q) = %p %v", k, u, ok)
		}
	}
	if _, ok := a.unit("b0b0b0b0"); ok {
		t.Error("unit of an unknown UID")
	}
}

// threeRouters: an App with three routers, each with its own unit and a fake router on its link.
func threeRouters(t *testing.T) (*App, []*fakeRouter) {
	t.Helper()
	a := newTestApp(t, func(c *config.Config) {
		c.MoreRouters = []config.Router{
			moreRouter("B", "Second", "10.0.0.8:8910", 3),
			moreRouter("C", "Third", "10.0.0.9:8910", 3),
		}
	})
	fs := []*fakeRouter{newFakeRouter(t, a)}
	for _, rc := range a.r().cfg.MoreRouters {
		f, _ := addFakeUnit(t, a, rc)
		fs = append(fs, f)
	}
	return a, fs
}

// The test kit puts a fake router on each of several units; each one answers only for its own router.
func TestKitHasSeveralFakeRouters(t *testing.T) {
	a, fs := threeRouters(t)
	if len(a.units()) != 3 {
		t.Fatalf("%d units, want 3", len(a.units()))
	}
	for i, f := range fs {
		f.u.send(swp08.ExtInterrogate(0, 0, 6), swp08.ReasonResync)
		f.seed(5, i+1, 0)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for i, f := range fs {
		if got := f.u.router.tally[5][1]; got != i+1 {
			t.Errorf("router %d: dst 5 level 1 = %d, want %d", i, got, i+1)
		}
		if n := f.sends(); n != 1 {
			t.Errorf("router %d: %d sends, want 1", i, n)
		}
		if f.u.checkTimeout != 40*time.Millisecond || f.u.routeTimeout != 40*time.Millisecond || f.u.routeGrace != 40*time.Millisecond {
			t.Errorf("router %d: timers %v %v %v, want short", i, f.u.checkTimeout, f.u.routeTimeout, f.u.routeGrace)
		}
	}
}

// shortTimers shortens the timers on every unit an App holds.
func TestShortTimersEveryRouter(t *testing.T) {
	a, fs := threeRouters(t)
	for _, f := range fs {
		f.u.checkTimeout, f.u.routeTimeout, f.u.routeGrace, f.u.queueMax = time.Second, time.Second, time.Second, time.Second
	}
	shortTimers(a)
	for uid, u := range a.units() {
		if u.checkTimeout != 40*time.Millisecond || u.routeTimeout != 40*time.Millisecond || u.routeGrace != 40*time.Millisecond || u.queueMax != 500*time.Millisecond {
			t.Errorf("%s: timers %v %v %v %v", uid, u.checkTimeout, u.routeTimeout, u.routeGrace, u.queueMax)
		}
	}
}

// A fake router can drop its link and restore it; the other routers stay up.
func TestKitDropsAndRestoresOneLink(t *testing.T) {
	a, fs := threeRouters(t)
	for _, f := range fs {
		f.restore()
	}
	lost := errors.New("connection reset")
	fs[1].drop(lost)
	if err := fs[1].u.sendMsg(swp08.ExtInterrogate(0, 0, 5), swp08.Why{Reason: swp08.ReasonResync}); !errors.Is(err, swp08.ErrDown) {
		t.Errorf("send on a dropped link: %v, want ErrDown", err)
	}
	a.mu.Lock()
	st := []statusMsg{fs[0].u.status, fs[1].u.status, fs[2].u.status}
	a.mu.Unlock()
	if st[1].Router != string(swp08.StateDown) || st[1].Err != lost.Error() {
		t.Errorf("dropped router status %+v", st[1])
	}
	if st[0].Router != string(swp08.StateUp) || st[2].Router != string(swp08.StateUp) {
		t.Errorf("other routers %q %q, want up", st[0].Router, st[2].Router)
	}
	before := fs[1].sends()
	fs[1].restore()
	a.mu.Lock()
	up := fs[1].u.status.Router
	a.mu.Unlock()
	if up != string(swp08.StateUp) {
		t.Errorf("restored router %q, want up", up)
	}
	if fs[1].sends() <= before {
		t.Error("restore sent no reads")
	}
}

// A filter set with no router is the first router's: a read on another router does not check it.
func TestBlankFilterSetIsFirstRouters(t *testing.T) {
	gone := namePoll{map[int]string{}, map[int]string{}}
	set := func() config.FilterSet { return config.FilterSet{Name: "Cams", Src: `"CAM 9"`} }
	other := &pollCheck{uid: "b0b0b0b0", first: "a0a0a0a0", prev: gone, cur: gone}
	if s := set(); other.filterSet(&s, 0, "Ana") || len(s.Broken) != 0 {
		t.Errorf("another router's read checked a set with no router: %+v", s.Broken)
	}
	first := &pollCheck{uid: "a0a0a0a0", first: "a0a0a0a0", prev: gone, cur: gone}
	if s := set(); !first.filterSet(&s, 0, "Ana") || len(s.Broken) != 1 {
		t.Errorf("the first router's read did not check it: %+v", s.Broken)
	}
}

// A message on one router's link changes only that router's state.
func TestRouterMessageStaysInItsUnit(t *testing.T) {
	a := newTestApp(t)
	other := newRouterUnit(a.r().cfg.Router)
	a.onUnitMsg(other, swp08.ExtTally(0, 0, 5, 7))
	if other.router.tally[5][1] != 7 {
		t.Errorf("other unit tally %v", other.router.tally)
	}
	if _, ok := a.first.router.tally[5]; ok {
		t.Errorf("first unit got the other router's route: %v", a.first.router.tally)
	}
	a.onUnitState(other, swp08.StateUp, nil)
	if other.status.Router != string(swp08.StateUp) || a.first.status.Router == string(swp08.StateUp) {
		t.Errorf("status: other %q, first %q", other.status.Router, a.first.status.Router)
	}
}
