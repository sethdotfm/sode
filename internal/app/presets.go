package app

import (
	"strings"
	"unicode"

	"github.com/sethdotfm/sode/internal/config"
)

// Firing a preset is an ordinary take and needs no PIN.
func (a *App) SavePresets(c *client, presets []config.Preset) error {
	return a.saveList(c, func(cur *config.Config) error {
		cur.Presets = a.namePresets(cur.UID, config.WithPresetExtras(presets, cur.Presets))
		return nil
	}, func(cfg config.Config) any { return presetsMsg{T: "presets", Presets: cfg.Presets} })
}

// The file keeps the save-time name for import and Broken checks; a number with no router name keeps its stored name.
// Normalize fills a blank router.
func (a *App) namePresets(uid string, ps []config.Preset) []config.Preset {
	a.mu.Lock()
	defer a.mu.Unlock()
	name := func(f *config.Ref, names map[int]string) {
		if f.Router != "" && f.Router != uid {
			return
		}
		if n, ok := names[f.N]; ok && strings.TrimSpace(n) != "" {
			f.Name = n
		}
	}
	for i := range ps {
		for j := range ps[i].Items {
			it := &ps[i].Items[j]
			name(&it.Src, a.router.srcNames)
			for k := range it.Dsts {
				name(&it.Dsts[k], a.router.dstNames)
			}
		}
	}
	return ps
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
