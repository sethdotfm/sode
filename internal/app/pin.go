package app

import (
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// --- settings PIN

// pinState counts wrong settings PINs from anyone, in a row.
type pinState struct {
	n     int
	until time.Time // no tries before this
	busy  bool      // a PIN is being checked
}

const (
	pinFreeTries = 4 // wrong PINs before waits start
	pinFirstWait = 30 * time.Second
	pinMaxWait   = time.Hour
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

// Unlock checks a settings PIN from c and, if it's right, unlocks c for the
// current PIN only: changing or removing the PIN locks it again. Wrong PINs
// count for everyone, since IPs behind a proxy or rotated are no guard: one
// PIN is checked at a time, waits double after pinFreeTries, and
// PinMaxTries in a row lock settings until reset or restart.
func (a *App) Unlock(c *client, pin string) error {
	hash := a.r().cfg.SettingsPin
	if hash == "" {
		return nil
	}
	if err := a.pinStart(); err != nil {
		return err
	}
	ok := config.CheckPin(hash, pin)

	a.mu.Lock()
	defer a.mu.Unlock()
	if err := a.pinEnd(ok, c.ip); err != nil {
		return err
	}
	if a.r().cfg.SettingsPin != hash {
		return errLocked
	}
	c.pin = hash
	return nil
}

// pinStart begins one PIN check (the settings PIN, a user PIN or the admin
// password: they share one count and one backoff, so a guesser gains nothing
// by switching between them). It refuses while locked out, waiting or while
// another check runs. Pair it with pinEnd.
func (a *App) pinStart() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if max := a.r().cfg.PinMaxTries; max > 0 && a.pin.n >= max {
		return fmt.Errorf("settings are locked after %d wrong PINs. On the Docker host run: docker compose exec sode /sode -reset-pin, then restart Sode", max)
	}
	if left := time.Until(a.pin.until); left > 0 {
		return fmt.Errorf("too many wrong PINs; try again in %s", left.Round(time.Second))
	}
	if a.pin.busy {
		return errors.New("another PIN is being checked; try again")
	}
	a.pin.busy = true
	return nil
}

// pinEnd ends the check pinStart began: nil if it was right, else the
// reason, counting it. Caller holds mu.
func (a *App) pinEnd(ok bool, from string) error {
	a.pin.busy = false
	if ok {
		a.pin = pinState{}
		return nil
	}
	a.pin.n++
	log.Printf("settings: wrong PIN from %s (%d in a row)", from, a.pin.n)
	if max := a.r().cfg.PinMaxTries; max > 0 && a.pin.n >= max {
		log.Printf("settings: locked after %d wrong PINs; run sode -reset-pin or restart to unlock", a.pin.n)
		return fmt.Errorf("wrong PIN; settings are now locked after %d wrong PINs. On the Docker host run: docker compose exec sode /sode -reset-pin, then restart Sode", a.pin.n)
	}
	if w := pinWait(a.pin.n); w > 0 {
		a.pin.until = time.Now().Add(w)
		return fmt.Errorf("wrong PIN; try again in %s", w)
	}
	return errors.New("wrong PIN")
}

// Lock forgets c's unlock.
func (a *App) Lock(c *client) {
	a.mu.Lock()
	c.pin = ""
	a.mu.Unlock()
}

// locked reports whether c may not change settings.
func (a *App) locked(c *client) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	hash := a.r().cfg.SettingsPin
	return hash != "" && c.pin != hash
}
