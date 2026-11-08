package app

import (
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// apiView is the one place a caller's visibility applies (ADR 0004). Read-only rights show
// everything; hidden rights drop out-of-rights sources, destinations and presets, and what
// names them. Every read handler and the stream ask it. Undo, Redo and Retry planning asks
// Entry: it returns the entry as the caller sees it, so a plan built from the result covers
// visible routes only, and false when the caller may not know the entry exists.
// Levels are not hidden: a rights list of levels only refuses commands.
type apiView struct{ rights config.Rights }

func (c *apiCtx) view() apiView { return apiView{c.Rights} }

func (v apiView) Hidden() bool { return v.rights.Hidden() }

func (v apiView) Source(n int) bool       { return !v.Hidden() || v.rights.AllowsSource(n) }
func (v apiView) Destination(n int) bool  { return !v.Hidden() || v.rights.AllowsDst(n) }
func (v apiView) Preset(name string) bool { return !v.Hidden() || v.rights.AllowsPreset(name) }

// WholePreset: the preset is visible by name and every source and destination it names is
// visible, since its items would otherwise teach a hidden number.
func (v apiView) WholePreset(p config.Preset) bool {
	if !v.Preset(p.Name) {
		return false
	}
	for _, it := range p.Items {
		if !v.Source(it.Src) || slices.ContainsFunc(it.Dsts, func(d int) bool { return !v.Destination(d) }) {
			return false
		}
	}
	return true
}

// Route: src < 0 means nothing routed, which is always visible.
func (v apiView) Route(dst, level, src int) bool {
	return v.Destination(dst) && (src < 0 || v.Source(src))
}

// Entry returns the History entry as this caller may see it, or false to drop it. The
// entry's own route must be visible. A level whose Before source (Prev) is hidden is taken
// out of the entry, so neither the entry nor an Undo built from it names that source. A
// Group's size would count hidden routes, so it is cleared for a hidden caller.
func (v apiView) Entry(e Entry) (Entry, bool) {
	if !v.Hidden() {
		return e, true
	}
	if !v.Destination(e.Dst) || (e.Src >= 0 && !v.Source(e.Src)) {
		return e, false
	}
	e = cloneEntry(e)
	var gone []int
	for _, p := range e.Prev {
		if p[1] >= 0 && !v.Source(p[1]) {
			gone = append(gone, p[0])
		}
	}
	if len(gone) > 0 {
		had := len(e.Levels)
		drop := func(l int) bool { return slices.Contains(gone, l) }
		e.Levels = slices.DeleteFunc(e.Levels, drop)
		e.Failed = slices.DeleteFunc(e.Failed, drop)
		e.Already = slices.DeleteFunc(e.Already, drop)
		e.Late = slices.DeleteFunc(e.Late, drop)
		e.Unsent = slices.DeleteFunc(e.Unsent, drop)
		e.Prev = slices.DeleteFunc(e.Prev, func(p [2]int) bool { return drop(p[0]) })
		e.Now = slices.DeleteFunc(e.Now, func(p [2]int) bool { return drop(p[0]) })
		if had > 0 && len(e.Levels) == 0 {
			return e, false
		}
	}
	// Now names the router's own answer, which can be any source.
	e.Now = slices.DeleteFunc(e.Now, func(p [2]int) bool { return p[1] >= 0 && !v.Source(p[1]) })
	if e.Preset != "" && !v.Preset(e.Preset) {
		e.Preset = ""
	}
	e.GroupN = 0
	return e, true
}
