package app

import (
	"reflect"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

func fillRouterDerived(a *App) {
	until := time.Now().Add(time.Minute)
	a.mu.Lock()
	defer a.mu.Unlock()
	a.firstUnit().router.srcNames[7] = "CAM 7"
	a.firstUnit().router.dstNames[5] = "MON 5"
	a.firstUnit().router.tally[5] = map[int]int{1: 7}
	a.firstUnit().router.tallyGen = 1
	a.firstUnit().router.namesGen = 1
	a.firstUnit().router.protect[5] = map[int]protHold{1: {9, 1}}
	a.firstUnit().router.protSeen[[2]int{5, 1}] = true
	a.firstUnit().router.protExt[protExtKey{dst: 5, dev: 9, kind: 1, set: true}] = &protExtLevels{levels: map[int]bool{1: true}, first: time.Now()}
	a.firstUnit().router.protAt = [2]time.Time{time.Now(), time.Now()}
	a.firstUnit().router.seenAt[[2]int{5, 1}] = time.Now()
	a.firstUnit().router.pend = []routeChange{{5, 1, 7}}
	a.firstUnit().router.pendProt = []protChange{{5, 1, 9, 1}}
	a.firstUnit().router.newSrc, a.firstUnit().router.newDst = map[int]string{1: "x"}, map[int]string{2: "y"}
	a.firstUnit().router.namesAt = time.Now()
	a.firstUnit().router.savedAt = time.Now()
	a.firstUnit().router.lastPoll = &namePoll{map[int]string{7: "CAM 7"}, map[int]string{5: "MON 5"}}
	a.firstUnit().router.faultSeen = true
	a.firstUnit().router.renamedTo = [2]map[string]string{{"cam 7": "CAM C"}, {"mon 5": "MON E"}}
	a.firstUnit().ext.add(extChange{xp: swp08.Crosspoint{Dst: 5, Level: 0, Src: 7}, ui: 1, old: 3, known: true, at: time.Now()})
	a.firstUnit().ext.own[ownKey{5, 0, 7}] = until
	a.firstUnit().ext.late[ownKey{5, 0, 9}] = until
	c := &cause{src: 7, dst: 5, id: 1, done: true}
	a.firstUnit().ext.causes = append(a.firstUnit().ext.causes, c)
	a.firstUnit().ext.leaders[6] = 5
	a.firstUnit().ext.lateRoute[ownKey{5, 0, 8}] = []lateRoute{{id: 1, until: until, cause: c}}
	a.firstUnit().ext.lateHits = append(a.firstUnit().ext.lateHits, lateHit{1, 1})
}

// noZeroFields fails for any field of the struct v points to that is still its
// zero value. So a field added to routerState or extTracker must be filled by
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
	cfg := a.r().cfg
	cl := &client{ip: "10.0.0.9", send: make(chan []byte, 16)}
	if err := a.SaveSettings(cl, cfg); err != nil {
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
	a.firstUnit().nameReqAt, a.firstUnit().nameRxAt, a.firstUnit().dumpAt, a.firstUnit().namesDirt = nameReq, nameRx, dump, true
	a.mu.Unlock()
	fillRouterDerived(a)
	a.mu.Lock()
	noZeroFields(t, a.firstUnit().router)
	noZeroFields(t, a.firstUnit().ext)
	w, ck := &waiter{done: make(chan struct{})}, &tallyCheck{done: make(chan struct{})}
	a.firstUnit().waiters[w], a.firstUnit().checks[ck] = struct{}{}, struct{}{}
	a.pendSent = []Sent{{}}
	a.mu.Unlock()

	cur := a.r().cfg.Clone()
	cur.DeviceDefaults.Theme = "dark"
	cur.Routers[0].NameRefreshSeconds++
	rs, ext := a.firstUnit().router, a.firstUnit().ext
	if err := a.ApplyConfig(cur); err != nil {
		t.Fatal(err)
	}
	if a.firstUnit().router != rs || a.firstUnit().ext != ext {
		t.Fatal("a settings change that is not router-side replaced router state")
	}
	a.mu.Lock()
	if len(a.firstUnit().router.tally) != 1 || len(a.firstUnit().router.srcNames) != 1 || len(a.firstUnit().ext.pending) != 1 || len(a.firstUnit().ext.own) != 1 ||
		len(a.firstUnit().ext.lateRoute) != 1 || len(a.firstUnit().ext.late) != 1 || len(a.firstUnit().waiters) != 1 || len(a.firstUnit().checks) != 1 || a.firstUnit().router.newSrc == nil || a.firstUnit().status.Addr != cur.Routers[0].Address {
		a.mu.Unlock()
		t.Fatal("a settings change that is not router-side cleared state")
	}
	a.mu.Unlock()

	cur.Routers[0].Address = "127.0.0.1:1"
	if err := a.ApplyConfig(cur); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if !reflect.DeepEqual(a.firstUnit().router, newRouterState()) {
		t.Fatalf("router state not cleared: %+v", a.firstUnit().router)
	}
	if !reflect.DeepEqual(a.firstUnit().ext, newExtTracker()) {
		t.Fatalf("ext state not cleared: %+v", a.firstUnit().ext)
	}
	if a.firstUnit().status.Router != string(swp08.StateConnecting) || a.firstUnit().status.Addr != "127.0.0.1:1" || a.firstUnit().status.Err != "" {
		t.Fatalf("status %+v", a.firstUnit().status)
	}
	if len(a.firstUnit().waiters) != 0 || len(a.firstUnit().checks) != 0 || !w.cut || !w.closed || !ck.shut {
		t.Fatalf("takes not cut: %d waiters, %d checks, cut=%v", len(a.firstUnit().waiters), len(a.firstUnit().checks), w.cut)
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
	if a.hist.Len() != 1 {
		t.Fatalf("history has %d entries", a.hist.Len())
	}
	if _, ok := a.clients[cl]; !ok {
		t.Fatal("client dropped")
	}
	if len(a.pendSent) != 1 {
		t.Fatal("pendSent cleared")
	}
	if !a.firstUnit().nameReqAt.Equal(nameReq) || !a.firstUnit().nameRxAt.Equal(nameRx) || !a.firstUnit().dumpAt.Equal(dump) || !a.firstUnit().namesDirt {
		t.Fatal("name/dump clock changed")
	}
	got := a.r().cfg
	if len(got.FilterSets) != 1 || got.FilterSets[0].Name != "mine" || got.DeviceDefaults.Theme != "dark" {
		t.Fatalf("config not kept: %+v", got)
	}
}
