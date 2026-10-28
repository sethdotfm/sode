package app

import (
	"slices"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// Routes changed elsewhere (a panel, Companion) reach us as Connected
// messages that no take of ours asked for. A panel take arrives as one
// message per level, so changes are collected until they go quiet and
// written as one entry per destination and source; a burst touching
// several destinations shares a history group.
//
// The router also moves destinations by itself: a multiviewer PiP set to
// follow a DSM is re-routed whenever the DSM is. Such a change, of the
// same source to another destination right after one of our takes, is
// written as a follow of that take (Entry.Follow) rather than a take of
// its own. Changes made elsewhere aren't followed: a panel's salvo of one
// source to several destinations stays one burst, grouped.
//
// The router doesn't announce every change (2026-10-23: KiPro 4 moved
// with no Connected sent). Those turn up later as a tally that differs from
// the cache, in the periodic dump or an interrogate, and are written as
// found entries: when they happened isn't known, so they're never follows.

const (
	extQuiet = 500 * time.Millisecond // a burst ends after this long with no changes
	extMax   = 3 * time.Second        // or this long after it began
	// ownFor is how long a Connected matching one of our connects counts
	// as ours: the take's whole wait, and some slack for a late reply.
	ownFor = takeTimeout + takeGrace + 5*time.Second // counted from when the connects have gone out (sendTake)

	followAfter  = 500 * time.Millisecond // a follow comes this soon after its take's last change
	followBefore = 50 * time.Millisecond  // or this much before its first (messages race)
	// A burst that may follow a take still in flight waits for it to be
	// recorded, up to this long after the burst began.
	// A take still queued behind others' messages holds nothing: its changes
	// can't have come yet (cause.sent).
	extHold   = takeTimeout + takeGrace + 2*time.Second
	causeKeep = 5 * time.Second // a recorded take can be followed for this long
)

type ownKey struct{ dst, wire, src int }

// cause is one of our takes, which changes elsewhere may follow (id -1
// until it's recorded).
type cause struct {
	src, dst    int
	first, last time.Time // its changes as the router reported them; zero = none yet
	sent        time.Time // when its connects had gone out; zero while they're queued
	id          int
	done        bool   // recorded
	group       string // history group of the multi-take confirm it belongs to, "" if alone
}

// saw notes one of the take's changes. Caller holds mu.
func (c *cause) saw(t time.Time) {
	if c.first.IsZero() {
		c.first = t
	}
	c.last = t
}

// leads reports whether a change of src to dst at t looks like a follow of c.
func (c *cause) leads(src, dst int, t time.Time) bool {
	return c.done && c.id >= 0 && !c.first.IsZero() && c.src == src && c.dst != dst &&
		!t.Before(c.first.Add(-followBefore)) && !t.After(c.last.Add(followAfter))
}

// extTracker is the state behind routes changed elsewhere: the burst being
// collected, our takes' connects (own), the takes changes may follow
// (causes, leaders) and takes that gave up and may yet be answered
// (late, lateTake, lateHits). It is replaced when the router changes, and the
// pointer is the router's generation: a take that holds an older one is cut
// and touches nothing of the new router's. Guarded by App.mu; its methods
// expect the caller to hold it.
type extTracker struct {
	burst                          // the burst being collected
	own      map[ownKey]time.Time  // our takes' connects → when to stop treating them as ours
	late     map[ownKey]time.Time  // finished takes' unconfirmed routes → until when a tally of one is ours
	causes   []*cause              // recent takes that changes elsewhere may follow
	leaders  map[int]int           // dst → the dst it was last seen following
	lateTake map[ownKey][]lateTake // finished takes' unconfirmed routes, with the history entry to amend if one turns up (#91)
	lateHits []lateHit             // late answers awaiting their entry's amendment (flushLate)
}

func newExtTracker() *extTracker {
	return &extTracker{
		burst:    burst{pending: map[int]map[int]*extLevel{}},
		own:      map[ownKey]time.Time{},
		late:     map[ownKey]time.Time{},
		leaders:  map[int]int{},
		lateTake: map[ownKey][]lateTake{},
	}
}

// verdict is whose a crosspoint that differs from the cache is.
type verdict int

const (
	verdictIgnore verdict = iota // a tally of a route a take asked for, or of one we had no value for
	verdictOurs                  // the answer to one of our connects
	verdictLate                  // the route a take gave up on, made after all
	verdictExt                   // a change made elsewhere
)

// classify says whose a route change is, and does what each rule does:
//   - a Connected that answers one of our connects is ours, and uses the key
//     up so the same route made again elsewhere later still counts;
//   - a route a take gave up on is claimed (late): its entry is amended by
//     flushLate. A tally of it (resync, interrogate) confirms it too;
//   - a tally of a route a take in flight, or one that just gave up, asked
//     for is ours without a claim (ignore), as is a tally of a route the
//     cache had no value for (known false);
//   - anything else was made elsewhere (ext).
//
// ui is the UI level of xp. Caller holds mu.
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

// ownAnswer reports whether a Connected for k is the answer to one of our
// connects, and uses the key up either way.
func (x *extTracker) ownAnswer(k ownKey, now time.Time) bool {
	if until, ok := x.own[k]; ok {
		delete(x.own, k)
		if now.Before(until) {
			return true
		}
	}
	return false
}

// flushExt writes a finished burst to history.
func (a *App) flushExt() {
	a.flushLate()
	a.mu.Lock()
	now := time.Now()
	burst, first, ok := a.ext.flush(now)
	if !ok {
		a.mu.Unlock()
		return
	}
	es := a.ext.entries(a.router.name, burst, first)
	a.mu.Unlock()
	for _, e := range es {
		a.record(e)
	}
}

// flush drops what has expired and, if the burst is finished, takes it
// (ok), with when it began.
func (x *extTracker) flush(now time.Time) (changes map[int]map[int]*extLevel, first time.Time, ok bool) {
	for k, lts := range x.lateTake {
		lts = slices.DeleteFunc(lts, func(lt lateTake) bool { return now.After(lt.until) })
		if len(lts) == 0 {
			delete(x.lateTake, k)
		} else {
			x.lateTake[k] = lts
		}
	}
	for k, until := range x.own {
		if now.After(until) {
			delete(x.own, k)
		}
	}
	for k, until := range x.late {
		if now.After(until) {
			delete(x.late, k)
		}
	}
	keep := x.causes[:0]
	for _, c := range x.causes {
		if !c.done || (!c.last.IsZero() && now.Sub(c.last) < causeKeep) {
			keep = append(keep, c)
		}
	}
	clear(x.causes[len(keep):])
	x.causes = keep
	return x.burst.flush(now, x.awaitsTake)
}

// entries turns a burst into history entries; see burst.entries.
func (x *extTracker) entries(name func(dst bool, n int) string, changes map[int]map[int]*extLevel, at time.Time) []*Entry {
	return x.burst.entries(name, changes, at, x.followed)
}

// flushFound writes the changes found on dst now, ahead of the rest of
// the burst, so they come before the take whose check found them rather
// than after it, where they'd look like what dst has now.
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
	// Grouped by now, not the burst's start: the rest of the burst, written
	// later, gets that one.
	es := a.ext.entries(a.router.name, map[int]map[int]*extLevel{dst: found}, time.Now())
	a.mu.Unlock()
	for _, e := range es {
		a.record(e)
	}
}

