package app

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// --- wrong-PIN pools

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

// userPoolMu returns the pool of u's name; nil u is every name that matches no user. Keyed by
// user id, so a rename keeps the count. Names that match no user always wait: nobody signs in
// with them. Administrator names wait even with the Wait after wrong PINs switch off.
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

// tryUserPin runs check in the pool of u's name.
func (a *App) tryUserPin(u *config.User, from string, check func() bool) error {
	a.mu.Lock()
	p := a.userPoolMu(u)
	a.mu.Unlock()
	return a.tryPin(p, from, check)
}

// tryPin refuses while waiting or busy, else runs check with mu free (hashing is slow). A right
// PIN resets the count. Takes and releases mu itself; callers needing a stable world re-check
// under their own lock.
func (a *App) tryPin(p *pinPool, from string, check func() bool) error {
	a.mu.Lock()
	if p.forget > 0 && p.n > 0 && time.Since(p.last) > p.forget {
		p.n, p.until = 0, time.Time{}
	}
	switch left := time.Until(p.until); {
	case left > 0 && p.wait != nil:
		a.mu.Unlock()
		return fmt.Errorf("too many wrong PINs; try again in %s", left.Round(time.Second))
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
		return fmt.Errorf("wrong PIN; try again in %s", w)
	}
	return errors.New("wrong PIN")
}
