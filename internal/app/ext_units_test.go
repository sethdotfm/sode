package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// The units behind extTracker, each on its own: no App, no lock.

func newBurst() *burst { return &burst{pending: map[int]map[int]*extLevel{}} }

func never(map[int]map[int]*extLevel) bool { return false }

func chg(dst, src, ui int, at time.Time) extChange {
	return extChange{xp: swp08.Crosspoint{Dst: dst, Src: src}, ui: ui, at: at}
}

func TestBurstAddKeepsFirst(t *testing.T) {
	b := newBurst()
	t0 := time.Now()
	b.add(chg(5, 7, 1, t0))
	b.add(chg(5, 8, 1, t0.Add(100*time.Millisecond))) // same level again: latest src wins
	b.add(chg(6, 7, 2, t0.Add(200*time.Millisecond)))
	if !b.first.Equal(t0) || !b.last.Equal(t0.Add(200*time.Millisecond)) {
		t.Fatalf("first %v last %v", b.first, b.last)
	}
	if l := b.pending[5][1]; l.src != 8 || !l.first.Equal(t0) || !l.at.Equal(t0.Add(100*time.Millisecond)) {
		t.Fatalf("level %+v", l)
	}
	if len(b.pending) != 2 {
		t.Fatalf("pending %v", b.pending)
	}
}

func TestBurstFlushQuietAndMax(t *testing.T) {
	b := newBurst()
	t0 := time.Now()
	if _, _, ok := b.flush(t0, never); ok {
		t.Fatal("empty burst flushed")
	}
	b.add(chg(5, 7, 1, t0))
	if _, _, ok := b.flush(t0.Add(extQuiet-time.Millisecond), never); ok {
		t.Fatal("flushed before it went quiet")
	}
	// Changes keep coming: it holds until extMax from the first.
	b.add(chg(5, 7, 2, t0.Add(extMax-time.Second)))
	if _, _, ok := b.flush(t0.Add(extMax-time.Second+extQuiet-time.Millisecond), never); ok {
		t.Fatal("flushed while changes were coming")
	}
	got, first, ok := b.flush(t0.Add(extMax), never)
	if !ok || !first.Equal(t0) || len(got[5]) != 2 {
		t.Fatalf("max age: %v %v %v", got, first, ok)
	}
	if len(b.pending) != 0 {
		t.Fatalf("not reset: %v", b.pending)
	}

	b.add(chg(5, 7, 1, t0))
	got, first, ok = b.flush(t0.Add(extQuiet), never)
	if !ok || !first.Equal(t0) || len(got) != 1 {
		t.Fatalf("quiet: %v %v %v", got, first, ok)
	}
}

func TestBurstFlushHeld(t *testing.T) {
	b := newBurst()
	t0 := time.Now()
	b.add(chg(5, 7, 1, t0))
	held := func(map[int]map[int]*extLevel) bool { return true }
	if _, _, ok := b.flush(t0.Add(extQuiet), held); ok {
		t.Fatal("flushed while held")
	}
	if _, _, ok := b.flush(t0.Add(extHold), held); !ok {
		t.Fatal("held past extHold")
	}
}

func TestBurstEntriesLeftAlone(t *testing.T) {
	b := newBurst()
	t0 := time.Now()
	b.add(extChange{xp: swp08.Crosspoint{Dst: 5, Src: 7}, ui: 1, old: 2, known: true, at: t0})
	b.add(extChange{xp: swp08.Crosspoint{Dst: 6, Src: 2}, ui: 1, old: 2, known: true, at: t0}) // back where it began
	name := func(dst bool, n int) string { return "n" }
	es := b.entries(name, b.pending, t0, func(int, int, time.Time) *cause { return nil })
	if len(es) != 1 || es[0].Dst != 5 || es[0].Src != 7 || !es[0].Ext || len(es[0].Prev) != 1 {
		t.Fatalf("entries %+v", es)
	}
}

func TestOwnAnswerConsumes(t *testing.T) {
	o := &ownership{own: map[ownKey]time.Time{}}
	now := time.Now()
	k := ownKey{5, 0, 7}
	o.own[k] = now.Add(time.Minute)
	if !o.ownAnswer(k, now) {
		t.Fatal("not ours")
	}
	if o.ownAnswer(k, now) {
		t.Fatal("key used twice")
	}
	o.own[k] = now.Add(-time.Second)
	if o.ownAnswer(k, now) {
		t.Fatal("expired key counted")
	}
	if _, ok := o.own[k]; ok {
		t.Fatal("expired key kept")
	}
	if o.ownAnswer(ownKey{1, 1, 1}, now) {
		t.Fatal("unknown key counted")
	}
}

