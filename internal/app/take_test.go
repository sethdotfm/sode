package app

import (
	"strings"
	"sync"
	"testing"

	"github.com/sethdotfm/sode/internal/swp08"
)

func TestWaiterAlreadyRouted(t *testing.T) {
	w := &waiter{dst: 9, src: 2, wire: map[int]bool{0: true, 1: true}, already: map[int]bool{}, done: make(chan struct{})}
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
	if w.already[0] || !w.already[1] {
		t.Fatalf("already = %v, want only level 1", w.already)
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
	failAt int            // this send and every later one fail (1-based; 0 = never)
	n      int
	out    chan []byte
}

func newFakeRouter(t *testing.T, a *App) *fakeRouter {
	f := &fakeRouter{a: a, tally: map[[2]int]int{}, ignore: map[int]bool{}, lose: map[int]bool{}, out: make(chan []byte, 256)}
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

func (f *fakeRouter) send(m []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
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
		if f.ignore[dst] {
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
	f.failAt = f.sends() + 3 // the third connect
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
