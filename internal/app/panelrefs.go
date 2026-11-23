package app

import (
	"maps"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// refNames is the config and router names taken at one moment. It takes no lock, so the panel store can use it
// under its own.
type refNames struct {
	cfg      config.Config
	src, dst map[int]string
}

func (a *App) refNames() refNames {
	n := refNames{cfg: a.r().cfg}
	first := a.firstUnit()
	a.mu.Lock()
	n.src, n.dst = maps.Clone(first.router.srcNames), maps.Clone(first.router.dstNames)
	a.mu.Unlock()
	return n
}

// Source and Destination cells get {router, n, name}; a blank router is this one, the name is the router's now, else the
// cell's. A Preset cell gets the preset's hidden id, so it survives a rename. Cells are copies, since a stored panel can
// share them.
func (a *App) refPages(pages []PanelPage, userID string) []PanelPage {
	return a.refNames().pages(pages, userID)
}

// The name: the router's now, else the body's, else old's (nil on create). Runs before validatePanel, so two entries
// for one item are caught when only one names the router.
func (a *App) refEdges(p *Panel, old *Panel) {
	var oldSrc, oldDst *GridEdge
	if old != nil {
		oldSrc, oldDst = old.Sources, old.Destinations
	}
	n := a.refNames()
	p.Sources, p.Destinations = n.edge(p.Sources, oldSrc, n.src), n.edge(p.Destinations, oldDst, n.dst)
}

// layoutRefer makes a layout's Buttons cells (for the panel's owner) or Grid edges router references.
type layoutRefer func(l panelLayout, owner string) panelLayout

func (a *App) layoutRefer() layoutRefer {
	n := a.refNames()
	return func(l panelLayout, owner string) panelLayout {
		if layoutType(l.Type) == panelTypeButtons {
			l.Pages = n.pages(l.Pages, owner)
		} else {
			l.Sources, l.Destinations = n.edge(l.Sources, nil, n.src), n.edge(l.Destinations, nil, n.dst)
		}
		return l
	}
}

func (n refNames) pages(pages []PanelPage, userID string) []PanelPage {
	var mine []config.Preset
	if u, ok := n.cfg.FindUserByID(userID); ok {
		mine = u.Profile.Presets
	}
	if pages == nil {
		pages = []PanelPage{}
	}
	out := panelLayout{Pages: pages}.walkRefs(func(_ refSlot, kind string, r config.Ref) []config.Ref {
		r = withRouter(r, n.cfg.UID)
		if name := strings.TrimSpace(sideNames(kind, n.src, n.dst)[r.N]); r.Router == n.cfg.UID && name != "" {
			r.Name = name
		}
		return []config.Ref{r}
	}, false).Pages
	for _, g := range out {
		for j, c := range g.Cells {
			g.Cells[j] = maps.Clone(c)
			// A cell's levels are levels of its router. A request can leave it out on a cell with no source or
			// destination, which names the one router.
			if r, _ := c["router"].(string); r == "" && c[panelLevelsKey] != nil {
				g.Cells[j]["router"] = n.cfg.UID
			}
			if c["kind"] != "preset" {
				continue
			}
			list := n.cfg.Presets
			if c["scope"] == "me" {
				list = mine
			}
			presetCell(g.Cells[j], list)
		}
	}
	return out
}

// edge is a copy, since a stored panel can share its slices. A reference to another router stays.
func (n refNames) edge(e, old *GridEdge, live map[int]string) *GridEdge {
	uid := n.cfg.UID
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

// The id wins, so a renamed preset is still found. A cell with no id is matched by name one time. An id not in the list
// stays: it can be another user's Me preset.
func presetCell(c map[string]any, list []config.Preset) {
	id, _ := c["id"].(string)
	name, _ := c["name"].(string)
	if id != "" {
		for _, p := range list {
			if p.ID == id {
				c["name"] = p.Name
			}
		}
		return
	}
	for _, p := range list {
		if strings.EqualFold(p.Name, strings.TrimSpace(name)) {
			c["id"], c["name"] = p.ID, p.Name
			return
		}
	}
}
