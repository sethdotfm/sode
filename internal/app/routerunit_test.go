package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/swp08"
)

func TestAppHoldsOneRouterUnitByUID(t *testing.T) {
	a := newTestApp(t)
	uid := a.r().cfg.UID
	if len(a.routers) != 1 || a.routers[uid] != a.routerUnit || a.routerUnit.uid != uid {
		t.Fatalf("routers %v, first %p, uid %q", a.routers, a.routerUnit, uid)
	}
	for _, k := range []string{"", uid} {
		if u, ok := a.unit(k); !ok || u != a.routerUnit {
			t.Errorf("unit(%q) = %p %v", k, u, ok)
		}
	}
	if _, ok := a.unit("b0b0b0b0"); ok {
		t.Error("unit of an unknown UID")
	}
	// The old fields are the first unit's.
	a.onRouterMsg(swp08.ExtTally(0, 0, 5, 7))
	if a.routers[uid].router.tally[5][1] != 7 || a.routeTimeout != a.routers[uid].routeTimeout {
		t.Errorf("tally through the alias %v", a.routers[uid].router.tally)
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
	if _, ok := a.router.tally[5]; ok {
		t.Errorf("first unit got the other router's route: %v", a.router.tally)
	}
	a.onUnitState(other, swp08.StateUp, nil)
	if other.status.Router != string(swp08.StateUp) || a.status.Router == string(swp08.StateUp) {
		t.Errorf("status: other %q, first %q", other.status.Router, a.status.Router)
	}
}
