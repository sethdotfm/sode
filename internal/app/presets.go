package app

import (
	"strings"
	"unicode"

	"github.com/sethdotfm/sode/internal/config"
)

// A fired preset is an ordinary take and needs no PIN.
func (a *App) SavePresets(c *client, presets []config.Preset) error {
	return a.saveList(c, func(cur *config.Config) error {
		was := cur.Presets
		cur.Presets = a.namePresets(cur.UID, config.WithPresetExtras(presets, cur.Presets))
		cur.Bookmarks = config.RenamedPresetBookmarks(cur.Bookmarks, "global", was, cur.Presets)
		return nil
	}, func(cfg config.Config) any {
		return presetsMsg{T: "presets", Presets: cfg.Presets, Bookmarks: cfg.Bookmarks}
	})
}

// The file keeps the save-time name for import and Broken checks; a number with no router name keeps its stored name.
// Normalize fills a blank router.
func (a *App) namePresets(uid string, ps []config.Preset) []config.Preset {
	a.mu.Lock()
	defer a.mu.Unlock()
	w := config.RefWalk{Fn: func(kind string, f config.Ref) []config.Ref {
		n := sideNames(kind, a.router.srcNames, a.router.dstNames)[f.N]
		if f.Router != "" && f.Router != uid || strings.TrimSpace(n) == "" {
			return nil
		}
		f.Name = n
		return []config.Ref{f}
	}}
	for i := range ps {
		w.Preset(&ps[i])
	}
	return ps
}

// cleanPresetName: safe to log (no control characters, trimmed, 32 runes). Not matched
// against cfg.Presets: a browser's own presets exist only there.
func cleanPresetName(s string) string { return cleanText(s, 32) }

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
