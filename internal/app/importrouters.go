package app

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Router UIDs of an import (docs/agents/profiles.md, Router map). The remap step gives each file router a target
// here, or none. An item that uses a router with no target is not imported, and Rights lose that router. Every
// place in a file that names a router goes through this map, so an import never leaves a UID that Sode does not
// have. Tie lines (their own export category) take their two routers through the same map.

// routerMap: file router UID → the UID of its target here, "" for none. A UID not in the map is not a router of
// the file's manifest and stays as it is.
type routerMap map[string]string

// to gives the UID that a file UID becomes here. ok is false for a file router with no target.
func (m routerMap) to(uid string) (string, bool) {
	to, in := m[uid]
	if !in {
		return uid, true
	}
	return to, to != ""
}

func (m routerMap) unmapped(uid string) bool {
	_, ok := m.to(uid)
	return !ok
}

// routerKeys visits each router UID in the file that is not a reference with a number. These are the keys of Rights
// levels and panel level subsets (`settings.levels`), and the router of a cell with no number. Also Settings
// hiddenDstsRouter and the router of This browser's filter sets. References with a number go through refWalk. fn answers the UID to put there; false drops
// a level list and blanks the others. item has the form of refWalk's.
func routerKeys(b *exportBundle, fn func(item, uid string) (string, bool)) {
	rights := func(item string, r *config.Rights) {
		if r.Levels == nil {
			return
		}
		out := config.RouterLevels{}
		for _, uid := range slices.Sorted(maps.Keys(r.Levels)) {
			to := uid
			if uid != "" {
				var keep bool
				if to, keep = fn(item, uid); !keep {
					continue
				}
			}
			out[to] = r.Levels[uid]
		}
		if len(out) == 0 {
			out = nil
		}
		r.Levels = out
	}
	for i := range b.Users {
		u := &b.Users[i]
		rights("user/"+cmp.Or(u.ID, u.Name), &u.Rights)
	}
	for i := range b.APIClients {
		rights("client/"+b.APIClients[i].Name, &b.APIClients[i].Rights)
	}
	if b.Settings != nil {
		rights("guest", &b.Settings.GuestRights)
		if uid := b.Settings.HiddenDstsRouter; uid != "" {
			to, keep := fn("settings", uid)
			if !keep {
				// A router with no target: its numbers match nothing here, as for Remove router.
				to, b.Settings.HiddenDsts = "", withoutNumberTerms(b.Settings.HiddenDsts)
			}
			b.Settings.HiddenDstsRouter = to
		}
	}
	layout := func(item string, l *panelLayout) {
		l.Settings = levelKeys(item, l.Settings, fn)
		if l.Pages == nil {
			return
		}
		pages := make([]PanelPage, len(l.Pages))
		for i, g := range l.Pages {
			pages[i] = g
			pages[i].Cells = slices.Clone(g.Cells)
			for j, c := range g.Cells {
				uid, _ := c["router"].(string)
				if _, ok := cellRef(c); uid == "" || ok && cellKinds[c["kind"]] != "" {
					continue
				}
				c = maps.Clone(c)
				if to, keep := fn(item, uid); keep {
					c["router"] = to
				} else {
					delete(c, "router")
				}
				pages[i].Cells[j] = c
			}
		}
		l.Pages = pages
	}
	for i := range b.Panels.Panels {
		p := &b.Panels.Panels[i]
		l := layoutOf(*p)
		layout("panel/"+p.ID, &l)
		p.Pages, p.Settings = l.Pages, l.Settings
	}
	for id, cps := range b.Panels.Checkpoints {
		for i := range cps {
			layout("panel/"+id, &cps[i].Layout)
		}
	}
	for i := range b.Panels.Templates {
		t := &b.Panels.Templates[i]
		l := panelLayout{Pages: t.Pages, Settings: t.Settings}
		layout("template/"+t.ID, &l)
		t.Pages, t.Settings = l.Pages, l.Settings
	}
	for _, s := range b.Panels.CheckpointSets {
		for i := range s.Entries {
			layout("set/"+s.ID, &s.Entries[i].panelLayout)
		}
	}
	sets, _ := b.Browser["filterSets"].([]any)
	for _, s := range sets {
		m, _ := s.(map[string]any)
		uid, _ := m["router"].(string)
		if uid == "" {
			continue
		}
		name, _ := m["name"].(string)
		to, keep := fn("browserSet/"+name, uid)
		if !keep {
			to = ""
		}
		m["router"] = to
	}
}

