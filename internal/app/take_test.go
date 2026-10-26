package app

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

func TestWaiterAlreadyRouted(t *testing.T) {
	w := &waiter{dst: 9, src: 2, levels: []*takeLevel{{ui: 1, wire: 0}, {ui: 2, wire: 1}}, done: make(chan struct{})}
	// Level 0 changes: announced with Connected.
	w.see(swp08.Crosspoint{Level: 0, Dst: 9, Src: 2, Cmd: swp08.CmdExtConnected})
	// Wrong source on level 1 doesn't count.
	w.see(swp08.Crosspoint{Level: 1, Dst: 9, Src: 3, Cmd: swp08.CmdExtTally})
	if w.closed {
		t.Fatal("finished early")
	}
	// Level 1 was already on src: confirmed by the interrogate's Tally.
	w.see(swp08.Crosspoint{Level: 1, Dst: 9, Src: 2, Cmd: swp08.CmdExtTally})
	if !w.closed {
		t.Fatal("not finished")
	}
	if w.levels[0].already || !w.levels[1].already {
		t.Fatalf("already = %v, %v, want only level 1", w.levels[0].already, w.levels[1].already)
	}
}

// fakeRouter answers takes like the Ultrix: Connected only for a connect
// that changes something, Tally for an interrogate, in order.
type fakeRouter struct {
	a      *App
	mu     sync.Mutex
	tally  map[[2]int]int // dst, wire level → src
	ignore map[int]bool   // dsts the router never answers for
	lose   map[int]bool   // dsts whose Connected goes missing (interrogates still answer)
	mute   map[int]bool   // dsts whose interrogates go unanswered (connects still do)
	failAt int            // this send and every later one fail (1-based; 0 = never)
	n      int
	whys   []sentWhy // every message's reason, in order
	out    chan []byte
}

type sentWhy struct {
	cmd byte
	why swp08.Why
}

func newFakeRouter(t *testing.T, a *App) *fakeRouter {
	f := &fakeRouter{a: a, tally: map[[2]int]int{}, ignore: map[int]bool{}, lose: map[int]bool{}, mute: map[int]bool{}, out: make(chan []byte, 256)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for m := range f.out {
			a.onRouterMsg(m)
		}
	}()
	t.Cleanup(func() { close(f.out); <-done })
	a.sendMsg = f.send
	return f
}

// seed puts src on dst's wire levels, in the router and in the app's cache.
func (f *fakeRouter) seed(dst, src int, wires ...int) {
	for _, l := range wires {
		f.mu.Lock()
		f.tally[[2]int{dst, l}] = src
		f.mu.Unlock()
		f.a.onRouterMsg(swp08.ExtTally(0, l, dst, src))
	}
}

func (f *fakeRouter) send(m []byte, why swp08.Why) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	f.whys = append(f.whys, sentWhy{m[0], why})
	if f.failAt > 0 && f.n >= f.failAt {
		return swp08.ErrBusy
	}
	switch m[0] {
	case swp08.CmdExtConnect:
		l, dst, src := int(m[2]), int(m[3])<<8|int(m[4]), int(m[5])<<8|int(m[6])
		k := [2]int{dst, l}
		if f.ignore[dst] {
			return nil
		}
		if old, ok := f.tally[k]; ok && old == src {
			return nil // no change, no Connected
		}
		f.tally[k] = src
		if !f.lose[dst] {
			f.out <- swp08.ExtConnected(int(m[1]), l, dst, src)
		}
	case swp08.CmdExtInterrogate:
		l, dst := int(m[2]), int(m[3])<<8|int(m[4])
		if f.ignore[dst] || f.mute[dst] {
			return nil
		}
		src, ok := f.tally[[2]int{dst, l}]
		if !ok {
			src = 0xFFFF
		}
		f.out <- swp08.ExtTally(int(m[1]), l, dst, src)
	}
	return nil
}

func (f *fakeRouter) sends() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.n
}

// takeOne runs one take and returns its history entry.
func takeOne(t *testing.T, a *App, src, dst int, levels ...int) Entry {
	t.Helper()
	a.take(&client{ip: "10.0.0.1"}, takeReq{ID: 1, Src: src, Dst: dst, Levels: levels})
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Ext {
		t.Fatalf("no take recorded: %+v", es)
	}
	return es[0]
}

// leftovers fails if a finished take left a waiter or own keys behind.
func leftovers(t *testing.T, a *App) {
	t.Helper()
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.waiters) != 0 || len(a.own) != 0 {
		t.Fatalf("left behind: %d waiters, own %v", len(a.waiters), a.own)
	}
}

