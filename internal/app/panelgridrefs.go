package app

import (
	"maps"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// A blank router is this one; a reference to another router stays. The name: the router's now, else the body's, else
// old's (nil on create). Runs before validatePanel, so two entries for one item are caught when only one names the
// router.
func (a *App) refEdges(p *Panel, old *Panel) {
	var oldSrc, oldDst *GridEdge
	if old != nil {
		oldSrc, oldDst = old.Sources, old.Destinations
	}
	p.Sources, p.Destinations = a.edgeRefer()(p.Sources, p.Destinations, oldSrc, oldDst)
}

// edgeRefer is refEdges over the UID and names taken now. The func takes no lock, so the panel store can call it
// under its own (layoutRefer).
func (a *App) edgeRefer() func(src, dst, oldSrc, oldDst *GridEdge) (*GridEdge, *GridEdge) {
	uid := a.r().cfg.UID
	a.mu.Lock()
	srcNames, dstNames := maps.Clone(a.router.srcNames), maps.Clone(a.router.dstNames)
	a.mu.Unlock()
	return func(src, dst, oldSrc, oldDst *GridEdge) (*GridEdge, *GridEdge) {
		return refEdge(src, oldSrc, uid, srcNames), refEdge(dst, oldDst, uid, dstNames)
	}
}

// The edge is a copy, since a stored panel can share its slices.
func refEdge(e, old *GridEdge, uid string, live map[int]string) *GridEdge {
	known := map[refKey]string{}
	for _, edge := range []*GridEdge{old, e} {
		edge.walkRefs("", func(_ refSlot, _ string, r config.Ref) []config.Ref {
			if r.Name != "" {
				known[keyOf(withRouter(r, uid))] = r.Name
			}
			return nil
		}, false)
	}
	return e.walkRefs("", func(_ refSlot, _ string, r config.Ref) []config.Ref {
		r = withRouter(r, uid)
		r.Shown = ""
		if name := strings.TrimSpace(live[r.N]); r.Router == uid && name != "" {
			r.Name = name
		} else if r.Router == uid && r.Name == "" {
			r.Name = known[keyOf(r)]
		}
		return []config.Ref{r}
	}, false)
}

func withRouter(r config.Ref, uid string) config.Ref {
	if r.Router == "" {
		r.Router = uid
	}
	return r
}
