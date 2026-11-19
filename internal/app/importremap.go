package app

import (
	"cmp"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// The remap step (docs/agents/profiles.md): the server answers each file router's target and its distinct
// references; the page matches names (web/remaprules.js) and sends its choices back. Here the choices are only
// applied. A reference with no choice keeps its number and is checked as before, so an unmatched name is Skip.

// remapInfo is the plan's remap part.
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
}

// remapCand: Found of the file router's Total names are on the target (trailing spaces and case ignored).
type remapCand struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	Found int    `json:"found"`
	Total int    `json:"total"`
}

// remapRef is one distinct file source or destination. Key is what importReq.Names takes.
type remapRef struct {
	Key     string `json:"key"`
	Kind    string `json:"kind"`
	N       int    `json:"n"`
	Name    string `json:"name"`
	Presets int    `json:"presets,omitempty"`
	Panels  int    `json:"panels,omitempty"`
	Users   int    `json:"users,omitempty"`
}

// remapTarget is a live router with the names the page matches against.
type remapTarget struct {
	UID          string      `json:"uid"`
	Title        string      `json:"title"`
	Sources      []namedItem `json:"sources"`
	Destinations []namedItem `json:"destinations"`
}

func remapRefKey(router, kind string, n int) string {
	return router + ":" + kind + ":" + strconv.Itoa(n)
}

func normName(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

// checkRouterTargets: a target is a live router or "", and two file routers cannot share one.
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

// refUse names what holds a reference, for the row's "used by" counts.
type refUse int

const (
	usePreset refUse = iota
	usePanel
	useUser
)

// refWalk calls fn on every source and destination reference the file's items hold. fn answers the references
// to put in its place, none to keep it; a slot that holds one reference takes the first.
func refWalk(b *exportBundle, fn func(kind string, r config.Ref, use refUse, item string) []config.Ref) {
	one := func(kind string, r config.Ref, use refUse, item string) config.Ref {
		if out := fn(kind, r, use, item); len(out) > 0 {
			return out[0]
		}
		return r
	}
	many := func(kind string, l []config.Ref, use refUse, item string) []config.Ref {
		if l == nil {
			return nil
		}
		out := make([]config.Ref, 0, len(l))
		for _, r := range l {
			got := fn(kind, r, use, item)
			if len(got) == 0 {
				got = []config.Ref{r}
			}
			for _, x := range got {
				if !slices.ContainsFunc(out, func(y config.Ref) bool { return y.Router == x.Router && y.N == x.N }) {
					out = append(out, x)
				}
			}
		}
		return out
	}
	presets := func(ps []config.Preset, owner string) {
		for i := range ps {
			item := owner + "/" + ps[i].ID
			for j := range ps[i].Items {
				it := &ps[i].Items[j]
				it.Src = one(config.MissingSource, it.Src, usePreset, item)
				it.Dsts = many(config.MissingDestination, it.Dsts, usePreset, item)
			}
		}
	}
	presets(b.Presets, "")
	for i := range b.Users {
		u := &b.Users[i]
		presets(u.Profile.Presets, u.ID)
		item := "user/" + cmp.Or(u.ID, u.Name)
		if u.Rights.Sources != nil {
			l := many(config.MissingSource, *u.Rights.Sources, useUser, item)
			u.Rights.Sources = &l
		}
		if u.Rights.Dsts != nil {
			l := many(config.MissingDestination, *u.Rights.Dsts, useUser, item)
			u.Rights.Dsts = &l
		}
	}
	for i := range b.Panels.Panels {
		p := &b.Panels.Panels[i]
		item := "panel/" + p.ID
		for _, g := range p.Pages {
			for _, c := range g.Cells {
				kind := map[any]string{"source": config.MissingSource, "destination": config.MissingDestination}[c["kind"]]
				r, ok := cellRef(c)
				if kind == "" || !ok {
					continue
				}
				x := one(kind, r, usePanel, item)
				c["router"], c["n"] = x.Router, x.N
				if x.Name != "" || c["name"] != nil {
					c["name"] = x.Name
				}
			}
		}
		for _, e := range []struct {
			edge *GridEdge
			kind string
		}{{p.Sources, config.MissingSource}, {p.Destinations, config.MissingDestination}} {
			if e.edge == nil {
				continue
			}
			e.edge.Members.List = many(e.kind, e.edge.Members.List, usePanel, item)
			e.edge.Hide = many(e.kind, e.edge.Hide, usePanel, item)
			for j := range e.edge.Items {
				gi := &e.edge.Items[j]
				x := one(e.kind, gi.ref(), usePanel, item)
				gi.Router, gi.N, gi.Name = x.Router, x.N, x.Name
			}
		}
	}
}

// planRemap answers the remap part and rewrites the file's references to what req chooses. It runs before
// the items are checked, so a moved reference is not Broken.
func (p *importPlanner) planRemap(src, dst map[int]string) {
	if len(p.b.Manifest.Routers) == 0 {
		return
	}
	live := p.live
	liveTable := namesTable{Router: live.UID, Sources: tableList(src), Destinations: tableList(dst)}
	info := &remapInfo{Targets: []remapTarget{{UID: live.UID, Title: cmp.Or(live.Title, live.Prefix), Sources: liveTable.Sources, Destinations: liveTable.Destinations}}}
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
		rr := &remapRouter{UID: r.UID, Title: cmp.Or(r.Title, r.Prefix), Target: target, Default: defaults[r.UID], Refs: []remapRef{}}
		found, total := 0, 0
		if t, ok := p.b.Names[r.UID]; ok {
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

	// The distinct references, before any rewrite.
	idx := map[string]int{}
	counted := map[string]bool{}
	refWalk(p.b, func(kind string, r config.Ref, use refUse, item string) []config.Ref {
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
	})
	for i := range info.Routers {
		rr := byRouter[info.Routers[i].UID]
		slices.SortFunc(rr.Refs, func(a, b remapRef) int { return cmp.Or(cmp.Compare(a.Kind, b.Kind), cmp.Compare(a.N, b.N)) })
		info.Routers[i].Refs = rr.Refs
	}
	p.plan.Remap = info

	// The rewrite: a targeted router's references move to the target; a chosen number replaces the file's.
	names := map[string]map[int]string{config.MissingSource: src, config.MissingDestination: dst}
	refWalk(p.b, func(kind string, r config.Ref, _ refUse, _ string) []config.Ref {
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
	})
	retarget := func(s *config.FilterSet) {
		if rr := byRouter[s.Router]; rr != nil && rr.Target != "" {
			s.Router = rr.Target
		}
	}
	for i := range p.b.FilterSets {
		retarget(&p.b.FilterSets[i])
	}
	for i := range p.b.Users {
		for j := range p.b.Users[i].Profile.FilterSets {
			retarget(&p.b.Users[i].Profile.FilterSets[j])
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
