package app

import (
	"slices"
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
	tally  map[[2]int]int  // dst, wire level → src
	ignore map[int]bool    // dsts the router never answers for
	deaf   map[[2]int]bool // dst, wire level whose connects are ignored (interrogates still answer)
	lose   map[int]bool    // dsts whose Connected goes missing (interrogates still answer)
	mute   map[int]bool    // dsts whose interrogates go unanswered (connects still do)
	failAt int             // this send and every later one fail (1-based; 0 = never)
	hold   time.Duration   // each message is only "written" (answered, Done closed) this long after Send, like a busy queue
	n      int
	whys   []sentWhy // every message's reason, in order
	out    chan []byte
}

type sentWhy struct {
	cmd byte
	why swp08.Why
}

func newFakeRouter(t *testing.T, a *App) *fakeRouter {
	f := &fakeRouter{a: a, tally: map[[2]int]int{}, ignore: map[int]bool{}, deaf: map[[2]int]bool{}, lose: map[int]bool{}, mute: map[int]bool{}, out: make(chan []byte, 256)}
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
	// Seeded routes count as not read lately, so takes check them first.
	f.a.mu.Lock()
	f.a.router.seenAt = map[[2]int]time.Time{}
	f.a.mu.Unlock()
}

func (f *fakeRouter) send(m []byte, why swp08.Why) error {
	if f.hold > 0 {
		go func() {
			time.Sleep(f.hold)
			f.deliver(m, why)
		}()
		return nil
	}
	return f.deliver(m, why)
}

