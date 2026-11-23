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

// Broken check: docs/agents/server.md. It acts only where the last two counted polls agree, so one bad read changes
// nothing.

// namePoll is one full name read as the router sent it, before refreshed lays old names over a short reply.
type namePoll struct{ src, dst map[int]string }

func (p *namePoll) side(dst bool) map[int]string {
	if dst {
		return p.dst
	}
	return p.src
}

// base is the last counted poll, else the names shown now (the saved names table at boot).
func routerFault(base, cur namePoll) bool {
	had, lost := 0, 0
	for _, dst := range []bool{false, true} {
		now := cur.side(dst)
		for n, name := range base.side(dst) {
			if strings.TrimSpace(name) == "" {
				continue
			}
			had++
			if strings.TrimSpace(now[n]) == "" {
				lost++
			}
		}
	}
	return lost*2 > had
}

// countPollMu returns the check to run after the caller releases mu, or nil. A fault read is not shown or saved.
// Caller holds mu.
func (a *App) countPollMu(u *routerUnit, cur namePoll) (check func(), fault bool) {
	rs := u.router
	base := rs.lastPoll
	if base == nil {
		base = &namePoll{rs.srcNames, rs.dstNames}
	}
	if routerFault(*base, cur) {
		if !rs.faultSeen {
			log.Printf("names: more than half the names went blank; read ignored")
		}
		rs.faultSeen = true
		return nil, true
	}
	rs.faultSeen = false
	noteRenamed(&rs.renamedTo, *base, cur)
	prev := rs.lastPoll
	rs.lastPoll = &namePoll{maps.Clone(cur.src), maps.Clone(cur.dst)} // flush can hand cur on as the live names
	if prev == nil || a.setupPending() {
		return nil, false
	}
	hints := [2]map[string]string{maps.Clone(rs.renamedTo[0]), maps.Clone(rs.renamedTo[1])}
	return func() { a.recheckRefs(u, *prev, cur, hints) }, false
}

func sideIndex(dst bool) int {
	if dst {
		return 1
	}
	return 0
}

// A name that comes back drops its entry.
func noteRenamed(to *[2]map[string]string, base, cur namePoll) {
	for _, dst := range []bool{false, true} {
		i := sideIndex(dst)
		now := cur.side(dst)
		for n, old := range base.side(dst) {
			old, next := strings.TrimSpace(old), strings.TrimSpace(now[n])
			if old == "" || next == "" || sameName(old, next) {
				continue
			}
			if to[i] == nil {
				to[i] = map[string]string{}
			}
			to[i][strings.ToLower(old)] = next
			delete(to[i], strings.ToLower(next))
		}
	}
}

type refVerdict int

const (
	refWait    refVerdict = iota // the two polls disagree on this number
	refOK                        // the stored name is the name at that number
	refRenamed                   // a new name at the same number; the old name is nowhere
	refMoved                     // the stored name now sits at another number
	refGone                      // no name at that number in either poll
)

type pollCheck struct {
	uid       string
	first     string // the first router's UID: a filter set with no router is that router's
	levels    int
	off       int // numberOffset, for the row
	prev, cur namePoll
	hints     [2]map[string]string // routerState.renamedTo when the read was counted
	renames   []string
	broke     []string
	moved     []string
}

func sameName(a, b string) bool { return strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) }

func (k *pollCheck) verdict(dst bool, r config.Ref) (refVerdict, string) {
	p, c := k.prev.side(dst), k.cur.side(dst)
	now := strings.TrimSpace(c[r.N])
	if !sameName(p[r.N], now) {
		return refWait, ""
	}
	if now == "" {
		return refGone, ""
	}
	if strings.TrimSpace(r.Name) == "" || sameName(r.Name, now) {
		return refOK, now
	}
	at := func(m map[int]string) bool {
		for n, name := range m {
			if n != r.N && sameName(name, r.Name) {
				return true
			}
		}
		return false
	}
	if at(c) && at(p) {
		return refMoved, now
	}
	if at(c) || at(p) {
		return refWait, ""
	}
	return refRenamed, now
}

func (k *pollCheck) noteRename(dst bool, r config.Ref, now string) {
	s := fmt.Sprintf("%s %d %s to %s", refKind(dst), r.N+k.off, strings.TrimSpace(r.Name), now)
	if !slices.Contains(k.renames, s) {
		k.renames = append(k.renames, s)
	}
}

