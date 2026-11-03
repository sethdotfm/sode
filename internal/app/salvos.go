package app

import (
	"github.com/sethdotfm/sode/internal/config"
)

// SaveSalvos replaces the salvos for everyone, from c (the Salvos dialog).
// Like filter sets they're settings, so a settings PIN guards them. Firing a
// salvo is an ordinary batch from the browser and needs no PIN.
func (a *App) SaveSalvos(c *client, salvos []config.Salvo) error {
	return a.saveShared(c, func(cur *config.Config) error {
		cur.Salvos = config.CloneSalvos(salvos)
		return nil
	}, func(cfg config.Config) any { return salvosMsg{T: "salvos", Salvos: cfg.Salvos} })
}
