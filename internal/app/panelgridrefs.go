package app

import (
	"iter"
	"maps"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// refEdges makes a Grid panel's edges router references, as refPages does for cells. A blank router is this one.
// The name is the router's name now, else one the body gave for the item, else the one old stored. old is nil on
// create. A reference to another router stays as it is. It runs before validatePanel, so two entries for one item
// are caught also when only one of them names the router.
func (a *App) refEdges(p *Panel, old *Panel) {
	uid := a.r().cfg.UID
	a.mu.Lock()
	src, dst := maps.Clone(a.router.srcNames), maps.Clone(a.router.dstNames)
	a.mu.Unlock()
	var oldSrc, oldDst *GridEdge
	if old != nil {
		oldSrc, oldDst = old.Sources, old.Destinations
	}
	p.Sources = refEdge(p.Sources, oldSrc, uid, src)
	p.Destinations = refEdge(p.Destinations, oldDst, uid, dst)
}

// The edge is a copy, since a stored panel can share its slices.
func refEdge(e, old *GridEdge, uid string, live map[int]string) *GridEdge {
	if e == nil {
		return nil
	}
	known := map[refKey]string{}
	note := func(edge *GridEdge) {
		if edge == nil {
			return
		}
		for r := range edgeRefs(edge) {
			if r.Name != "" {
				known[keyOf(withRouter(r, uid))] = r.Name
			}
		}
	}
	note(old)
	note(e)
	fill := func(r config.Ref) config.Ref {
		r = withRouter(r, uid)
		r.Shown = ""
		if r.Router != uid {
			return r
		}
		if name := strings.TrimSpace(live[r.N]); name != "" {
			r.Name = name
		} else if r.Name == "" {
			r.Name = known[keyOf(r)]
		}
		return r
	}
	out := *e
	if e.Members.List != nil {
		out.Members.List = make([]config.Ref, len(e.Members.List))
		for i, r := range e.Members.List {
			out.Members.List[i] = fill(r)
		}
	}
	if e.Hide != nil {
		out.Hide = make([]config.Ref, len(e.Hide))
		for i, r := range e.Hide {
			out.Hide[i] = fill(r)
		}
	}
	if e.Items != nil {
		out.Items = make([]GridItem, len(e.Items))
		for i, it := range e.Items {
			r := fill(it.ref())
			it.Router, it.Name = r.Router, r.Name
			out.Items[i] = it
		}
	}
	return &out
}

func withRouter(r config.Ref, uid string) config.Ref {
	if r.Router == "" {
		r.Router = uid
	}
	return r
}

// edgeRefs yields every reference an edge holds: members, hidden items and items with settings.
func edgeRefs(e *GridEdge) iter.Seq[config.Ref] {
	return func(yield func(config.Ref) bool) {
		for _, r := range e.Members.List {
			if !yield(r) {
				return
			}
		}
		for _, r := range e.Hide {
			if !yield(r) {
				return
			}
		}
		for _, it := range e.Items {
			if !yield(it.ref()) {
				return
			}
		}
	}
}
