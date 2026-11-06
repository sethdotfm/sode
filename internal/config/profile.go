package config

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
)

// A user's profile (#161): the "This device" prefs, filter sets and presets
// that follow a person from one browser to another. It lives on the User in
// the config file, is never sent in a hello (User.Profile is json:"-") and
// reaches only connections unlocked as that user (app/profile.go).

// MaxProfileBytes caps a whole profile as JSON, so a profileSave message
// (which can carry every part) fits the 64 KiB read limit with room to spare.
const MaxProfileBytes = 48 << 10

// Profile is what one user keeps. Prefs maps a pref name to its value string
// as web/prefs.js has it ("on"/"off" for the yes/no ones).
type Profile struct {
	Prefs      map[string]string `yaml:"prefs,omitempty" json:"prefs"`
	FilterSets []FilterSet       `yaml:"filterSets,omitempty" json:"filterSets"`
	Presets    []Preset          `yaml:"presets,omitempty" json:"presets"`
	// PinOrder is the order of this user's pinned presets in the strip and the
	// Presets popup: keys "scope:lowercase name" (scope global, local or user).
	// Keys that match nothing are ignored; pinned presets not listed come after.
	PinOrder []string `yaml:"pinOrder,omitempty" json:"pinOrder"`
}

// MaxPinOrder caps PinOrder, and MaxPinKey one key, in runes.
const (
	MaxPinOrder = 200
	MaxPinKey   = 80
)

// PrefValues is the Go twin of PREFS in web/prefs.js: every per-browser
// setting and the values it may have. Keep the two in step
// (TestPrefTable reads prefs.js).
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

// Empty reports whether the profile holds nothing.
func (p Profile) Empty() bool {
	return len(p.Prefs) == 0 && len(p.FilterSets) == 0 && len(p.Presets) == 0 && len(p.PinOrder) == 0
}

// Items counts what the profile holds (prefs, filter sets, presets).
func (p Profile) Items() int {
	return len(p.Prefs) + len(p.FilterSets) + len(p.Presets) + len(p.PinOrder)
}

// Clone copies p so the copy shares no maps or slices.
func (p Profile) Clone() Profile {
	return Profile{Prefs: maps.Clone(p.Prefs), FilterSets: slices.Clone(p.FilterSets), Presets: ClonePresets(p.Presets), PinOrder: slices.Clone(p.PinOrder)}
}

// Normalized is p made ready to send: unknown pref names dropped, names
// trimmed, presets filled in like the global ones; no nil lists, so JSON says
// [] and {}. It works on a copy.
func (p Profile) Normalized() Profile {
	p = p.Clone()
	prefs := map[string]string{}
	for k, v := range p.Prefs {
		if _, ok := PrefValues[k]; ok {
			prefs[k] = v
		}
	}
	p.Prefs = prefs
	p.FilterSets = normalizeFilterSets(p.FilterSets)
	tmp := Config{Presets: p.Presets}
	tmp.normalizePresets()
	p.Presets = tmp.Presets
	p.PinOrder = normalizePinOrder(p.PinOrder)
	return p
}

// normalizePinOrder trims the keys, drops empty, repeated and over-long ones
// and cuts the list at MaxPinOrder. Never nil. A key is opaque here: the
// browser builds it ("scope:" and the lowercased preset name, presetKey in
// presetstore.js) and compares it byte for byte, so it is kept as sent. Go's
// ToLower and JavaScript's toLowerCase disagree for some letters (İ), and
// lowercasing again here would turn a key the browser wrote into one it can't
// find.
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

// normalizeProfiles normalizes every user's profile, and keeps an empty one
// as nothing so the file stays small.
func (c *Config) normalizeProfiles() {
	for i := range c.Users {
		p := c.Users[i].Profile.Normalized()
		if p.Empty() {
			p = Profile{}
		}
		c.Users[i].Profile = p
	}
}

// validateProfile reports what is wrong with one user's profile (already
// normalized): pref values, filter sets and presets by the same rules as
// the global ones, and the total size.
func validateProfile(owner string, p Profile) []error {
	var errs []error
	bad := func(format string, args ...any) {
		errs = append(errs, fmt.Errorf("profile of %s: "+format, append([]any{owner}, args...)...))
	}
	for _, k := range slices.Sorted(maps.Keys(p.Prefs)) {
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

// normalizeFilterSets trims the names and expressions and makes the list
// non-nil. It edits in place: callers pass a clone.
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

// validateFilterSets is the one set of rules for filter sets, global or a
// profile's.
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
