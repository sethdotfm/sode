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
	a, err := New(cfg, "", h, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	shortTimers(a)
	return a
}

// shortTimers keeps tests that wait on an unanswered router fast. queueMax stays well above
// the others so a held reply still counts as queued, not lost.
func shortTimers(a *App) {
	a.checkTimeout = 40 * time.Millisecond
	a.routeTimeout = 40 * time.Millisecond
	a.routeGrace = 40 * time.Millisecond
	a.queueMax = 500 * time.Millisecond
	a.drainLimit = 2 * time.Second
}

// endBurst makes the pending burst old enough to be written.
func endBurst(a *App) {
	a.mu.Lock()
	a.ext.last = time.Now().Add(-time.Second)
	a.mu.Unlock()
	a.flushExt()
}

func TestExtBurst(t *testing.T) {
	a := newTestApp(t)
	for l := 0; l < 3; l++ {
		a.onRouterMsg(swp08.ExtTally(0, l, 5, 1))
		a.onRouterMsg(swp08.ExtTally(0, l, 6, 1))
	}
	if n := a.hist.Len(); n != 0 {
		t.Fatalf("tally replies recorded %d entries", n)
	}
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
	a.ext.own[ownKey{5, 0, 7}] = time.Now().Add(time.Minute)
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	if n := a.hist.Len(); n != 0 {
		t.Fatalf("own take recorded as external: %d", n)
	}
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 1))
	endBurst(a)
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 7))
	endBurst(a)
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || es[0].Src != 7 || !es[0].Ext {
		t.Fatalf("repeat of our route elsewhere: %+v", es)
	}
	a.hist.Clear()
	a.mu.Lock()
	a.ext.own[ownKey{5, 0, 1}] = time.Now().Add(-time.Second)
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 5, 1))
	endBurst(a)
	if n := a.hist.Len(); n != 1 {
		t.Fatalf("External after expiry: %d entries", n)
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

	a.mu.Lock()
	now := time.Now()
	a.ext.causes = append(a.ext.causes, &cause{src: 7, dst: 5, first: now, last: now, id: 42, done: true})
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

	a.mu.Lock()
	a.ext.causes = nil
	a.mu.Unlock()
	for l := 0; l < 3; l++ {
		a.onRouterMsg(swp08.ExtConnected(0, l, 5, 3))
	}
	time.Sleep(5 * time.Millisecond)
	a.onRouterMsg(swp08.ExtConnected(0, 0, 11, 3))
	endBurst(a)
	es, _ = a.hist.Page(-1, 2)
	if es[0].Follow != nil || es[1].Follow != nil || es[0].Group == "" || es[0].Group != es[1].Group {
		t.Fatalf("panel salvo %+v", es)
	}

	a.mu.Lock()
	t0 := time.Now()
	a.ext.causes = []*cause{
		{src: 2, dst: 5, first: t0.Add(-200 * time.Millisecond), last: t0.Add(-200 * time.Millisecond), id: 60, done: true},
		{src: 2, dst: 9, first: t0.Add(-20 * time.Millisecond), last: t0.Add(-20 * time.Millisecond), id: 61, done: true},
	}
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 1, 11, 2))
	endBurst(a)
	if e := last(); e.Follow == nil || *e.Follow != 61 {
		t.Fatalf("follow of a salvo %+v", e)
	}
	a.mu.Lock()
	t1 := time.Now()
	a.ext.causes = []*cause{{src: 4, dst: 5, first: t1, last: t1, id: 70, done: true}}
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 2, 11, 4))
	endBurst(a)
	a.mu.Lock()
	t2 := time.Now()
	a.ext.causes = []*cause{
		{src: 5, dst: 5, first: t2, last: t2, id: 80, done: true},
		{src: 5, dst: 9, first: t2, last: t2.Add(time.Millisecond), id: 81, done: true},
	}
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 11, 5))
	endBurst(a)
	if e := last(); e.Follow == nil || *e.Follow != 80 {
		t.Fatalf("salvo follow by what it followed before: %+v", e)
	}

	a.mu.Lock()
	t3 := time.Now()
	a.ext.causes = []*cause{
		{src: 3, dst: 5, first: t3, last: t3, id: 90, done: true, group: "g1"},
		{src: 3, dst: 6, first: t3, last: t3, id: 91, done: true, group: "g1"},
	}
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 9, 3))
	a.onRouterMsg(swp08.ExtConnected(0, 0, 11, 3))
	endBurst(a)
	es, _ = a.hist.Page(-1, 2)
	for _, e := range es {
		if e.FollowGroup != "g1" || e.Follow != nil || e.Group != "" || e.GroupN != 0 {
			t.Fatalf("follower of a grouped cause %+v", e)
		}
	}
	a.mu.Lock()
	t4 := time.Now()
	a.ext.causes = []*cause{{src: 13, dst: 5, first: t4, last: t4, id: 92, done: true}}
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 9, 13))
	endBurst(a)
	if e := last(); e.Follow == nil || *e.Follow != 92 || e.FollowGroup != "" {
		t.Fatalf("ungrouped follow %+v", e)
	}

	a.mu.Lock()
	a.ext.causes = nil
	a.mu.Unlock()

	a.mu.Lock()
	old := time.Now().Add(-2 * time.Second)
	a.ext.causes = append(a.ext.causes, &cause{src: 4, dst: 5, first: old, last: old, id: 50, done: true})
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 1, 9, 4))
	endBurst(a)
	if e := last(); e.Follow != nil {
		t.Fatalf("late change counted as follow: %+v", e)
	}

	c := &cause{src: 6, dst: 5, id: -1, sent: time.Now().Add(-time.Second)}
	a.mu.Lock()
	a.ext.causes = append(a.ext.causes, c)
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