// quiet renames a reference of a store with no Broken mark (Rights, fixed destinations, Default artwork). A gone
// number stays and matches nothing.
func (k *pollCheck) quiet(dst bool, r *config.Ref) bool {
	if r.Router != k.uid {
		return false
	}
	v, now := k.verdict(dst, *r)
	if v != refRenamed && !(v == refOK && r.Name != now) {
		return false
	}
	if v == refRenamed {
		k.noteRename(dst, *r, now)
	}
	r.Name = now
	return true
}

// itemCheck rebuilds one item's Broken and Moved lists. It owns the router, source, destination and level kinds
// of its references; other kinds (an import's skipped preset) stay as they are.
type itemCheck struct {
	k             *pollCheck
	oldBroken     []config.Missing
	oldMoved      []config.Missing
	broken, moved brokenList
	renamed       bool
}

func (k *pollCheck) item(broken, moved []config.Missing) *itemCheck {
	return &itemCheck{k: k, oldBroken: broken, oldMoved: moved}
}

func refKind(dst bool) string {
	if dst {
		return config.MissingDestination
	}
	return config.MissingSource
}

func markOf(l []config.Missing, kind string, r config.Ref) (config.Missing, bool) {
	i := slices.IndexFunc(l, func(m config.Missing) bool {
		return m.Kind == kind && m.Ref.Router == r.Router && m.Ref.N == r.N
	})
	if i < 0 {
		return config.Missing{}, false
	}
	return l[i], true
}

func hasMissing(l []config.Missing, kind string, r config.Ref) bool {
	_, ok := markOf(l, kind, r)
	return ok
}

// A save stores each reference with the name now at its number, so the name in a mark decides. A Broken reference
// recovers only when its number holds that name again.
func (c *itemCheck) ref(dst bool, r *config.Ref) {
	kind := refKind(dst)
	if r.Router != c.k.uid {
		c.broken.add(config.Missing{Kind: config.MissingRouter, Ref: config.Ref{Router: r.Router}}, true)
		return
	}
	broken, isBroken := markOf(c.oldBroken, kind, *r)
	moved, isMoved := markOf(c.oldMoved, kind, *r)
	for _, m := range []struct {
		mark config.Missing
		ok   bool
	}{{broken, isBroken}, {moved, isMoved}} {
		if m.ok && strings.TrimSpace(m.mark.Ref.Name) != "" && r.Name != m.mark.Ref.Name {
			r.Name, c.renamed = m.mark.Ref.Name, true
			break
		}
	}
	v, now := c.k.verdict(dst, *r)
	if isBroken {
		if v != refOK {
			c.broken.add(config.Missing{Kind: kind, Ref: *r}, true)
		}
		return
	}
	switch v {
	case refGone:
		c.broken.add(config.Missing{Kind: kind, Ref: *r}, true)
	case refMoved:
		c.moved.add(config.Missing{Kind: kind, Ref: *r}, true)
	case refRenamed:
		c.k.noteRename(dst, *r, now)
		r.Name, c.renamed = now, true
	case refOK:
		if r.Name != now {
			r.Name, c.renamed = now, true
		}
	case refWait:
		c.moved.add(config.Missing{Kind: kind, Ref: *r}, isMoved)
	}
}

func (c *itemCheck) level(router string, ui int) {
	if router != c.k.uid {
		return
	}
	c.broken.add(config.Missing{Kind: config.MissingLevel, Ref: config.Ref{Router: router, N: ui}}, ui < 1 || ui > c.k.levels)
}

func ownedKind(kind string) bool {
	switch kind {
	case config.MissingRouter, config.MissingSource, config.MissingDestination, config.MissingLevel:
		return true
	}
	return false
}

// name is for the Router names row.
func (c *itemCheck) done(name string) (broken, moved []config.Missing, changed bool) {
	for _, m := range c.oldBroken {
		if !ownedKind(m.Kind) {
			c.broken.add(m, true)
		}
	}
	broken, moved = c.broken.out, c.moved.out
	if len(broken) > len(c.oldBroken) && len(broken) > 0 {
		c.k.broke = append(c.k.broke, name)
	}
	if len(moved) > len(c.oldMoved) {
		c.k.moved = append(c.k.moved, name)
	}
	changed = c.renamed || !missingEqual(broken, c.oldBroken) || !missingEqual(moved, c.oldMoved)
	return broken, moved, changed
}

