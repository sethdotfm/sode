package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Profile lives on the User. It is never in a hello (json:"-") and reaches only connections signed in as that user.

// MaxProfileBytes keeps a profileSave message under the 64 KiB read limit.
const MaxProfileBytes = 48 << 10

// Prefs values are strings as in web/prefs.js ("on"/"off" for yes/no).
type Profile struct {
	Prefs      map[string]string `yaml:"prefs,omitempty" json:"prefs"`
	FilterSets []FilterSet       `yaml:"filterSets,omitempty" json:"filterSets"`
	Presets    []Preset          `yaml:"presets,omitempty" json:"presets"`
	// Bookmarks is the user's Me list (GLOSSARY.md, Bookmark).
	Bookmarks []Bookmark `yaml:"bookmarks,omitempty" json:"bookmarks"`
	// PalettePicks holds the keys of the user's last Palette picks, newest first.
	PalettePicks []string       `yaml:"palettePicks,omitempty" json:"palettePicks"`
	Extra        map[string]any `yaml:",inline" json:"-"`
}

// Go twin of PREFS in web/prefs.js; keep in step (TestPrefTable).
var PrefValues = map[string][]string{
	"theme":         {"system", "light", "dark"},
	"layout":        {"auto", "phone", "desktop"},
	"clock":         {"24", "12", "locale"},
	"flip":          {"on", "off"},
	"scrollTop":     {"on", "off"},
	"setStack":      {"and", "or"},
	"nextMode":      {"number", "fuzzy"},
	"stackGap":      {"0", "10", "30", "60", "120", "300"},
	"searchFilters": {"on", "off"},
	"compact":       {"on", "off"},
	"paletteChar":   paletteValues(),
}

func (p Profile) Empty() bool {
	return len(p.Prefs) == 0 && len(p.FilterSets) == 0 && len(p.Presets) == 0 && len(p.Bookmarks) == 0 && len(p.PalettePicks) == 0
}

func (p Profile) Clone() Profile {
	return Profile{Prefs: maps.Clone(p.Prefs), FilterSets: slices.Clone(p.FilterSets), Presets: ClonePresets(p.Presets), Bookmarks: CloneBookmarks(p.Bookmarks), PalettePicks: slices.Clone(p.PalettePicks), Extra: p.Extra}
}

func knownPref(k string) bool {
	_, ok := PrefValues[k]
	return ok
}

// A browser cannot add or remove a pref this release does not know; only the file holds those.
func WithUnknownPrefs(sent, saved map[string]string) map[string]string {
	out := maps.Clone(sent)
	if out == nil {
		out = map[string]string{}
	}
	maps.DeleteFunc(out, func(k, _ string) bool { return !knownPref(k) })
	for k, v := range saved {
		if !knownPref(k) {
			out[k] = v
		}
	}
	return out
}

// Works on a copy; no nil lists, so JSON says [] and {}.
func (p Profile) Normalized() Profile {
	p = p.Clone()
	p.Prefs = maps.Clone(p.Prefs)
	if p.Prefs == nil {
		p.Prefs = map[string]string{}
	}
	maps.DeleteFunc(p.Prefs, func(k, _ string) bool { return !knownPref(k) })
	p.FilterSets = normalizeFilterSets(p.FilterSets)
	p.Presets = normalizePresets(p.Presets, "")
	p.Bookmarks = normalizeBookmarks(p.Bookmarks)
	p.PalettePicks = normalizePalettePicks(p.PalettePicks)
	return p
}

func (c *Config) normalizeProfiles() {
	for i := range c.Users {
		p := c.Users[i].Profile.Normalized()
		p.Presets = normalizePresets(p.Presets, c.UID)
		// Normalized is what the browser gets; the file keeps prefs a newer release wrote.
		p.Prefs = WithUnknownPrefs(p.Prefs, c.Users[i].Profile.Prefs)
		if p.Empty() {
			p = Profile{Extra: p.Extra}
		}
		c.Users[i].Profile = p
	}
}

func validateProfile(owner string, p Profile) []error {
	var errs []error
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("profile of %s: "+format, append([]any{owner}, args...)...))
	}
	for _, k := range slices.Sorted(maps.Keys(p.Prefs)) {
		if knownPref(k) && !slices.Contains(PrefValues[k], p.Prefs[k]) {
			bad("%q is not a value of %s", p.Prefs[k], k)
		}
	}
	for _, e := range validateFilterSets(p.FilterSets) {
		bad("%v", e)
	}
	tmp := Config{Presets: p.Presets}
	for _, e := range tmp.validatePresets() {
		bad("%v", e)
	}
	for _, e := range validateBookmarks(p.Bookmarks) {
		bad("%v", e)
	}
	if b, err := json.Marshal(p); err == nil && len(b) > MaxProfileBytes {
		bad("too large to save (%d KiB, at most %d KiB)", len(b)>>10, MaxProfileBytes>>10)
	}
	return errs
}

func (c Config) validateProfiles() []error {
	var errs []error
	for _, u := range c.Users {
		errs = append(errs, validateProfile(u.Name, u.Profile)...)
	}
	return errs
}

// Edits in place: callers pass a clone.
func normalizeFilterSets(in []FilterSet) []FilterSet {
	if in == nil {
		return []FilterSet{}
	}
	ids := map[string]bool{}
	for i := range in {
		f := &in[i]
		f.Name, f.Src, f.Dst = strings.TrimSpace(f.Name), strings.TrimSpace(f.Src), strings.TrimSpace(f.Dst)
		f.ID = strings.TrimSpace(f.ID)
		for f.ID == "" || ids[f.ID] {
			f.ID = NewFilterSetID()
		}
		ids[f.ID] = true
	}
	return in
}

func validateFilterSets(sets []FilterSet) []error {
	var errs []error
	names := make([]string, len(sets))
	for i, f := range sets {
		names[i] = f.Name
		if len([]rune(f.Src)) > 500 || len([]rune(f.Dst)) > 500 {
			errs = append(errs, fmt.Errorf("filter set %q: expressions must be at most 500 characters", f.Name))
		}
	}
	errs = append(errs, validateNames("filter set", names, 30)...)
	// Like presets, all of them in one message under the read limit.
	if b, err := json.Marshal(sets); err == nil && len(b) > MaxFilterSetsBytes {
		errs = append(errs, fmt.Errorf("filter sets are too large to save (%d KiB, at most %d KiB)", len(b)>>10, MaxFilterSetsBytes>>10))
	}
	return errs
}
