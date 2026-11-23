package app

import (
	"slices"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// protDump is one 0x94 for one wire level of dsts 0..len(holds)-1; a zero hold is a free level.
func protDump(level int, holds ...protHold) []byte {
	msg := []byte{swp08.CmdExtProtectDump, 0, byte(level), byte(len(holds)), 0, 0}
	for _, h := range holds {
		hi, lo := swp08.ProtectWord(h.kind, h.dev)
		msg = append(msg, hi, lo)
	}
	return msg
}

// dumpAll sends one dump for each of the 3 wire levels with the same holds.
func dumpAll(a *App, holds ...protHold) {
	for l := 0; l < 3; l++ {
		a.onUnitMsg(a.first, protDump(l, holds...))
	}
}

// endProtBurst makes the pending burst old enough to be written, and writes it.
func endProtBurst(a *App) []Entry {
	a.mu.Lock()
	a.first.router.protAt = [2]time.Time{time.Now().Add(-time.Minute), time.Now().Add(-time.Second)}
	a.mu.Unlock()
	before := a.hist.Len()
	a.flushProtExt()
	es, _ := a.hist.Page(-1, a.hist.Len()-before)
	slices.Reverse(es)
	if a.hist.Len() == before {
		return nil
	}
	return es
}

func extProt(t *testing.T, e Entry, protect, kind string, dev int, found bool, dst int, levels ...int) {
	t.Helper()
	if !e.Ext || e.Protect != protect || e.HoldKind != kind || e.Device != dev || e.Found != found || e.Dst != dst ||
		!slices.Equal(e.Levels, levels) || e.Src != -1 {
		t.Fatalf("entry %+v, want ext %s %s device %d found %v dst %d levels %v", e, protect, kind, dev, found, dst, levels)
	}
}

// The first state the router reports is the baseline; a later re-read that differs is Found, and the same re-read
// again adds nothing.
func TestProtExtFirstDumpIsTheBaseline(t *testing.T) {
	a, _ := protectApp(t)
	lock := protHold{dev: 9, kind: swp08.KindRouterLock}
	dumpAll(a, protHold{}, protHold{}, lock)
	if es := endProtBurst(a); es != nil {
		t.Fatalf("the first dump wrote %+v", es)
	}
	prot := protHold{dev: 112, kind: swp08.KindProtect}
	dumpAll(a, protHold{}, prot, protHold{})
	es := endProtBurst(a)
	if len(es) != 2 {
		t.Fatalf("entries %+v", es)
	}
	slices.SortFunc(es, func(x, y Entry) int { return x.Dst - y.Dst })
	extProt(t, es[0], "set", "protect", 112, true, 1, 1, 2, 3)
	extProt(t, es[1], "clear", "routerLock", 9, true, 2, 1, 2, 3)
	dumpAll(a, protHold{}, prot, protHold{})
	if es := endProtBurst(a); es != nil {
		t.Fatalf("the same dump again wrote %+v", es)
	}
}

// A 0x8D or 0x8F from another device is a change even before a dump, and not Found. Levels gather into one entry;
// two devices on one destination get one entry each.
func TestProtExtAnnouncedChanges(t *testing.T) {
	a, _ := protectApp(t)
	for l := 0; l < 3; l++ {
		a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 0, l, swp08.KindProtect, 5, 40))
	}
	a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectConnected, 0, 0, swp08.KindProtect, 5, 41))
	es := endProtBurst(a)
	if len(es) != 3 {
		t.Fatalf("entries %+v", es)
	}
	// Level 1 went from device 40 to 41: the one that went comes first.
	extProt(t, es[0], "set", "protect", 40, false, 5, 1, 2, 3)
	extProt(t, es[1], "clear", "protect", 40, false, 5, 1)
	extProt(t, es[2], "set", "protect", 41, false, 5, 1)
	for l := 0; l < 3; l++ {
		a.onUnitMsg(a.first, swp08.ExtProtectChange(swp08.CmdExtProtectDisconnected, 0, l, 0, 5, 0))
	}
	es = endProtBurst(a)
	if len(es) != 2 {
		t.Fatalf("entries %+v", es)
	}
	slices.SortFunc(es, func(x, y Entry) int { return x.Device - y.Device })
	extProt(t, es[0], "clear", "protect", 40, false, 5, 2, 3)
	extProt(t, es[1], "clear", "protect", 41, false, 5, 1)
}

// Sode's own Protect and Free Protect have their own entries, so they add no External one. Another system that
// frees Sode's Protect gets one.
func TestProtExtSkipsSodesOwn(t *testing.T) {
	a, f := protectApp(t)
	dumpAll(a, make([]protHold, 8)...)
	ana := signedClient(t, a, "Ana", "1234")
	protEntry(t, a.protectSet(ana, 4, nil))
	protEntry(t, a.protectClear(ana, 4, nil, nil))
	if es := endProtBurst(a); es != nil {
		t.Fatalf("Sode's own Protect wrote %+v", es)
	}
	protEntry(t, a.protectSet(ana, 4, nil))
	// A re-read that shows Sode's Protect adds nothing either.
	dumpAll(a, protHold{}, protHold{}, protHold{}, protHold{}, protHold{dev: 126, kind: swp08.KindProtect})
	if es := endProtBurst(a); es != nil {
		t.Fatalf("a re-read of Sode's Protect wrote %+v", es)
	}
	f.mu.Lock()
	clear(f.prot.holds)
	f.mu.Unlock()
	dumpAll(a, make([]protHold, 5)...)
	es := endProtBurst(a)
	if len(es) != 1 {
		t.Fatalf("entries %+v", es)
	}
	extProt(t, es[0], "clear", "protect", 126, true, 4, 1, 2, 3)
	if e := es[0]; e.DstName != "MON 4" {
		t.Fatalf("dstName %q", e.DstName)
	}
}

// A router change starts a new baseline.
func TestProtExtRouterChangeIsANewBaseline(t *testing.T) {
	a, _ := protectApp(t)
	dumpAll(a, protHold{})
	a.mu.Lock()
	a.first.router = newRouterState()
	a.mu.Unlock()
	dumpAll(a, protHold{dev: 3, kind: swp08.KindRouterLock})
	if es := endProtBurst(a); es != nil {
		t.Fatalf("the first dump after a router change wrote %+v", es)
	}
}
