package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Preset caps. A preset is sent as one batch, so these keep one button press
// to a sane number of takes.
const (
	MaxPresets        = 30
	MaxPresetItems    = 100
	MaxPresetDsts     = 100
	MaxPresetLevelNum = 64
	// MaxPresetTotal caps the destinations over all of a preset's items (its
	// takes), so one batch stays well under the 64 KiB WebSocket read limit.
	MaxPresetTotal = 200
	// MaxPresetsBytes caps all presets as JSON, so a "presets" message (which
	// carries the whole list) fits the 64 KiB read limit with room to spare.
	MaxPresetsBytes = 48 << 10
)

// Preset is a named set of takes sent together as one confirm: each item
// takes one source to its destinations. Everything is a number (internal
// source/destination numbers, UI levels 1..N), never a name, because names
// are re-polled from the router. Empty Levels means the default levels.
type Preset struct {
	Name  string       `yaml:"name" json:"name"`
	Items []PresetItem `yaml:"items" json:"items"`
	// Pinned: shown in the strip above Recent. nil (an older file) = true;
	// normalizePresets always fills it. A browser may override it for itself.
	Pinned *bool `yaml:"pinned,omitempty" json:"pinned,omitempty"`
}

// PresetItem is one source going to one or more destinations.
type PresetItem struct {
	Src    int   `yaml:"src" json:"src"`
	Dsts   []int `yaml:"dsts" json:"dsts"`
	Levels []int `yaml:"levels,omitempty" json:"levels"`
}

// ClonePresets copies presets so the copy shares no slices.
func ClonePresets(in []Preset) []Preset {
	if in == nil {
		return nil
	}
	out := make([]Preset, len(in))
	for i, s := range in {
		out[i] = Preset{Name: s.Name, Items: make([]PresetItem, len(s.Items))}
		if s.Pinned != nil {
			p := *s.Pinned
			out[i].Pinned = &p
		}
		for j, it := range s.Items {
			out[i].Items[j] = PresetItem{Src: it.Src, Dsts: slices.Clone(it.Dsts), Levels: slices.Clone(it.Levels)}
		}
	}
	return out
}

// normalizePresets trims names, sorts and de-duplicates levels, and makes
// every list non-nil so JSON sends [] rather than null. It edits in place:
// callers pass a Clone.
func (c *Config) normalizePresets() {
	if c.Presets == nil {
		c.Presets = []Preset{}
	}
	for i := range c.Presets {
		s := &c.Presets[i]
		s.Name = strings.TrimSpace(s.Name)
		if s.Pinned == nil {
			t := true
			s.Pinned = &t
		}
		if s.Items == nil {
			s.Items = []PresetItem{}
		}
		for j := range s.Items {
			it := &s.Items[j]
			if it.Dsts == nil {
				it.Dsts = []int{}
			}
			slices.Sort(it.Levels)
			it.Levels = slices.Compact(it.Levels)
			if it.Levels == nil {
				it.Levels = []int{}
			}
		}
	}
}

// validatePresets returns what is wrong with the presets, if anything.
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
			bad("preset %q: a preset can take at most %d destinations in total", s.Name, MaxPresetTotal)
		}
		if err := presetConflict(s); err != nil {
			errs = append(errs, err)
		}
		for _, it := range s.Items {
			if it.Src < 0 || it.Src > 0xFFFE {
				bad("preset %q: source %d is out of range", s.Name, it.Src)
			}
			if len(it.Dsts) == 0 {
				bad("preset %q: a source has no destinations", s.Name)
			}
			if len(it.Dsts) > MaxPresetDsts {
				bad("preset %q: at most %d destinations per source", s.Name, MaxPresetDsts)
			}
			for _, d := range it.Dsts {
				if d < 0 || d > 0xFFFE {
					bad("preset %q: destination %d is out of range", s.Name, d)
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

// presetConflict reports two items that take different sources to the same
// destination on overlapping levels (empty levels = the default levels,
// which overlap with everything): they would race. The same source twice is
// fine; it merges.
func presetConflict(s Preset) error {
	for i, a := range s.Items {
		for _, b := range s.Items[i+1:] {
			if a.Src == b.Src || !levelsOverlap(a.Levels, b.Levels) {
				continue
			}
			for _, d := range a.Dsts {
				if slices.Contains(b.Dsts, d) {
					return fmt.Errorf("preset %q: destination %d is taken from both source %d and source %d on the same levels", s.Name, d, a.Src, b.Src)
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