func missingEqual(a, b []config.Missing) bool {
	return slices.EqualFunc(a, b, func(x, y config.Missing) bool {
		return x.Kind == y.Kind && x.Ref.Router == y.Ref.Router && x.Ref.N == y.Ref.N && x.Ref.Name == y.Ref.Name
	})
}

func (k *pollCheck) preset(p *config.Preset, who string) bool {
	c := k.item(p.Broken, p.Moved)
	w := config.RefWalk{Fn: func(kind string, r config.Ref) []config.Ref {
		c.ref(kind == config.MissingDestination, &r)
		return []config.Ref{r}
	}}
	for i := range p.Items {
		it := &p.Items[i]
		w.Item(it)
		for _, l := range it.Levels {
			c.level(it.Src.Router, l)
		}
	}
	var changed bool
	p.Broken, p.Moved, changed = c.done(fmt.Sprintf("preset %q (%s)", p.Name, who))
	return changed
}

// filterSet: only a positive exact term (`"CAM 1"`) or number term that matches nothing in both polls breaks a
// set. Contains, regex and range terms are searches and never break. Text that does not parse is left alone.
func (k *pollCheck) filterSet(s *config.FilterSet, off int, who string) bool {
	if owner := cmp.Or(s.Router, k.first); owner != k.uid {
		return false
	}
	var b brokenList
	for _, m := range s.Broken {
		if m.Kind != config.MissingSource && m.Kind != config.MissingDestination {
			b.add(m, true)
		}
	}
	for _, side := range []struct {
		text string
		dst  bool
	}{{s.Src, false}, {s.Dst, true}} {
		terms, ok := filterTerms(side.text)
		if !ok {
			continue
		}
		for _, t := range terms {
			if t.Not {
				continue
			}
			switch t.Kind {
			case "num":
				n := t.A - off
				gone := strings.TrimSpace(k.prev.side(side.dst)[n]) == "" && strings.TrimSpace(k.cur.side(side.dst)[n]) == ""
				b.add(config.Missing{Kind: refKind(side.dst), Ref: config.Ref{Router: k.uid, N: n}}, gone)
			case "exact":
				named := func(m map[int]string) bool {
					for _, name := range m {
						if strings.EqualFold(name, t.Text) {
							return true
						}
					}
					return false
				}
				gone := !named(k.prev.side(side.dst)) && !named(k.cur.side(side.dst))
				// N -1: a name term, which names no number.
				m := config.Missing{Kind: refKind(side.dst), Ref: config.Ref{Router: k.uid, N: -1, Name: t.Text}}
				m.Now = k.hints[sideIndex(side.dst)][normName(t.Text)]
				if i := slices.IndexFunc(s.Broken, func(o config.Missing) bool {
					return o.Kind == m.Kind && o.Ref.N == -1 && sameName(o.Ref.Name, t.Text)
				}); i >= 0 && m.Now == "" {
					m.Now = s.Broken[i].Now
				}
				b.add(m, gone)
			}
		}
	}
	if missingEqual(b.out, s.Broken) {
		return false
	}
	if len(b.out) > len(s.Broken) {
		k.broke = append(k.broke, fmt.Sprintf("filter set %q (%s)", s.Name, who))
	}
	s.Broken = b.out
	return true
}

var errNoCheckChange = errors.New("no change")

// fits keeps a list whose new names would pass its byte cap as it was, so the next Load still reads the file.
func fits(what string, v any, max int) bool {
	b, err := json.Marshal(v)
	if err == nil && len(b) <= max {
		return true
	}
	log.Printf("names: %s kept their old names and marks: with the new ones they pass %d KiB", what, max>>10)
	return false
}

func (k *pollCheck) config(c *config.Config) bool {
	changed := false
	for _, s := range itemScopes(c, "") {
		who := cmp.Or(s.Name, "Everyone")
		ps, fs := config.ClonePresets(*s.Presets), slices.Clone(*s.FilterSets)
		chP, chF := false, false
		for i := range ps {
			chP = k.preset(&ps[i], who) || chP
		}
		for i := range fs {
			fs[i].Broken = slices.Clone(fs[i].Broken)
			chF = k.filterSet(&fs[i], c.NumberOffset, who) || chF
		}
		if s.Profile == nil {
			if chP && fits("Everyone presets", ps, config.MaxPresetsBytes) {
				c.Presets, changed = ps, true
			}
			if chF {
				c.FilterSets, changed = fs, true
			}
			continue
		}
		pr := s.Profile.Clone()
		pr.Presets, pr.FilterSets = ps, fs
		if (chP || chF) && fits(s.Name+"'s Me items", pr, config.MaxProfileBytes) {
			*s.Profile, changed = pr, true
		}
	}
	rights := config.RefWalk{Fn: func(kind string, r config.Ref) []config.Ref {
		if !k.quiet(kind == config.MissingDestination, &r) {
			return nil
		}
		changed = true
		return []config.Ref{r}
	}}
	for _, r := range everyRights(c) {
		rights.Rights(r)
	}
	return changed
}

