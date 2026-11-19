package app

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Remap step: docs/agents/profiles.md.

type remapInfo struct {
	Routers []remapRouter `json:"routers"`
	Targets []remapTarget `json:"targets"`
}

type remapRouter struct {
	UID        string      `json:"uid"`
	Title      string      `json:"title"`
	Target     string      `json:"target"` // "" is none: its items arrive Broken
	Default    string      `json:"default"`
	Candidates []remapCand `json:"candidates"`
	Refs       []remapRef  `json:"refs"`
	// Names is the file's names table and Offset its shown number of source 0, for the filter term rows.
	Names  *remapNames `json:"names,omitempty"`
	Offset int         `json:"offset"`
	Sets   []remapSet  `json:"sets,omitempty"`
}

type remapNames struct {
	Sources      []namedItem `json:"sources"`
	Destinations []namedItem `json:"destinations"`
}

// remapSet is a file filter set on the router. Key is the key in importReq.Filters: the owner's file user id
// ("" for a shared set), "/" and the name.
type remapSet struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	User string `json:"user,omitempty"` // the owner's name
	Src  string `json:"src"`
	Dst  string `json:"dst"`
}

// remapFilter is the page's text for a set's sides; nil keeps the file's.
type remapFilter struct {
	Src *string `json:"src,omitempty"`
	Dst *string `json:"dst,omitempty"`
}

// remapCand: Found of the file router's Total names are on the target (trailing spaces and case ignored).
type remapCand struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	Found int    `json:"found"`
	Total int    `json:"total"`
}

// remapRef is one distinct file source or destination. Key is the key in importReq.Names.
type remapRef struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	N       int    `json:"n"`
	Name    string `json:"name"`
	Presets int    `json:"presets,omitempty"`
	Panels  int    `json:"panels,omitempty"`
	Users   int    `json:"users,omitempty"`
}

type remapTarget struct {
	UID          string      `json:"uid"`
	Title        string      `json:"title"`
	Sources      []namedItem `json:"sources"`
	Destinations []namedItem `json:"destinations"`
	Offset       int         `json:"offset"`
	Offline      bool        `json:"offline,omitempty"`
	// SavedAt: the names are the saved names table from then, since no read has finished since Sode started.
	SavedAt time.Time `json:"savedAt,omitzero"`
	NamesAt time.Time `json:"namesAt,omitzero"`
	NoNames bool      `json:"noNames,omitempty"` // no names at all: items with references wait
}

type namesState struct {
	Offline          bool
	SavedAt, NamesAt time.Time
}

func remapRefKey(router, kind string, n int) string {
	return router + ":" + kind + ":" + strconv.Itoa(n)
}

func normName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// checkRouterTargets: a target is a live router or "". Two file routers must not share one target.
func checkRouterTargets(req importReq, live config.Config) error {
	taken := map[string]string{}
	for _, from := range slices.Sorted(maps.Keys(req.Routers)) {
		to := req.Routers[from]
		if to == "" {
			continue
		}
		if to != live.UID {
			return fmt.Errorf("router %s is not a router here", to)
		}
		if other, ok := taken[to]; ok {
			return fmt.Errorf("routers %s and %s cannot both go to one router", other, from)
		}
		taken[to] = from
	}
	return nil
}

// refUse names what holds a reference, for the row's "used by" counts. useOther is not counted.
type refUse int

const (
	usePreset refUse = iota
	usePanel
	useUser
	useOther
)

