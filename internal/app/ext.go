package app

import (
	"sort"
	"strconv"
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
	ownFor = takeTimeout + takeGrace + 5*time.Second

	followAfter  = 500 * time.Millisecond // a follow comes this soon after its take's last change
	followBefore = 50 * time.Millisecond  // or this much before its first (messages race)
	// A burst that may follow a take still in flight waits for it to be
	// recorded, up to this long after the burst began.
	extHold   = checkTimeout + takeTimeout + takeGrace + 2*time.Second
	causeKeep = 5 * time.Second // a recorded take can be followed for this long
)

type extLevel struct {
	prev      int // src before the burst
	prevKnown bool
	src       int
	first, at time.Time // first and last change
	found     bool      // the last change came as a tally, not a Connected
}

type ownKey struct{ dst, wire, src int }

// cause is one of our takes, which changes elsewhere may follow (id -1
// until it's recorded).
type cause struct {
	src, dst    int
	first, last time.Time // its changes as the router reported them; zero = none yet
	id          int
	done        bool // recorded
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

// noteExt records a changed crosspoint unless one of our takes sent it.
// old/known is the cached src before the change. Caller holds mu.
func (a *App) noteExt(xp swp08.Crosspoint, ui, old int, known bool) {
	// One of our connects: the router's answer to it, used up so the same
	// route made again elsewhere later still counts.
	found := !xp.Changed()
	k := ownKey{xp.Dst, xp.Level, xp.Src}
	if until, ok := a.own[k]; ok && !found {
		delete(a.own, k)
		if time.Now().Before(until) {
			return
		}
	}
	now := time.Now()
	if len(a.ext) == 0 {
		a.extFirst = now
	}
	a.extLast = now
	d := a.ext[xp.Dst]
	if d == nil {
		d = map[int]*extLevel{}
		a.ext[xp.Dst] = d
	}
	if l := d[ui]; l != nil {
		l.src, l.at, l.found = xp.Src, now, found
	} else {
		d[ui] = &extLevel{prev: old, prevKnown: known, src: xp.Src, first: now, at: now, found: found}
	}
}

// flushExt writes a finished burst to history.
func (a *App) flushExt() {
	a.mu.Lock()
	now := time.Now()
	for _, m := range []map[ownKey]time.Time{a.own, a.late} {
		for k, until := range m {
			if now.After(until) {
				delete(m, k)
			}
		}
	}
	keep := a.causes[:0]
	for _, c := range a.causes {
		if !c.done || (!c.last.IsZero() && now.Sub(c.last) < causeKeep) {
			keep = append(keep, c)
		}
	}
	clear(a.causes[len(keep):])
	a.causes = keep
	if len(a.ext) == 0 || (now.Sub(a.extLast) < extQuiet && now.Sub(a.extFirst) < extMax) ||
		(now.Sub(a.extFirst) < extHold && a.extAwaitsTake()) {
		a.mu.Unlock()
		return
	}
	es := a.extEntries(a.ext, a.extFirst)
	a.ext = map[int]map[int]*extLevel{}
	a.mu.Unlock()
	for _, e := range es {
		a.record(e)
	}
}

// flushFound writes the changes found on dst now, ahead of the rest of
// the burst, so they come before the take whose check found them rather
// than after it, where they'd look like what dst has now.
func (a *App) flushFound(dst int) {
	a.mu.Lock()
	found := map[int]*extLevel{}
	for ui, l := range a.ext[dst] {
		if l.found {
			found[ui] = l
			delete(a.ext[dst], ui)
		}
	}
	if len(a.ext[dst]) == 0 {
		delete(a.ext, dst)
	}
	// Grouped by now, not the burst's start: the rest of the burst, written
	// later, gets that one.
	es := a.extEntries(map[int]map[int]*extLevel{dst: found}, time.Now())
	a.mu.Unlock()
	for _, e := range es {
		a.record(e)
	}
}

// extAwaitsTake reports whether a pending change may follow one of our
// takes still waiting for the router. Caller holds mu.
func (a *App) extAwaitsTake() bool {
	for _, c := range a.causes {
		if c.done {
			continue
		}
		for dst, ls := range a.ext {
			for _, l := range ls {
				if !l.found && l.src == c.src && dst != c.dst {
					return true
				}
			}
		}
	}
	return false
}

// extEntries turns the pending burst into history entries, one per
// destination, source and found, dropping levels that ended where they
// began, earliest first. Changes that follow one of our takes name it;
// the rest share a group when there's more than one, found ones apart.
// Group ids are made from at. Caller holds mu.
func (a *App) extEntries(burst map[int]map[int]*extLevel, at time.Time) []*Entry {
	type key struct {
		dst, src int
		found    bool
	}
	type out struct {
		e     *Entry
		first time.Time
	}
	byKey := map[key]*out{}
	var outs []*out
	for dst, ls := range burst {
		for ui, l := range ls {
			// Back where it began, or cleared (nothing to show as a take).
			if (l.prevKnown && l.prev == l.src) || l.src < 0 {
				continue
			}
			k := key{dst, l.src, l.found}
			o := byKey[k]
			if o == nil {
				o = &out{e: &Entry{Src: l.src, Dst: dst, Prev: [][2]int{}, OK: true, Ext: true, Found: l.found,
					SrcName: a.srcNames[l.src], DstName: a.dstNames[dst]}, first: l.first}
				byKey[k] = o
				outs = append(outs, o)
			}
			o.first = minTime(o.first, l.first)
			o.e.Levels = append(o.e.Levels, ui)
			if l.prevKnown {
				o.e.Prev = append(o.e.Prev, [2]int{ui, l.prev})
			}
		}
	}
	sort.Slice(outs, func(i, j int) bool {
		a, b := outs[i], outs[j]
		if !a.first.Equal(b.first) {
			return a.first.Before(b.first)
		}
		if a.e.Dst != b.e.Dst {
			return a.e.Dst < b.e.Dst
		}
		if a.e.Src != b.e.Src {
			return a.e.Src < b.e.Src
		}
		return !a.e.Found && b.e.Found
	})
	es := make([]*Entry, len(outs))
	var takes, found []*Entry
	for i, o := range outs {
		e := o.e
		e.Time = o.first
		sort.Ints(e.Levels)
		sort.Slice(e.Prev, func(i, j int) bool { return e.Prev[i][0] < e.Prev[j][0] })
		if e.Found {
			found = append(found, e)
		} else if c := a.followed(e.Src, e.Dst, o.first); c != nil {
			id := c.id
			e.Follow = &id
		} else {
			takes = append(takes, e)
		}
		es[i] = e
	}
	for p, g := range map[string][]*Entry{"x": takes, "f": found} {
		if len(g) < 2 {
			continue
		}
		id := p + strconv.FormatInt(at.UnixNano(), 36)
		for _, e := range g {
			e.Group, e.GroupN = id, len(g)
		}
	}
	return es
}

// followed is the take of ours a change of src to dst at t seems to
// follow, or nil. A salvo of one source leaves several that fit, and
// timing can't tell them apart; a destination follows the same one each
// time (a PiP its DSM), so when only one fits it's remembered, and a
// salvo goes by that. Failing that, the take whose last change came
// latest. Caller holds mu.
func (a *App) followed(src, dst int, t time.Time) *cause {
	var fits []*cause
	for _, c := range a.causes {
		if c.leads(src, dst, t) {
			fits = append(fits, c)
		}
	}
	switch len(fits) {
	case 0:
		return nil
	case 1:
		a.leaders[dst] = fits[0].dst
		return fits[0]
	}
	best := fits[0]
	for _, c := range fits {
		if lead, ok := a.leaders[dst]; ok && c.dst == lead {
			return c
		}
		if c.last.After(best.last) {
			best = c
		}
	}
	return best
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
