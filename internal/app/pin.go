package app

import (
	"cmp"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

type pinState struct {
	n     int
	until time.Time // no tries before this
	last  time.Time // the last wrong PIN
	busy  bool      // a PIN is being checked
}

// pinPool: wrong-PIN count and rules for one user name's PINs. tryPin is the only mutator.
type pinPool struct {
	pinState
	wait   func(tries int) time.Duration // the wait after this many wrong PINs in a row; nil = no waits
	forget time.Duration                 // 0 = never
	what   string                        // names the pool in the log
}

const (
	pinFreeTries   = 4 // wrong PINs before waits start
	pinFirstWait   = 30 * time.Second
	pinMaxWait     = time.Hour
	userPinMaxWait = 5 * time.Minute
)

// inWords is a wait as the lock shows it ("30 seconds", "5 minutes"), not a Go duration such as "1m30s". It rounds
// up, so a short wait never reads as 0 seconds.
func inWords(d time.Duration) string {
	s := int((d + time.Second - 1) / time.Second)
	switch {
	case s <= 1:
		return "1 second"
	case s < 120:
		return fmt.Sprintf("%d seconds", s)
	}
	return fmt.Sprintf("%d minutes", (s+59)/60)
}

// userPinWait: first is App.pinFirstWait, which tests shorten.
func userPinWait(first time.Duration, n int) time.Duration {
	return doubling(first, min(userPinMaxWait, first<<10), n)
}

func doubling(first, most time.Duration, n int) time.Duration {
	if n <= pinFreeTries {
		return 0
	}
	w := first
	for i := pinFreeTries + 1; i < n && w < most; i++ {
		w *= 2
	}
	return min(w, most)
}

// Keyed by user id, so a rename keeps the count. nil u is the shared pool for unknown names, which always waits.
func (a *App) userPoolMu(u *config.User) *pinPool {
	key := ""
	if u != nil {
		key = u.ID
	}
	p := a.userPins[key]
	if p == nil {
		p = &pinPool{what: "users", forget: pinMaxWait}
		a.userPins[key] = p
	}
	first := a.pinFirstWait
	p.wait = func(n int) time.Duration { return userPinWait(first, n) }
	if u != nil {
		p.what = "user " + u.Name
		cfg := a.r().cfg
		live, ok := cfg.FindUserByID(u.ID) // u can be a sign-in copy with an old Role
		if !cfg.PinWaits && ok && live.Role != config.RoleAdministrator {
			p.wait = nil
		}
	}
	return p
}

// provePin checks pin against hit in the pool of counts (hit when nil). It runs with mu free; the
// caller re-reads the user under its own lock (livePinMu), since the check can wait.
func (a *App) provePin(c *client, counts, hit *config.User, pin string) error {
	a.mu.Lock()
	p := a.userPoolMu(cmp.Or(counts, hit))
	a.mu.Unlock()
	return a.tryPin(p, c.ip, func() bool { return pinMatches(hit, pin) })
}

// livePinMu: hit as it is now; false when it is gone or has a new PIN.
func (a *App) livePinMu(hit *config.User) (config.User, bool) {
	live, ok := a.r().cfg.FindUserByID(hit.ID)
	return live, ok && live.Pin == hit.Pin
}

// check runs with mu free, since hashing is slow; a caller that needs a stable state checks again under its own lock.
func (a *App) tryPin(p *pinPool, from string, check func() bool) error {
	a.mu.Lock()
	if p.forget > 0 && p.n > 0 && time.Since(p.last) > p.forget {
		p.n, p.until = 0, time.Time{}
	}
	switch left := time.Until(p.until); {
	case left > 0 && p.wait != nil:
		a.mu.Unlock()
		return fmt.Errorf("too many wrong PINs; try again in %s", inWords(left))
	case p.busy:
		a.mu.Unlock()
		return errors.New("another PIN is being checked; try again")
	}
	p.busy = true
	a.mu.Unlock()

	ok := check()

	a.mu.Lock()
	defer a.mu.Unlock()
	p.busy = false
	if ok {
		p.pinState = pinState{}
		return nil
	}
	p.n++
	p.last = time.Now()
	log.Printf("%s: wrong PIN from %s (%d in a row)", p.what, from, p.n)
	if p.wait == nil {
		p.until = time.Time{}
	} else if w := p.wait(p.n); w > 0 {
		p.until = time.Now().Add(w)
		return fmt.Errorf("wrong PIN; try again in %s", inWords(w))
	}
	return errors.New("wrong PIN")
}
