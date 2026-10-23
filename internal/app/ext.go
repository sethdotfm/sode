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
// same source to another destination right after a take (ours or one made
// elsewhere), is written as a follow of that take (Entry.Follow) rather
// than a take of its own.

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
	extHold   = takeTimeout + takeGrace + 2*time.Second
	causeKeep = 5 * time.Second // a recorded take can be followed for this long
)

type extLevel struct {
	prev      int // src before the burst
	prevKnown bool
	src       int
	first, at time.Time // first and last change
}

type ownKey struct{ dst, wire, src int }

// cause is a take that changes elsewhere may follow: one of ours (id -1
// until recorded) or an entry written from a burst.
type cause struct {
	src, dst    int
	first, last time.Time // its changes as the router reported them; zero = none yet
	id          int
	sode        bool
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
	if until, ok := a.own[ownKey{xp.Dst, xp.Level, xp.Src}]; ok && time.Now().Before(until) {
		return
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
		l.src, l.at = xp.Src, now
	} else {
		d[ui] = &extLevel{prev: old, prevKnown: known, src: xp.Src, first: now, at: now}
	}
}

// flushExt writes a finished burst to history.
func (a *App) flushExt() {
	a.mu.Lock()
	now := time.Now()
	for k, until := range a.own {
		if now.After(until) {
			delete(a.own, k)
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
	outs := a.extEntries()
	a.ext = map[int]map[int]*extLevel{}
	a.mu.Unlock()

	// Takes first, so follows can name them.
	for _, o := range outs {
		if o.cause == nil && o.after == nil {
			a.record(o.e)
		}
	}
	a.mu.Lock()
	for _, o := range outs {
		if o.cause == nil && o.after == nil {
			a.causes = append(a.causes, &cause{src: o.e.Src, dst: o.e.Dst, first: o.first, last: o.last, id: o.e.ID, done: true})
		}
	}
	a.mu.Unlock()
	for _, o := range outs {
		if o.cause == nil && o.after == nil {
			continue
		}
		var id int
		if o.cause != nil {
			id = o.cause.id
		} else {
			id = o.after.e.ID
		}
		o.e.Follow = &id
		a.record(o.e)
	}
}

// extAwaitsTake reports whether a pending change may follow one of our
// takes still waiting for the router. Caller holds mu.
func (a *App) extAwaitsTake() bool {
	for _, c := range a.causes {
		if !c.sode || c.done {
			continue
		}
		for dst, ls := range a.ext {
			for _, l := range ls {
				if l.src == c.src && dst != c.dst {
					return true
				}
			}
		}
	}
	return false
}

// extOut is one entry from a burst: a take, or a follow of a recorded
// cause or of an earlier take in the same burst.
type extOut struct {
	e           *Entry
	first, last time.Time
	cause       *cause
	after       *extOut
}

// extEntries turns the pending burst into history entries, one per
// destination and source, dropping levels that ended where they began,
// earliest first. Caller holds mu.
func (a *App) extEntries() []*extOut {
	type key struct{ dst, src int }
	byKey := map[key]*extOut{}
	var outs []*extOut
	for dst, ls := range a.ext {
		for ui, l := range ls {
			// Back where it began, or cleared (nothing to show as a take).
			if (l.prevKnown && l.prev == l.src) || l.src < 0 {
				continue
			}
			k := key{dst, l.src}
			o := byKey[k]
			if o == nil {
				o = &extOut{e: &Entry{Src: l.src, Dst: dst, Prev: [][2]int{}, OK: true, Ext: true,
					SrcName: a.srcNames[l.src], DstName: a.dstNames[dst]}, first: l.first, last: l.at}
				byKey[k] = o
				outs = append(outs, o)
			}
			o.first, o.last = minTime(o.first, l.first), maxTime(o.last, l.at)
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
		return a.e.Src < b.e.Src
	})
	var takes []*extOut
	for _, o := range outs {
		e := o.e
		e.Time = o.first
		sort.Ints(e.Levels)
		sort.Slice(e.Prev, func(i, j int) bool { return e.Prev[i][0] < e.Prev[j][0] })
		o.cause = a.followed(e.Src, e.Dst, o.first)
		if o.cause == nil {
			for _, t := range takes {
				if t.e.Src == e.Src && t.e.Dst != e.Dst && !o.first.After(t.last.Add(followAfter)) {
					o.after = t
					break
				}
			}
		}
		if o.cause == nil && o.after == nil {
			takes = append(takes, o)
		}
	}
	if len(takes) > 1 {
		g := "x" + strconv.FormatInt(a.extFirst.UnixNano(), 36)
		for _, o := range takes {
			o.e.Group, o.e.GroupN = g, len(takes)
		}
	}
	return outs
}

// followed is the recorded take a change of src to dst at t seems to
// follow, ours first; or nil. Caller holds mu.
func (a *App) followed(src, dst int, t time.Time) *cause {
	var ext *cause
	for _, c := range a.causes {
		if !c.leads(src, dst, t) {
			continue
		}
		if c.sode {
			return c
		}
		if ext == nil {
			ext = c
		}
	}
	return ext
}

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}

func maxTime(a, b time.Time) time.Time {
	if b.After(a) {
		return b
	}
	return a
}
