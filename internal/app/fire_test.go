package app

import (
	"cmp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// fakeSalvo is the fake router's group salvo memory, like the Ultrix's: a
// staged list per group shared by every client, 122 for each 120, 123 for
// each 121, and no Connected after a Go. Guarded by fakeRouter.mu.
type fakeSalvo struct {
	off       bool // no group salvos: 120 and 121 are ignored
	loseStage bool // stages, but never answers with 122
	loseExt   bool // stages with 248, but never answers with 250
	noGoAck   bool // carries out a Go set, but never answers it
	wipe      bool // the group is emptied (someone else's clear) just before each Go set
	// announce: after a Go set, a Connected for each staged crosspoint that
	// changed (0 = none, as the Ultrix sometimes; 1 = all; 2 = every other
	// one), in destination and level order, 4 ms apart, from announceAfter
	// on (the Ultrix's: ~100 ms). pending is those still to come.
	announce      int
	announceAfter time.Duration
	pending       sync.WaitGroup
	staged        map[int][]swp08.Staged
	goes          int    // Go sets carried out
	mixed         bool   // a Go set crosspoints of more than one source
	seq           []byte // c clear, s stage, g go, in order
}

// salvoMsg handles 120 and 121. Caller holds f.mu.
func (f *fakeRouter) salvoMsg(m []byte) {
	s := &f.salvo
	if s.off {
		return
	}
	if s.staged == nil {
		s.staged = map[int][]swp08.Staged{}
	}
	if x, ok := swp08.StagedOf(m); ok {
		s.seq = append(s.seq, 's')
		s.staged[x.Group] = append(s.staged[x.Group], x)
		if !(s.loseStage || (s.loseExt && x.Ext)) {
			f.out <- swp08.StageAck(x)
		}
		return
	}
	g := int(m[2])
	if m[1] == 1 {
		s.seq = append(s.seq, 'c')
		st := swp08.GoNothing
		if len(s.staged[g]) > 0 {
			st = swp08.GoCleared
		}
		delete(s.staged, g)
		f.out <- swp08.GoDoneAck(st, g)
		return
	}
	s.seq = append(s.seq, 'g')
	if s.wipe {
		delete(s.staged, g)
	}
	st := swp08.GoNothing
	var tell []swp08.Staged
	if xs := s.staged[g]; len(xs) > 0 {
		st = swp08.GoSet
		s.goes++
		for _, x := range xs {
			k := [2]int{x.Dst, x.Level}
			if old, ok := f.tally[k]; !ok || old != x.Src {
				tell = append(tell, x)
			}
			f.tally[k] = x.Src // all at once
			if x.Src != xs[0].Src {
				s.mixed = true
			}
		}
	}
	f.announceGo(tell)
	delete(s.staged, g)
	if !s.noGoAck {
		f.out <- swp08.GoDoneAck(st, g)
	}
}

// announceGo sends the Connecteds of a Go set, per announce. Caller holds f.mu.
func (f *fakeRouter) announceGo(xs []swp08.Staged) {
	s := &f.salvo
	if s.announce == 0 || len(xs) == 0 {
		return
	}
	slices.SortFunc(xs, func(a, b swp08.Staged) int { return cmp.Or(a.Dst-b.Dst, a.Level-b.Level) })
	var tell []swp08.Staged
	for i, x := range xs {
		if s.announce == 1 || i%2 == 0 {
			tell = append(tell, x)
		}
	}
	after := s.announceAfter
	s.pending.Add(1)
	go func() {
		defer s.pending.Done()
		time.Sleep(after)
		for _, x := range tell {
			f.out <- swp08.ExtConnected(x.Matrix, x.Level, x.Dst, x.Src)
			time.Sleep(4 * time.Millisecond)
		}
	}()
}

// settle waits for the Go's Connecteds to have been sent and handled.
func (f *fakeRouter) settle() {
	f.salvo.pending.Wait()
	for len(f.out) > 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond) // the last one handled
}

// fireLeftovers fails if a fired batch left a waiter behind, or own keys
// other than those of staged levels a tally confirmed, which last only
// salvoAnnounceFor from the Go and then expire with nothing logged.
func fireLeftovers(t *testing.T, a *App) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.waiters) != 0 {
		t.Fatalf("left behind: %d waiters", len(a.waiters))
	}
	later := time.Now().Add(salvoAnnounceFor + 50*time.Millisecond)
	for k, until := range a.ext.own {
		if until.After(later) {
			t.Fatalf("own key %v lasts until %v, past the Go's Connecteds", k, until)
		}
	}
	a.ext.ownership.expire(later)
	if len(a.ext.own) != 0 {
		t.Fatalf("own keys left after expiry: %v", a.ext.own)
	}
}

