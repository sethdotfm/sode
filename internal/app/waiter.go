package app

import (
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// confirmedBy: how the router answered a level; waiter.see sets it one time.
type confirmedBy uint8

func (l *routeLevel) confirmed() bool { return l.by != pending }

func (l *routeLevel) tallied() bool { return l.by == byTally || l.by == byTallyAlready }

func (l *routeLevel) already() bool { return l.by == byTallyAlready }

type routeLevel struct {
	ui, wire int
	connect  []byte
	same     bool        // the cache said it was already on src
	unsent   bool        // the router link refused it
	by       confirmedBy // how the router confirmed it (see decides one time)
	at       time.Time
	native   bool // staged in the take's group salvo (fire.go): confirmed by a Connected or a tally
	seen     bool // the router reported some other route for the level...
	now      int  // ...this src (-1 = nothing routed), the latest report
}

type waiter struct {
	u        *routerUnit // the router of the route
	dst, src int
	why      swp08.Why // the connects' reason and browser, for the Communication Log
	levels   []*routeLevel
	done     chan struct{}
	closed   bool
	cause    *cause // for changes that follow this take
	// ext: tracker of the router the take started on; a different u.ext means stale.
	ext  *extTracker
	cut  bool      // the router changed under the take
	goAt time.Time // when a native salvo's Go set its staged levels (fire.go); zero if none
}

var errRouterChanged = errors.New("the router changed")

func wanted(l *routeLevel) bool { return !l.confirmed() && !l.unsent }

func (w *waiter) filter(keep func(*routeLevel) bool) []*routeLevel {
	var ls []*routeLevel
	for _, l := range w.levels {
		if keep(l) {
			ls = append(ls, l)
		}
	}
	return ls
}

// Routers announce only changes, so an interrogate answer confirms a level already on src.
// Other reports stay in seen and now for the failure reason. Caller holds mu.
func (w *waiter) see(xp swp08.Crosspoint) bool {
	if xp.Dst != w.dst {
		return false
	}
	for _, l := range w.levels {
		if l.wire != xp.Level || !wanted(l) {
			continue
		}
		if xp.Src != w.src {
			l.seen, l.now = true, xp.Src
			return false
		}
		// The router does not always announce a Go: confirmed by Connected or tally, "already
		// there" only if the cache said so.
		l.by, l.at = byConnected, time.Now()
		if !xp.Changed() {
			l.by = byTally
			if !l.native || l.same {
				l.by = byTallyAlready
			}
		}
		w.finishIfDone()
		return true
	}
	return false
}

// awaited: a take in flight (or recently given up) on router u asked for xp, so its tally is ours without a
// Connected. Caller holds mu.
func (u *routerUnit) awaited(xp swp08.Crosspoint) bool {
	if until, ok := u.ext.late[ownKey{xp.Dst, xp.Level, xp.Src}]; ok && time.Now().Before(until) {
		return true
	}
	for w := range u.waiters {
		if w.dst == xp.Dst && w.src == xp.Src && slices.ContainsFunc(w.levels, func(l *routeLevel) bool { return l.wire == xp.Level }) {
			return true
		}
	}
	return false
}

// Caller holds mu.
func (w *waiter) finishIfDone() {
	if len(w.filter(wanted)) == 0 {
		w.finish()
	}
}

func (w *waiter) finish() {
	if !w.closed {
		w.closed = true
		close(w.done)
	}
}

// register: own keys keep the take's answers out of changes made elsewhere. Only levels
// answered with a Connected get one (a level already on src would swallow a real change
// back to it). Staged levels get one too, as the Ultrix sometimes announces a Go's
// crosspoints (fire.go). Nothing for a replaced router. All of it goes in the unit of the route's router. Caller
// holds mu.
func (w *waiter) register() {
	if w.u.ext != w.ext {
		return
	}
	w.u.waiters[w] = struct{}{}
	w.ext.causes = append(w.ext.causes, w.cause)
	// Connects can queue up to queueMax; sendRoute shortens the keys to ownFor after they are sent.
	w.ownKeys()
}

// Caller holds mu and has checked w.u.ext == w.ext.
func (w *waiter) ownKeys() {
	until := time.Now().Add(w.u.queueMax + ownFor)
	for _, l := range w.levels {
		if !l.same {
			w.ext.own[ownKey{w.dst, l.wire, w.src}] = until
		}
	}
}

// unregister: unconfirmed, unsent and tally-confirmed levels lose their own keys, which
// would swallow a real change to src. mu is App.mu.
func (w *waiter) unregister(mu *sync.Mutex, id int) {
	mu.Lock()
	defer mu.Unlock()
	w.cause.id, w.cause.done = id, true
	delete(w.u.waiters, w)
	if w.ext != w.u.ext {
		return // the router changed: its keys have no place in the new one's tracker
	}
	x := w.ext
	for _, l := range w.levels {
		k := ownKey{w.dst, l.wire, w.src}
		if !l.confirmed() || l.unsent || l.already() {
			delete(x.own, k)
		} else if until, ok := x.own[k]; ok && l.native && l.tallied() && !w.goAt.IsZero() && w.goAt.Add(salvoAnnounceFor).Before(until) {
			// Set by a Go and confirmed by a tally. It is possible that a Connected follows up to
			// about 340 ms later, or never, so the key waits salvoAnnounceFor.
			x.own[k] = w.goAt.Add(salvoAnnounceFor)
		}
		// A late interrogate answer or slow Connected is still ours; the entry gets amended.
		if !l.confirmed() && !l.unsent {
			until := time.Now().Add(lateFor)
			x.late[k] = until
			// Only a level the router said nothing about can be answered late; retries of
			// one route each wait for the answer.
			if !l.seen {
				x.lateRoute[k] = append(x.lateRoute[k], lateRoute{id: id, until: until, cause: w.cause})
			}
		}
	}
}

const (
	pending        confirmedBy = iota // no answer yet
	byConnected                       // a Connected: the route changed
	byTally                           // a tally for a staged level that was elsewhere before the Go
	byTallyAlready                    // a tally: it was on src before the take
)
