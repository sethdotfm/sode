package app

import "time"

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

// follows is how changes the router makes by itself are tied to our takes:
// the recent takes (causes) and which destination leads which (leaders).
// Guarded by App.mu, which its callers hold.
type follows struct {
	causes  []*cause    // recent takes that changes elsewhere may follow
	leaders map[int]int // dst → the dst it was last seen following
}

// expire forgets takes that can no longer be followed.
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

// awaitsTake reports whether a pending change may follow one of our
// takes still waiting for the router. Caller holds mu.
func (f *follows) awaitsTake(pending map[int]map[int]*extLevel) bool {
	for _, c := range f.causes {
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