// nativeApp is a test app in native fire mode with n levels (wire 0..n-1).
func nativeApp(t *testing.T, n int) (*App, *fakeRouter) {
	t.Helper()
	h, _ := OpenHistory("")
	cfg := config.Defaults()
	cfg.Levels = n
	cfg.FireMode = "native"
	cfg.Normalize()
	a, err := New(cfg, "", h, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return a, newFakeRouter(t, a)
}

// batchOf runs one confirm: src to each dst on levels, and returns the
// entries newest first.
func batchOf(t *testing.T, a *App, src int, dsts []int, levels ...int) []Entry {
	t.Helper()
	var items []takeReq
	for i, d := range dsts {
		items = append(items, takeReq{ID: i + 1, Src: src, Dst: d, Levels: levels})
	}
	a.batch(&client{ip: "10.0.0.1"}, items)
	es, _ := a.hist.Page(-1, len(dsts))
	if len(es) != len(dsts) {
		t.Fatalf("entries %+v", es)
	}
	return es
}

// cmds counts the messages with command byte cmd sent so far.
func (f *fakeRouter) cmds(cmd byte) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, w := range f.whys {
		if w.cmd == cmd {
			n++
		}
	}
	return n
}

func (f *fakeRouter) salvoSeq() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return string(f.salvo.seq)
}

// The Ultrix sometimes announces a Go's crosspoints and sometimes not: the
// takes come out the same either way.
func TestNativeFire(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		announce int
		after    time.Duration
	}{
		{"no Connected", 0, 0},
		{"all late", 1, 100 * time.Millisecond},
		{"some late", 2, 100 * time.Millisecond},
		{"all at once", 1, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testNativeFire(t, c.announce, c.after)
		})
	}
}

func testNativeFire(t *testing.T, announce int, after time.Duration) {
	a, f := nativeApp(t, 3)
	f.salvo.announce, f.salvo.announceAfter = announce, after
	f.seed(5, 1, 0, 1, 2)
	f.seed(6, 1, 0, 1, 2)
	f.seed(7, 1, 0, 1)
	f.seed(7, 9, 2) // dst 7 level 3 is on src 9 already
	es := batchOf(t, a, 9, []int{5, 6, 7}, 1, 2, 3)
	if got := f.salvoSeq(); got != "csssssssssgc" {
		t.Fatalf("salvo messages %q, want clear, 9 stages, go, clear", got)
	}
	if n := f.cmds(swp08.CmdExtConnect); n != 0 {
		t.Fatalf("%d ordinary connects sent", n)
	}
	group := es[0].Group
	for _, e := range es {
		if !e.OK || e.Err != "" || group == "" || e.Group != group || e.GroupN != 3 {
			t.Fatalf("take %+v", e)
		}
		want := [][2]int{{1, 1}, {2, 1}, {3, 1}}
		if e.Dst == 7 {
			want[2][1] = 9
			// Level 3 was on src 9 already: staged (a no-op), and says so.
			if !equalInts(e.Already, []int{3}) {
				t.Fatalf("already %v", e.Already)
			}
		} else if len(e.Already) != 0 {
			t.Fatalf("a salvo's tally confirm counted as already: %+v", e)
		}
		if !equalPrev(e.Prev, want) {
			t.Fatalf("dst %d prev %v, want %v", e.Dst, e.Prev, want)
		}
	}
	f.settle()
	if announce == 1 {
		// Every key was used up by its Connected.
		leftovers(t, a)
	}
	fireLeftovers(t, a)
	a.mu.Lock()
	for _, c := range a.ext.causes {
		if c.first.IsZero() || c.sent.IsZero() {
			t.Errorf("cause not marked at the Go: %+v", c)
		}
		// A late Connected is one of the take's changes: what follows the
		// destination then still follows the take.
		if announce != 0 && after > 0 && c.last.Sub(c.first) < after/2 {
			t.Errorf("late Connected not seen as the take's change: %+v", c)
		}
	}
	a.mu.Unlock()
	// The batch's timing is logged.
	timed := false
	for len(a.evq) > 0 {
		if e := <-a.evq; e.Why == "salvo" && strings.Contains(e.Note, "9 of 9 crosspoints confirmed, the last") {
			timed = true
		}
	}
	if !timed {
		t.Error("no salvo timing event")
	}
	// The confirming tallies and Connecteds are ours: nothing logged as
	// made elsewhere, nor found.
	endBurst(a)
	if n := a.hist.Len(); n != 3 {
		t.Fatalf("%d entries, want the 3 takes", n)
	}
	// The router has every route.
	for _, e := range es {
		f.mu.Lock()
		if f.tally[[2]int{e.Dst, 0}] != 9 {
			t.Fatalf("router dst %d on %d", e.Dst, f.tally[[2]int{e.Dst, 0}])
		}
		f.mu.Unlock()
	}
}

