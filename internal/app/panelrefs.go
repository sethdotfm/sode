package app

import (
	"maps"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Source and Destination cells get {router, n, name}; a blank router is this one, the name is the router's now, else the
// cell's. A Preset cell gets the preset's hidden id, so it survives a rename. Cells are copies, since a stored panel can
// share them.
func (a *App) refPages(pages []PanelPage, userID string) []PanelPage {
	return a.pageRefer()(pages, userID)
}

// pageRefer is refPages over a copy of the config and names taken now. The func takes no lock, so the panel store
// can call it under its own.
func (a *App) pageRefer() func(pages []PanelPage, userID string) []PanelPage {
	cfg := a.r().cfg
	a.mu.Lock()
	src, dst := maps.Clone(a.router.srcNames), maps.Clone(a.router.dstNames)
	a.mu.Unlock()
	return func(pages []PanelPage, userID string) []PanelPage {
		return refPagesWith(cfg, src, dst, pages, userID)
	}
}

// layoutRefer makes a layout's Buttons cells (for the panel's owner) or Grid edges router references.
type layoutRefer func(l panelLayout, owner string) panelLayout

func (a *App) layoutRefer() layoutRefer {
	refer, edges := a.pageRefer(), a.edgeRefer()
	return func(l panelLayout, owner string) panelLayout {
		if layoutType(l.Type) == panelTypeButtons {
			l.Pages = refer(l.Pages, owner)
		} else {
			l.Sources, l.Destinations = edges(l.Sources, l.Destinations, nil, nil)
		}
		return l
	}
}

func refPagesWith(cfg config.Config, src, dst map[int]string, pages []PanelPage, userID string) []PanelPage {
	var mine []config.Preset
	if u, ok := cfg.FindUserByID(userID); ok {
		mine = u.Profile.Presets
	}
	if pages == nil {
		pages = []PanelPage{}
	}
	out := panelLayout{Pages: pages}.walkRefs(func(_ refSlot, kind string, r config.Ref) []config.Ref {
		r = withRouter(r, cfg.UID)
		if name := strings.TrimSpace(sideNames(kind, src, dst)[r.N]); r.Router == cfg.UID && name != "" {
			r.Name = name
		}
		return []config.Ref{r}
	}, false).Pages
	for _, g := range out {
		for j, c := range g.Cells {
			g.Cells[j] = maps.Clone(c)
			if c["kind"] != "preset" {
				continue
			}
			list := cfg.Presets
			if c["scope"] == "me" {
				list = mine
			}
			presetCell(g.Cells[j], list)
		}
	}
	return out
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