// Cells and members carry Broken marks; hidden items, item settings and fixed destinations rename quietly.
func (k *pollCheck) panel(p *Panel) bool {
	c := k.item(p.Broken, p.Moved)
	quiet := false
	l := layoutOf(*p).walkRefs(func(s refSlot, kind string, r config.Ref) []config.Ref {
		dst, was := kind == config.MissingDestination, r.Name
		if s == slotCell || s == slotMember {
			c.ref(dst, &r)
		} else {
			quiet = k.quiet(dst, &r) || quiet
		}
		if r.Name == was {
			return nil
		}
		return []config.Ref{r}
	}, false)
	p.Pages, p.Settings, p.Sources, p.Destinations = l.Pages, l.Settings, l.Sources, l.Destinations
	var changed bool
	p.Broken, p.Moved, changed = c.done(fmt.Sprintf("panel %q", p.Name))
	return changed || quiet
}

// itemScope is Everyone's presets and filter sets (Profile nil) or one user's. ID is the owner part of an item id.
type itemScope struct {
	ID, Name   string
	Presets    *[]config.Preset
	FilterSets *[]config.FilterSet
	Profile    *config.Profile
}

// itemScopes: Everyone's and every user's, or only user mine's. c gets its own Users list.
func itemScopes(c *config.Config, mine string) []itemScope {
	var out []itemScope
	if mine == "" {
		out = append(out, itemScope{ID: "everyone", Presets: &c.Presets, FilterSets: &c.FilterSets})
	}
	c.Users = slices.Clone(c.Users)
	for i := range c.Users {
		if u := &c.Users[i]; mine == "" || u.ID == mine {
			out = append(out, itemScope{ID: u.ID, Name: u.Name, Presets: &u.Profile.Presets, FilterSets: &u.Profile.FilterSets, Profile: &u.Profile})
		}
	}
	return out
}

// No Settings saves row: the Router names row says what changed.
func (a *App) recheckRefs(u *routerUnit, prev, cur namePoll, hints [2]map[string]string) {
	rc := a.r().routerOf(u.uid)
	k := &pollCheck{uid: rc.UID, first: a.r().cfg.UID, levels: rc.Levels, off: rc.NumberOffset, prev: prev, cur: cur, hints: hints}
	a.cfgMu.Lock()
	if !a.setupPending() {
		err := a.applyConfigWith(func(c *config.Config) error {
			// The router is gone, or an import gave it a new UID, since the read.
			if _, ok := c.RouterByUID(k.uid); !ok || !k.config(c) {
				return errNoCheckChange
			}
			return nil
		}, true)
		if err != nil && !errors.Is(err, errNoCheckChange) {
			log.Printf("names: stored items not updated: %v", err)
		}
	}
	a.cfgMu.Unlock()
	if err := a.panels.rewrite(k.panel); err != nil {
		log.Printf("names: panels not updated: %v", err)
	}
	if err := a.artDefaults.rename(k); err != nil {
		log.Printf("names: Default artwork not updated: %v", err)
	}
	k.logRow(a)
}

func (k *pollCheck) logRow(a *App) {
	var parts []string
	result := "renamed"
	if len(k.renames) > 0 {
		parts = append(parts, "renamed "+strings.Join(k.renames, ", "))
	}
	if len(k.moved) > 0 {
		parts = append(parts, "Name moved: "+strings.Join(k.moved, ", "))
		result = "moved"
	}
	if len(k.broke) > 0 {
		parts = append(parts, "Broken: "+strings.Join(k.broke, ", "))
		result = "broken"
	}
	if len(parts) == 0 {
		return
	}
	e := event(kindLink, "routerNames", "Sode", result)
	e.Note, e.Router = strings.Join(parts, "; "), k.uid
	a.logEvent(e)
}
