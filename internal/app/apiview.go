package app

import (
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// The one place a caller's visibility applies (ADR 0004). Read-only rights show everything. Hidden rights drop
// sources, destinations and presets outside the rights, and what names them. Levels are not hidden: a rights list
// of levels only refuses commands. A number is visible on its own router only (on), as a Rights check matches it.
type apiView struct {
	rights config.Rights
	// first is the settings UID of the first router, for a blank router and for firstUnit (rightsUID).
	first, firstUnit string
}

func (a *App) view(c *apiCtx) apiView {
	v := apiView{rights: c.Rights, first: a.r().cfg.UID}
	if a.first != nil {
		v.firstUnit = a.first.uid
	}
	return v
}

func (v apiView) Hidden() bool { return v.rights.Hidden() }

func (v apiView) ref(f config.Ref) config.Ref {
	if f.Router == "" || f.Router == v.firstUnit {
		f.Router = v.first
	}
	return config.Ref{Router: f.Router, N: f.N}
}

func (v apiView) source(f config.Ref) bool      { return !v.Hidden() || v.rights.AllowsSource(v.ref(f)) }
func (v apiView) destination(f config.Ref) bool { return !v.Hidden() || v.rights.AllowsDst(v.ref(f)) }
func (v apiView) Preset(id string) bool         { return !v.Hidden() || v.rights.AllowsPreset(id) }

// routerView is an apiView for the numbers of one router.
type routerView struct {
	v   apiView
	uid string
}

// on gives the view of router uid; blank is the first router.
func (v apiView) on(uid string) routerView { return routerView{v, uid} }

func (rv routerView) Source(n int) bool { return rv.v.source(config.Ref{Router: rv.uid, N: n}) }
func (rv routerView) Destination(n int) bool {
	return rv.v.destination(config.Ref{Router: rv.uid, N: n})
}

// Route: src < 0 means nothing routed, which is always visible.
func (rv routerView) Route(dst, level, src int) bool {
	return rv.Destination(dst) && (src < 0 || rv.Source(src))
}

// The items of a preset would teach a hidden number, so every source and destination it names must be visible.
func (v apiView) WholePreset(p config.Preset) bool {
	if !v.Preset(p.ID) {
		return false
	}
	for _, it := range p.Items {
		if !v.source(it.Src) || slices.ContainsFunc(it.Dsts, func(d config.Ref) bool { return !v.destination(d) }) {
			return false
		}
	}
	return true
}

// false drops the entry. A level whose Before source (Prev) is hidden goes, so no Undo built from the result names it.
// A Group's size would count hidden routes, so it is cleared for a hidden caller.
func (v apiView) Entry(e Entry) (Entry, bool) {
	if !v.Hidden() {
		return e, true
	}
	// A tie take's entry is seen by its typed source (Origin), as Rights cover the typed ends only.
	rv := v.on(e.Router)
	srcRV, src := rv, e.Src
	if e.Origin != nil {
		srcRV, src = v.on(e.Origin.Router), e.Origin.N
	}
	if !rv.Destination(e.Dst) || (src >= 0 && !srcRV.Source(src)) {
		return e, false
	}
	e = cloneEntry(e)
	for i := range e.ViaPrev {
		if o := e.ViaPrev[i].Origin; o != nil && !v.on(o.Router).Source(o.N) {
			e.ViaPrev[i].Origin = nil
		}
	}
	var gone []int
	for _, p := range e.Prev {
		if p[1] >= 0 && !rv.Source(p[1]) {
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
	e.Now = slices.DeleteFunc(e.Now, func(p [2]int) bool { return p[1] >= 0 && !rv.Source(p[1]) })
	if e.Preset != "" && !v.Preset(e.PresetID) {
		e.Preset, e.PresetID = "", ""
	}
	e.GroupN = 0
	return e, true
}
