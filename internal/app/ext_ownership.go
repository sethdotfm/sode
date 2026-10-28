package app

import (
	"slices"
	"time"
)

type ownKey struct{ dst, wire, src int }

// ownership is who the router's answers belong to: the connects our takes
// sent (own) and the routes of takes that gave up, which a late answer may
// still claim (late, lateTake, lateHits). Guarded by App.mu, which its
// callers hold.
type ownership struct {
	own      map[ownKey]time.Time  // our takes' connects → when to stop treating them as ours
	late     map[ownKey]time.Time  // finished takes' unconfirmed routes → until when a tally of one is ours
	lateTake map[ownKey][]lateTake // finished takes' unconfirmed routes, with the history entry to amend if one turns up (#91)
	lateHits []lateHit             // late answers awaiting their entry's amendment (flushLate)
}

// ownAnswer reports whether a Connected for k is the answer to one of our
// connects, and uses the key up either way.
func (o *ownership) ownAnswer(k ownKey, now time.Time) bool {
	if until, ok := o.own[k]; ok {
		delete(o.own, k)
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

// expire drops the keys and claims that have run out.
func (o *ownership) expire(now time.Time) {
	for k, lts := range o.lateTake {
		lts = slices.DeleteFunc(lts, func(lt lateTake) bool { return now.After(lt.until) })
		if len(lts) == 0 {
			delete(o.lateTake, k)
		} else {
			o.lateTake[k] = lts
		}
	}
	for k, until := range o.own {
		if now.After(until) {
			delete(o.own, k)
		}
	}
	for k, until := range o.late {
		if now.After(until) {
			delete(o.late, k)
		}
	}
}
