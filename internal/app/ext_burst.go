package app

import (
	"sort"
	"strconv"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// burst: routes changed elsewhere pile up in pending until quiet. Guarded by App.mu.
type burst struct {
	pending     map[int]map[int]*extLevel
	first, last time.Time
}

type extLevel struct {
	prev      int // src before the burst
	prevKnown bool
	src       int
	first, at time.Time
	found     bool // the last change came as a tally, not a Connected
}

// extChange: old/known is the cached src before; found = came as a tally, not a Connected.
type extChange struct {
	xp           swp08.Crosspoint
	ui, old      int
	known, found bool
	at           time.Time
}

func (b *burst) add(c extChange) {
	xp, ui, old, known, found, now := c.xp, c.ui, c.old, c.known, c.found, c.at
	if len(b.pending) == 0 {
		b.first = now
	}
	b.last = now
	d := b.pending[xp.Dst]
	if d == nil {
		d = map[int]*extLevel{}
		b.pending[xp.Dst] = d
	}
	if l := d[ui]; l != nil {
		l.src, l.at, l.found = xp.Src, now, found
	} else {
		d[ui] = &extLevel{prev: old, prevKnown: known, src: xp.Src, first: now, at: now, found: found}
	}
}

// flush: finished when quiet for extQuiet or going for extMax; awaits can hold it up to
// extHold.
func (b *burst) flush(now time.Time, awaits func(pending map[int]map[int]*extLevel) bool) (changes map[int]map[int]*extLevel, first time.Time, ok bool) {
	if len(b.pending) == 0 || (now.Sub(b.last) < extQuiet && now.Sub(b.first) < extMax) ||
		(now.Sub(b.first) < extHold && awaits(b.pending)) {
		return nil, time.Time{}, false
	}
	changes, first = b.pending, b.first
	b.pending = map[int]map[int]*extLevel{}
	return changes, first, true
}

// entries: one per destination, source and found, levels ending where they began dropped,
// earliest first. Followers name their take; the rest share a group when more than one,
// found apart. Group ids come from at. Caller holds mu.
func (b *burst) entries(name func(dst bool, n int) string, changes map[int]map[int]*extLevel, at time.Time, followed func(src, dst int, t time.Time) *cause) []*Entry {
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
	for dst, ls := range changes {
		for ui, l := range ls {
			if (l.prevKnown && l.prev == l.src) || l.src < 0 {
				continue
			}
			k := key{dst, l.src, l.found}
			o := byKey[k]
			if o == nil {
				o = &out{e: &Entry{Src: l.src, Dst: dst, Prev: [][2]int{}, OK: true, Ext: true, Found: l.found,
					SrcName: name(false, l.src), DstName: name(true, dst)}, first: l.first}
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
		} else if c := followed(e.Src, e.Dst, o.first); c != nil {
			if c.group != "" {
				// Which take of a multi-take confirm is a guess: tied to the whole
				// confirm, not a panel route (no Group) nor a follow of one take.
				e.FollowGroup = c.group
			} else {
				id := c.id
				e.Follow = &id
			}
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

func minTime(a, b time.Time) time.Time {
	if b.Before(a) {
		return b
	}
	return a
}
