package app

import (
	"cmp"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Replace import (docs/agents/profiles.md): every file item is Use new, then emptyRest removes the live items of
// the category that the file does not hold. So hidden ids, key links and Authentication modes of the items the
// file holds stay, and an equal item writes nothing.

// importRemovals is what a Replace import removes from the stores outside the config, after the config commit.
type importRemovals struct {
	users     map[string]bool // live user ids; their panels and API clients go to the importing Administrator
	icons     []string
	defaults  []ArtDefault
	panels    []string
	templates []string
	sets      []string

	// kept: live ids and Default artwork keys that a file item matched, refused or not, so emptyRest leaves them.
	kept map[string]bool
}

func (r *importRemovals) keep(id string) {
	if r.kept == nil {
		r.kept = map[string]bool{}
	}
	r.kept[id] = true
}

// fileHash: the file user carries a PIN hash this Sode can check, with its length.
func fileHash(u config.User) bool {
	return config.ValidPinHash(u.Pin) && u.Len >= config.UserPinMin && u.Len <= config.UserPinMax
}

// adminFor is who gets the items of removed owners. That is the importing user while the import keeps them.
// Else it is the first Administrator with a PIN that the import leaves.
func (p *importPlanner) adminFor(cfg config.Config) string {
	if _, ok := cfg.FindUserByID(p.actor); ok || p.actor == "" {
		return p.actor
	}
	for _, u := range cfg.Users {
		if u.Role == config.RoleAdministrator && config.ValidPinHash(u.Pin) {
			return u.ID
		}
	}
	return p.actor
}

func removeItem(cat *importCat, key, name, kind string) {
	cat.Items = append(cat.Items, importItem{Key: key, Name: name, Kind: kind, Remove: true})
}

// A refused item keeps its twin too.
func keptNames(cat *importCat) map[string]bool {
	out := map[string]bool{}
	for _, it := range cat.Items {
		out[strings.ToLower(strings.TrimSpace(cmp.Or(it.As, it.Clash, it.Name)))] = true
	}
	return out
}

// keptIDs is the live ids m maps file items to, with those kept.
func (p *importPlanner) keptIDs(m map[string]string) map[string]bool {
	out := map[string]bool{}
	maps.Copy(out, p.removed.kept)
	for _, v := range m {
		if v != "" {
			out[v] = true
		}
	}
	return out
}

// keepWaiting keeps the live twins of the file items that wait for router names.
func (p *importPlanner) keepWaiting(cat string, cand *config.Config) {
	for key := range p.waits[cat] {
		if !p.wantedItem(cat, key) {
			continue
		}
		switch cat {
		case catUsers:
			if i := slices.IndexFunc(p.b.Users, func(u config.User) bool { return cmp.Or(u.ID, u.Name) == key }); i >= 0 {
				if j := matchUser(p.same, cand.Users, p.b.Users[i]); j >= 0 {
					p.removed.keep(cand.Users[j].ID)
				}
			}
		case catPresets:
			if i := slices.IndexFunc(p.b.Presets, func(s config.Preset) bool { return s.ID == key }); i >= 0 {
				if j := matchPreset(p.same, cand.Presets, p.b.Presets[i]); j >= 0 {
					p.removed.keep(cand.Presets[j].ID)
				}
			}
		case catPanels:
			if i := slices.IndexFunc(p.b.Panels.Panels, func(x Panel) bool { return x.ID == key }); i >= 0 {
				p.keepLivePanel(p.b.Panels.Panels[i])
			}
		}
	}
}

// emptyRest removes from cand, or lists for removal after the commit, the live items of cat that the step
// did not match or add.
func (p *importPlanner) emptyRest(cat string, cand *config.Config, c *importCat) error {
	p.keepWaiting(cat, cand)
	switch cat {
	case catUsers:
		keep := p.keptIDs(p.users)
		var out []config.User
		for _, u := range cand.Users {
			_, live := p.live.FindUserByID(u.ID)
			if keep[u.ID] || p.keepAdmins && live && u.Role == config.RoleAdministrator || p.noHashes && u.ID == p.actor {
				out = append(out, u)
				continue
			}
			removeItem(c, u.ID, u.Name, "")
			p.removed.users[u.ID] = true
		}
		cand.Users = out
		// As for a user removed in Settings: their API clients go to the importing Administrator.
		to := p.adminFor(*cand)
		for i, cl := range cand.APIClients {
			if p.removed.users[cl.Owner] {
				cand.APIClients[i].Owner = to
				p.pending = append(p.pending, "apiClients")
			}
		}
	case catPresets:
		keep := p.keptIDs(p.presets)
		cand.Presets = slices.DeleteFunc(cand.Presets, func(x config.Preset) bool {
			if !keep[x.ID] {
				removeItem(c, x.ID, x.Name, "")
			}
			return !keep[x.ID]
		})
	case catFilterSets:
		keep := keptNames(c)
		cand.FilterSets = slices.DeleteFunc(cand.FilterSets, func(x config.FilterSet) bool {
			gone := !keep[strings.ToLower(strings.TrimSpace(x.Name))]
			if gone {
				removeItem(c, x.Name, x.Name, "")
			}
			return gone
		})
	case catAPIClients:
		keep := keptNames(c)
		// A panel's own client goes with its panel, not with this category.
		cand.APIClients = slices.DeleteFunc(cand.APIClients, func(x config.APIClient) bool {
			gone := x.Panel == "" && !keep[strings.ToLower(strings.TrimSpace(x.Name))]
			if gone {
				removeItem(c, x.Name, x.Name, "")
			}
			return gone
		})
	case catArtwork:
		keep := p.keptIDs(p.icons)
		for _, ic := range p.liveIcons {
			if !keep[ic.ID] {
				removeItem(c, ic.ID, ic.Name, kindIcon)
				p.removed.icons = append(p.removed.icons, ic.ID)
			}
		}
		for _, d := range p.liveDefaults {
			if !keep[d.key()] {
				removeItem(c, d.key(), d.Name, kindDefault)
				p.removed.defaults = append(p.removed.defaults, d)
			}
		}
	case catPanels:
		keep := p.keptIDs(p.panelIDs)
		gone := map[string]bool{}
		for _, lp := range p.livePanels {
			if !keep[lp.ID] {
				removeItem(c, lp.ID, lp.Name, kindPanel)
				p.removed.panels = append(p.removed.panels, lp.ID)
				gone[lp.ID] = true
			}
		}
		adds := 0
		for _, pp := range p.panels {
			if pp.Live == nil {
				adds++
			}
		}
		if len(p.livePanels)-len(gone)+adds > panelMax {
			return errPanelCap
		}
		// A removed panel's own API client goes with it.
		n := len(cand.APIClients)
		cand.APIClients = slices.DeleteFunc(cand.APIClients, func(x config.APIClient) bool { return gone[x.Panel] })
		if len(cand.APIClients) != n {
			p.pending = append(p.pending, "apiClients")
		}
		for _, t := range p.liveTemplates {
			if !keep[t.ID] {
				removeItem(c, t.ID, t.Name, kindTemplate)
				p.removed.templates = append(p.removed.templates, t.ID)
			}
		}
		for _, s := range p.liveSets {
			if !keep[s.ID] {
				removeItem(c, s.ID, s.Name, kindSet)
				p.removed.sets = append(p.removed.sets, s.ID)
			}
		}
	}
	return nil
}

// writeImportRemovals runs before the stores take the file's items, so a freed name or place is theirs.
func (a *App) writeImportRemovals(p *importPlanner, failed map[string]string) {
	r := p.removed
	fail := func(key, what string, err error) {
		if err != nil {
			failed[key] = err.Error()
			log.Printf("import: removing %s %s: %v", what, key, err)
		}
	}
	for _, id := range r.icons {
		_, _, err := a.icons.remove(id)
		fail(id, "icon", err)
	}
	for _, d := range r.defaults {
		_, err := a.artDefaults.clear(d)
		fail(d.key(), "Default artwork", err)
	}
	for _, id := range r.panels {
		_, err := a.panels.delete(id)
		fail(id, "panel", err)
	}
	for _, id := range r.templates {
		_, err := a.templates.delete(id)
		fail(id, "template", err)
	}
	for _, id := range r.sets {
		fail(id, "Checkpoint set", a.panels.sets.delete(id))
	}
}

// As removing a user in Settings does.
func (a *App) moveRemovedUsersPanels(p *importPlanner) {
	if len(p.removed.users) == 0 || p.actor == "" {
		return
	}
	to := map[string]string{}
	for id := range p.removed.users {
		to[id] = p.actor
	}
	if n, err := a.panels.moveOwners(to); err != nil {
		log.Printf("import: moved %d panels of removed users, then: %v", n, err)
	}
}
