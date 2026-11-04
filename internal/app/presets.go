package app

import (
	"strings"
	"unicode"

	"github.com/sethdotfm/sode/internal/config"
)

// SavePresets replaces the presets for everyone, from c (the Presets dialog).
// Like filter sets they're settings, so a settings PIN guards them. Firing a
// preset is an ordinary batch from the browser and needs no PIN.
func (a *App) SavePresets(c *client, presets []config.Preset) error {
	return a.saveShared(c, func(cur *config.Config) error {
		cur.Presets = config.ClonePresets(presets)
		return nil
	}, func(cfg config.Config) any { return presetsMsg{T: "presets", Presets: cfg.Presets} })
}

// cleanPresetName makes a browser-supplied preset name safe to log: control
// and non-printable characters dropped, trimmed, at most 32 runes (the preset
// name limit). It is not matched against cfg.Presets because a browser's own
// presets exist only there.
func cleanPresetName(s string) string {
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
