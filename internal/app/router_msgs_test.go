package app

import (
	"maps"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// namePages feeds names as separate pages of two, the way a router splits a long list.
func namePages(a *App, dest bool, names []string) {
	r := a.r()
	for first := 0; first < len(names); first += 2 {
		end := min(first+2, len(names))
		a.onUnitMsg(a.first, swp08.NamesResp(dest, r.cfg.Routers[0].NameRequests == "ext", r.cfg.Routers[0].Matrix, 0, r.levelsOf("").lenCode, first, names[first:end]))
	}
}

func TestNamePagesFillAppAndTake(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(2, 1, 0, 1, 2)
	namePages(a, false, []string{"CAM 0", "CAM 1", "CAM 2"})
	namePages(a, true, []string{"MON 0", "MON 1", "MON 2"})
	a.mu.Lock()
	src, dst := maps.Clone(a.first.router.srcNames), maps.Clone(a.first.router.dstNames)
	a.mu.Unlock()
	if !maps.Equal(src, map[int]string{0: "CAM 0", 1: "CAM 1", 2: "CAM 2"}) || !maps.Equal(dst, map[int]string{0: "MON 0", 1: "MON 1", 2: "MON 2"}) {
		t.Fatalf("names src %v dst %v", src, dst)
	}
	e := routeOne(t, a, 1, 2, 1)
	if e.SrcName != "CAM 1" || e.DstName != "MON 2" {
		t.Fatalf("take recorded %q -> %q", e.SrcName, e.DstName)
	}
}

func TestNameRefreshSwapsAfterQuiet(t *testing.T) {
	a := newTestApp(t)
	newFakeRouter(t, a)
	namePages(a, false, []string{"OLD 0", "OLD 1"})
	a.RefreshNames()
	namePages(a, false, []string{"NEW 0", "NEW 1"})
	a.mu.Lock()
	if a.first.router.srcNames[0] != "OLD 0" {
		t.Fatalf("names replaced before the refresh ended: %v", a.first.router.srcNames)
	}
	a.first.nameReqAt, a.first.nameRxAt = time.Now().Add(-2*time.Second), time.Now().Add(-time.Second) // quiet long enough
	a.mu.Unlock()
	a.flush()
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.first.router.srcNames[0] != "NEW 0" || a.first.router.srcNames[1] != "NEW 1" || a.first.router.newSrc != nil {
		t.Fatalf("after refresh: %v (pending %v)", a.first.router.srcNames, a.first.router.newSrc)
	}
}

func TestProtectDumpRefusesTake(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	// Dsts 5-6 on level 1 on the wire (UI 2): 5 is protected by device 9, 6 is not.
	a.onUnitMsg(a.first, []byte{swp08.CmdExtProtectDump, 0x00, 0x01, 0x02, 0x00, 0x05, 0x20, 0x09, 0x00, 0x00})
	a.mu.Lock()
	got := maps.Clone(a.first.router.protect[5])
	_, other := a.first.router.protect[6]
	a.mu.Unlock()
	if !maps.Equal(got, map[int]protHold{2: {9, swp08.KindRouterLock}}) || other {
		t.Fatalf("protect state %v, dst 6 protected %v", got, other)
	}
	n := f.sends()
	e := routeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || !strings.Contains(e.Err, "protected on levels 2") {
		t.Fatalf("take %+v", e)
	}
	if f.sends() != n {
		t.Fatal("sent to a protected destination")
	}
	// The router later reports it unprotected: the same take goes through.
	a.onUnitMsg(a.first, []byte{swp08.CmdExtProtectDump, 0x00, 0x01, 0x01, 0x00, 0x05, 0x00, 0x00})
	if e := routeOne(t, a, 7, 5, 1, 2, 3); !e.OK {
		t.Fatalf("take after unprotect %+v", e)
	}
}

// One DashBoard press on a Ross Ultrix protects all 17 levels with 17 0x8D frames, one for each level. The
// next press clears them with 17 0x8F frames. Real frames: dst 27, device 10.
func TestDashBoardProtectAndClearAllLevels(t *testing.T) {
	a := newTestApp(t, func(c *config.Config) { c.Routers[0].Levels = 17 })
	protect := func() map[int]protHold {
		a.mu.Lock()
		defer a.mu.Unlock()
		return maps.Clone(a.first.router.protect[27])
	}
	want := map[int]protHold{}
	for ll := byte(0); ll <= 0x10; ll++ {
		a.onUnitMsg(a.first, []byte{0x8D, 0x00, ll, 0x02, 0x00, 0x1B, 0x00, 0x0A})
		want[int(ll)+1] = protHold{10, swp08.KindRouterLock}
		if got := protect(); !maps.Equal(got, want) {
			t.Fatalf("after 0x8D level %d: %v, want %v", ll, got, want)
		}
	}
	for ll := byte(0); ll <= 0x10; ll++ {
		a.onUnitMsg(a.first, []byte{0x8F, 0x00, ll, 0x00, 0x00, 0x1B, 0x00, 0x00})
		delete(want, int(ll)+1)
		if got := protect(); !maps.Equal(got, want) {
			t.Fatalf("after 0x8F level %d: %v, want %v", ll, got, want)
		}
	}
}

func TestTallyDumpSetsCrosspoints(t *testing.T) {
	a := newTestApp(t)
	// Level 1 on the wire (UI 2), dsts 3-5; 0xFFFF is "nothing routed".
	a.onUnitMsg(a.first, swp08.ExtTallyDump(0, 1, 3, []int{7, 0xFFFF, 9}))
	// Another matrix is ignored.
	a.onUnitMsg(a.first, swp08.ExtTallyDump(1, 1, 8, []int{4}))
	a.mu.Lock()
	for dst, want := range map[int]int{3: 7, 4: -1, 5: 9} {
		if got, ok := a.first.router.tally[dst][2]; !ok || got != want {
			t.Errorf("dst %d level 2: %d (known %v), want %d", dst, got, ok, want)
		}
	}
	if _, ok := a.first.router.tally[3][1]; ok {
		t.Error("dump for level 2 set level 1")
	}
	if len(a.first.router.pend) != 3 {
		t.Errorf("pending changes %v", a.first.router.pend)
	}
	if _, ok := a.first.router.tally[8]; ok {
		t.Error("dump for another matrix applied")
	}
	a.mu.Unlock()
	// A dump carries no command set, so a route this app never saw is no ext change.
	endBurst(a)
	if n := a.hist.Len(); n != 0 {
		t.Errorf("dump of unseen routes recorded %d entries", n)
	}
}

func TestTallyDumpDifferingFromKnownRouteIsFound(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(3, 1, 1) // dst 3 on level 1 on the wire (UI 2) routed from source 1
	a.onUnitMsg(a.first, swp08.ExtTallyDump(0, 1, 3, []int{7}))
	endBurst(a)
	es, _ := a.hist.Page(-1, 10)
	if len(es) != 1 || !es[0].Found || !es[0].Ext || es[0].Dst != 3 {
		t.Fatalf("entries %+v, want one found entry for dst 3", es)
	}
	leftovers(t, a)
}

type recorder struct {
	mu   sync.Mutex
	cmds []byte
	why  []swp08.Reason
}

func (r *recorder) reasons(cmd byte) []swp08.Reason {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []swp08.Reason
	for i, c := range r.cmds {
		if c == cmd {
			out = append(out, r.why[i])
		}
	}
	return out
}

func (r *recorder) wantReason(t *testing.T, cmd byte, want swp08.Reason) {
	t.Helper()
	for _, got := range r.reasons(cmd) {
		if got != want {
			t.Errorf("cmd 0x%02X sent with reason %q, want %q", cmd, got, want)
		}
	}
}

func (r *recorder) count(cmd byte) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, c := range r.cmds {
		if c == cmd {
			n++
		}
	}
	return n
}