func (f *fakeRouter) deliver(m []byte, why swp08.Why) error {
	if why.Done != nil {
		defer close(why.Done) // answered at once; after the router's reply
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.n++
	rec := why
	rec.Done = nil // a fresh channel each time; not part of what a message is for
	f.whys = append(f.whys, sentWhy{m[0], rec})
	if f.failAt > 0 && f.n >= f.failAt {
		return swp08.ErrBusy
	}
	switch m[0] {
	case swp08.CmdExtConnect:
		l, dst, src := int(m[2]), int(m[3])<<8|int(m[4]), int(m[5])<<8|int(m[6])
		k := [2]int{dst, l}
		if f.ignore[dst] || f.deaf[k] {
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
	if len(a.waiters) != 0 || len(a.ext.own) != 0 {
		t.Fatalf("left behind: %d waiters, own %v", len(a.waiters), a.ext.own)
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
	if len(a.ext.causes) != 1 || !a.ext.causes[0].done || a.ext.causes[0].id != e.ID || a.ext.causes[0].first.IsZero() {
		t.Fatalf("cause %+v", a.ext.causes)
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
	a.router.protect[5] = map[int]int{2: 9}
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
	if len(a.ext.causes) != 0 {
		t.Fatalf("causes %+v", a.ext.causes)
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
	if e.OK || e.Err != "no confirmation from router: level 1 no answer; level 2 no answer; level 3 no answer" {
		t.Fatalf("take %+v", e)
	}
	if !equalInts(e.Failed, []int{1, 2, 3}) || len(e.Now) != 0 {
		t.Fatalf("failed %v, now %v", e.Failed, e.Now)
	}
	leftovers(t, a)
	last := func() Entry {
		es, _ := a.hist.Page(-1, 1)
		return es[0]
	}
	// A late Connected is the take's after all (#91): not an ext entry, and
	// the take's entry loses that level from Failed.
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	if n := a.hist.Len(); n != 1 {
		t.Fatalf("late Connected of our route recorded: %d entries", n)
	}
	e = last()
	if e.OK || !equalInts(e.Failed, []int{2, 3}) || !equalInts(e.Late, []int{1}) ||
		e.Err != "no confirmation from router: level 2 no answer; level 3 no answer" {
		t.Fatalf("partly late %+v", e)
	}
	// So is a late answer to the last interrogate.
	a.onRouterMsg(swp08.ExtTally(0, 1, 5, 7))
	endBurst(a)
	e = last()
	if a.hist.Len() != 1 || e.OK || !equalInts(e.Failed, []int{3}) || !equalInts(e.Late, []int{1, 2}) ||
		e.Err != "no confirmation from router: level 3 no answer" {
		t.Fatalf("entries %d, %+v", a.hist.Len(), e)
	}
	// The last level: nothing left failing.
	a.onRouterMsg(swp08.ExtConnected(0, 2, 5, 7))
	endBurst(a)
	e = last()
	if a.hist.Len() != 1 || !e.OK || e.Err != "" || len(e.Failed) != 0 || !equalInts(e.Late, []int{1, 2, 3}) {
		t.Fatalf("all late %+v", e)
	}
	// The claim is used up: the route made again elsewhere is someone else's.
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 8))
	endBurst(a)
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	if es, _ := a.hist.Page(-1, 1); a.hist.Len() != 3 || !es[0].Ext || es[0].Src != 7 {
		t.Fatalf("later change elsewhere %+v", es)
	}
}

// Past the window, a Connected for the route is a change made elsewhere.
func TestTakeLateTooLate(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.ignore[5] = true
	e := takeOne(t, a, 7, 5, 1)
	a.mu.Lock()
	if len(a.ext.lateTake) != 1 {
		t.Fatalf("lateTake %v", a.ext.lateTake)
	}
	for _, lts := range a.ext.lateTake {
		for i := range lts {
			lts[i].until = time.Now().Add(-time.Second)
		}
	}
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	es, _ := a.hist.Page(-1, 2)
	if len(es) != 2 || !es[0].Ext || es[0].Src != 7 || es[1].OK || len(es[1].Late) != 0 || es[1].ID != e.ID {
		t.Fatalf("too late %+v", es)
	}
}

// An answer between the take's entry being recorded and its waiter being
// removed confirms the level in the waiter; the entry is amended too.
func TestTakeAnswerInGap(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.ignore[5] = true
	a.gapHook = func() { a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7)) }
	takeOne(t, a, 7, 5, 1, 2, 3)
	leftovers(t, a)
	endBurst(a)
	es, _ := a.hist.Page(-1, 2)
	if len(es) != 1 || es[0].OK || !equalInts(es[0].Failed, []int{2, 3}) || !equalInts(es[0].Late, []int{1}) ||
		es[0].Err != "no confirmation from router: level 2 no answer; level 3 no answer" {
		t.Fatalf("amended %+v", es)
	}
}

// A late amendment re-renders the error from the entry's fields, so source
// names with "; " and "level " in them can't be mistaken for its parts.
func TestLateConfirmNames(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	a.mu.Lock()
	a.router.srcNames[5] = "CAM; level 3 no answer"
	a.router.srcNames[6] = "level 1 B"
	a.mu.Unlock()
	e := &Entry{Src: 7, Dst: 2, Levels: []int{1, 2, 3}, Prev: [][2]int{{1, 5}, {2, 6}, {3, 4}},
		Failed: []int{1, 2, 3, 4}, Unsent: []int{4}, SendErr: "busy; level 1 no", Now: [][2]int{{1, 5}, {2, 6}}}
	a.record(e)
	a.lateConfirm(e.ID, []int{3})
	got, _ := a.hist.Page(-1, 1)
	want := "no confirmation from router: level 1 still on CAM; level 3 no answer; level 2 still on level 1 B; levels 4 not sent: busy; level 1 no"
	if len(got) != 1 || got[0].Err != want {
		t.Fatalf("got %+v", got)
	}
}

// Level 1's connect is ignored but the router answers the interrogate: it
// is still on its old src, a real failure. The others switch.
func TestTakeIgnoredLevel(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.deaf[[2]int{5, 0}] = true
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || e.Err != "no confirmation from router: level 1 still on source 2" {
		t.Fatalf("take %+v", e)
	}
	if !equalInts(e.Failed, []int{1}) || !equalPrev(e.Now, [][2]int{{1, 1}}) {
		t.Fatalf("failed %v, now %v", e.Failed, e.Now)
	}
	leftovers(t, a)
}

// Every connect switches without a Connected, except level 1's, which is
// ignored: the interrogate confirms the switched levels (not failures) and
// shows level 1 unchanged.
func TestTakeSilentSwitch(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.lose[5] = true
	f.deaf[[2]int{5, 0}] = true
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || e.Err != "no confirmation from router: level 1 still on source 2" {
		t.Fatalf("take %+v", e)
	}
	if !equalInts(e.Already, []int{2, 3}) || !equalInts(e.Failed, []int{1}) {
		t.Fatalf("already %v, failed %v", e.Already, e.Failed)
	}
	leftovers(t, a)
}

// Something else routes level 1 to another src meanwhile: a failure that
// says where it is.
func TestTakeOtherSource(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.deaf[[2]int{5, 0}] = true
	go func() {
		time.Sleep(200 * time.Millisecond)
		f.mu.Lock()
		f.tally[[2]int{5, 0}] = 9
		f.mu.Unlock()
	}()
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if e.OK || e.Err != "no confirmation from router: level 1 on source 10" || !equalPrev(e.Now, [][2]int{{1, 9}}) {
		t.Fatalf("take %+v", e)
	}
}

func equalInts(a, b []int) bool {
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
	a.router.srcNames[7], a.router.dstNames[5] = "CAM 7", "MON 5"
	a.mu.Unlock()
	e := a.describe(swp08.TxEvent{Msg: swp08.ExtConnect(0, 1, 5, 7), Why: swp08.Why{Reason: "take", By: "10.0.0.1"}, Tries: 1, Result: swp08.TxAck})
	if e.Kind != "connect" || e.Level != 2 || e.Wire != 1 || e.Dst != 5 || e.DstName != "MON 5" || e.Src != 7 || e.SrcName != "CAM 7" ||
		e.By != "10.0.0.1" || e.Why != "take" || e.Hex != "82 00 01 00 05 00 07" {
		t.Fatalf("connect = %+v", e)
	}
	e = a.describe(swp08.TxEvent{Msg: swp08.ExtInterrogate(0, 0, 0), Why: swp08.Why{Reason: "keepalive"}})
	if e.Kind != "keepalive" || e.Level != 0 || e.Wire != -1 || e.Dst != -1 || e.Src != -1 {
		t.Fatalf("probe = %+v", e)
	}
}

// With the router down, only refused sends a browser asked for are logged;
// Sode's own re-reads aren't.
func TestUnsentOnlyForTakes(t *testing.T) {
	a := newTestApp(t) // never connected: every Send is refused
	if err := a.sendMsg(swp08.NamesReq(false, false, 0, 0, 2), swp08.Why{Reason: "names"}); err == nil {
		t.Fatal("send with no router succeeded")
	}
	a.sendMsg(swp08.ExtTallyDumpReq(0, 0), swp08.Why{Reason: "resync"})
	a.sendMsg(swp08.ExtConnect(0, 0, 5, 7), swp08.Why{Reason: "take", By: "10.0.0.1"})
	if n := len(a.txq); n != 1 {
		t.Fatalf("%d events, want just the take's", n)
	}
	if ev := <-a.txq; ev.Why.Reason != "take" || ev.Result != "unsent" {
		t.Fatalf("event %+v, want the take, unsent", ev)
	}
}

// A take gives up on dst 5, then the router answers late: a PiP (dst 9)
// that follows it changes with that answer, so it's the take's follow, not
// a panel change (#91).
func TestTakeLateFollow(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	f.seed(9, 1, 0, 1, 2)
	f.ignore[5] = true
	e := takeOne(t, a, 7, 5, 1)
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	a.onRouterMsg(swp08.ExtConnected(0, 0, 9, 7))
	endBurst(a)
	es, _ := a.hist.Page(-1, 3)
	if len(es) != 2 || !es[0].Ext || es[0].Dst != 9 || es[0].Follow == nil || *es[0].Follow != e.ID || !es[1].OK {
		t.Fatalf("late follow %+v", es)
	}
}

// A PiP that follows a take of a multi-take confirm is tied to the confirm
// (FollowGroup), not to that take.
func TestBatchFollowGroup(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	for _, d := range []int{5, 6, 9} {
		f.seed(d, 1, 0, 1, 2)
	}
	f.ignore[5] = true
	a.batch(&client{ip: "10.0.0.1"}, []takeReq{
		{ID: 1, Src: 7, Dst: 5, Levels: []int{1}},
		{ID: 2, Src: 7, Dst: 6, Levels: []int{1}},
	})
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	a.onRouterMsg(swp08.ExtConnected(0, 0, 9, 7))
	endBurst(a)
	es, _ := a.hist.Page(-1, 5)
	group := ""
	var follower *Entry
	for i, e := range es {
		if e.Ext {
			follower = &es[i]
		} else if e.Group != "" {
			group = e.Group
		}
	}
	if group == "" || follower == nil || follower.Dst != 9 || follower.FollowGroup != group || follower.Follow != nil || follower.Group != "" {
		t.Fatalf("batch follower: group %q, entries %+v", group, es)
	}
}

// Counts what the router was sent for, by reason.
func (f *fakeRouter) sentFor(reason string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, w := range f.whys {
		if w.why.Reason == reason {
			n++
		}
	}
	return n
}

// Levels the router reported a moment ago aren't read again before a take:
// an undo comes straight after the Connected that set them.
func TestTakeFreshSkipsCheck(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	a.mu.Lock()
	for l := 1; l <= 3; l++ {
		a.router.seenAt[[2]int{5, l}] = time.Now()
	}
	a.mu.Unlock()
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if !e.OK || f.sentFor("check") != 0 || !equalPrev(e.Prev, [][2]int{{1, 1}, {2, 1}, {3, 1}}) {
		t.Fatalf("take %+v, checks %d", e, f.sentFor("check"))
	}
	// Stale ones are read.
	a.mu.Lock()
	a.router.seenAt[[2]int{5, 2}] = time.Now().Add(-time.Minute)
	a.mu.Unlock()
	takeOne(t, a, 8, 5, 1, 2, 3)
	if n := f.sentFor("check"); n != 1 {
		t.Fatalf("checks %d, want just the stale level", n)
	}
}

// A big confirm doesn't read every destination's routes first.
func TestBatchBigSkipsCheck(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	var items []takeReq
	for d := 0; d < bigBatch; d++ {
		f.seed(d, 1, 0, 1, 2)
		items = append(items, takeReq{ID: d, Src: 7, Dst: d, Levels: []int{1, 2, 3}})
	}
	a.batch(&client{ip: "10.0.0.1"}, items)
	if n := f.sentFor("check"); n != 0 {
		t.Fatalf("%d checks for a big batch", n)
	}
	if es, _ := a.hist.Page(-1, bigBatch); len(es) != bigBatch || slices.ContainsFunc(es, func(e Entry) bool { return !e.OK }) {
		t.Fatalf("takes %+v", es)
	}
	// One fewer reads them.
	b := newTestApp(t)
	g := newFakeRouter(t, b)
	items = items[:0]
	for d := 0; d < bigBatch-1; d++ {
		g.seed(d, 1, 0, 1, 2)
		items = append(items, takeReq{ID: d, Src: 7, Dst: d, Levels: []int{1, 2, 3}})
	}
	b.batch(&client{ip: "10.0.0.1"}, items)
	if n := g.sentFor("check"); n != 3*(bigBatch-1) {
		t.Fatalf("%d checks for a small batch", n)
	}
}

// The router is only asked about a take once its messages have gone out, so
// a take waiting in a long queue isn't failed for the queue's delay.
func TestTakeWaitsForQueue(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0, 1, 2)
	a.mu.Lock()
	for l := 1; l <= 3; l++ {
		a.router.seenAt[[2]int{5, l}] = time.Now() // no check: only the connects wait
	}
	a.mu.Unlock()
	f.hold = takeTimeout + takeGrace + 200*time.Millisecond
	e := takeOne(t, a, 7, 5, 1, 2, 3)
	if !e.OK || len(e.Failed) != 0 {
		t.Fatalf("take %+v", e)
	}
	leftovers(t, a)
}

// A retry that fails too must not take over the first take's claim: when
// the router answers late, both entries are amended (#91).
func TestTakeLateRetryKeepsBoth(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0)
	f.ignore[5] = true
	e1 := takeOne(t, a, 7, 5, 1)
	e2 := takeOne(t, a, 7, 5, 1)
	if e1.OK || e2.OK || e1.ID == e2.ID {
		t.Fatalf("takes %+v %+v", e1, e2)
	}
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	es, _ := a.hist.Page(-1, 3)
	if len(es) != 2 || !es[0].OK || !es[1].OK || !equalInts(es[0].Late, []int{1}) || !equalInts(es[1].Late, []int{1}) {
		t.Fatalf("both should be amended: %+v", es)
	}
}