func TestOwnershipExpire(t *testing.T) {
	now := time.Now()
	live, dead := now.Add(time.Minute), now.Add(-time.Second)
	a, b := ownKey{5, 0, 7}, ownKey{5, 0, 8}
	o := &ownership{
		own:      map[ownKey]time.Time{a: live, b: dead},
		late:     map[ownKey]time.Time{a: dead, b: live},
		lateTake: map[ownKey][]lateTake{a: {{id: 1, until: dead}, {id: 2, until: live}}, b: {{id: 3, until: dead}}},
	}
	o.expire(now)
	if _, ok := o.own[a]; !ok || len(o.own) != 1 {
		t.Fatalf("own %v", o.own)
	}
	if _, ok := o.late[b]; !ok || len(o.late) != 1 {
		t.Fatalf("late %v", o.late)
	}
	if lts := o.lateTake[a]; len(lts) != 1 || lts[0].id != 2 || len(o.lateTake) != 1 {
		t.Fatalf("lateTake %v", o.lateTake)
	}
}

func TestFollowedLatestConfirmWins(t *testing.T) {
	t0 := time.Now()
	f := &follows{leaders: map[int]int{}}
	f.causes = []*cause{
		{src: 4, dst: 1, first: t0, last: t0, id: 1, done: true},
		{src: 4, dst: 2, first: t0, last: t0.Add(100 * time.Millisecond), id: 2, done: true},
		{src: 4, dst: 3, first: t0, last: t0.Add(50 * time.Millisecond), id: 3, done: true},
	}
	if c := f.followed(4, 9, t0.Add(150*time.Millisecond)); c == nil || c.id != 2 {
		t.Fatalf("followed %+v", c)
	}
	if len(f.leaders) != 0 {
		t.Fatalf("leader remembered from a guess: %v", f.leaders)
	}
	// Wrong source, or the take's own destination: no follow.
	if f.followed(5, 9, t0) != nil {
		t.Fatal("followed another source")
	}
	if c := f.followed(4, 2, t0.Add(150*time.Millisecond)); c == nil || c.id != 3 {
		t.Fatalf("a take's own destination followed it: %+v", c)
	}
}

func TestFollowedLeaderRemembered(t *testing.T) {
	t0 := time.Now()
	f := &follows{leaders: map[int]int{}}
	f.causes = []*cause{{src: 4, dst: 1, first: t0, last: t0, id: 1, done: true}}
	if c := f.followed(4, 9, t0); c == nil || c.id != 1 || f.leaders[9] != 1 {
		t.Fatalf("followed %+v leaders %v", c, f.leaders)
	}
	// Later a salvo fits two; dst 9 goes with the one it followed before.
	f.causes = []*cause{
		{src: 4, dst: 1, first: t0, last: t0, id: 1, done: true},
		{src: 4, dst: 2, first: t0, last: t0.Add(100 * time.Millisecond), id: 2, done: true},
	}
	if c := f.followed(4, 9, t0.Add(100*time.Millisecond)); c == nil || c.id != 1 {
		t.Fatalf("leader ignored: %+v", c)
	}
}

func TestFollowedWindow(t *testing.T) {
	t0 := time.Now()
	f := &follows{leaders: map[int]int{}}
	f.causes = []*cause{{src: 4, dst: 1, first: t0, last: t0, id: 1, done: true}}
	if f.followed(4, 9, t0.Add(followAfter+time.Millisecond)) != nil {
		t.Fatal("followed after the window")
	}
	if f.followed(4, 9, t0.Add(-followBefore-time.Millisecond)) != nil {
		t.Fatal("followed before the window")
	}
	f.causes[0].done = false
	if f.followed(4, 9, t0) != nil {
		t.Fatal("followed a take not yet recorded")
	}
}

func TestFollowsAwaitsTake(t *testing.T) {
	t0 := time.Now()
	pending := map[int]map[int]*extLevel{9: {1: {src: 4, first: t0.Add(10 * time.Millisecond)}}}
	f := &follows{causes: []*cause{{src: 4, dst: 1, id: -1, sent: t0}}}
	if !f.awaitsTake(pending) {
		t.Fatal("not held for a take in flight")
	}
	f.causes[0].sent = time.Time{} // still queued
	if f.awaitsTake(pending) {
		t.Fatal("held for a queued take")
	}
	f.causes[0].sent = t0.Add(time.Second) // change came before the connects went out
	if f.awaitsTake(pending) {
		t.Fatal("held for a change that came first")
	}
	f.causes[0].sent, f.causes[0].done = t0, true
	if f.awaitsTake(pending) {
		t.Fatal("held for a recorded take")
	}
}

func TestFollowsExpire(t *testing.T) {
	now := time.Now()
	old, fresh := now.Add(-causeKeep-time.Second), now.Add(-time.Second)
	inflight := &cause{src: 1, id: -1}
	f := &follows{causes: []*cause{
		{src: 2, last: old, done: true},
		{src: 3, last: fresh, done: true},
		inflight,
	}}
	f.expire(now)
	if len(f.causes) != 2 || f.causes[0].src != 3 || f.causes[1] != inflight {
		t.Fatalf("causes %+v", f.causes)
	}
}
