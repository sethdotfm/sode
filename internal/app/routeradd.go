package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Add router and Remove router (Settings > Routers, docs/agents/settings-saves.md). The one Save keys are
// `router/new:<tmp>/add` and `router/<uid>/remove`. The other router keys are in onesavefields.go (routerAttrs). The
// first router stays.

// routersSection: Settings > Routers, the list of routers.
const routersSection = "routers"

// newRouter is the value of `router/new:<tmp>/add`: prefix, title, address, matrix and levels, and any other setting
// of routerAttrs by its JSON name. Levels 0 is the default level count.
type newRouter struct {
	Prefix  string `json:"prefix"`
	Title   string `json:"title"`
	Address string `json:"address"`
	Matrix  int    `json:"matrix"`
	Levels  int    `json:"levels"`
}

// addRouterSpec adds a router with the default settings and the values the page sent. Its UID is made one time, so
// the check and the commit agree. The detail view of a new router sends every setting; an older page sends five.
func addRouterSpec(key string) fieldSpec {
	return fieldSpec{section: routersSection, adds: true, item: key,
		get: func(config.Config) any { return nil },
		set: func(c *config.Config, v json.RawMessage, x *saveCtx) error {
			n, err := decodeJSON[newRouter](v)
			if err != nil {
				return err
			}
			var more map[string]json.RawMessage
			if err := json.Unmarshal(v, &more); err != nil {
				return err
			}
			uid, _ := x.once(key, func() (string, error) { return config.NewRouterUID(), nil })
			r := config.Defaults().DefaultRouter()
			for _, at := range routerAttrNames() {
				raw, ok := more[at]
				switch at {
				case "prefix", "title", "address", "matrix", "levels":
					continue
				}
				if !ok || string(raw) == "null" {
					continue
				}
				if err := json.Unmarshal(raw, routerField(&r, at)); err != nil {
					return fmt.Errorf("%s: %w", at, err)
				}
			}
			r.UID, r.Address, r.Matrix = uid, strings.TrimSpace(n.Address), n.Matrix
			r.Prefix = cmp.Or(strings.ToUpper(strings.TrimSpace(n.Prefix)), c.FreePrefix())
			r.Title = cmp.Or(strings.TrimSpace(n.Title), "Router "+r.Prefix)
			if n.Levels > 0 {
				r.Levels = n.Levels
			}
			c.Routers = append(slices.Clone(c.Routers), r)
			return nil
		}}
}

// removeRouterSpec removes a router, the first one too, while another router stays. What names it changes as
// dropRouter says. With the first router gone, the next one is the default router.
func removeRouterSpec(uid string) fieldSpec {
	return fieldSpec{section: routersSection, item: "router/" + uid,
		get: func(c config.Config) any { _, ok := c.RouterByUID(uid); return !ok },
		set: func(c *config.Config, _ json.RawMessage, _ *saveCtx) error {
			if _, ok := c.RouterByUID(uid); ok && len(c.Routers) < 2 {
				return errors.New("the last router stays: Sode needs one router")
			}
			dropRouter(c, uid)
			return nil
		}}
}

// routersOrderKey: the one Save key of the order of the routers, a list of router UIDs. The first router of the list
// is the default router. A router that the list does not name (one that the same save adds) comes after the named
// ones. Sode skips a UID that it does not have. A reorder reconnects no router.
const routersOrderKey = "routers/order"

func routersOrderSpec() fieldSpec {
	return fieldSpec{section: routersSection,
		get: func(c config.Config) any { return c.RouterUIDs() },
		set: func(c *config.Config, v json.RawMessage, _ *saveCtx) error {
			var uids []string
			if err := json.Unmarshal(v, &uids); err != nil {
				return err
			}
			c.Routers = reorderRouters(c.Routers, uids)
			return nil
		}}
}

// reorderRouters gives the routers in the order of uids, then the routers that uids does not name, in their order.
func reorderRouters(rs []config.Router, uids []string) []config.Router {
	out := make([]config.Router, 0, len(rs))
	used := map[string]bool{}
	for _, uid := range uids {
		if i := slices.IndexFunc(rs, func(r config.Router) bool { return r.UID == uid }); i >= 0 && !used[uid] {
			out = append(out, rs[i])
			used[uid] = true
		}
	}
	for _, r := range rs {
		if !used[r.UID] {
			out = append(out, r)
		}
	}
	return out
}

