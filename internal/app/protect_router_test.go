package app

import (
	"slices"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// protectTwoRouters: protectApp (device 126 on the first router) with a second router, B, up, with device 127.
func protectTwoRouters(t *testing.T) (a *App, first, second *fakeRouter, u *routerUnit) {
	t.Helper()
	a, first = protectApp(t)
	cfg := a.r().cfg.Clone()
	b := moreRouter("B", "Second", "10.0.0.9:8910", 3)
	b.ProtectDevice = 127
	cfg.Routers = append(cfg.Routers[:1:1], []config.Router{b}...)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	second, u = addFakeUnit(t, a, a.r().cfg.Routers[1])
	a.mu.Lock()
	a.first.status.Router = string(swp08.StateUp)
	u.status.Router = string(swp08.StateUp)
	for d := 0; d < 8; d++ {
		a.first.router.dstNames[d] = "A MON " + string(rune('0'+d))
		u.router.dstNames[d] = "B MON " + string(rune('0'+d))
	}
	a.mu.Unlock()
	return a, first, second, u
}

// A Protect names its router: the messages, the device, the waits, the record and the History entry are that
// router's. The same destination number on the other router is a different destination.
func TestProtectUsesItsRouter(t *testing.T) {
	a, first, second, u := protectTwoRouters(t)
	ana := signedClient(t, a, "Ana", "1234")

	onB := protEntry(t, a.protectMany(ana, []protectDst{{Dst: 4, Router: u.uid()}}, true, nil)[0])
	if !onB.OK || onB.Router != u.uid() || onB.RouterTitle != "Second" || onB.DstName != "B MON 4" {
		t.Fatalf("entry on B %+v", onB)
	}
	if n := first.sends(); n != 0 {
		t.Fatalf("the first router got %d messages", n)
	}
	if hs := second.holdsOn(4); len(hs) != 3 || hs[0].dev != 127 {
		t.Fatalf("holds on B %+v", hs)
	}
	onA := protEntry(t, a.protectSet(ana, 4, nil))
	if !onA.OK || onA.Router != a.r().cfg.Routers[0].UID || onA.DstName != "A MON 4" {
		t.Fatalf("entry on A %+v", onA)
	}
	if hs := first.holdsOn(4); len(hs) != 3 || hs[0].dev != 126 {
		t.Fatalf("holds on A %+v", hs)
	}

	// The stored record names the router; a fresh load reads both records apart.
	byRouter := map[string]protectRec{}
	for _, rec := range readProtects(t, a) {
		byRouter[rec.Router] = rec
	}
	if rec := byRouter[u.uid()]; rec.Dst != 4 || rec.Device != 127 || len(rec.Levels) != 3 || rec.By != "Ana" {
		t.Fatalf("record of B %+v", rec)
	}
	if rec := byRouter[a.r().cfg.Routers[0].UID]; rec.Dst != 4 || rec.Device != 126 {
		t.Fatalf("record of A %+v", rec)
	}
	loaded := loadProtects(a.cfgPath)
	if rec, ok := loaded.get(u.uid(), 4); !ok || rec.Router != u.uid() || rec.Device != 127 {
		t.Fatalf("loaded record of B %+v %v", rec, ok)
	}

	// Sode's own Protects are not Externals on either router.
	for _, x := range []*routerUnit{a.first, u} {
		a.mu.Lock()
		x.router.protAt = [2]time.Time{time.Now().Add(-time.Minute), time.Now().Add(-time.Second)}
		a.mu.Unlock()
	}
	a.flushProtExt()
	if n := a.hist.Len(); n != 2 {
		es, _ := a.hist.Page(-1, n)
		t.Fatalf("entries %+v", es)
	}

	// An Undo names the router of its entry.
	protRefused(t, a.protectMany(ana, []protectDst{{Dst: 4, Router: u.uid(), Undo: &onA.ID}}, false, nil)[0], "bad_request", "not a Protect to undo")
	free := protEntry(t, a.protectMany(ana, []protectDst{{Dst: 4, Router: u.uid(), Undo: &onB.ID}}, false, nil)[0])
	if !free.OK || free.Router != u.uid() || len(second.holdsOn(4)) != 0 || len(first.holdsOn(4)) != 3 {
		t.Fatalf("free on B %+v, B holds %v, A holds %v", free, second.holdsOn(4), first.holdsOn(4))
	}
	if _, ok := a.protects.get(u.uid(), 4); ok {
		t.Error("the record of B stays")
	}
	if _, ok := a.protects.get(a.r().cfg.Routers[0].UID, 4); !ok {
		t.Error("the record of A went")
	}

	protRefused(t, a.protectMany(ana, []protectDst{{Dst: 4, Router: "b0b0b0b0"}}, true, nil)[0], "not_found", "no such router")
}

// A level that router B does not confirm rolls back only on B.
func TestProtectRollbackOnItsRouter(t *testing.T) {
	a, first, second, u := protectTwoRouters(t)
	ana := signedClient(t, a, "Ana", "1234")
	second.mu.Lock()
	second.prot.deafSet[a.r().byUI[2].Wire] = true
	second.mu.Unlock()
	e := protEntry(t, a.protectMany(ana, []protectDst{{Dst: 5, Router: u.uid()}}, true, nil)[0])
	if e.OK || len(e.Failed) != 1 || e.Failed[0] != 2 || len(e.Still) != 0 || e.Router != u.uid() {
		t.Fatalf("entry %+v", e)
	}
	if len(second.holdsOn(5)) != 0 || first.sends() != 0 {
		t.Fatalf("B holds %v, the first router got %d messages", second.holdsOn(5), first.sends())
	}
}

// Free every Sode Protect frees them on each router that is up.
func TestClearSodeProtectsOnEveryRouter(t *testing.T) {
	a, first, second, u := protectTwoRouters(t)
	ana := signedClient(t, a, "Ana", "1234")
	protEntry(t, a.protectSet(ana, 1, nil))
	protEntry(t, a.protectMany(ana, []protectDst{{Dst: 2, Router: u.uid()}}, true, nil)[0])
	if n, still, _, err := a.clearSodeProtects(adminClient(t, a), nil); n != 2 || still != nil || err != nil {
		t.Fatalf("cleared %d, still %v, err %v", n, still, err)
	}
	if len(first.holdsOn(1)) != 0 || len(second.holdsOn(2)) != 0 {
		t.Fatalf("A holds %v, B holds %v", first.holdsOn(1), second.holdsOn(2))
	}
	es, _ := a.hist.Page(-1, 2)
	if es[0].Group == "" || es[0].Group != es[1].Group || es[0].Router == es[1].Router {
		t.Fatalf("entries %+v", es)
	}

	// With router B down, it frees what it can on A and leaves B.
	protEntry(t, a.protectSet(ana, 1, nil))
	protEntry(t, a.protectMany(ana, []protectDst{{Dst: 2, Router: u.uid()}}, true, nil)[0])
	a.mu.Lock()
	u.status.Router = string(swp08.StateDown)
	a.mu.Unlock()
	n, _, per, err := a.clearSodeProtects(adminClient(t, a), nil)
	if n != 1 || err != nil {
		t.Fatalf("cleared %d, err %v", n, err)
	}
	if len(second.holdsOn(2)) != 3 {
		t.Fatalf("B holds %v", second.holdsOn(2))
	}
	want := []protectClearRouter{{Router: a.firstUnit().uid(), Cleared: 1}, {Router: u.uid(), Err: "the router is down"}}
	if !slices.Equal(per, want) {
		t.Fatalf("per router %+v, want %+v", per, want)
	}
}

// The hello gives a router that is not up the time its link went down, the age of its Protect rows. An up router has
// none.
func TestHelloGivesProtectAtOfALostRouter(t *testing.T) {
	a, _, _, u := protectTwoRouters(t)
	before := time.Now()
	a.onUnitState(u, swp08.StateDown, nil)
	a.mu.Lock()
	rs := a.helloRouters(a.r())
	a.mu.Unlock()
	if !rs[0].ProtectAt.IsZero() {
		t.Fatalf("the up router has protectAt %v", rs[0].ProtectAt)
	}
	if rs[1].ProtectAt.Before(before) {
		t.Fatalf("the lost router has protectAt %v, want after %v", rs[1].ProtectAt, before)
	}
}

// Free Sode's Protects with routers acts only on those routers, and names one it does not know.
func TestClearSodeProtectsOnPickedRouters(t *testing.T) {
	a, first, second, u := protectTwoRouters(t)
	ana := signedClient(t, a, "Ana", "1234")
	protEntry(t, a.protectSet(ana, 1, nil))
	protEntry(t, a.protectMany(ana, []protectDst{{Dst: 2, Router: u.uid()}}, true, nil)[0])
	n, _, per, err := a.clearSodeProtects(adminClient(t, a), []string{u.uid(), "nope"})
	if n != 1 || err != nil {
		t.Fatalf("cleared %d, err %v", n, err)
	}
	if len(first.holdsOn(1)) != 3 || len(second.holdsOn(2)) != 0 {
		t.Fatalf("A holds %v, B holds %v", first.holdsOn(1), second.holdsOn(2))
	}
	want := []protectClearRouter{{Router: u.uid(), Cleared: 1}, {Router: "nope", Err: "no such router"}}
	if !slices.Equal(per, want) {
		t.Fatalf("per router %+v, want %+v", per, want)
	}
	if _, _, _, err := a.clearSodeProtects(adminClient(t, a), []string{"nope"}); err == nil {
		t.Fatal("no router to act on, and no error")
	}
}

// A Protect that another system sets on router B is an External on B.
func TestProtExtUsesItsRouter(t *testing.T) {
	a, _, _, u := protectTwoRouters(t)
	wire := a.r().byUI[1].Wire
	a.onUnitMsg(u, swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 0, wire, 1, 3, 112))
	for _, x := range []*routerUnit{a.first, u} {
		a.mu.Lock()
		x.router.protAt = [2]time.Time{time.Now().Add(-time.Minute), time.Now().Add(-time.Second)}
		a.mu.Unlock()
	}
	a.flushProtExt()
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 1 || !es[0].Ext || es[0].Router != u.uid() || es[0].Device != 112 || es[0].DstName != "B MON 3" {
		t.Fatalf("entries %+v", es)
	}
}

