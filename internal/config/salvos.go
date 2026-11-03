package config

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
)

// Salvo caps. A salvo is sent as one batch, so these keep one button press
// to a sane number of takes.
const (
	MaxSalvos        = 30
	MaxSalvoItems    = 100
	MaxSalvoDsts     = 100
	MaxSalvoLevelNum = 64
	// MaxSalvoTotal caps the destinations over all of a salvo's items (its
	// takes), so one batch stays well under the 64 KiB WebSocket read limit.
	MaxSalvoTotal = 200
	// MaxSalvosBytes caps all salvos as JSON, so a "salvos" message (which
	// carries the whole list) fits the 64 KiB read limit with room to spare.
	MaxSalvosBytes = 48 << 10
)

// Salvo is a named set of takes sent together as one confirm: each item
// takes one source to its destinations. Everything is a number (internal
// source/destination numbers, UI levels 1..N), never a name, because names
// are re-polled from the router. Empty Levels means the default levels.
type Salvo struct {
	Name  string      `yaml:"name" json:"name"`
	Items []SalvoItem `yaml:"items" json:"items"`
}

// SalvoItem is one source going to one or more destinations.
type SalvoItem struct {
	Src    int   `yaml:"src" json:"src"`
	Dsts   []int `yaml:"dsts" json:"dsts"`
	Levels []int `yaml:"levels,omitempty" json:"levels"`
}

// CloneSalvos copies salvos so the copy shares no slices.
func CloneSalvos(in []Salvo) []Salvo {
	if in == nil {
		return nil
	}
	out := make([]Salvo, len(in))
	for i, s := range in {
		out[i] = Salvo{Name: s.Name, Items: make([]SalvoItem, len(s.Items))}
		for j, it := range s.Items {
			out[i].Items[j] = SalvoItem{Src: it.Src, Dsts: slices.Clone(it.Dsts), Levels: slices.Clone(it.Levels)}
		}
	}
	return out
}

// normalizeSalvos trims names, sorts and de-duplicates levels, and makes
// every list non-nil so JSON sends [] rather than null. It edits in place:
// callers pass a Clone.
func (c *Config) normalizeSalvos() {
	if c.Salvos == nil {
		c.Salvos = []Salvo{}
	}
	for i := range c.Salvos {
		s := &c.Salvos[i]
		s.Name = strings.TrimSpace(s.Name)
		if s.Items == nil {
			s.Items = []SalvoItem{}
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

// validateSalvos returns what is wrong with the salvos, if anything.
func (c Config) validateSalvos() []error {
	var errs []error
	bad := func(f string, a ...any) { errs = append(errs, fmt.Errorf(f, a...)) }
	snames := make([]string, len(c.Salvos))
	for i, s := range c.Salvos {
		snames[i] = s.Name
	}
	errs = append(errs, validateNames("salvo", snames, MaxSalvos)...)
	for _, s := range c.Salvos {
		if len(s.Items) == 0 {
			bad("salvo %q has no takes", s.Name)
		}
		if len(s.Items) > MaxSalvoItems {
			bad("salvo %q: at most %d sources", s.Name, MaxSalvoItems)
		}
		total := 0
		for _, it := range s.Items {
			total += len(it.Dsts)
		}
		if total > MaxSalvoTotal {
			bad("salvo %q: a salvo can take at most %d destinations in total", s.Name, MaxSalvoTotal)
		}
		if err := salvoConflict(s); err != nil {
			errs = append(errs, err)
		}
		for _, it := range s.Items {
			if it.Src < 0 || it.Src > 0xFFFE {
				bad("salvo %q: source %d is out of range", s.Name, it.Src)
			}
			if len(it.Dsts) == 0 {
				bad("salvo %q: a source has no destinations", s.Name)
			}
			if len(it.Dsts) > MaxSalvoDsts {
				bad("salvo %q: at most %d destinations per source", s.Name, MaxSalvoDsts)
			}
			for _, d := range it.Dsts {
				if d < 0 || d > 0xFFFE {
					bad("salvo %q: destination %d is out of range", s.Name, d)
				}
			}
			for _, l := range it.Levels {
				if l < 1 || l > MaxSalvoLevelNum {
					bad("salvo %q: level %d must be 1-%d", s.Name, l, MaxSalvoLevelNum)
				}
			}
		}
	}
	if b, err := json.Marshal(c.Salvos); err == nil && len(b) > MaxSalvosBytes {
		bad("salvos are too large to save (%d KiB, at most %d KiB)", len(b)>>10, MaxSalvosBytes>>10)
	}
	return errs
}

// salvoConflict reports two items that take different sources to the same
// destination on overlapping levels (empty levels = the default levels,
// which overlap with everything): they would race. The same source twice is
// fine; it merges.
func salvoConflict(s Salvo) error {
	for i, a := range s.Items {
		for _, b := range s.Items[i+1:] {
			if a.Src == b.Src || !levelsOverlap(a.Levels, b.Levels) {
				continue
			}
			for _, d := range a.Dsts {
				if slices.Contains(b.Dsts, d) {
					return fmt.Errorf("salvo %q: destination %d is taken from both source %d and source %d on the same levels", s.Name, d, a.Src, b.Src)
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
