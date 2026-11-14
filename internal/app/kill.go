package app

import (
	"errors"
	"log"
	"os"
	"time"
)

// exitProcess is App.exit's default. internal/app's TestMain replaces it, so no test can end
// the test binary.
var exitProcess = os.Exit

// killFlush: how long a Kill waits for its Communication Log row before it exits anyway.
const killFlush = 100 * time.Millisecond

var (
	errKillOff   = errors.New("Kill is turned off. Turn it on in Settings > Access")
	errKillNoPin = errors.New("Kill needs an Advanced Settings PIN; set one first")
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
	cfg := a.r().cfg
	switch {
	case !cfg.KillEnabled:
		return errKillOff
	case cfg.SettingsPin == "":
		return errKillNoPin
	}
	if _, err := a.checkAdmin(c, pin); err != nil {
		return err
	}
	who := c.who()
	log.Printf("kill: Sode stopped by %s", who)
	e := event(kindCommand, "kill", who, "ok")
	e.written = make(chan struct{})
	a.logEvent(e)
	select {
	case <-e.written:
	case <-time.After(killFlush):
	}
	a.exit(0)
	return nil
}
