package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	h, _ := OpenHistory("")
	cfg := config.Defaults()
	cfg.Levels = 3
	cfg.Normalize()
	a, err := New(cfg, "", h, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

// endBurst makes the pending burst old enough to be written.
func endBurst(a *App) {
	a.mu.Lock()
	a.extLast = time.Now().Add(-time.Second)
	a.mu.Unlock()
	a.flushExt()
}

func TestExtBurst(t *testing.T) {
	a := newTestApp(t)
	// Known state: dst 5 has src 1 on wire levels 0-2 (UI 1-3).
	for l := 0; l < 3; l++ {
		a.onRouterMsg(swp08.ExtTally(0, l, 5, 1))
		a.onRouterMsg(swp08.ExtTally(0, l, 6, 1))
	}
	if n := a.hist.Len(); n != 0 {
		t.Fatalf("tally replies recorded %d entries", n)
	}
	// A panel: dst 5 gets src 7 on every level, dst 6 src 8 on level 1;
	// dst 6 level 2 goes to 9 and back, which is no change.
	for l := 0; l < 3; l++ {
		a.onRouterMsg(swp08.ExtConnected(0, l, 5, 7))
	}
	a.onRouterMsg(swp08.ExtConnected(0, 0, 6, 8))
	a.onRouterMsg(swp08.ExtConnected(0, 1, 6, 9))
	a.onRouterMsg(swp08.ExtConnected(0, 1, 6, 1))
	a.flushExt()
	if n := a.hist.Len(); n != 0 {
		t.Fatalf("burst written before it went quiet: %d", n)
	}
	endBurst(a)
	es, _ := a.hist.Page(-1, 10)
	if len(es) != 2 {
		t.Fatalf("entries %+v", es)
	}
	e5, e6 := es[1], es[0] // newest first
	if e5.Dst != 5 || e5.Src != 7 || len(e5.Levels) != 3 || !e5.Ext || !e5.OK || len(e5.Prev) != 3 || e5.Prev[0] != [2]int{1, 1} {
		t.Fatalf("dst 5 entry %+v", e5)
	}
	if e6.Dst != 6 || e6.Src != 8 || len(e6.Levels) != 1 || e6.Levels[0] != 1 {
		t.Fatalf("dst 6 entry %+v", e6)
	}
	if e5.Group == "" || e5.Group != e6.Group || e5.GroupN != 2 {
		t.Fatalf("groups %q/%d %q/%d", e5.Group, e5.GroupN, e6.Group, e6.GroupN)
	}

	// A single-destination change is not grouped.
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 2))
	endBurst(a)
	es, _ = a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Group != "" || es[0].Prev[0] != [2]int{1, 7} {
		t.Fatalf("single %+v", es)
	}
}

func TestExtSkipsOwnTakes(t *testing.T) {
	a := newTestApp(t)
	a.onRouterMsg(swp08.ExtTally(0, 0, 5, 1))
	a.mu.Lock()
	a.own[ownKey{5, 0, 7}] = time.Now().Add(time.Minute)
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	if n := a.hist.Len(); n != 0 {
		t.Fatalf("own take recorded as external: %d", n)
	}
	// The same route made elsewhere once ours has expired is external.
	a.mu.Lock()
	a.own[ownKey{5, 0, 1}] = time.Now().Add(-time.Second)
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 1))
	endBurst(a)
	if n := a.hist.Len(); n != 1 {
		t.Fatalf("external change after expiry: %d entries", n)
	}
}

func TestExtFollows(t *testing.T) {
	a := newTestApp(t)
	for _, d := range []int{5, 9, 11} {
		for l := 0; l < 3; l++ {
			a.onRouterMsg(swp08.ExtTally(0, l, d, 1))
		}
	}
	last := func() Entry {
		es, _ := a.hist.Page(-1, 1)
		return es[0]
	}

	// Our take of src 7 to dst 5 was recorded as #42; the router then moves
	// dst 9 (a PiP following dst 5) to src 7 too.
	a.mu.Lock()
	now := time.Now()
	a.causes = append(a.causes, &cause{src: 7, dst: 5, first: now, last: now, id: 42, sode: true, done: true})
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 9, 7))
	a.onRouterMsg(swp08.ExtConnected(0, 0, 11, 8)) // another source: not a follow
	endBurst(a)
	es, _ := a.hist.Page(-1, 10)
	if len(es) != 2 {
		t.Fatalf("entries %+v", es)
	}
	for _, e := range es {
		switch e.Dst {
		case 9:
			if e.Follow == nil || *e.Follow != 42 || e.Group != "" {
				t.Fatalf("follow %+v", e)
			}
		case 11:
			if e.Follow != nil || e.Group != "" {
				t.Fatalf("other source %+v", e)
			}
		}
	}

	// A panel take to dst 5 and its follow on dst 11 in one burst: the
	// follow names the take, and isn't grouped with it.
	a.mu.Lock()
	a.causes = nil
	a.mu.Unlock()
	for l := 0; l < 3; l++ {
		a.onRouterMsg(swp08.ExtConnected(0, l, 5, 3))
	}
	time.Sleep(5 * time.Millisecond)
	a.onRouterMsg(swp08.ExtConnected(0, 0, 11, 3))
	endBurst(a)
	f := last()
	es, _ = a.hist.Page(-1, 2)
	take := es[1]
	if take.Dst != 5 || take.Follow != nil || take.Group != "" || f.Dst != 11 || f.Follow == nil || *f.Follow != take.ID {
		t.Fatalf("take %+v follow %+v", take, f)
	}

	// Too long after a take: a take of its own.
	a.mu.Lock()
	old := time.Now().Add(-2 * time.Second)
	a.causes = append(a.causes, &cause{src: 4, dst: 5, first: old, last: old, id: 50, sode: true, done: true})
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 1, 9, 4))
	endBurst(a)
	if e := last(); e.Follow != nil {
		t.Fatalf("late change counted as follow: %+v", e)
	}

	// A change that may follow our take still in flight waits for it.
	c := &cause{src: 6, dst: 5, id: -1, sode: true}
	a.mu.Lock()
	a.causes = append(a.causes, c)
	c.saw(time.Now())
	a.mu.Unlock()
	n := a.hist.Len()
	a.onRouterMsg(swp08.ExtConnected(0, 2, 9, 6))
	endBurst(a)
	if a.hist.Len() != n {
		t.Fatal("burst written while the take it follows was in flight")
	}
	a.mu.Lock()
	c.id, c.done = 77, true
	a.mu.Unlock()
	a.flushExt()
	if e := last(); e.Follow == nil || *e.Follow != 77 {
		t.Fatalf("held follow %+v", e)
	}
}
