package config

import (
	"fmt"
	"slices"
	"strings"
)

// A TieLine (GLOSSARY.md, Tie line) is a one-way cable from a destination on one router (From) to a source on another
// router (To). An Administrator enters it in Settings > Routers > Tie lines. Sode trusts the pair, since it cannot see
// the signal. Each end keeps the router name it had at save time, for the Name moved check (docs/tielines-design.md).
type TieLine struct {
	// ID is hidden and stays through a rename, as a preset's does. Presets and panels store it.
	ID string `yaml:"id" json:"id"`
	// Name is the Tie line name, unique on the server and apart from the router names of the two ends.
	Name string `yaml:"name" json:"name"`
	// From is the upstream end: a destination. To is the downstream end: a source on another router.
	From  Ref            `yaml:"from" json:"from"`
	To    Ref            `yaml:"to" json:"to"`
	Extra map[string]any `yaml:",inline" json:"-"`
}

// DefaultTieHopWeight: one hop costs as much as one Affected destination.
const (
	DefaultTieHopWeight = 1
	MaxTieHopWeight     = 100
)

// MaxTieLines keeps the tieLines message well under the 64 KiB read limit.
const MaxTieLines = 200

func CloneTieLines(in []TieLine) []TieLine { return slices.Clone(in) }

// Blank or repeated ids get a new one. A blank name takes the name of the downstream source.
func normalizeTieLines(in []TieLine) []TieLine {
	out := make([]TieLine, len(in))
	ids := map[string]bool{}
	for i, t := range in {
		t.ID, t.Name = strings.TrimSpace(t.ID), strings.TrimSpace(t.Name)
		for _, r := range []*Ref{&t.From, &t.To} {
			r.Router, r.Name = strings.TrimSpace(r.Router), strings.TrimSpace(r.Name)
		}
		if t.Name == "" {
			t.Name = t.To.Name
		}
		for t.ID == "" || ids[t.ID] {
			t.ID = newRouterUID()
		}
		ids[t.ID] = true
		out[i] = t
	}
	return out
}

// TieLineProblem says why Sode refuses t on its own, or "". The page shows the same words (tielinerules.js).
func (c Config) TieLineProblem(t TieLine) string {
	for _, end := range []struct {
		r    Ref
		side string
	}{{t.From, "destination"}, {t.To, "source"}} {
		if end.r.Router == "" {
			return fmt.Sprintf("a tie line needs the router of its %s", end.side)
		}
		if _, ok := c.RouterByUID(end.r.Router); !ok {
			return fmt.Sprintf("a tie line names a router that Sode does not have (%s)", end.r.Router)
		}
		if end.r.N < 0 || end.r.N > 0xffff {
			return fmt.Sprintf("a tie line needs a %s number from the router", end.side)
		}
	}
	if t.From.Router == t.To.Router {
		return "a tie line goes from one router to a different router"
	}
	return ""
}

func (c Config) validateTieLines() []error {
	var errs []error
	names := make([]string, len(c.TieLines))
	type ends struct {
		fr, to string
		fn, tn int
	}
	pairs := map[ends]bool{}
	for i, t := range c.TieLines {
		names[i] = t.Name
		if why := c.TieLineProblem(t); why != "" {
			errs = append(errs, fmt.Errorf("tie line %d (%q): %s", i+1, t.Name, why))
		}
		k := ends{t.From.Router, t.To.Router, t.From.N, t.To.N}
		if pairs[k] {
			errs = append(errs, fmt.Errorf("tie line %d (%q): another tie line has the same two ends", i+1, t.Name))
		}
		pairs[k] = true
	}
	return append(errs, validateNames("tie line", names, MaxTieLines)...)
}

// WithTieLineExtras: a browser can neither drop nor add unknown keys. A tie line matches the saved one with its id.
func WithTieLineExtras(in, saved []TieLine) []TieLine {
	out := CloneTieLines(in)
	for i := range out {
		t := &out[i]
		j := slices.IndexFunc(saved, func(s TieLine) bool { return s.ID != "" && s.ID == t.ID })
		t.Extra, t.From.Extra, t.To.Extra = nil, nil, nil
		if j >= 0 {
			t.Extra, t.From.Extra, t.To.Extra = saved[j].Extra, saved[j].From.Extra, saved[j].To.Extra
		}
	}
	return out
}
