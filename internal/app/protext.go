package app

import (
	"sort"
	"strconv"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// Protects and Router locks that another system set or freed (docs/agents/history.md). They come one level at a
// time, as routes do. Sode collects them into a burst with the External timers (extQuiet, extMax). It writes one
// History entry for each destination, device, kind and direction. The first state the router reports for a level is
// the baseline, as for routes; an announced change (0x8D, 0x8F) is always a change.

// holdKindWord: the API's and History's word for the router's kind.
func holdKindWord(kind int) string {
	if kind == swp08.KindRouterLock {
		return "routerLock"
	}
	return "protect"
}

type protExtKey struct {
	dst, dev, kind int
	set, found     bool
}

type protExtLevels struct {
	levels map[int]bool
	first  time.Time
}

// noteProtect: h appeared (set) or went (!set) on UI level ui of dst. known: the router reported this level before.
// Caller holds mu.
func (a *App) noteProtect(r *runtime, dst, ui int, h protHold, set, known, announced bool) {
	if !known && !announced {
		return
	}
	if a.sodesProtChange(r, dst, h, set) {
		return
	}
	now := time.Now()
	k := protExtKey{dst: dst, dev: h.dev, kind: h.kind, set: set, found: !announced}
	b := a.router.protExt[k]
	if b == nil {
		b = &protExtLevels{levels: map[int]bool{}, first: now}
		a.router.protExt[k] = b
	}
	b.levels[ui] = true
	if a.router.protAt[0].IsZero() {
		a.router.protAt[0] = now
	}
	a.router.protAt[1] = now
}

// sodesProtChange: Sode made this change, so its own Protect or Free Protect entry has it already. A Protect with
// Sode's device number appearing is Sode's. One going is Sode's only while Sode frees it; another system that frees
// Sode's Protect gets an entry. Caller holds mu.
func (a *App) sodesProtChange(r *runtime, dst int, h protHold, set bool) bool {
	if !set {
		for w := range a.protWaits {
			if w.dst == dst && !w.set && w.dev == h.dev {
				return true
			}
		}
		return false
	}
	recDev := 0
	if rec, ok := a.protects.get(r.cfg.UID, dst); ok {
		recDev = rec.Device
	}
	return sodesHold(h, r.cfg.ProtectDevice, recDev)
}

// flushProtExt writes the burst when it is quiet for extQuiet or has gone on for extMax.
func (a *App) flushProtExt() {
	a.mu.Lock()
	es := a.protExtEntriesMu(time.Now())
	a.mu.Unlock()
	for _, e := range es {
		a.record(e)
	}
}

// Caller holds mu.
func (a *App) protExtEntriesMu(now time.Time) []*Entry {
	rs := a.router
	if len(rs.protExt) == 0 || (now.Sub(rs.protAt[1]) < extQuiet && now.Sub(rs.protAt[0]) < extMax) {
		return nil
	}
	var es []*Entry
	for k, b := range rs.protExt {
		e := &Entry{Src: -1, Dst: k.dst, DstName: rs.dstNames[k.dst], Prev: [][2]int{}, OK: true, Ext: true, Found: k.found,
			Protect: "clear", HoldKind: holdKindWord(k.kind), Device: k.dev, Time: b.first}
		if k.set {
			e.Protect = "set"
		}
		for ui := range b.levels {
			e.Levels = append(e.Levels, ui)
		}
		sort.Ints(e.Levels)
		for _, ui := range e.Levels {
			if s, ok := rs.tally[k.dst][ui]; ok {
				e.Prev = append(e.Prev, [2]int{ui, s})
			}
		}
		es = append(es, e)
	}
	// Oldest first; a hold that went is written before the one that took its place.
	sort.Slice(es, func(i, j int) bool {
		x, y := es[i], es[j]
		if !x.Time.Equal(y.Time) {
			return x.Time.Before(y.Time)
		}
		if x.Dst != y.Dst {
			return x.Dst < y.Dst
		}
		if x.Protect != y.Protect {
			return x.Protect == "clear"
		}
		return x.Device < y.Device
	})
	// One Group for each kind, direction and found in the burst, so each Group has one chip, as ext routes have
	// (ext_burst.go). The "p" keeps them apart from those.
	groups := map[[3]string][]*Entry{}
	for _, e := range es {
		k := [3]string{e.HoldKind, e.Protect, strconv.FormatBool(e.Found)}
		groups[k] = append(groups[k], e)
	}
	for k, g := range groups {
		if len(g) < 2 {
			continue
		}
		p := "xp"
		if k[2] == "true" {
			p = "fp"
		}
		id := p + strconv.FormatInt(rs.protAt[0].UnixNano(), 36) + k[0] + k[1]
		for _, e := range g {
			e.Group, e.GroupN = id, len(g)
		}
	}
	rs.protExt = map[protExtKey]*protExtLevels{}
	rs.protAt = [2]time.Time{}
	return es
}
