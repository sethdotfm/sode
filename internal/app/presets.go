package app

import (
	"strings"
	"unicode"

	"github.com/sethdotfm/sode/internal/config"
)

// Firing a preset is an ordinary take and needs no PIN.
func (a *App) SavePresets(c *client, presets []config.Preset) error {
	return a.saveShared(c, func(cur *config.Config) error {
		cur.Presets = config.WithPresetExtras(presets, cur.Presets)
		return nil
	}, func(cfg config.Config) any { return presetsMsg{T: "presets", Presets: cfg.Presets} })
}

// cleanPresetName: safe to log (no control characters, trimmed, 32 runes). Not matched
// against cfg.Presets: a browser's own presets exist only there.
func cleanPresetName(s string) string { return cleanText(s, 32) }

// cleanText drops non-printing runes, keeps at most max runes, then trims.
func cleanText(s string, max int) string {
	r := make([]rune, 0, max)
	for _, c := range s {
		if !unicode.IsPrint(c) {
			continue
		}
		r = append(r, c)
		if len(r) == max {
			break
		}
	}
	return strings.TrimSpace(string(r))
}
