package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

const (
	MaxPresets        = 30
	MaxPresetItems    = 100
	MaxPresetDsts     = 100
	MaxPresetLevelNum = 64
	// MaxPresetTotal keeps one take well under the 64 KiB read limit.
	MaxPresetTotal = 200
	// MaxPresetsBytes keeps a presets message under the 64 KiB read limit.
	MaxPresetsBytes = 48 << 10
)

// Reference names are the names at save time, for import and Broken checks. ID survives a rename.
type Preset struct {
	ID    string       `yaml:"id" json:"id"`
	Name  string       `yaml:"name" json:"name"`
	Items []PresetItem `yaml:"items" json:"items"`
	// Broken: references the target lacked at import, or that two name polls found gone (GLOSSARY.md, Broken).
	Broken []Missing `yaml:"broken,omitempty" json:"broken,omitempty"`
	// Moved: references whose stored name now sits at another number (GLOSSARY.md, Name moved).
	Moved []Missing      `yaml:"moved,omitempty" json:"moved,omitempty"`
	Extra map[string]any `yaml:",inline" json:"-"`
}

// One item is one route on one router: Src and every Dsts entry share Router.
type PresetItem struct {
	Src    Ref            `yaml:"src" json:"src"`
	Dsts   []Ref          `yaml:"dsts" json:"dsts"`
	Levels []int          `yaml:"levels,omitempty" json:"levels"`
	Extra  map[string]any `yaml:",inline" json:"-"`
}

func ClonePresets(in []Preset) []Preset {
	if in == nil {
		return nil
	}
	out := make([]Preset, len(in))
	for i, s := range in {
		out[i] = Preset{ID: s.ID, Name: s.Name, Items: make([]PresetItem, len(s.Items)), Broken: slices.Clone(s.Broken), Moved: slices.Clone(s.Moved), Extra: s.Extra}
		for j, it := range s.Items {
			out[i].Items[j] = PresetItem{Src: it.Src, Dsts: slices.Clone(it.Dsts), Levels: slices.Clone(it.Levels), Extra: it.Extra}
		}
	}
	return out
}

func (it PresetItem) DstNs() []int {
	out := make([]int, len(it.Dsts))
	for i, d := range it.Dsts {
		out[i] = d.N
	}
	return out
}

// Edits in place: callers pass a Clone.
func (c *Config) normalizePresets() {
	c.Presets = normalizePresets(c.Presets, c.UID)
}

// Blank or repeated ids get a new one, so a list never names two presets by one id.
func normalizePresets(ps []Preset, router string) []Preset {
	if ps == nil {
		return []Preset{}
	}
	ids := map[string]bool{}
	refs := RefWalk{Fn: func(_ string, r Ref) []Ref { return []Ref{presetRef(r, router)} }}
	for i := range ps {
		s := &ps[i]
		s.Name = strings.TrimSpace(s.Name)
		s.ID = strings.TrimSpace(s.ID)
		for s.ID == "" || ids[s.ID] {
			s.ID = newRouterUID()
		}
		ids[s.ID] = true
		if s.Items == nil {
			s.Items = []PresetItem{}
		}
		refs.Preset(s)
		for j := range s.Items {
			it := &s.Items[j]
			if it.Dsts == nil {
				it.Dsts = []Ref{}
			}
			slices.Sort(it.Levels)
			it.Levels = slices.Compact(it.Levels)
			if it.Levels == nil {
				it.Levels = []int{}
			}
		}
	}
	return ps
}

// Shown goes stale when numbering changes, so a stored preset keeps {router, n, name} only.
func presetRef(r Ref, router string) Ref {
	r.Shown = ""
	r.Name = strings.TrimSpace(r.Name)
	if r.Router == "" {
		r.Router = router
	}
	return r
}

func (c Config) validatePresets() []error {
	var errs []error
	bad := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }
	snames := make([]string, len(c.Presets))
	for i, s := range c.Presets {
		snames[i] = s.Name
	}
	errs = append(errs, validateNames("preset", snames, MaxPresets)...)
	for _, s := range c.Presets {
		if len(s.Items) == 0 {
			bad("preset %q has no takes", s.Name)
		}
		if len(s.Items) > MaxPresetItems {
			bad("preset %q: at most %d sources", s.Name, MaxPresetItems)
		}
		total := 0
		for _, it := range s.Items {
			total += len(it.Dsts)
		}
		if total > MaxPresetTotal {
			bad("preset %q: a preset can hold at most %d destinations in total", s.Name, MaxPresetTotal)
		}
		if err := presetConflict(s); err != nil {
			errs = append(errs, err)
		}
		for _, it := range s.Items {
			if it.Src.N < 0 || it.Src.N > 0xFFFE {
				bad("preset %q: source %d is out of range", s.Name, it.Src.N)
			}
			// An unknown router is not refused here: Load runs Validate, and such an item is Broken.
			if slices.ContainsFunc(it.Dsts, func(d Ref) bool { return d.Router != it.Src.Router }) {
				bad("preset %q: a source and its destinations must be on one router", s.Name)
			}
			if len(it.Dsts) == 0 {
				bad("preset %q: a source has no destinations", s.Name)
			}
			if len(it.Dsts) > MaxPresetDsts {
				bad("preset %q: at most %d destinations per source", s.Name, MaxPresetDsts)
			}
			for _, d := range it.Dsts {
				if d.N < 0 || d.N > 0xFFFE {
					bad("preset %q: destination %d is out of range", s.Name, d.N)
				}
			}
			for _, l := range it.Levels {
				if l < 1 || l > MaxPresetLevelNum {
					bad("preset %q: level %d must be 1-%d", s.Name, l, MaxPresetLevelNum)
				}
			}
		}
	}
	if b, err := json.Marshal(c.Presets); err == nil && len(b) > MaxPresetsBytes {
		bad("presets are too large to save (%d KiB, at most %d KiB)", len(b)>>10, MaxPresetsBytes>>10)
	}
	return errs
}

// presetConflict: different sources to one destination on overlapping levels (empty = all) would
// race; the same source twice merges.
func presetConflict(s Preset) error {
	for i, a := range s.Items {
		for _, b := range s.Items[i+1:] {
			if a.Src.Router != b.Src.Router || a.Src.N == b.Src.N || !levelsOverlap(a.Levels, b.Levels) {
				continue
			}
			for _, d := range a.Dsts {
				if slices.ContainsFunc(b.Dsts, func(x Ref) bool { return x.N == d.N }) {
					return fmt.Errorf("preset %q: destination %d gets both source %d and source %d on the same levels", s.Name, d.N, a.Src.N, b.Src.N)
				}
			}
		}
	}
	return nil
}

func levelsOverlap(a, b []int) bool {
	if len(a) == 0 || len(b) == 0 {
		return true
	}
	for _, l := range a {
		if slices.Contains(b, l) {
			return true
		}
	}
	return false
}