func (r *recorder) total() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.cmds)
}

func (r *recorder) reset() {
	r.mu.Lock()
	r.cmds, r.why = nil, nil
	r.mu.Unlock()
}

func newRecorder(a *App) *recorder {
	rec := &recorder{}
	a.first.sendMsg = func(m []byte, why swp08.Why) error {
		rec.mu.Lock()
		rec.cmds = append(rec.cmds, m[0])
		rec.why = append(rec.why, why.Reason)
		rec.mu.Unlock()
		if why.Done != nil {
			close(why.Done)
		}
		return nil
	}
	return rec
}

func nameReqCmds(a *App) (src, dst byte) {
	if a.r().cfg.Routers[0].NameRequests == "ext" {
		return swp08.CmdExtSrcNamesReq, swp08.CmdExtDstNamesReq
	}
	return swp08.CmdSrcNamesReq, swp08.CmdDstNamesReq
}

func TestRouterUpRequestsNamesAndDumps(t *testing.T) {
	a := newTestApp(t)
	rec := newRecorder(a)
	a.onUnitUp(a.first)
	srcReq, dstReq := nameReqCmds(a)
	n := len(a.r().levels)
	for cmd, want := range map[byte]int{srcReq: 1, dstReq: 1, swp08.CmdExtTallyDumpReq: n, swp08.CmdExtProtectDumpReq: n} {
		if got := rec.count(cmd); got != want {
			t.Errorf("cmd 0x%02X sent %d times, want %d", cmd, got, want)
		}
	}
	for _, cmd := range []byte{srcReq, dstReq} {
		rec.wantReason(t, cmd, swp08.ReasonNames)
	}
	for _, cmd := range []byte{swp08.CmdExtTallyDumpReq, swp08.CmdExtProtectDumpReq} {
		rec.wantReason(t, cmd, swp08.ReasonResync)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.first.router.newSrc == nil || a.first.router.newDst == nil || a.first.nameReqAt.IsZero() || a.first.dumpAt.IsZero() {
		t.Error("up did not start a name refresh and note the dump")
	}
}

func TestPeriodicRepollsWhenDue(t *testing.T) {
	a := newTestApp(t)
	rec := newRecorder(a)
	n := len(a.r().levels)

	a.mu.Lock()
	a.first.status.Router = string(swp08.StateDown)
	a.first.nameReqAt, a.first.dumpAt = time.Now().Add(-time.Hour), time.Now().Add(-time.Hour)
	a.mu.Unlock()
	a.periodic()
	if rec.total() != 0 {
		t.Fatalf("polled while the router is down: % X", rec.cmds)
	}

	a.mu.Lock()
	a.first.status.Router = string(swp08.StateUp)
	a.mu.Unlock()
	a.periodic()
	srcReq, dstReq := nameReqCmds(a)
	if rec.count(srcReq) != 1 || rec.count(dstReq) != 1 || rec.count(swp08.CmdExtTallyDumpReq) != n || rec.count(swp08.CmdExtProtectDumpReq) != n {
		t.Fatalf("due poll sent % X", rec.cmds)
	}
	rec.wantReason(t, srcReq, swp08.ReasonNames)
	rec.wantReason(t, dstReq, swp08.ReasonNames)
	rec.wantReason(t, swp08.CmdExtTallyDumpReq, swp08.ReasonResync)
	rec.wantReason(t, swp08.CmdExtProtectDumpReq, swp08.ReasonResync)

	rec.reset()
	a.periodic()
	if rec.total() != 0 {
		t.Fatalf("polled again before the interval: % X", rec.cmds)
	}
}
