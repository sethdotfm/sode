package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// Profile lives on the User; never in a hello (json:"-"), reaches only connections unlocked as that user.

// MaxProfileBytes keeps a profileSave message under the 64 KiB read limit.
const MaxProfileBytes = 48 << 10

// Prefs values are strings as in web/prefs.js ("on"/"off" for yes/no).
type Profile struct {
	Prefs      map[string]string `yaml:"prefs,omitempty" json:"prefs"`
	FilterSets []FilterSet       `yaml:"filterSets,omitempty" json:"filterSets"`
	Presets    []Preset          `yaml:"presets,omitempty" json:"presets"`
	// PinOrder keys are "scope:lowercase name"; unmatched ignored, unlisted pinned presets come after.
	PinOrder []string       `yaml:"pinOrder,omitempty" json:"pinOrder"`
	Extra    map[string]any `yaml:",inline" json:"-"`
}

// MaxPinOrder caps PinOrder; MaxPinKey caps one key, in runes.
const (
	MaxPinOrder = 200
	MaxPinKey   = 80
)

// Go twin of PREFS in web/prefs.js; keep in step (TestPrefTable).
var PrefValues = map[string][]string{
	"theme":         {"system", "light", "dark"},
	"layout":        {"auto", "phone", "desktop"},
	"clock":         {"24", "12", "locale"},
	"flip":          {"on", "off"},
	"scrollTop":     {"on", "off"},
	"setStack":      {"and", "or"},
	"nextMode":      {"number", "fuzzy"},
	"searchFilters": {"on", "off"},
}

func (p Profile) Empty() bool {
	return len(p.Prefs) == 0 && len(p.FilterSets) == 0 && len(p.Presets) == 0 && len(p.PinOrder) == 0
}

func (p Profile) Items() int {
	return len(p.Prefs) + len(p.FilterSets) + len(p.Presets) + len(p.PinOrder)
}

func (p Profile) Clone() Profile {
	return Profile{Prefs: maps.Clone(p.Prefs), FilterSets: slices.Clone(p.FilterSets), Presets: ClonePresets(p.Presets), PinOrder: slices.Clone(p.PinOrder), Extra: p.Extra}
}

// Works on a copy; no nil lists, so JSON says [] and {}.
func (p Profile) Normalized() Profile {
	p = p.Clone()
	// Carry prefs this release does not know through normalization, so a rollback
	// keeps them; validateProfile skips them below. Known prefs are still checked.
	if p.Prefs == nil {
		p.Prefs = map[string]string{}
	} else {
		p.Prefs = maps.Clone(p.Prefs)
	}
	p.FilterSets = normalizeFilterSets(p.FilterSets)
	tmp := Config{Presets: p.Presets}
	tmp.normalizePresets()
	p.Presets = tmp.Presets
	p.PinOrder = normalizePinOrder(p.PinOrder)
	return p
}

// Keys are opaque: the browser builds them (presetKey in presetstore.js) and compares bytes. Go's
// ToLower differs from JS for some letters, so lowercasing again could break a key.
func normalizePinOrder(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, k := range in {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] || len([]rune(k)) > MaxPinKey || len(out) >= MaxPinOrder {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

func (c *Config) normalizeProfiles() {
	for i := range c.Users {
		p := c.Users[i].Profile.Normalized()
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
		if _, ok := PrefValues[k]; !ok {
			continue // unknown pref: carried, not validated
		}
		if !slices.Contains(PrefValues[k], p.Prefs[k]) {
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
	for i := range in {
		f := &in[i]
		f.Name, f.Src, f.Dst = strings.TrimSpace(f.Name), strings.TrimSpace(f.Src), strings.TrimSpace(f.Dst)
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