// levelKeys gives settings with the router UIDs of its level subset as fn answers them.
func levelKeys(item string, settings map[string]any, fn func(item, uid string) (string, bool)) map[string]any {
	by, ok := settings[panelLevelsKey].(map[string]any)
	if !ok {
		return settings
	}
	out := map[string]any{}
	for _, uid := range slices.Sorted(maps.Keys(by)) {
		to := uid
		if uid != "" {
			var keep bool
			if to, keep = fn(item, uid); !keep {
				continue
			}
		}
		out[to] = by[uid]
	}
	settings = maps.Clone(settings)
	if len(out) == 0 {
		delete(settings, panelLevelsKey)
	} else {
		settings[panelLevelsKey] = out
	}
	return settings
}

// noteDrops fills p.drops: each item that uses a file router with no target. It runs before the move, while every
// reference still names its file router.
func (p *importPlanner) noteDrops() {
	note := func(item, uid string) {
		if !p.routers.unmapped(uid) {
			return
		}
		title := p.fileRouterTitle(uid)
		if !slices.Contains(p.drops[item], title) {
			p.drops[item] = append(p.drops[item], title)
		}
	}
	refWalk(p.b, func(_ string, r config.Ref, _ refUse, item string) []config.Ref {
		note(item, r.Router)
		return nil
	})
	routerKeys(p.b, func(item, uid string) (string, bool) {
		note(item, uid)
		return uid, true
	})
	p.eachFileSet(func(key string, s *config.FilterSet) { note("filterSet/"+key, s.Router) })
}

func (p *importPlanner) fileRouterTitle(uid string) string {
	for _, r := range p.b.Manifest.Routers {
		if r.UID == uid {
			return cmp.Or(r.Title, r.Prefix)
		}
	}
	return uid
}

// dropNote: why the import leaves item out, or "" when it uses no router without a target.
func (p *importPlanner) dropNote(item string) string {
	titles := p.drops[item]
	if len(titles) == 0 {
		return ""
	}
	return "it uses router " + strings.Join(titles, ", ") + ", which has no target here"
}

// rightsNote: the note on an item whose Rights lose the routers with no target.
func (p *importPlanner) rightsNote(item string) string {
	titles := p.drops[item]
	if len(titles) == 0 {
		return ""
	}
	return "Its Rights lose router " + strings.Join(titles, ", ") + ", which has no target here"
}

// dropBrowser removes This browser's presets and filter sets that use a router with no target.
func (p *importPlanner) dropBrowser() {
	for _, k := range []struct{ list, item string }{{"presets", "browser/"}, {"filterSets", "browserSet/"}} {
		list, ok := p.b.Browser[k.list].([]any)
		if !ok {
			continue
		}
		p.b.Browser[k.list] = slices.DeleteFunc(slices.Clone(list), func(x any) bool {
			m, _ := x.(map[string]any)
			name, _ := m["name"].(string)
			return len(p.drops[k.item+name]) > 0
		})
	}
}

// dropMine leaves out the Me presets and filter sets of file user fileID that use a router with no target, and says
// so in note.
func (p *importPlanner) dropMine(fileID string, pr config.Profile, note string) (config.Profile, string) {
	n := 0
	drop := func(item string) bool {
		gone := len(p.drops[item]) > 0
		if gone {
			n++
		}
		return gone
	}
	if pr.Presets != nil {
		pr.Presets = slices.DeleteFunc(slices.Clone(pr.Presets), func(s config.Preset) bool { return drop(fileID + "/" + s.ID) })
	}
	if pr.FilterSets != nil {
		pr.FilterSets = slices.DeleteFunc(slices.Clone(pr.FilterSets), func(s config.FilterSet) bool { return drop("filterSet/" + fileID + "/" + s.ID) })
	}
	if n > 0 {
		note = joinNotes(note, fmt.Sprintf("Leaves out %d of its presets and filter sets, which use a router with no target here", n))
	}
	return pr, note
}

// joinNotes puts two item notes in one.
func joinNotes(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	}
	return a + ". " + b
}
