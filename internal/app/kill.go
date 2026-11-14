package app

import (
	"errors"
	"log"
	"time"
)

// killFlush bounds how long a Kill waits to queue and write its Communication Log row before
// it exits anyway: a stuck disk must not block an emergency stop.
const killFlush = time.Second

var (
	errKillOff   = errors.New("kill is turned off; turn it on in Settings > Access")
	errKillNoPin = errors.New("kill needs an Advanced Settings PIN; set one first")
)

func (a *App) serveKill(c *client, m *inMsg) {
	if err := a.Kill(c, m.Pin); err != nil {
		a.replyOp(c, "kill", m.ID, 0, "", err)
	}
}

// Kill stops the process at once with exit code 0, so a restart policy of on-failure keeps
// Sode down. No graceful shutdown: unsent takes are dropped on purpose. The PIN is checked
// again here even on an unlocked connection, and a wrong one counts in the settings pool.
func (a *App) Kill(c *client, pin string) error {
	if !a.r().cfg.KillEnabled {
		return errKillOff
	}
	// checkAdmin passes any PIN when it reads none, so its read decides: a PIN removed after
	// admit can't let a Kill through.
	hash, err := a.checkAdmin(c, pin)
	if err != nil {
		return err
	}
	if hash == "" {
		return errKillNoPin
	}
	who := c.who()
	log.Printf("kill: Sode stopped by %s", who)
	e := event(kindCommand, "kill", who, "ok")
	e.Time = time.Now()
	it := evItem{Sent: e, done: make(chan struct{})}
	// Unlike logEvent this waits for room in a full queue, so the row is not dropped.
	limit := time.After(killFlush)
	select {
	case a.evq <- it:
		select {
		case <-it.done:
		case <-limit:
		}
	case <-limit:
		a.txLost.Add(1)
	}
	a.exit(0)
	return nil
}