// Level 17 (wire 16) and numbers past 1023 ride the salvo too, staged with
// the extended 248 beside 120's, under one Go, in every announce world.
func TestNativeFits(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		name     string
		announce int
		after    time.Duration
	}{
		{"no Connected", 0, 0},
		{"all late", 1, 100 * time.Millisecond},
		{"some late", 2, 100 * time.Millisecond},
		{"all at once", 1, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			testNativeExt(t, c.announce, c.after)
		})
	}
}

func testNativeExt(t *testing.T, announce int, after time.Duration) {
	a, f := nativeApp(t, 17)
	f.salvo.announce, f.salvo.announceAfter = announce, after
	for _, d := range []int{5, 1100, 7} {
		f.seed(d, 1, 0, 15, 16)
	}
	items := []takeReq{
		{ID: 1, Src: 9, Dst: 5, Levels: []int{1, 17}},
		{ID: 2, Src: 1500, Dst: 1100, Levels: []int{1, 17}}, // all past 1023: 248
		{ID: 3, Src: 9, Dst: 7, Levels: []int{16, 17}},
	}
	a.batch(&client{ip: "10.0.0.1"}, items)
	es, _ := a.hist.Page(-1, 3)
	if len(es) != 3 {
		t.Fatalf("entries %+v", es)
	}
	for _, e := range es {
		if !e.OK || len(e.Already) != 0 || e.Err != "" {
			t.Fatalf("take %+v", e)
		}
	}
	if got := f.salvoSeq(); got != "csssssssgc"[:1]+"ssssss"+"gc" {
		t.Fatalf("salvo messages %q, want clear, 6 stages, go, clear", got)
	}
	if n := f.cmds(swp08.CmdConnectOnGo); n != 2 {
		t.Fatalf("%d standard stages, want 2", n)
	}
	if n := f.cmds(swp08.CmdExtConnectOnGo); n != 4 {
		t.Fatalf("%d extended stages, want 4", n)
	}
	if n := f.cmds(swp08.CmdExtConnect); n != 0 {
		t.Fatalf("%d ordinary connects", n)
	}
	f.mu.Lock()
	goes := f.salvo.goes
	f.mu.Unlock()
	if goes != 1 || f.cmds(swp08.CmdGoSalvo) != 3 {
		t.Fatalf("%d Go sets, %d Go commands (want 1 set and 2 clears)", goes, f.cmds(swp08.CmdGoSalvo))
	}
	f.settle()
	fireLeftovers(t, a)
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, w := range []struct{ dst, level, src int }{{5, 0, 9}, {5, 16, 9}, {1100, 0, 1500}, {1100, 16, 1500}, {7, 15, 9}, {7, 16, 9}} {
		if got := f.tally[[2]int{w.dst, w.level}]; got != w.src {
			t.Errorf("router dst %d wire %d on %d, want %d", w.dst, w.level, got, w.src)
		}
	}
}

// A router that stages with 248 but never answers with 250: the group is
// cleared and the batch goes as ordinary connects.
func TestNativeNoExtStageAck(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 17)
	f.salvo.loseExt = true
	f.seed(5, 1, 16)
	f.seed(6, 1, 16)
	es := batchOf(t, a, 9, []int{5, 6}, 17)
	for _, e := range es {
		if !e.OK || len(e.Already) != 0 {
			t.Fatalf("take %+v", e)
		}
	}
	if got := f.salvoSeq(); got != "cssc" {
		t.Fatalf("salvo messages %q: want clear, 2 stages, no go, clear", got)
	}
	f.mu.Lock()
	left := len(f.salvo.staged[127])
	f.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d crosspoints left staged", left)
	}
	if n := f.cmds(swp08.CmdExtConnect); n != 2 {
		t.Fatalf("%d connects, want 2", n)
	}
	fireLeftovers(t, a)
}