// refWalk calls fn on every source and destination reference the file's items hold, as config.RefWalk with Dedupe.
func refWalk(b *exportBundle, fn func(kind string, r config.Ref, use refUse, item string) []config.Ref) {
	walk := func(use refUse, item string) config.RefWalk {
		return config.RefWalk{Dedupe: true, Fn: func(kind string, r config.Ref) []config.Ref { return fn(kind, r, use, item) }}
	}
	layout := func(l *panelLayout, item string) {
		*l = l.walkRefs(func(_ refSlot, kind string, r config.Ref) []config.Ref { return fn(kind, r, usePanel, item) }, true)
	}
	presets := func(ps []config.Preset, owner string) {
		for i := range ps {
			walk(usePreset, owner+"/"+ps[i].ID).Preset(&ps[i])
		}
	}
	presets(b.Presets, "")
	for i := range b.Users {
		u := &b.Users[i]
		presets(u.Profile.Presets, u.ID)
		walk(useUser, "user/"+cmp.Or(u.ID, u.Name)).Rights(&u.Rights)
	}
	for i := range b.APIClients {
		walk(useOther, "client/"+b.APIClients[i].Name).Rights(&b.APIClients[i].Rights)
	}
	if b.Settings != nil {
		walk(useOther, "guest").Rights(&b.Settings.GuestRights)
	}
	for i := range b.Panels.Panels {
		p := &b.Panels.Panels[i]
		l := layoutOf(*p)
		layout(&l, "panel/"+p.ID)
		p.Pages, p.Settings, p.Sources, p.Destinations = l.Pages, l.Settings, l.Sources, l.Destinations
	}
	for id, cps := range b.Panels.Checkpoints {
		for i := range cps {
			layout(&cps[i].Layout, "panel/"+id)
		}
	}
	for i := range b.Panels.Templates {
		t := &b.Panels.Templates[i]
		l := panelLayout{Pages: t.Pages}
		layout(&l, "template/"+t.ID)
		t.Pages = l.Pages
	}
	for _, s := range b.Panels.CheckpointSets {
		for i := range s.Entries {
			layout(&s.Entries[i].panelLayout, "set/"+s.ID)
		}
	}
	list, _ := b.Browser["presets"].([]any)
	for _, sp := range list {
		sp, _ := sp.(map[string]any)
		items, _ := sp["items"].([]any)
		name, _ := sp["name"].(string)
		w := walk(useOther, "browser/"+name)
		for _, it := range items {
			it, _ := it.(map[string]any)
			if src, ok := it["src"].(map[string]any); ok {
				if r, ok := cellRef(src); ok {
					maps.Copy(src, withCellRef(src, w.One(config.MissingSource, r)))
				}
			}
			dl, _ := it["dsts"].([]any)
			var refs []config.Ref
			for _, d := range dl {
				if m, ok := d.(map[string]any); ok {
					if r, ok := cellRef(m); ok {
						refs = append(refs, r)
					}
				}
			}
			if len(refs) == len(dl) && len(refs) > 0 {
				out := []any{}
				for _, r := range w.List(config.MissingDestination, refs) {
					out = append(out, map[string]any{"router": r.Router, "n": r.N, "name": r.Name})
				}
				it["dsts"] = out
			}
		}
	}
}

