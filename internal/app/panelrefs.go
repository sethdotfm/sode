package app

import (
	"maps"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// refPages makes a Buttons panel's or template's cells router references, as namePresets does for presets.
// Source and Destination cells get {router, n, name}. A blank router is this one. The name is the router's name
// now, else the one the cell came with. A Preset cell gets the preset's hidden id and its name now, so it survives
// a rename. Me presets are the saving user's (userID). Cells are copies, since a stored panel can share them.
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
	out := make([]PanelPage, len(pages))
	for i, g := range pages {
		out[i] = g
		out[i].Cells = make([]map[string]any, len(g.Cells))
		for j, c := range g.Cells {
			c = maps.Clone(c)
			switch c["kind"] {
			case "source":
				refCell(c, cfg.UID, src)
			case "destination":
				refCell(c, cfg.UID, dst)
			case "preset":
				list := cfg.Presets
				if c["scope"] == "me" {
					list = mine
				}
				presetCell(c, list)
			}
			out[i].Cells[j] = c
		}
	}
	return out
}

// Numbers are float64 from JSON and int from YAML. A cell with no number is unassigned and stays as it is.
func refCell(c map[string]any, uid string, names map[int]string) {
	var n int
	switch x := c["n"].(type) {
	case float64:
		n = int(x)
	case int:
		n = x
	default:
		return
	}
	delete(c, "shown")
	r, _ := c["router"].(string)
	if r == "" {
		r = uid
		c["router"] = r
	}
	if r != uid {
		return
	}
	if name := strings.TrimSpace(names[n]); name != "" {
		c["name"] = name
	}
}

// The id wins, so a renamed preset is still found. A cell with no id is matched by name once. An id not in the
// list stays: it can be another user's Me preset, when a Trusted user saves someone else's panel.
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
