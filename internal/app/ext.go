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

const (
	extQuiet = 500 * time.Millisecond // a burst ends after this long with no changes
	extMax   = 3 * time.Second        // or this long after it began
	// ownFor is how long a Connected matching one of our connects counts
	// as ours: the take's whole wait, and some slack for a late reply.
	ownFor = takeTimeout + takeGrace + 5*time.Second
)

type extLevel struct {
	prev      int // src before the burst
	prevKnown bool
	src       int
}

type ownKey struct{ dst, wire, src int }

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
		l.src = xp.Src
	} else {
		d[ui] = &extLevel{prev: old, prevKnown: known, src: xp.Src}
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
	if len(a.ext) == 0 || (now.Sub(a.extLast) < extQuiet && now.Sub(a.extFirst) < extMax) {
		a.mu.Unlock()
		return
	}
	es := a.extEntries()
	a.ext = map[int]map[int]*extLevel{}
	a.mu.Unlock()
	for _, e := range es {
		a.record(e)
	}
}

// extEntries turns the pending burst into history entries, one per
// destination and source, dropping levels that ended where they began.
// Caller holds mu.
func (a *App) extEntries() []*Entry {
	type key struct{ dst, src int }
	byKey := map[key]*Entry{}
	var es []*Entry
	for dst, ls := range a.ext {
		for ui, l := range ls {
			// Back where it began, or cleared (nothing to show as a take).
			if (l.prevKnown && l.prev == l.src) || l.src < 0 {
				continue
			}
			k := key{dst, l.src}
			e := byKey[k]
			if e == nil {
				e = &Entry{Time: a.extFirst, Src: l.src, Dst: dst, Prev: [][2]int{}, OK: true, Ext: true,
					SrcName: a.srcNames[l.src], DstName: a.dstNames[dst]}
				byKey[k] = e
				es = append(es, e)
			}
			e.Levels = append(e.Levels, ui)
			if l.prevKnown {
				e.Prev = append(e.Prev, [2]int{ui, l.prev})
			}
		}
	}
	for _, e := range es {
		sort.Ints(e.Levels)
		sort.Slice(e.Prev, func(i, j int) bool { return e.Prev[i][0] < e.Prev[j][0] })
	}
	sort.Slice(es, func(i, j int) bool {
		if es[i].Dst != es[j].Dst {
			return es[i].Dst < es[j].Dst
		}
		return es[i].Src < es[j].Src
	})
	if len(es) > 1 {
		g := "x" + strconv.FormatInt(a.extFirst.UnixNano(), 36)
		for _, e := range es {
			e.Group, e.GroupN = g, len(es)
		}
	}
	return es
}
