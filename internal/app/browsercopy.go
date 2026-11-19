package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Copy this browser to My user (docs/agents/profiles.md): a profileSave of the connection's own
// user that also writes one Settings saves row. Never Critical, since it touches only that user.

var errCopyNoUser = errors.New("sign in to copy this browser to your user")

// prefLabels: the This browser labels in index.html (TestPrefLabelsMatchPage).
var prefLabels = map[string]string{
	"theme":         "Theme",
	"layout":        "Layout",
	"clock":         "Clock",
	"flip":          "Destination before Source",
	"scrollTop":     "Scroll to top on focus",
	"searchFilters": "Searching history respects filters",
	"setStack":      "Stacked filters behavior",
	"stackGap":      "Stack quick changes to one destination",
	"nextMode":      "Next match behavior",
	"compact":       "Compact view",
}

// BrowserCopy saves req to the profile of the user signed in on c.
func (a *App) BrowserCopy(c *client, req profileReq) error {
	a.cfgMu.Lock()
	a.mu.Lock()
	ok := a.userOKMu(c, a.r().cfg)
	a.mu.Unlock()
	if !ok {
		a.cfgMu.Unlock()
		return errCopyNoUser
	}
	id := c.userID()
	var was, now config.Profile
	var owner string
	err := a.commit(func(cfg *config.Config) error {
		for i := range cfg.Users {
			u := &cfg.Users[i]
			if u.ID != id {
				continue
			}
			owner, was = u.Name, u.Profile.Clone()
			a.applyProfileReq(cfg, u, req)
			return nil
		}
		return errNoSuchUser
	}, func(_, nr *runtime) {
		u, _ := nr.cfg.FindUserByID(id)
		now = u.Profile
		a.pushProfileOfMu(owner)
	})
	a.cfgMu.Unlock()
	if err != nil {
		return err
	}
	a.recordSave(connActor(c), "browserCopy", copyFields(was, now), false)
	return nil
}

func (a *App) serveBrowserCopy(c *client, m *inMsg) {
	err := a.BrowserCopy(c, profileReq{Prefs: m.Prefs, FilterSets: m.FilterSets, Presets: m.Presets, PinOrder: m.PinOrder})
	a.replyOp(c, "browserCopy", m.ID, len(m.Prefs)+len(m.FilterSets)+len(m.Presets), "item", err)
}

// copyFields: one field per changed pref with old and new, and one per list naming what was
// added, with replaced names in the note. Item values stay out: a preset is too long to read.
func copyFields(was, now config.Profile) []SavedField {
	var out []SavedField
	for _, k := range slices.Sorted(maps.Keys(config.PrefValues)) {
		o, n := was.Prefs[k], now.Prefs[k]
		if o == n {
			continue
		}
		f := newSavedField("profile/prefs/"+k, "my-user", prefLabels[k])
		if o != "" {
			f.Old, _ = json.Marshal(o)
		}
		if n != "" {
			f.New, _ = json.Marshal(n)
		}
		out = append(out, f)
	}
	sets := func(l []config.FilterSet) map[string][]byte {
		m := map[string][]byte{}
		for _, s := range l {
			m[strings.ToLower(s.Name)], _ = json.Marshal(s)
		}
		return m
	}
	presets := func(l []config.Preset) map[string][]byte {
		m := map[string][]byte{}
		for _, p := range l {
			pinned := p.Pinned == nil || *p.Pinned
			m[strings.ToLower(p.Name)], _ = json.Marshal(map[string]any{"items": p.Items, "pinned": pinned})
		}
		return m
	}
	setNames := func(l []config.FilterSet) []string {
		var out []string
		for _, s := range l {
			out = append(out, s.Name)
		}
		return out
	}
	presetNames := func(l []config.Preset) []string {
		var out []string
		for _, p := range l {
			out = append(out, p.Name)
		}
		return out
	}
	out = append(out, listField("profile/filterSets", "Filter sets", sets(was.FilterSets), sets(now.FilterSets), setNames(now.FilterSets))...)
	out = append(out, listField("profile/presets", "Presets", presets(was.Presets), presets(now.Presets), presetNames(now.Presets))...)
	return out
}

// Maps are lowercase name to item JSON; names keep now's order.
func listField(key, label string, was, now map[string][]byte, names []string) []SavedField {
	f := newSavedField(key, "my-user", label)
	var replaced []string
	for _, n := range names {
		old, had := was[strings.ToLower(n)]
		switch {
		case !had:
			v, _ := json.Marshal(n)
			f.Added = append(f.Added, v)
		case !bytes.Equal(old, now[strings.ToLower(n)]):
			replaced = append(replaced, n)
		}
	}
	if len(f.Added) == 0 && len(replaced) == 0 {
		return nil
	}
	if len(replaced) > 0 {
		f.Note = "replaced: " + strings.Join(replaced, ", ")
		f.Changed = len(f.Added) == 0
	}
	return []SavedField{f}
}