// A take still queued behind others' messages (not sent yet) can't have
// caused a change that's already here, so it doesn't hold the burst.
func TestExtNotHeldByQueuedTake(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	a.ext.causes = append(a.ext.causes, &cause{src: 6, dst: 5, id: -1})
	a.mu.Unlock()
	n := a.hist.Len()
	a.onRouterMsg(swp08.ExtConnected(0, 2, 9, 6))
	endBurst(a)
	if a.hist.Len() != n+1 {
		t.Fatal("burst held for a take whose connects hadn't gone out")
	}
}

func TestExtFound(t *testing.T) {
	a := newTestApp(t)
	for l := 0; l < 3; l++ {
		for _, d := range []int{5, 6, 7} {
			a.onRouterMsg(swp08.ExtTally(0, l, d, 1))
		}
	}
	endBurst(a)
	if n := a.hist.Len(); n != 0 {
		t.Fatalf("first dump recorded %d entries", n)
	}
	a.mu.Lock()
	now := time.Now()
	a.ext.causes = append(a.ext.causes, &cause{src: 9, dst: 8, first: now, last: now, id: 42, done: true})
	a.mu.Unlock()
	a.onRouterMsg(swp08.ExtConnected(0, 0, 7, 9))
	a.onRouterMsg(swp08.ExtTally(0, 0, 5, 9))
	a.onRouterMsg(swp08.ExtTally(0, 1, 5, 9))
	a.onRouterMsg(swp08.ExtTally(0, 2, 5, 1)) // unchanged
	a.onRouterMsg(swp08.ExtTally(0, 0, 6, 4))
	endBurst(a)
	es, _ := a.hist.Page(-1, 10)
	if len(es) != 3 {
		t.Fatalf("entries %+v", es)
	}
	var found []Entry
	for _, e := range es {
		if e.Dst == 7 {
			if e.Found || e.Group != "" || e.Follow == nil {
				t.Fatalf("announced change %+v", e)
			}
			continue
		}
		found = append(found, e)
		if !e.Found || !e.Ext || e.Follow != nil || e.Prev[0][1] != 1 {
			t.Fatalf("found %+v", e)
		}
	}
	if len(found[0].Levels)+len(found[1].Levels) != 3 || found[0].Group == "" || found[0].Group != found[1].Group || found[0].GroupN != 2 {
		t.Fatalf("found entries %+v", found)
	}
	a.onRouterMsg(swp08.ExtTally(0, 0, 5, 9))
	endBurst(a)
	if n := a.hist.Len(); n != 3 {
		t.Fatalf("repeat tally recorded: %d entries", n)
	}
}
