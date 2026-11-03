package app

import (
	"strings"
	"unicode"

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

// cleanSalvoName makes a browser-supplied salvo name safe to log: control
// and non-printable characters dropped, trimmed, at most 32 runes (the salvo
// name limit). It is not matched against cfg.Salvos because a browser's own
// salvos exist only there.
func cleanSalvoName(s string) string {
	r := make([]rune, 0, 32)
	for _, c := range s {
		if !unicode.IsPrint(c) {
			continue
		}
		r = append(r, c)
		if len(r) == 32 {
			break
		}
	}
	return strings.TrimSpace(string(r))
}