func removeKeyRouter(key string) (string, bool) {
	rest, ok := strings.CutPrefix(key, "router/")
	if !ok {
		return "", false
	}
	return strings.CutSuffix(rest, "/remove")
}

// dropRouter removes router uid from c. Presets and filter sets that name it become Broken (kind router), so their
// owners see what to fix. Rights lose the references and levels of that router, and each tie line with an end on it
// goes. Validate refuses a router that Sode does not have, and Sode never uses a router UID again.
func dropRouter(c *config.Config, uid string) {
	i := routerIndex(c, uid)
	if i < 0 {
		return
	}
	c.Routers = slices.Delete(slices.Clone(c.Routers), i, i+1)
	c.APIClients = slices.Clone(c.APIClients)
	for _, s := range itemScopes(c, "") {
		ps, fs := config.ClonePresets(*s.Presets), slices.Clone(*s.FilterSets)
		for j := range ps {
			if presetNamesRouter(ps[j], uid) {
				ps[j].Broken = withRouterMark(ps[j].Broken, uid)
			}
		}
		for j := range fs {
			if fs[j].Router == uid {
				fs[j].Broken = withRouterMark(fs[j].Broken, uid)
			}
		}
		if s.Profile == nil {
			c.Presets, c.FilterSets = ps, fs
			continue
		}
		pr := s.Profile.Clone()
		pr.Presets, pr.FilterSets = ps, fs
		*s.Profile = pr
	}
	gone := config.RefWalk{Fn: func(_ string, r config.Ref) []config.Ref {
		if r.Router == uid {
			return []config.Ref{}
		}
		return nil
	}}
	for _, r := range everyRights(c) {
		*r = r.Clone()
		gone.Rights(r)
		delete(r.Levels, uid)
		if len(r.Levels) == 0 {
			r.Levels = nil
		}
	}
	c.TieLines = slices.DeleteFunc(config.CloneTieLines(c.TieLines), func(t config.TieLine) bool { return tieNamesRouter(t, uid) })
	if c.HiddenDstsRouter == uid {
		c.HiddenDsts, c.HiddenDstsRouter = withoutNumberTerms(c.HiddenDsts), ""
	}
}

// withoutNumberTerms drops the number and range terms of a filter text whose router is gone. With a blank router,
// web/filter.js matches those terms on every router, so the text would match more. Name terms stay. A text that
// had a term that matches but keeps none, or that does not parse, becomes blank: it then matches nothing.
func withoutNumberTerms(text string) string {
	terms, ok := filterTerms(text)
	if !ok {
		return ""
	}
	r := []rune(text)
	var kept []string
	dropped, matches := false, false
	for _, t := range terms {
		if t.Kind == "num" || t.Kind == "range" {
			dropped = dropped || !t.Not
			continue
		}
		matches = matches || !t.Not
		s := string(r[t.at:t.end])
		if t.Not {
			s = "!" + s
		}
		kept = append(kept, s)
	}
	if dropped && !matches {
		return ""
	}
	return strings.Join(kept, ", ")
}

func tieNamesRouter(t config.TieLine, uid string) bool {
	return t.From.Router == uid || t.To.Router == uid
}

// withRouterMark adds the Broken mark of a router that is gone, one time.
func withRouterMark(l []config.Missing, uid string) []config.Missing {
	var b brokenList
	b.out = slices.Clone(l)
	b.add(config.Missing{Kind: config.MissingRouter, Ref: config.Ref{Router: uid}}, true)
	return b.out
}

func presetNamesRouter(p config.Preset, uid string) bool {
	found := false
	w := config.RefWalk{Fn: func(_ string, r config.Ref) []config.Ref { found = found || r.Router == uid; return nil }}
	for _, it := range p.Items {
		w.Item(&it)
	}
	return found
}

// panelNames: a reference of p, or its level subset, names router uid.
func panelNames(p Panel, uid string) bool {
	found := false
	layoutOf(p).walkRefs(func(_ refSlot, _ string, r config.Ref) []config.Ref {
		found = found || r.Router == uid
		return nil
	}, false)
	if by, ok := p.Settings[panelLevelsKey].(map[string]any); ok {
		_, named := by[uid]
		found = found || named
	}
	return found
}

