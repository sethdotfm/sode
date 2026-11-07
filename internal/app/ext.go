package app

import (
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// External changes (a hardware panel, Companion) arrive as Connecteds no take asked for. An
// external change arrives as one message per level, so changes are collected until quiet and written as
// one entry per destination and source; a burst over several destinations shares a history
// group.
// The router also re-routes destinations itself (a multiviewer PiP following a DSM). The
// same source moved to another destination right after one of our takes is written as a
// follow of it (Entry.Follow), not a take. Changes made elsewhere aren't followed: a
// group send from a panel stays one burst.
// The router doesn't announce every change (2026-10-23: KiPro 4 moved with no Connected).
// Those surface as a tally differing from the cache (periodic dump, interrogate) and are
// written as found entries, never follows (time unknown).

const (
	extQuiet = 500 * time.Millisecond // a burst ends after this long with no changes
	extMax   = 3 * time.Second        // or this long after it began
	// ownFor: how long a matching Connected counts as ours: the take's whole wait plus
	// slack for a late reply.
	ownFor = routeTimeout + routeGrace + 5*time.Second // counted from when the connects have gone out (sendRoute)

	followAfter  = 500 * time.Millisecond // a follow comes this soon after its take's last change
	followBefore = 50 * time.Millisecond  // or this much before its first (messages race)
	// A burst that may follow an in-flight take waits for it to be recorded, up to this
	// long after the burst began. A take still queued holds nothing (cause.sent).
	extHold   = routeTimeout + routeGrace + 2*time.Second
	causeKeep = 5 * time.Second // a recorded take can be followed for this long
)

// extTracker: burst, own connects, causes/leaders (takes changes may follow), and
// late/lateRoute/lateHits (takes that gave up). Replaced on a router change; the pointer is
// the router's generation, so an older holder is cut. Guarded by App.mu; callers hold it.
type extTracker struct {
	burst
	ownership
	follows
}

func newExtTracker() *extTracker {
	return &extTracker{
		burst: burst{pending: map[int]map[int]*extLevel{}},
		ownership: ownership{
			own:       map[ownKey]time.Time{},
			late:      map[ownKey]time.Time{},
			lateRoute: map[ownKey][]lateRoute{},
		},
		follows: follows{leaders: map[int]int{}},
	}
}

type verdict int

const (
	verdictIgnore verdict = iota // a tally of a route a take asked for, or of one we had no value for
	verdictOurs                  // the answer to one of our connects
	verdictLate                  // the route a take gave up on, made after all
	verdictExt                   // a change made elsewhere
)

// classify: a Connected answering our connect is ours (key used up, so the same route
// later elsewhere still counts); a route a take gave up on is claimed late (flushLate
// amends; a tally confirms too); a tally of a route a take in flight or just given up
// asked for is ours (ignore), as is one the cache had no value for (known false); else
// made elsewhere (ext). Caller holds mu.
func (a *App) classify(xp swp08.Crosspoint, ui int, known bool) verdict {
	k := ownKey{xp.Dst, xp.Level, xp.Src}
	if xp.Changed() {
		if a.ext.ownAnswer(k, time.Now()) {
			return verdictOurs
		}
		if a.claimLate(k, ui) {
			return verdictLate
		}
		return verdictExt
	}
	if a.claimLate(k, ui) {
		return verdictLate
	}
	if known && !a.awaited(xp) {
		return verdictExt
	}
	return verdictIgnore
}

func (x *extTracker) flush(now time.Time) (changes map[int]map[int]*extLevel, first time.Time, ok bool) {
	x.ownership.expire(now)
	x.follows.expire(now)
	return x.burst.flush(now, x.follows.awaitsRoute)
}

func (x *extTracker) entries(name func(dst bool, n int) string, changes map[int]map[int]*extLevel, at time.Time) []*Entry {
	return x.burst.entries(name, changes, at, x.follows.followed)
}

// flushFound writes found changes ahead of the rest of the burst, so they precede the take
// whose check found them.
func (a *App) flushFound(dst int) {
	a.mu.Lock()
	found := map[int]*extLevel{}
	for ui, l := range a.ext.pending[dst] {
		if l.found {
			found[ui] = l
			delete(a.ext.pending[dst], ui)
		}
	}
	if len(a.ext.pending[dst]) == 0 {
		delete(a.ext.pending, dst)
	}
	// Grouped by now; the rest of the burst, written later, gets that group.
	es := a.ext.entries(a.router.name, map[int]map[int]*extLevel{dst: found}, time.Now())
	a.mu.Unlock()
	for _, e := range es {
		a.record(e)
	}
}
