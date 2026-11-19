package app

import (
	"maps"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// refSlot is where a panel layout holds a reference. Callers treat slots apart: the Broken check marks cells and
// members only.
type refSlot int

const (
	slotCell   refSlot = iota // a Buttons Source or Destination cell
	slotMember                // a Grid edge's member list
	slotHide
	slotItem  // a Grid item's settings
	slotFixed // settings.fixedDests
)

type slotFn func(slot refSlot, kind string, r config.Ref) []config.Ref

var cellKinds = map[any]string{"source": config.MissingSource, "destination": config.MissingDestination}

// walkRefs is config.RefWalk over every reference slot of l (LegacyCells aside), with the slot. What changes is a
// copy, since a stored panel shares its cells and lists.
func (l panelLayout) walkRefs(fn slotFn, dedupe bool) panelLayout {
	if l.Pages != nil {
		pages := make([]PanelPage, len(l.Pages))
		for i, g := range l.Pages {
			pages[i] = g
			pages[i].Cells = slices.Clone(g.Cells)
			for j, c := range g.Cells {
				r, ok := cellRef(c)
				kind := cellKinds[c["kind"]]
				if !ok || kind == "" {
					continue
				}
				if out := fn(slotCell, kind, r); len(out) > 0 {
					pages[i].Cells[j] = withCellRef(c, out[0])
				}
			}
		}
		l.Pages = pages
	}
	l.Sources = l.Sources.walkRefs(config.MissingSource, fn, dedupe)
	l.Destinations = l.Destinations.walkRefs(config.MissingDestination, fn, dedupe)
	if list, err := fixedList(l.Settings[panelFixedKey]); err == nil && list != nil {
		answered := false
		w := config.RefWalk{Dedupe: dedupe, Fn: func(kind string, r config.Ref) []config.Ref {
			out := fn(slotFixed, kind, r)
			answered = answered || out != nil
			return out
		}}
		if list = w.List(config.MissingDestination, list); answered {
			l.Settings = maps.Clone(l.Settings)
			l.Settings[panelFixedKey] = list
		}
	}
	return l
}

func (e *GridEdge) walkRefs(kind string, fn slotFn, dedupe bool) *GridEdge {
	if e == nil {
		return nil
	}
	in := func(s refSlot) config.RefWalk {
		return config.RefWalk{Dedupe: dedupe, Fn: func(kind string, r config.Ref) []config.Ref { return fn(s, kind, r) }}
	}
	out := *e
	out.Members.List, out.Hide = in(slotMember).List(kind, e.Members.List), in(slotHide).List(kind, e.Hide)
	if e.Items != nil {
		out.Items = make([]GridItem, 0, len(e.Items))
		for _, it := range e.Items {
			if got := fn(slotItem, kind, it.ref()); got != nil {
				if len(got) == 0 {
					continue
				}
				it.Router, it.N, it.Name = got[0].Router, got[0].N, got[0].Name
			}
			out.Items = append(out.Items, it)
		}
	}
	return &out
}

func withCellRef(c map[string]any, r config.Ref) map[string]any {
	c = maps.Clone(c)
	delete(c, "shown")
	c["router"], c["n"] = r.Router, r.N
	if r.Name != "" || c["name"] != nil {
		c["name"] = r.Name
	}
	return c
}

// readRefs calls fn on every reference of l, for a check that changes nothing.
func (l panelLayout) readRefs(fn func(slot refSlot, kind string, r config.Ref)) {
	l.walkRefs(func(s refSlot, kind string, r config.Ref) []config.Ref { fn(s, kind, r); return nil }, false)
}