func TestTakeChanges(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if !e.OK || e.Err != "" || len(e.Already) != 0 {
		t.Fatalf("take %+v", e)
	}
	if want := [][2]int{{1, 1}, {2, 1}, {3, 1}}; !equalPrev(e.Prev, want) {
		t.Fatalf("prev %v, want %v", e.Prev, want)
	}
	leftovers(t, a)
	a.mu.Lock()
	if len(a.causes) != 1 || !a.causes[0].done || a.causes[0].id != e.ID || a.causes[0].first.IsZero() {
		t.Fatalf("cause %+v", a.causes)
	}
	a.mu.Unlock()
	endBurst(a)
	if n := a.hist.Len(); n != 1 {
		t.Fatalf("our own Connected logged as ext: %d entries", n)
	}
}

func TestTakeAlready(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 7, 0)
	f.seed(5, 1, 1, 2)
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if !e.OK || len(e.Already) != 1 || e.Already[0] != 1 {
		t.Fatalf("take %+v", e)
	}
	leftovers(t, a)
	endBurst(a)
	if n := a.hist.Len(); n != 1 {
		t.Fatalf("entries %d", n)
	}
}

func TestTakeProtected(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	a.mu.Lock()
	a.protect[5] = map[int]int{2: 9}
	a.mu.Unlock()
	n := f.sends()
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || !strings.Contains(e.Err, "protected on levels 2") {
		t.Fatalf("take %+v", e)
	}
	if f.sends() != n {
		t.Fatal("sent to a protected destination")
	}
	leftovers(t, a)
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.causes) != 0 {
		t.Fatalf("causes %+v", a.causes)
	}
}

func TestTakeLostConnected(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.lose[5] = true
	// No Connected: the interrogate after the timeout confirms it.
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if !e.OK || len(e.Already) != 3 {
		t.Fatalf("take %+v", e)
	}
	// No Connected is coming for those levels, so their own keys went.
	leftovers(t, a)
	// The interrogate's answer is ours, not a change found.
	endBurst(a)
	if n := a.hist.Len(); n != 1 {
		t.Fatalf("entries %d", n)
	}
}

func TestTakeNoConfirmation(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.ignore[5] = true
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || e.Err != "no confirmation from router for levels 1, 2, 3" {
		t.Fatalf("take %+v", e)
	}
	leftovers(t, a)
	// A late answer to the last interrogate is still ours, not a change found.
	a.onRouterMsg(swp08.ExtTally(0, 1, 5, 7))
	endBurst(a)
	if n := a.hist.Len(); n != 1 {
		t.Fatalf("late tally of our route recorded: %d entries", n)
	}
	// The route made later elsewhere is someone else's.
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || !es[0].Ext || es[0].Src != 7 {
		t.Fatalf("later change elsewhere %+v", es)
	}
}

func TestTakeSendFails(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.failAt = f.sends() + 3 + 3 // the third connect, after an interrogate per level
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || !strings.HasPrefix(e.Err, "levels 3 not sent:") {
		t.Fatalf("take %+v", e)
	}
	f.mu.Lock()
	got := [3]int{f.tally[[2]int{5, 0}], f.tally[[2]int{5, 1}], f.tally[[2]int{5, 2}]}
	f.mu.Unlock()
	if got != [3]int{7, 7, 1} {
		t.Fatalf("router %v", got)
	}
	leftovers(t, a)
}

// The router has src 4 on dst 5, which it never announced: Prev comes
// from the router, and the change is logged as found.
func TestTakeFreshPrev(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.mu.Lock()
	for l := 0; l < 3; l++ {
		f.tally[[2]int{5, l}] = 4
	}
	f.tally[[2]int{5, 2}] = 7
	f.mu.Unlock()
	// A resync is meanwhile finding changes on dst 6, not yet written.
	f.seed(6, 1, 0, 1)
	f.seed(6, 8, 0)
	f.seed(6, 9, 1)
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if want := [][2]int{{1, 4}, {2, 4}, {3, 7}}; !e.OK || !equalPrev(e.Prev, want) {
		t.Fatalf("take %+v, want prev %v", e, want)
	}
	if len(e.Already) != 1 || e.Already[0] != 3 {
		t.Fatalf("already %v", e.Already)
	}
	leftovers(t, a)
	// What the check found is written before the take, not after it.
	endBurst(a)
	es, _ := a.hist.Page(-1, 10)
	if len(es) != 5 || es[2].ID != e.ID || !es[3].Found || !es[4].Found || es[3].Dst != 5 || es[4].Dst != 5 {
		t.Fatalf("entries %+v", es)
	}
	// Each set of found changes is a group of its own.
	if g5, g6 := es[3].Group, es[0].Group; g5 == "" || g6 == "" || g5 != es[4].Group || g6 != es[1].Group || g5 == g6 {
		t.Fatalf("groups %+v", es)
	}
}

