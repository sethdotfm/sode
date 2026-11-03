package config

import (
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
	if len(c.Salvos) > MaxSalvos {
		bad("at most %d salvos", MaxSalvos)
	}
	names := map[string]bool{}
	for _, s := range c.Salvos {
		key := strings.ToLower(s.Name)
		switch n := len([]rune(s.Name)); {
		case n == 0:
			bad("every salvo needs a name")
		case n > 32:
			bad("salvo name %q must be at most 32 characters", s.Name)
		case names[key]:
			bad("two salvos are named %q", s.Name)
		}
		names[key] = true
		if len(s.Items) == 0 {
			bad("salvo %q has no takes", s.Name)
		}
		if len(s.Items) > MaxSalvoItems {
			bad("salvo %q: at most %d sources", s.Name, MaxSalvoItems)
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
	return errs
}