// routerGonePanel marks the cells and Grid members of p that name router uid Broken and drops its level subset of
// that router. Hidden items, item settings and fixed destinations stay and match nothing, as for a gone number.
func routerGonePanel(p *Panel, uid string) bool {
	if !panelNames(*p, uid) {
		return false
	}
	marked := false
	layoutOf(*p).walkRefs(func(s refSlot, _ string, r config.Ref) []config.Ref {
		marked = marked || (s == slotCell || s == slotMember) && r.Router == uid
		return nil
	}, false)
	if marked {
		p.Broken = withRouterMark(p.Broken, uid)
	}
	if by, ok := p.Settings[panelLevelsKey].(map[string]any); ok {
		if _, named := by[uid]; named {
			by = maps.Clone(by)
			delete(by, uid)
			p.Settings = maps.Clone(p.Settings)
			if len(by) == 0 {
				delete(p.Settings, panelLevelsKey)
			} else {
				p.Settings[panelLevelsKey] = by
			}
		}
	}
	return true
}

// routersGoneStores runs after a save that removed routers: panels, Default artwork and the saved names table.
func (a *App) routersGoneStores(uids []string) {
	for _, uid := range uids {
		if err := a.panels.rewrite(func(p *Panel) bool { return routerGonePanel(p, uid) }); err != nil {
			log.Printf("routers: panels not all marked for the removed router: %v", err)
		}
		if err := a.artDefaults.dropRouter(uid); err != nil {
			log.Printf("routers: Default artwork of the removed router not cleared: %v", err)
		}
		if err := a.names.remove(uid); err != nil {
			log.Printf("routers: names table of the removed router not removed: %v", err)
		}
	}
}

// routerUse: one item that a router delete breaks. ID is the swapItem id.
type routerUse struct {
	ID    string `json:"id"`
	Kind  string `json:"kind"` // preset, filterSet, panel or tieLine
	Name  string `json:"name"`
	Owner string `json:"owner"` // the user's name, "" for Everyone and a tie line
}

type routerUsesResult struct {
	opResult
	Router string      `json:"router"`
	Items  []routerUse `json:"items"`
}

// routerUses lists every preset, filter set and panel that names router uid, and every tie line that the removal
// deletes, for the Remove router note.
func routerUses(cfg config.Config, panels []Panel, uid string) []routerUse {
	out := []routerUse{}
	for _, t := range cfg.TieLines {
		if tieNamesRouter(t, uid) {
			out = append(out, routerUse{"tieLine/" + t.ID, "tieLine", t.Name, ""})
		}
	}
	for _, s := range itemScopes(&cfg, "") {
		for _, p := range *s.Presets {
			if presetNamesRouter(p, uid) {
				out = append(out, routerUse{"preset/" + s.ID + "/" + p.ID, "preset", p.Name, s.Name})
			}
		}
		for _, f := range *s.FilterSets {
			if f.Router == uid {
				out = append(out, routerUse{"filterSet/" + s.ID + "/" + f.ID, "filterSet", f.Name, s.Name})
			}
		}
	}
	for _, p := range panels {
		if panelNames(p, uid) {
			u, _ := cfg.FindUserByID(p.Owner)
			out = append(out, routerUse{"panel/" + p.ID, "panel", p.Name, u.Name})
		}
	}
	slices.SortStableFunc(out, func(a, b routerUse) int {
		return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.Owner, b.Owner), cmp.Compare(a.Name, b.Name))
	})
	return out
}

// serveRouterUses: everyone's items, so Administrator only (the policy row).
func (a *App) serveRouterUses(c *client, m *inMsg) {
	cfg := a.r().cfg
	var err error
	var items []routerUse
	if _, ok := cfg.RouterByUID(m.Router); !ok || m.Router == "" {
		err = errors.New("no such router: reload the page")
	} else {
		items = routerUses(cfg, a.panels.list(), m.Router)
	}
	a.reply(c, routerUsesResult{opResult: opResultFor("routerUses", &m.ID, err), Router: m.Router, Items: items})
}