// No answer to the interrogates: the take goes ahead from the cache.
func TestTakeCheckUnanswered(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.mute[5] = true
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if want := [][2]int{{1, 1}, {2, 1}, {3, 1}}; !e.OK || !equalPrev(e.Prev, want) {
		t.Fatalf("take %+v", e)
	}
	leftovers(t, a)
}

// With the link down, nothing waits for answers that can't come.
func TestTakeLinkDown(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.failAt = f.sends() + 1
	start := time.Now()
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || !strings.HasPrefix(e.Err, "levels 1, 2, 3 not sent:") {
		t.Fatalf("take %+v", e)
	}
	if d := time.Since(start); d > checkTimeout/2 {
		t.Fatalf("took %v", d)
	}
	leftovers(t, a)
}

// Duplicate levels in a request are taken and recorded once.
func TestTakeDuplicateLevels(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	e := takeOne(t, a, 7, 5, 1, 1, 2)
	if !e.OK || len(e.Levels) != 2 || len(e.Prev) != 2 {
		t.Fatalf("take %+v", e)
	}
}

func equalPrev(a, b [][2]int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// A take's messages carry what they're for and whose take it is, for the
// sent log: the check's interrogates, then the connects.
func TestTakeWhy(t *testing.T) {
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	takeOne(t, a, 7, 5, 1, 2)
	f.mu.Lock()
	defer f.mu.Unlock()
	check := swp08.Why{Reason: "check", By: "10.0.0.1"}
	take := swp08.Why{Reason: "take", By: "10.0.0.1"}
	want := []sentWhy{{swp08.CmdExtInterrogate, check}, {swp08.CmdExtInterrogate, check}, {swp08.CmdExtConnect, take}, {swp08.CmdExtConnect, take}}
	if len(f.whys) != len(want) {
		t.Fatalf("sent %+v, want %+v", f.whys, want)
	}
	for i := range want {
		if f.whys[i] != want[i] {
			t.Fatalf("sent %+v, want %+v", f.whys, want)
		}
	}
}

// Sent log entries are decoded with UI levels and names.
func TestDescribeSent(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	a.srcNames[7], a.dstNames[5] = "CAM 7", "MON 5"
	a.mu.Unlock()
	e := a.describe(swp08.TxEvent{Msg: swp08.ExtConnect(0, 1, 5, 7), Why: swp08.Why{Reason: "take", By: "10.0.0.1"}, Tries: 1, Result: swp08.TxAck})
	if e.Kind != "connect" || e.Level != 2 || e.Wire != 1 || e.Dst != 5 || e.DstName != "MON 5" || e.Src != 7 || e.SrcName != "CAM 7" ||
		e.By != "10.0.0.1" || e.Why != "take" || e.Hex != "82 00 01 00 05 00 07" {
		t.Fatalf("connect = %+v", e)
	}
	e = a.describe(swp08.TxEvent{Msg: swp08.ExtInterrogate(0, 0, 0), Why: swp08.Why{Reason: "keepalive"}})
	if e.Kind != "keepalive" || e.Level != 1 || e.Src != -1 {
		t.Fatalf("probe = %+v", e)
	}
}

// Link checks that went fine are counted, not logged; failed ones are logged.
func TestKeepalivesCounted(t *testing.T) {
	a := newTestApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.logSent(ctx)
	probe := swp08.ExtInterrogate(0, 0, 0)
	keep := swp08.Why{Reason: "keepalive"}
	a.txq <- swp08.TxEvent{Msg: probe, Why: keep, Tries: 1, Result: swp08.TxAck}
	a.txq <- swp08.TxEvent{Msg: probe, Why: keep, Tries: 3, Result: swp08.TxNoAck}
	a.txq <- swp08.TxEvent{Msg: probe, Why: swp08.Why{Reason: "check"}, Tries: 1, Result: swp08.TxAck}
	deadline := time.Now().Add(2 * time.Second)
	var page []Sent
	for time.Now().Before(deadline) {
		if page, _, _ = a.sent.Page(-1, 10); len(page) == 2 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if len(page) != 2 || page[1].Result != swp08.TxNoAck || page[0].Why != "check" {
		t.Fatalf("logged %+v, want the failed keepalive and the check", page)
	}
	if n := a.keepalives.Load(); n != 1 {
		t.Fatalf("keepalives %d, want 1", n)
	}
}