// A level the router answered for (still on its old route) isn't waiting
// for the take: a Connected for src soon after is someone else's change.
func TestTakeLateOnlyUnanswered(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	f := newFakeRouter(t, a)
	f.seed(5, 1, 0)
	f.deaf[[2]int{5, 0}] = true // connect ignored, interrogate answers: still on source 1
	e := takeOne(t, a, 7, 5, 1)
	if e.OK || len(e.Now) != 1 {
		t.Fatalf("take %+v", e)
	}
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	es, _ := a.hist.Page(-1, 3)
	if len(es) != 2 || !es[0].Ext || es[0].Src != 7 || es[1].OK || len(es[1].Late) != 0 {
		t.Fatalf("panel change claimed by the take: %+v", es)
	}
}

// The connects' own keys last through the longest wait in the send queue.
func TestRegisterOwnKeysOutlastQueue(t *testing.T) {
	t.Parallel()
	a := newTestApp(t)
	w, err := planTake(a.r(), takeReq{Src: 7, Dst: 5, Levels: []int{1}})
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	w.ext = a.ext // take sets it
	w.register(a)
	until := a.ext.own[ownKey{5, w.levels[0].wire, 7}]
	a.mu.Unlock()
	if time.Until(until) < queueMax+ownFor-time.Second {
		t.Fatalf("own key ends in %v", time.Until(until))
	}
}