// awaitsTake reports whether a pending change may follow one of our
// takes still waiting for the router. Caller holds mu.
func (x *extTracker) awaitsTake(pending map[int]map[int]*extLevel) bool {
	for _, c := range x.causes {
		if c.done || c.sent.IsZero() {
			continue
		}
		for dst, ls := range pending {
			for _, l := range ls {
				// A change before the connects went out can't follow them.
				if !l.found && l.src == c.src && dst != c.dst && !l.first.Before(c.sent.Add(-followBefore)) {
					return true
				}
			}
		}
	}
	return false
}

// followed is the take of ours a change of src to dst at t seems to
// follow, or nil. A salvo of one source leaves several that fit, and
// timing can't tell them apart; a destination follows the same one each
// time (a PiP its DSM), so when only one fits it's remembered, and a
// salvo goes by that. Failing that, the take whose last change came
// latest. Caller holds mu.
func (x *extTracker) followed(src, dst int, t time.Time) *cause {
	var fits []*cause
	for _, c := range x.causes {
		if c.leads(src, dst, t) {
			fits = append(fits, c)
		}
	}
	switch len(fits) {
	case 0:
		return nil
	case 1:
		x.leaders[dst] = fits[0].dst
		return fits[0]
	}
	best := fits[0]
	for _, c := range fits {
		if lead, ok := x.leaders[dst]; ok && c.dst == lead {
			return c
		}
		if c.last.After(best.last) {
			best = c
		}
	}
	return best
}
