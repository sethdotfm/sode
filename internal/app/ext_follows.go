package app

import "time"

// cause: one of our takes that changes elsewhere may follow (id -1 until recorded).
type cause struct {
	src, dst    int
	first, last time.Time // its changes as the router reported them; zero = none yet
	sent        time.Time // when its connects had gone out; zero while they're queued
	id          int
	done        bool
	group       string // history group of the multi-take confirm it belongs to, "" if alone
}

// Caller holds mu.
func (c *cause) saw(t time.Time) {
	if c.first.IsZero() {
		c.first = t
	}
	c.last = t
}

func (c *cause) leads(src, dst int, t time.Time) bool {
	return c.done && c.id >= 0 && !c.first.IsZero() && c.src == src && c.dst != dst &&
		!t.Before(c.first.Add(-followBefore)) && !t.After(c.last.Add(followAfter))
}

// follows: recent takes (causes) and which destination leads which (leaders). Guarded by
// App.mu.
type follows struct {
	causes  []*cause
	leaders map[int]int
}

func (f *follows) expire(now time.Time) {
	keep := f.causes[:0]
	for _, c := range f.causes {
		if !c.done || (!c.last.IsZero() && now.Sub(c.last) < causeKeep) {
			keep = append(keep, c)
		}
	}
	clear(f.causes[len(keep):])
	f.causes = keep
}

// Caller holds mu.
func (f *follows) awaitsRoute(pending map[int]map[int]*extLevel) bool {
	for _, c := range f.causes {
		if c.done || c.sent.IsZero() {
			continue
		}
		for dst, ls := range pending {
			for _, l := range ls {
				if !l.found && l.src == c.src && dst != c.dst && !l.first.Before(c.sent.Add(-followBefore)) {
					return true
				}
			}
		}
	}
	return false
}

// followed: a salvo of one source leaves several fits timing can't separate; a destination
// follows the same take each time (a PiP its DSM), so a lone fit is remembered and a salvo
// goes by it; else the take whose last change came latest. Caller holds mu.
func (f *follows) followed(src, dst int, t time.Time) *cause {
	var fits []*cause
	for _, c := range f.causes {
		if c.leads(src, dst, t) {
			fits = append(fits, c)
		}
	}
	switch len(fits) {
	case 0:
		return nil
	case 1:
		f.leaders[dst] = fits[0].dst
		return fits[0]
	}
	best := fits[0]
	for _, c := range fits {
		if lead, ok := f.leaders[dst]; ok && c.dst == lead {
			return c
		}
		if c.last.After(best.last) {
			best = c
		}
	}
	return best
}
