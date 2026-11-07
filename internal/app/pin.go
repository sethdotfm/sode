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

// pinPool: wrong-PIN count and rules for one class of secret. The settings PIN and user
// PINs have separate pools so neither's success or lockout touches the other. tryPin is
// the only mutator.
type pinPool struct {
	pinState
	wait func(tries int) time.Duration // the wait after this many wrong PINs in a row
	// resetOnOK: not for the users' pool: a PIN there identifies the user, so one valid
	// PIN would let a stranger reset the count and guess the rest. That pool forgets after
	// quiet instead.
	resetOnOK bool
	forget    time.Duration           // 0 = never
	maxTries  func(config.Config) int // wrong PINs that lock for good; nil or 0 = never
	what      string                  // names the class in the log
}

const (
	pinFreeTries   = 4 // wrong PINs before waits start
	pinFirstWait   = 30 * time.Second
	pinMaxWait     = time.Hour
	userPinMaxWait = 5 * time.Minute
)

var errLocked = errors.New("settings are locked; enter the PIN again")

func pinWait(n int) time.Duration {
	if n <= pinFreeTries {
		return 0
	}
	w := pinFirstWait
	for i := pinFreeTries + 1; i < n && w < pinMaxWait; i++ {
		w *= 2
	}
	return min(w, pinMaxWait)
}

// settingsPool: PinMaxTries wrong in a row lock it until -reset-pin or restart.
func settingsPool() pinPool {
	return pinPool{what: "settings", wait: pinWait, resetOnOK: true, maxTries: func(c config.Config) int { return c.PinMaxTries }}
}

// userPool: waits stop growing at userPinMaxWait; no permanent lock, so a stranger can't
// shut out operators.
func userPool() pinPool {
	return pinPool{what: "users", wait: func(n int) time.Duration { return min(pinWait(n), userPinMaxWait) }, forget: pinMaxWait}
}

const resetHint = "On the Docker host run: docker compose exec sode /sode -reset-pin, then restart Sode"

// tryPin refuses while locked out, waiting or busy, else runs check with mu free (hashing
// is slow). Takes and releases mu itself; callers needing a stable world re-check under
// their own lock.
func (a *App) tryPin(p *pinPool, from string, check func() bool) error {
	a.mu.Lock()
	if p.forget > 0 && p.n > 0 && time.Since(p.last) > p.forget {
		p.n, p.until = 0, time.Time{}
	}
	max := 0
	if p.maxTries != nil {
		max = p.maxTries(a.r().cfg)
	}
	switch left := time.Until(p.until); {
	case max > 0 && p.n >= max:
		a.mu.Unlock()
		return fmt.Errorf("settings are locked after %d wrong PINs. %s", max, resetHint)
	case left > 0:
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
		if p.resetOnOK {
			p.pinState = pinState{}
		}
		return nil
	}
	p.n++
	p.last = time.Now()
	log.Printf("%s: wrong PIN from %s (%d in a row)", p.what, from, p.n)
	if max > 0 && p.n >= max {
		log.Printf("%s: locked after %d wrong PINs; run sode -reset-pin or restart to unlock", p.what, p.n)
		return fmt.Errorf("wrong PIN; settings are now locked after %d wrong PINs. %s", p.n, resetHint)
	}
	if w := p.wait(p.n); w > 0 {
		p.until = time.Now().Add(w)
		return fmt.Errorf("wrong PIN; try again in %s", w)
	}
	return errors.New("wrong PIN")
}

// checkAdmin returns the PIN's hash ("" when none set). Wrong PINs count for everyone: IPs
// behind a proxy are no guard.
func (a *App) checkAdmin(c *client, pin string) (string, error) {
	hash := a.r().cfg.SettingsPin
	if hash == "" {
		return "", nil
	}
	if err := a.tryPin(&a.pin, c.ip, func() bool { return config.CheckPin(hash, pin) }); err != nil {
		return "", err
	}
	if a.r().cfg.SettingsPin != hash {
		return "", errLocked
	}
	return hash, nil
}

// Unlock covers the current PIN only: changing or removing it relocks.
func (a *App) Unlock(c *client, pin string) error {
	hash, err := a.checkAdmin(c, pin)
	if err != nil || hash == "" {
		return err
	}
	a.mu.Lock()
	c.pin = hash
	a.mu.Unlock()
	return nil
}

func (a *App) Lock(c *client) {
	a.mu.Lock()
	c.pin = ""
	a.mu.Unlock()
}

func (a *App) settingsUnlocked(c *client) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settingsUnlockedLocked(c)
}

func (a *App) settingsUnlockedLocked(c *client) bool {
	hash := a.r().cfg.SettingsPin
	return hash == "" || c.pin == hash
}

// Trusted is read live, so unticking takes effect at once. Caller holds mu.
func (a *App) trustedLocked(c *client) bool {
	if a.settingsUnlockedLocked(c) {
		return true
	}
	cfg := a.r().cfg
	if !a.userOKLocked(c, cfg) {
		return false
	}
	u, _ := cfg.FindUser(c.u.user.Load().Name)
	return u.Trusted
}