// Runs before the item check, so a moved reference is not Broken.
func (p *importPlanner) planRemap(src, dst map[int]string) {
	if len(p.b.Manifest.Routers) == 0 {
		return
	}
	live := p.live
	liveTable := namesTable{Router: live.UID, Sources: tableList(src), Destinations: tableList(dst)}
	ns := p.names
	info := &remapInfo{Targets: []remapTarget{{UID: live.UID, Title: cmp.Or(live.Title, live.Prefix), Sources: liveTable.Sources,
		Destinations: liveTable.Destinations, Offset: live.NumberOffset, Offline: ns.Offline, SavedAt: ns.SavedAt, NamesAt: ns.NamesAt,
		NoNames: len(src) == 0 && len(dst) == 0}}}
	liveNorm := map[string]map[string]bool{config.MissingSource: normSet(src), config.MissingDestination: normSet(dst)}

	// Default targets: UID match first, then names-table hash, then the only router.
	defaults := map[string]string{}
	taken := false
	for _, pass := range []func(exportRouter) bool{
		func(r exportRouter) bool { return r.UID == live.UID },
		func(r exportRouter) bool { return r.NamesHash != "" && r.NamesHash == namesHash(liveTable) },
		func(exportRouter) bool { return true },
	} {
		for _, r := range p.b.Manifest.Routers {
			if _, done := defaults[r.UID]; !done && !taken && pass(r) {
				defaults[r.UID], taken = live.UID, true
			}
		}
	}

	byRouter := map[string]*remapRouter{}
	for _, r := range p.b.Manifest.Routers {
		target, chosen := p.req.Routers[r.UID]
		if !chosen {
			target = defaults[r.UID]
		}
		rr := &remapRouter{UID: r.UID, Title: cmp.Or(r.Title, r.Prefix), Target: target, Default: defaults[r.UID], Refs: []remapRef{},
			Offset: live.NumberOffset, Sets: p.remapSets(r.UID)}
		if i := slices.IndexFunc(p.b.Routers, func(x config.Router) bool { return x.UID == r.UID }); i >= 0 {
			rr.Offset = p.b.Routers[i].NumberOffset
		}
		found, total := 0, 0
		if t, ok := p.b.Names[r.UID]; ok {
			rr.Names = &remapNames{Sources: t.Sources, Destinations: t.Destinations}
			for kind, l := range map[string][]namedItem{config.MissingSource: t.Sources, config.MissingDestination: t.Destinations} {
				for _, x := range l {
					if normName(x.Name) == "" {
						continue
					}
					total++
					if liveNorm[kind][normName(x.Name)] {
						found++
					}
				}
			}
		}
		rr.Candidates = []remapCand{{UID: live.UID, Title: info.Targets[0].Title, Found: found, Total: total}}
		byRouter[r.UID] = rr
		info.Routers = append(info.Routers, *rr)
	}

	idx := map[string]int{}
	counted := map[string]bool{}
	count := func(kind string, r config.Ref, use refUse, item string) []config.Ref {
		rr := byRouter[r.Router]
		if rr == nil {
			return nil
		}
		key := remapRefKey(r.Router, kind, r.N)
		i, ok := idx[key]
		if !ok {
			i = len(rr.Refs)
			idx[key] = i
			rr.Refs = append(rr.Refs, remapRef{Key: key, Kind: kind, N: r.N, Name: r.Name})
		}
		ref := &rr.Refs[i]
		if ref.Name == "" {
			ref.Name = r.Name
		}
		if !counted[key+"|"+item] {
			counted[key+"|"+item] = true
			switch use {
			case usePreset:
				ref.Presets++
			case usePanel:
				ref.Panels++
			case useUser:
				ref.Users++
			}
		}
		return nil
	}
	refWalk(p.b, count)
	p.defaultRefs(func(_ int, kind string, r config.Ref) { count(kind, r, useOther, "") })
	for i := range info.Routers {
		rr := byRouter[info.Routers[i].UID]
		slices.SortFunc(rr.Refs, func(a, b remapRef) int { return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.N, b.N)) })
		info.Routers[i].Refs = rr.Refs
	}
	p.plan.Remap = info

	names := map[string]map[int]string{config.MissingSource: src, config.MissingDestination: dst}
	move := func(kind string, r config.Ref, _ refUse, _ string) []config.Ref {
		rr := byRouter[r.Router]
		if rr == nil || rr.Target == "" {
			return nil
		}
		to := p.req.Names[remapRefKey(r.Router, kind, r.N)]
		var out []config.Ref
		for _, n := range to {
			name, ok := names[kind][n]
			if !ok {
				continue
			}
			out = append(out, config.Ref{Router: rr.Target, N: n, Name: name})
			if kind == config.MissingSource {
				break
			}
		}
		if len(out) == 0 {
			r.Router, r.Shown = rr.Target, ""
			out = []config.Ref{r}
		}
		return out
	}
	refWalk(p.b, move)
	// Default artwork keys on its number, so the file's item stays as is and keeps the key the page picks by.
	p.movedDefaults = map[int]config.Ref{}
	p.defaultRefs(func(i int, kind string, r config.Ref) {
		if out := move(kind, r, useOther, ""); len(out) > 0 {
			p.movedDefaults[i] = out[0]
		}
	})
	target := func(uid string) (string, bool) {
		rr := byRouter[uid]
		if rr == nil || rr.Target == "" {
			return uid, false
		}
		return rr.Target, true
	}
	if p.b.Settings != nil {
		p.b.Settings.HiddenDstsRouter, _ = target(p.b.Settings.HiddenDstsRouter)
	}
	sets, _ := p.b.Browser["filterSets"].([]any)
	for _, s := range sets {
		if m, ok := s.(map[string]any); ok {
			if uid, ok := m["router"].(string); ok {
				m["router"], _ = target(uid)
			}
		}
	}
	p.eachFileSet(func(key string, s *config.FilterSet) {
		to, ok := target(s.Router)
		if !ok {
			return
		}
		s.Router = to
		f := p.req.Filters[key]
		for _, side := range []struct{ to, from *string }{{&s.Src, f.Src}, {&s.Dst, f.Dst}} {
			if side.from == nil {
				continue
			}
			if _, ok := filterTerms(*side.from); ok {
				*side.to = *side.from
			}
		}
	})
}

func (p *importPlanner) eachFileSet(fn func(key string, s *config.FilterSet)) {
	for i := range p.b.FilterSets {
		fn("/"+p.b.FilterSets[i].Name, &p.b.FilterSets[i])
	}
	for i := range p.b.Users {
		u := &p.b.Users[i]
		for j := range u.Profile.FilterSets {
			fn(u.ID+"/"+u.Profile.FilterSets[j].Name, &u.Profile.FilterSets[j])
		}
	}
}

func (p *importPlanner) remapSets(uid string) []remapSet {
	var out []remapSet
	owner := map[string]string{}
	for _, u := range p.b.Users {
		owner[u.ID] = u.Name
	}
	p.eachFileSet(func(key string, s *config.FilterSet) {
		if s.Router != uid {
			return
		}
		user, _, _ := strings.Cut(key, "/")
		out = append(out, remapSet{Key: key, Name: s.Name, User: owner[user], Src: s.Src, Dst: s.Dst})
	})
	return out
}

func (p *importPlanner) defaultRefs(fn func(i int, kind string, r config.Ref)) {
	for i, d := range p.b.Artwork.Defaults {
		if kind := cellKinds[d.Kind]; kind != "" && d.N != nil {
			fn(i, kind, config.Ref{Router: d.Router, N: *d.N, Name: d.Name})
		}
	}
}

func normSet(m map[int]string) map[string]bool {
	out := make(map[string]bool, len(m))
	for _, s := range m {
		if s = normName(s); s != "" {
			out[s] = true
		}
	}
	return out
}
