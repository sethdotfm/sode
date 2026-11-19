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

// namePresets puts the router's names into each reference to this router. The file keeps the
// name at save time for import and Broken checks. A number with no router name keeps its name.
// A blank router is this one (Normalize fills it after).
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
