package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// fillRouterDerived puts something in every router-derived field.
func fillRouterDerived(a *App) {
	until := time.Now().Add(time.Minute)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.router.srcNames[7] = "CAM 7"
	a.router.dstNames[5] = "MON 5"
	a.router.tally[5] = map[int]int{1: 7}
	a.router.protect[5] = map[int]int{1: 9}
	a.router.seenAt[[2]int{5, 1}] = time.Now()
	a.router.pend = []routeChange{{5, 1, 7}}
	a.router.pendProt = []routeChange{{5, 1, 9}}
	a.router.newSrc, a.router.newDst = map[int]string{1: "x"}, map[int]string{2: "y"}
	a.router.namesAt = time.Now()
	a.ext.add(extChange{xp: swp08.Crosspoint{Dst: 5, Level: 0, Src: 7}, ui: 1, old: 3, known: true, at: time.Now()})
	a.ext.own[ownKey{5, 0, 7}] = until
	a.ext.late[ownKey{5, 0, 9}] = until
	c := &cause{src: 7, dst: 5, id: 1, done: true}
	a.ext.causes = append(a.ext.causes, c)
	a.ext.leaders[6] = 5
	a.ext.lateTake[ownKey{5, 0, 8}] = []lateTake{{id: 1, until: until, cause: c}}
	a.ext.lateHits = append(a.ext.lateHits, lateHit{1, 1})
}

// noZeroFields fails for any field of the struct v points to that is still its
// zero value, so a field added to routerState or extTracker must be filled by
// fillRouterDerived (and so is checked to be cleared).
func noZeroFields(t *testing.T, v any) {
	t.Helper()
	rv := reflect.ValueOf(v).Elem()
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).IsZero() {
			t.Errorf("%s.%s is zero after fillRouterDerived: fill it in the test", rv.Type().Name(), rv.Type().Field(i).Name)
		}
	}
}

func TestRouterChangeClearsRouterState(t *testing.T) {
	a := newTestApp(t)
	// Kept state.
	cfg := a.r().cfg
	cl := &client{ip: "10.0.0.9", send: make(chan []byte, 16)}
	if err := a.SaveSettings(cl, cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	if err := a.Unlock(cl, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveFilterSets(cl, []config.FilterSet{{Name: "mine", Src: "CAM"}}); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.clients[cl] = struct{}{}
	a.mu.Unlock()
	a.record(&Entry{Src: 1, Dst: 5, OK: true})
	nameReq, nameRx, dump := time.Now().Add(-time.Hour), time.Now().Add(-2*time.Hour), time.Now().Add(-3*time.Hour)
	a.mu.Lock()
	a.nameReqAt, a.nameRxAt, a.dumpAt, a.namesDirt = nameReq, nameRx, dump, true
	a.mu.Unlock()
	fillRouterDerived(a)
	a.mu.Lock()
	noZeroFields(t, a.router)
	noZeroFields(t, a.ext)
	w, ck := &waiter{done: make(chan struct{})}, &tallyCheck{done: make(chan struct{})}
	a.waiters[w], a.checks[ck] = struct{}{}, struct{}{}
	a.pendSent = []Sent{{}}
	a.mu.Unlock()

	// Not router-side: nothing is cleared.
	cur := a.r().cfg
	cur.DeviceDefaults.Theme = "dark"
	cur.NameRefreshSeconds++
	rs, ext := a.router, a.ext
	if err := a.ApplyConfig(cur); err != nil {
		t.Fatal(err)
	}
	if a.router != rs || a.ext != ext {
		t.Fatal("a settings change that is not router-side replaced router state")
	}
	a.mu.Lock()
	if len(a.router.tally) != 1 || len(a.router.srcNames) != 1 || len(a.ext.pending) != 1 || len(a.ext.own) != 1 ||
		len(a.ext.lateTake) != 1 || len(a.ext.late) != 1 || len(a.waiters) != 1 || len(a.checks) != 1 || a.router.newSrc == nil || a.status.Addr != cur.Router {
		a.mu.Unlock()
		t.Fatal("a settings change that is not router-side cleared state")
	}
	a.mu.Unlock()

	// Router-side: every router-derived field is back as New made it.
	cur.Router = "127.0.0.1:1"
	if err := a.ApplyConfig(cur); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !reflect.DeepEqual(a.router, newRouterState()) {
		t.Fatalf("router state not cleared: %+v", a.router)
	}
	if !reflect.DeepEqual(a.ext, newExtTracker()) {
		t.Fatalf("ext state not cleared: %+v", a.ext)
	}
	if a.status.Router != string(swp08.StateConnecting) || a.status.Addr != "127.0.0.1:1" || a.status.Err != "" {
		t.Fatalf("status %+v", a.status)
	}
	// Takes and checks in flight are cut: gone, and their waits released.
	if len(a.waiters) != 0 || len(a.checks) != 0 || !w.cut || !w.closed || !ck.shut {
		t.Fatalf("takes not cut: %d waiters, %d checks, cut=%v", len(a.waiters), len(a.checks), w.cut)
	}
	select {
	case <-w.done:
	default:
		t.Fatal("cut waiter's done still open")
	}
	select {
	case <-ck.done:
	default:
		t.Fatal("cut check's done still open")
	}
	// Kept (see the field comments on App).
	if a.hist.Len() != 1 {
		t.Fatalf("history has %d entries", a.hist.Len())
	}
	if _, ok := a.clients[cl]; !ok {
		t.Fatal("client dropped")
	}
	if len(a.pendSent) != 1 {
		t.Fatal("pendSent cleared")
	}
	if !a.nameReqAt.Equal(nameReq) || !a.nameRxAt.Equal(nameRx) || !a.dumpAt.Equal(dump) || !a.namesDirt {
		t.Fatal("name/dump clock changed")
	}
	got := a.r().cfg
	if len(got.FilterSets) != 1 || got.FilterSets[0].Name != "mine" || got.SettingsPin == "" || got.DeviceDefaults.Theme != "dark" {
		t.Fatalf("config not kept: %+v", got)
	}
	if cl.pin != got.SettingsPin {
		t.Fatal("unlock lost")
	}
}