func TestMarkStages(t *testing.T) {
	cfg := config.Defaults()
	cfg.Normalize()
	r, _ := newRuntime(cfg)
	items := []takeReq{
		{Src: 1, Dst: 5, Levels: []int{1, 17}},
		{Src: 1, Dst: 6, Levels: []int{1}},
		{Src: 2, Dst: 6, Levels: []int{1, 2}}, // dst 6 level 1 twice: neither staged
		{Src: 1024, Dst: 7, Levels: []int{1}},
		{Src: 1, Dst: 1023, Levels: []int{16}},
	}
	if n := markStages(r, items); n != 4 {
		t.Fatalf("%d items staged", n)
	}
	want := []map[int]bool{{0: true, 16: true}, nil, {1: true}, {0: true}, {15: true}}
	for i, it := range items {
		if len(it.Stage) != len(want[i]) {
			t.Fatalf("item %d stage %v, want %v", i, it.Stage, want[i])
		}
		for k := range want[i] {
			if !it.Stage[k] {
				t.Fatalf("item %d stage %v, want %v", i, it.Stage, want[i])
			}
		}
	}
}

// A router that never answers a stage: the group is cleared and the batch
// goes as ordinary connects.
func TestNativeNoStageAck(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	f.salvo.loseStage = true
	f.seed(5, 1, 0, 1, 2)
	f.seed(6, 1, 0, 1, 2)
	es := batchOf(t, a, 9, []int{5, 6}, 1, 2, 3)
	for _, e := range es {
		if !e.OK || len(e.Already) != 0 {
			t.Fatalf("take %+v", e)
		}
	}
	if got := f.salvoSeq(); got != "cssssssc" {
		t.Fatalf("salvo messages %q: want clear, 6 stages, no go, clear", got)
	}
	f.mu.Lock()
	left := len(f.salvo.staged[127])
	f.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d crosspoints left staged", left)
	}
	if n := f.cmds(swp08.CmdExtConnect); n != 6 {
		t.Fatalf("%d connects, want 6", n)
	}
	fireLeftovers(t, a)
	endBurst(a)
	if n := a.hist.Len(); n != 2 {
		t.Fatalf("entries %d", n)
	}
}

// A router without group salvos never answers the first clear: nothing is
// staged and the batch goes as ordinary connects.
func TestNativeUnsupported(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	f.salvo.off = true
	f.seed(5, 1, 0)
	f.seed(6, 1, 0)
	es := batchOf(t, a, 9, []int{5, 6}, 1)
	for _, e := range es {
		if !e.OK {
			t.Fatalf("take %+v", e)
		}
	}
	if n := f.cmds(swp08.CmdConnectOnGo); n != 0 {
		t.Fatalf("%d staged after an unanswered clear", n)
	}
	if n := f.cmds(swp08.CmdExtConnect); n != 2 {
		t.Fatalf("%d connects", n)
	}
	fireLeftovers(t, a)
}

// A protected destination is refused as ever and left out of the salvo.
func TestNativeProtected(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	for _, d := range []int{5, 6, 7} {
		f.seed(d, 1, 0, 1)
	}
	a.mu.Lock()
	a.router.protect[6] = map[int]int{2: 9}
	a.mu.Unlock()
	es := batchOf(t, a, 9, []int{5, 6, 7}, 1, 2)
	for _, e := range es {
		switch {
		case e.Dst == 6 && (e.OK || !strings.Contains(e.Err, "protected on levels 2")):
			t.Fatalf("protected take %+v", e)
		case e.Dst != 6 && !e.OK:
			t.Fatalf("take %+v", e)
		}
	}
	if got := f.salvoSeq(); got != "cssssgc" {
		t.Fatalf("salvo messages %q", got)
	}
	if n := f.cmds(swp08.CmdConnectOnGo); n != 4 {
		t.Fatalf("%d staged, want 4 (dst 6 left out)", n)
	}
	f.mu.Lock()
	on := f.tally[[2]int{6, 0}]
	f.mu.Unlock()
	if on != 1 {
		t.Fatalf("protected dst 6 switched to %d", on)
	}
	fireLeftovers(t, a)
}

