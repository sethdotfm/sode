package app

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// --- wrong-PIN pools

// pinState counts wrong PINs of one class from anyone, in a row.
type pinState struct {
	n     int
	until time.Time // no tries before this
	busy  bool      // a PIN is being checked
}

// pinPool is the wrong-PIN count one class of secret uses, and the rules for
// it. The settings PIN and the admin password (the same secret) share one
// pool; user PINs have their own, so a success or a lockout in one never
// touches the other. tryPin is the only thing that changes it.
type pinPool struct {
	pinState
	wait     func(tries int) time.Duration // the wait after this many wrong PINs in a row
	maxTries func(config.Config) int       // wrong PINs that lock for good; nil or 0 = never
	what     string                        // names the class in the log
}

const (
	pinFreeTries   = 4 // wrong PINs before waits start
	pinFirstWait   = 30 * time.Second
	pinMaxWait     = time.Hour
	userPinMaxWait = 5 * time.Minute
)

var errLocked = errors.New("settings are locked; enter the PIN again")

// pinWait is the wait after the nth wrong PIN in a row: none for the first
// pinFreeTries, then pinFirstWait doubling, up to pinMaxWait.
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

// settingsPool: the settings PIN and the admin password. PinMaxTries wrong
// ones in a row lock it until -reset-pin or a restart.
func settingsPool() pinPool {
	return pinPool{what: "settings", wait: pinWait, maxTries: func(c config.Config) int { return c.PinMaxTries }}
}

// userPool: user PINs. The waits stop growing at userPinMaxWait and there is
// no permanent lock: a stranger's guesses must not shut out the operators.
func userPool() pinPool {
	return pinPool{what: "users", wait: func(n int) time.Duration { return min(pinWait(n), userPinMaxWait) }}
}

const resetHint = "On the Docker host run: docker compose exec sode /sode -reset-pin, then restart Sode"

// tryPin makes one try against pool p: it refuses while the pool is locked
// out, waiting or busy with another check, else runs check (with mu free,
// since hashing is slow) and counts the answer. nil means check passed. It
// takes and releases mu itself; a caller that needs the world to be unchanged
// afterwards re-checks under its own lock.
func (a *App) tryPin(p *pinPool, from string, check func() bool) error {
	a.mu.Lock()
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
		p.pinState = pinState{}
		return nil
	}
	p.n++
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

// checkAdmin checks pin against the settings PIN and returns that PIN's hash
// (empty when none is set, in which case nothing was asked). Wrong PINs count
// for everyone, since IPs behind a proxy or rotated are no guard. Unlock and
// AdminUnlock both come through here.
func (a *App) checkAdmin(c *client, pin string) (string, error) {
	hash := a.r().cfg.SettingsPin
	if hash == "" {
		return "", nil
	}
	if err := a.tryPin(&a.pin, c.ip, func() bool { return config.CheckPin(hash, pin) }); err != nil {
		return "", err
	}
	// Changed while it was being checked?
	if a.r().cfg.SettingsPin != hash {
		return "", errLocked
	}
	return hash, nil
}

// Unlock checks a settings PIN from c and, if it's right, unlocks c for the
// current PIN only: changing or removing the PIN locks it again. One PIN is
// checked at a time; waits double after pinFreeTries and PinMaxTries in a
// row lock settings until reset or restart.
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

// Lock forgets c's unlock.
func (a *App) Lock(c *client) {
	a.mu.Lock()
	c.pin = ""
	a.mu.Unlock()
}

// settingsUnlocked reports whether c may change settings: no settings PIN is
// set, or c entered the one in force.
func (a *App) settingsUnlocked(c *client) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.settingsUnlockedLocked(c)
}

// settingsUnlockedLocked is settingsUnlocked; caller holds mu.
func (a *App) settingsUnlockedLocked(c *client) bool {
	hash := a.r().cfg.SettingsPin
	return hash == "" || c.pin == hash
}
