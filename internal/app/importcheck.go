package app

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// refCheck checks references against the names of each live router. A router can have no names read yet. Then a
// source or destination on it cannot be checked, so the item waits (waited) and its category is not imported.
type refCheck struct {
	routers map[string]*routerCheck // by router UID
	waited  bool
}

type routerCheck struct {
	levels   int
	src, dst map[int]string
	unknown  bool // no names at all
}

func newRefCheck(live config.Config, names liveNameSet) *refCheck {
	k := &refCheck{routers: map[string]*routerCheck{}}
	for _, r := range live.Routers {
		n := names[r.UID]
		k.routers[r.UID] = &routerCheck{levels: r.Levels, src: n.src, dst: n.dst, unknown: len(n.src) == 0 && len(n.dst) == 0}
	}
	return k
}

// setLevels takes the level counts of c, after a category that can change a router.
func (k *refCheck) setLevels(c config.Config) {
	for _, r := range c.Routers {
		if rc := k.routers[r.UID]; rc != nil {
			rc.levels = r.Levels
		}
	}
}

// unknown: router uid is live and has no names at all.
func (k *refCheck) unknown(uid string) bool {
	rc := k.routers[uid]
	return rc != nil && rc.unknown
}

func (k *refCheck) missing(kind string, r config.Ref) (config.Missing, bool) {
	rc := k.routers[r.Router]
	if rc == nil {
		return config.Missing{Kind: config.MissingRouter, Ref: r}, true
	}
	if kind == config.MissingLevel {
		return config.Missing{Kind: kind, Ref: r}, r.N < 1 || r.N > rc.levels
	}
	if rc.unknown {
		k.waited = true
		return config.Missing{}, false
	}
	names := rc.src
	if kind == config.MissingDestination {
		names = rc.dst
	}
	return config.Missing{Kind: kind, Ref: r}, !sameName(names[r.N], r.Name)
}

type brokenList struct{ out []config.Missing }

func (b *brokenList) add(m config.Missing, bad bool) {
	if !bad {
		return
	}
	m.Ref.Shown, m.Ref.Extra = "", nil
	if !slices.ContainsFunc(b.out, func(x config.Missing) bool {
		return x.Kind == m.Kind && x.Ref.Router == m.Ref.Router && x.Ref.N == m.Ref.N && x.Ref.Name == m.Ref.Name
	}) {
		b.out = append(b.out, m)
	}
}

func (k *refCheck) presetBroken(p config.Preset) []config.Missing {
	var b brokenList
	for _, it := range p.Items {
		b.add(k.missing(config.MissingSource, it.Src))
		for _, d := range it.Dsts {
			b.add(k.missing(config.MissingDestination, d))
		}
		for _, l := range it.Levels {
			b.add(k.missing(config.MissingLevel, config.Ref{Router: it.Src.Router, N: l}))
		}
	}
	return b.out
}

func (k *refCheck) checked(ps []config.Preset) []config.Preset {
	out := config.ClonePresets(ps)
	for i := range out {
		out[i].Broken = k.presetBroken(out[i])
	}
	return out
}

// panelBroken points preset buttons at their presets here and lists what the target lacks. presetTo maps a file preset
// id to its id here, "" for none.
func (k *refCheck) panelBroken(fp *Panel, cfg config.Config, presetTo func(id string) string) []config.Missing {
	var b brokenList
	layoutOf(*fp).readRefs(func(s refSlot, kind string, r config.Ref) {
		if s != slotFixed {
			b.add(k.missing(kind, r))
		}
	})
	pages := make([]PanelPage, len(fp.Pages))
	for i, g := range fp.Pages {
		pages[i] = g
		pages[i].Cells = slices.Clone(g.Cells)
		for j, c := range g.Cells {
			if c["kind"] != "preset" || c["scope"] == "me" {
				continue
			}
			id, _ := c["id"].(string)
			name, _ := c["name"].(string)
			to := presetTo(id)
			if !slices.ContainsFunc(cfg.Presets, func(x config.Preset) bool { return to != "" && x.ID == to }) {
				b.add(config.Missing{Kind: config.MissingPreset, Ref: config.Ref{Name: name}}, id != "" || name != "")
				continue
			}
			pages[i].Cells[j] = maps.Clone(c)
			pages[i].Cells[j]["id"] = to
		}
	}
	fp.Pages = pages
	return b.out
}

// filterSetBroken: only the router is checked here; terms break on router changes, not on import.
func (k *refCheck) filterSetBroken(s config.FilterSet) []config.Missing {
	if s.Router == "" || k.routers[s.Router] != nil {
		return nil
	}
	return []config.Missing{{Kind: config.MissingRouter, Ref: config.Ref{Router: s.Router}}}
}

// matchIdx finds the live item for a file item: by hidden id on a file from this install, then by name.
func matchIdx[T any](same bool, list []T, id, name string, idOf, nameOf func(T) string) int {
	if same && id != "" && idOf != nil {
		if i := slices.IndexFunc(list, func(x T) bool { return idOf(x) == id }); i >= 0 {
			return i
		}
	}
	return slices.IndexFunc(list, func(x T) bool { return sameName(nameOf(x), name) })
}

func matchUser(same bool, list []config.User, u config.User) int {
	return matchIdx(same, list, u.ID, u.Name, func(x config.User) string { return x.ID }, func(x config.User) string { return x.Name })
}

func matchPreset(same bool, list []config.Preset, s config.Preset) int {
	return matchIdx(same, list, s.ID, s.Name, func(x config.Preset) string { return x.ID }, func(x config.Preset) string { return x.Name })
}

func matchSet(same bool, list []config.FilterSet, s config.FilterSet) int {
	return matchIdx(same, list, s.ID, s.Name, func(x config.FilterSet) string { return x.ID }, setName)
}

// nextName is "Name 2", or the first number after it that taken does not hold.
func nextName(name string, taken func(string) bool) string {
	for n := 2; ; n++ {
		if s := fmt.Sprintf("%s %d", strings.TrimSpace(name), n); !taken(s) {
			return s
		}
	}
}

func sameYAML(a, b any) bool {
	x, err1 := yaml.Marshal(a)
	y, err2 := yaml.Marshal(b)
	return err1 == nil && err2 == nil && string(x) == string(y)
}