// One take is sent as today, even in native mode; so is a batch whose
// other takes all fall away (protected).
func TestNativeSingleTake(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	f.seed(5, 1, 0)
	batchOf(t, a, 9, []int{5}, 1)
	if n := f.cmds(swp08.CmdGoSalvo) + f.cmds(swp08.CmdConnectOnGo); n != 0 {
		t.Fatalf("%d salvo messages for one take", n)
	}
	f.seed(6, 1, 0)
	a.mu.Lock()
	a.router.protect[6] = map[int]int{1: 9}
	a.mu.Unlock()
	es := batchOf(t, a, 9, []int{5, 6}, 1)
	if n := f.cmds(swp08.CmdGoSalvo) + f.cmds(swp08.CmdConnectOnGo); n != 0 {
		t.Fatalf("%d salvo messages when one take is left", n)
	}
	for _, e := range es {
		if e.Dst == 5 && !e.OK {
			t.Fatalf("take %+v", e)
		}
	}
	fireLeftovers(t, a)
}

// Two batches at once: their salvos never mix in the router's group.
func TestNativeFireLock(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	for d := 5; d < 11; d++ {
		f.seed(d, 1, 0, 1, 2)
	}
	var wg sync.WaitGroup
	for i, src := range []int{20, 30} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d := 5 + i*3
			a.batch(&client{ip: "10.0.0.1"}, []takeReq{
				{ID: 1, Src: src, Dst: d, Levels: []int{1, 2, 3}},
				{ID: 2, Src: src, Dst: d + 1, Levels: []int{1, 2, 3}},
				{ID: 3, Src: src, Dst: d + 2, Levels: []int{1, 2, 3}},
			})
		}()
	}
	wg.Wait()
	f.mu.Lock()
	mixed, goes, seq := f.salvo.mixed, f.salvo.goes, string(f.salvo.seq)
	f.mu.Unlock()
	if mixed || goes != 2 {
		t.Fatalf("mixed %v, %d goes", mixed, goes)
	}
	if want := "csssssssssgc"; seq != want+want {
		t.Fatalf("salvo messages %q interleaved", seq)
	}
	es, _ := a.hist.Page(-1, 6)
	for _, e := range es {
		if !e.OK {
			t.Fatalf("take %+v", e)
		}
	}
	fireLeftovers(t, a)
}

// No 123 for the Go: it is never sent again, nor followed by connects; the
// group is cleared and the interrogates confirm what the Go did.
func TestNativeNoGoAck(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	f.salvo.noGoAck = true
	f.seed(5, 1, 0)
	f.seed(6, 1, 0)
	es := batchOf(t, a, 9, []int{5, 6}, 1)
	for _, e := range es {
		if !e.OK {
			t.Fatalf("take %+v", e)
		}
	}
	if got := f.salvoSeq(); got != "cssgc" {
		t.Fatalf("salvo messages %q", got)
	}
	if n := f.cmds(swp08.CmdExtConnect); n != 0 {
		t.Fatalf("%d connects after a Go", n)
	}
	fireLeftovers(t, a)
}

// Someone else clears the group before the Go: 123 says nothing was set.
// The takes fail on what the router reports, and nothing is resent.
func TestNativeGoNothing(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	f.salvo.wipe = true
	f.seed(5, 1, 0)
	f.seed(6, 1, 0)
	es := batchOf(t, a, 9, []int{5, 6}, 1)
	for _, e := range es {
		if e.OK || !equalInts(e.Failed, []int{1}) || !strings.Contains(e.Err, "level 1 still on source 2") {
			t.Fatalf("take %+v", e)
		}
	}
	if n := f.cmds(swp08.CmdExtConnect); n != 0 || f.salvoSeq() != "cssgc" {
		t.Fatalf("%d connects, salvo %q", n, f.salvoSeq())
	}
	fireLeftovers(t, a)
}

// Three takes in a native batch where two take the same destination level:
// those two are not staged, and the lone stageable take must still be
// connected (in turn), not left staged.
func TestNativeOneStageable(t *testing.T) {
	t.Parallel()
	a, f := nativeApp(t, 3)
	f.seed(8, 1, 0)
	f.seed(9, 1, 0)
	items := []takeReq{{ID: 1, Src: 3, Dst: 8, Levels: []int{1}}, {ID: 2, Src: 4, Dst: 9, Levels: []int{1}}, {ID: 3, Src: 5, Dst: 9, Levels: []int{1}}}
	stages, conns := f.cmds(swp08.CmdConnectOnGo), f.cmds(swp08.CmdExtConnect)
	a.batch(&client{ip: "10.0.0.1"}, items)
	if n := f.cmds(swp08.CmdConnectOnGo) - stages; n != 0 {
		t.Fatalf("%d staged, want 0", n)
	}
	if n := f.cmds(swp08.CmdExtConnect) - conns; n != 3 {
		t.Fatalf("%d connects, want 3", n)
	}
	fireLeftovers(t, a)
}