// An External on router B is written with B's names and names B. A take on the first router never leads a change on
// B: an External follow never crosses routers.
func TestExtUsesItsRouter(t *testing.T) {
	a, _, _, u := secondRouter(t)
	for _, d := range []int{5, 9} {
		a.onUnitMsg(a.first, swp08.ExtTally(0, 0, d, 1))
		a.onUnitMsg(u, swp08.ExtTally(0, 0, d, 1))
	}
	a.mu.Lock()
	u.router.dstNames[9] = "B MON 9"
	now := time.Now()
	a.first.ext.causes = append(a.first.ext.causes, &cause{src: 7, dst: 5, first: now, last: now, id: 42, done: true})
	a.mu.Unlock()
	a.onUnitMsg(u, swp08.ExtConnected(0, 0, 9, 7))
	a.mu.Lock()
	pendA := len(a.first.ext.pending)
	u.ext.last = time.Now().Add(-time.Second)
	a.mu.Unlock()
	if pendA != 0 {
		t.Fatalf("the first router has %d pending", pendA)
	}
	a.flushExt()
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 1 || es[0].Router != u.uid() || es[0].RouterTitle != "Second" || es[0].DstName != "B MON 9" || es[0].Follow != nil {
		t.Fatalf("entries %+v", es)
	}

	// The same change on the first router follows the take.
	a.onUnitMsg(a.first, swp08.ExtConnected(0, 0, 9, 7))
	endBurst(a)
	es, _ = a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Router != a.r().cfg.Routers[0].UID || es[0].Follow == nil || *es[0].Follow != 42 {
		t.Fatalf("entry on A %+v", es)
	}
}
