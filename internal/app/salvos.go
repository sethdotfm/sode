package app

import (
	"fmt"
	"log"

	"github.com/sethdotfm/sode/internal/config"
)

// SaveSalvos replaces the salvos for everyone, from c (the Salvos dialog).
// Like filter sets they're settings, so a settings PIN guards them. Firing a
// salvo is an ordinary batch from the browser and needs no PIN.
func (a *App) SaveSalvos(c *client, salvos []config.Salvo) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.locked(c) {
		return errLocked
	}
	_, nr, err := a.update(func(cur *config.Config) error {
		cur.Salvos = config.CloneSalvos(salvos)
		return nil
	})
	if err != nil {
		return err
	}
	// Only the salvos changed: browsers get them alone, not a full snapshot
	// that would reset their Recent.
	a.mu.Lock()
	a.rt.Store(nr)
	a.broadcast(salvosMsg{T: "salvos", Salvos: nr.cfg.Salvos})
	a.mu.Unlock()
	return nil
}

// handleSalvos answers a "salvos" message with a "salvosResult".
func (a *App) handleSalvos(c *client, salvos []config.Salvo, id int) {
	res := opResult{T: "salvosResult", OK: true, ID: &id}
	err := a.SaveSalvos(c, salvos)
	a.command(c, "salvos", outcome(err), fmt.Sprintf("%d %s", len(salvos), plural(len(salvos), "salvo", "salvos")), err)
	if err != nil {
		res.OK, res.Err = false, err.Error()
		res.Locked = locked(err)
	} else {
		log.Printf("config: salvos saved by %s", c.ip)
	}
	a.reply(c, res)
}
