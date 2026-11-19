package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Swap source and Swap destination (GLOSSARY.md, Swap; docs/agents/settings-saves.md, Swap). The Administrator's
// Swap is the one Save's `swap` key; a user's own Swap from My user is swapMine and saves at once.

const swapKey = "swap"

// swapReq: numbers are internal (Ref.N), as the page holds them. Items are ticked swapItem ids; the server checks
// ownership and use again, so ids only narrow what changes.
type swapReq struct {
	Router string   `json:"router"`
	Side   string   `json:"side"` // config.MissingSource or config.MissingDestination
	From   int      `json:"from"`
	To     int      `json:"to"`
	Items  []string `json:"items"`
	Mine   bool     `json:"mine"` // swapList: only the asking user's items
}

// swapItem: one preset, filter set or panel that uses From. Owner is the user's name, "" for Everyone.
type swapItem struct {
	ID     string `json:"id"`
	Kind   string `json:"kind"` // preset, filterSet or panel
	Name   string `json:"name"`
	Owner  string `json:"owner"`
	Broken bool   `json:"broken"` // From is a Broken reference of the item
}

const swapEveryone = "Everyone"

// swapper rewrites one reference. fromNames: names that From went by (router, stored refs, marks), so an exact
// filter term for it matches.
type swapper struct {
	uid       string
	dst       bool
	from, to  int
	off       int
	toName    string
	fromNames []string
}

func (sw *swapper) ours(router string) bool { return router == "" || router == sw.uid }

func (sw *swapper) kind() string { return refKind(sw.dst) }

func (sw *swapper) ref(r *config.Ref) bool {
	if !sw.ours(r.Router) || r.N != sw.from {
		return false
	}
	r.N, r.Name = sw.to, sw.toName
	return true
}

func (sw *swapper) named(name string) bool {
	name = strings.TrimSpace(name)
	return name != "" && slices.ContainsFunc(sw.fromNames, func(n string) bool { return sameName(n, name) })
}

// marked: m is a mark for From. A name term's mark has N -1.
func (sw *swapper) marked(m config.Missing) bool {
	return m.Kind == sw.kind() && sw.ours(m.Ref.Router) && (m.Ref.N == sw.from || m.Ref.N == -1 && sw.named(m.Ref.Name))
}

func (sw *swapper) brokenIn(l []config.Missing) bool { return slices.ContainsFunc(l, sw.marked) }

func (sw *swapper) unmark(l []config.Missing) []config.Missing {
	if !sw.brokenIn(l) {
		return l
	}
	return slices.DeleteFunc(slices.Clone(l), sw.marked)
}

// dedupe drops a later reference to the same number, since an item that held To and From now holds To twice.
func dedupe(l []config.Ref) []config.Ref {
	out := l[:0:0]
	for _, r := range l {
		if !slices.ContainsFunc(out, func(o config.Ref) bool { return o.Router == r.Router && o.N == r.N }) {
			out = append(out, r)
		}
	}
	return out
}

// preset changes p in place; callers pass a copy (config.ClonePresets).
func (sw *swapper) preset(p *config.Preset) bool {
	changed := false
	for i := range p.Items {
		it := &p.Items[i]
		if !sw.dst {
			changed = sw.ref(&it.Src) || changed
			continue
		}
		hit := false
		for j := range it.Dsts {
			hit = sw.ref(&it.Dsts[j]) || hit
		}
		if hit {
			it.Dsts, changed = dedupe(it.Dsts), true
		}
	}
	if changed {
		p.Broken, p.Moved = sw.unmark(p.Broken), sw.unmark(p.Moved)
	}
	return changed
}

// filterTerm text for To: the number as shown, else its exact name. Negated terms swap too.
func (sw *swapper) filterText(text string) (string, bool) {
	terms, ok := filterTerms(text)
	if !ok {
		return text, false
	}
	r := []rune(text)
	changed := false
	for i := len(terms) - 1; i >= 0; i-- {
		t := terms[i]
		var put string
		switch {
		case t.Kind == "num" && t.A-sw.off == sw.from:
			put = strconv.Itoa(sw.to + sw.off)
		case t.Kind == "exact" && sw.named(t.Text):
			put = strconv.Itoa(sw.to + sw.off)
			if strings.TrimSpace(sw.toName) != "" {
				put = `"` + strings.ReplaceAll(strings.TrimSpace(sw.toName), `"`, `""`) + `"`
			}
		default:
			continue
		}
		r = slices.Concat(r[:t.at], []rune(put), r[t.end:])
		changed = true
	}
	return string(r), changed
}

func (sw *swapper) filterSet(s *config.FilterSet) bool {
	if !sw.ours(s.Router) {
		return false
	}
	text := &s.Src
	if sw.dst {
		text = &s.Dst
	}
	next, changed := sw.filterText(*text)
	if changed {
		*text, s.Broken = next, sw.unmark(s.Broken)
	}
	return changed
}

// panel swaps cells, the Grid edge on its side (members, hidden items, item settings) and fixed destinations. It
// copies what it changes, since a stored panel shares its lists.
func (sw *swapper) panel(p *Panel) bool {
	changed := false
	pages := slices.Clone(p.Pages)
	for i, g := range pages {
		cloned := false
		for j, cell := range g.Cells {
			r, ok := cellRef(cell)
			if !ok || cell["kind"] != sw.kind() || !sw.ref(&r) {
				continue
			}
			if !cloned {
				pages[i].Cells, cloned = slices.Clone(g.Cells), true
			}
			c := maps.Clone(cell)
			c["n"], c["name"] = r.N, r.Name
			pages[i].Cells[j] = c
			changed = true
		}
	}
	p.Pages = pages
	edge := &p.Sources
	if sw.dst {
		edge = &p.Destinations
	}
	if e := *edge; e != nil {
		out := *e
		hit := false
		out.Members.List = slices.Clone(e.Members.List)
		for i := range out.Members.List {
			hit = sw.ref(&out.Members.List[i]) || hit
		}
		out.Members.List = dedupe(out.Members.List)
		out.Hide = slices.Clone(e.Hide)
		for i := range out.Hide {
			hit = sw.ref(&out.Hide[i]) || hit
		}
		out.Hide = dedupe(out.Hide)
		// An item that already has settings for To keeps them; From's go.
		hasTo := slices.ContainsFunc(e.Items, func(it GridItem) bool { return sw.ours(it.Router) && it.N == sw.to })
		out.Items = nil
		for _, it := range e.Items {
			r := it.ref()
			if sw.ref(&r) {
				hit = true
				if hasTo {
					continue
				}
				it.N, it.Name = r.N, r.Name
			}
			out.Items = append(out.Items, it)
		}
		if e.Items != nil && out.Items == nil {
			out.Items = []GridItem{}
		}
		if hit {
			*edge, changed = &out, true
		}
	}
	if v, ok := p.Settings[panelFixedKey]; ok && sw.dst {
		if list, err := fixedList(v); err == nil {
			list = slices.Clone(list)
			hit := false
			for i := range list {
				hit = sw.ref(&list[i]) || hit
			}
			if hit {
				p.Settings = maps.Clone(p.Settings)
				p.Settings[panelFixedKey] = dedupe(list)
				changed = true
			}
		}
	}
	if changed {
		p.Broken, p.Moved = sw.unmark(p.Broken), sw.unmark(p.Moved)
	}
	return changed
}

// swapHit: an item the Swap changed (or, for a list, would change), with its Broken flag from before.
type swapHit func(it swapItem)

// config swaps the presets and filter sets of c that pick allows and calls hit for each it changed. mine: a user
// id, which limits the Swap to that user's own items; "" for every owner.
func (sw *swapper) config(c *config.Config, mine string, pick func(id string) bool, hit swapHit) {
	presets := func(ps []config.Preset, owner, ownerName string) []config.Preset {
		out := config.ClonePresets(ps)
		for i := range out {
			id := "preset/" + owner + "/" + out[i].ID
			was := sw.brokenIn(out[i].Broken)
			if !pick(id) {
				continue
			}
			q := config.ClonePresets(out[i : i+1])[0]
			if sw.preset(&q) {
				out[i] = q
				hit(swapItem{ID: id, Kind: "preset", Name: q.Name, Owner: ownerName, Broken: was})
			}
		}
		return out
	}
	sets := func(fs []config.FilterSet, owner, ownerName string) []config.FilterSet {
		out := slices.Clone(fs)
		for i := range out {
			id := "filterSet/" + owner + "/" + out[i].Name
			was := sw.brokenIn(out[i].Broken)
			if pick(id) && sw.filterSet(&out[i]) {
				hit(swapItem{ID: id, Kind: "filterSet", Name: out[i].Name, Owner: ownerName, Broken: was})
			}
		}
		return out
	}
	if mine == "" {
		c.Presets = presets(c.Presets, "everyone", "")
		c.FilterSets = sets(c.FilterSets, "everyone", "")
	}
	c.Users = slices.Clone(c.Users)
	for i := range c.Users {
		u := &c.Users[i]
		if mine != "" && u.ID != mine {
			continue
		}
		u.Profile = u.Profile.Clone()
		u.Profile.Presets = presets(u.Profile.Presets, u.ID, u.Name)
		u.Profile.FilterSets = sets(u.Profile.FilterSets, u.ID, u.Name)
	}
}

// panels swaps the panels pick allows (only those mine owns when mine is set) and writes each one changed.
func (sw *swapper) panels(s *panelStore, cfg config.Config, mine string, pick func(id string) bool, hit swapHit) error {
	return s.rewrite(func(p *Panel) bool {
		if mine != "" && p.Owner != mine || !pick("panel/"+p.ID) {
			return false
		}
		was := sw.brokenIn(p.Broken)
		if !sw.panel(p) {
			return false
		}
		u, _ := cfg.FindUserByID(p.Owner)
		hit(swapItem{ID: "panel/" + p.ID, Kind: "panel", Name: p.Name, Owner: u.Name, Broken: was})
		return true
	})
}

var errSwapSide = errors.New("swap: side is source or destination")

// newSwapper checks req against cfg and reads the names it needs. Caller does not hold mu.
func (a *App) newSwapper(cfg config.Config, req swapReq) (*swapper, error) {
	if req.Side != config.MissingSource && req.Side != config.MissingDestination {
		return nil, errSwapSide
	}
	if req.Router != "" && req.Router != cfg.UID {
		return nil, fmt.Errorf("swap: no router %s", req.Router)
	}
	if req.From < 0 || req.To < 0 || req.From > dstNumberMax || req.To > dstNumberMax {
		return nil, fmt.Errorf("swap: numbers run from %d to %d", cfg.NumberOffset, dstNumberMax+cfg.NumberOffset)
	}
	sw := &swapper{uid: cfg.UID, dst: req.Side == config.MissingDestination, from: req.From, to: req.To, off: cfg.NumberOffset}
	a.mu.Lock()
	names := a.router.srcNames
	if sw.dst {
		names = a.router.dstNames
	}
	sw.toName = strings.TrimSpace(names[req.To])
	sw.fromNames = []string{names[req.From]}
	a.mu.Unlock()
	if req.To == req.From {
		return nil, errors.New("swap: choose a different number to swap to")
	}
	// The names stored for From, so an exact filter term for an old name still matches.
	note := func(r config.Ref) {
		if sw.ours(r.Router) && r.N == sw.from && strings.TrimSpace(r.Name) != "" && !sw.named(r.Name) {
			sw.fromNames = append(sw.fromNames, r.Name)
		}
	}
	notePresets := func(ps []config.Preset) {
		for _, p := range ps {
			for _, it := range p.Items {
				if sw.dst {
					for _, d := range it.Dsts {
						note(d)
					}
				} else {
					note(it.Src)
				}
			}
			for _, m := range slices.Concat(p.Broken, p.Moved) {
				if m.Kind == sw.kind() {
					note(m.Ref)
				}
			}
		}
	}
	notePresets(cfg.Presets)
	for _, u := range cfg.Users {
		notePresets(u.Profile.Presets)
	}
	for _, p := range a.panels.list() {
		for _, g := range p.Pages {
			for _, cell := range g.Cells {
				if r, ok := cellRef(cell); ok && cell["kind"] == sw.kind() {
					note(r)
				}
			}
		}
	}
	return sw, nil
}

func pickOf(ids []string) func(string) bool {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	return func(id string) bool { return set[id] }
}

func everyItem(string) bool { return true }

// swapLabel: "Swap destination 23 for 40", numbers as shown.
func (sw *swapper) label() string {
	side := "source"
	if sw.dst {
		side = "destination"
	}
	return fmt.Sprintf("Swap %s %d for %d", side, sw.from+sw.off, sw.to+sw.off)
}

// swapCounts: items changed per owner, Everyone for shared items.
type swapCounts map[string]int

func (n swapCounts) add(it swapItem) { n[cmpOr(it.Owner, swapEveryone)]++ }

// swapField: the Settings saves row's field for a Swap: counts per owner as the new value.
func (sw *swapper) field(n swapCounts) SavedField {
	f := newSavedField(swapKey, "routers", sw.label())
	f.New, _ = json.Marshal(n)
	from := cmpOr(strings.TrimSpace(sw.fromNames[0]), "no name")
	f.Note = from + " to " + cmpOr(sw.toName, "no name")
	return f
}

// swapSpec: the one Save's swap key. Panels are not in Config, so the commit writes them after (saveSettingsFields).
func swapSpec() fieldSpec {
	return fieldSpec{section: "routers", action: true,
		get: func(config.Config) any { return nil },
		set: func(c *config.Config, v json.RawMessage, x *saveCtx) error {
			req, err := decodeJSON[swapReq](v)
			if err != nil {
				return err
			}
			if x.swap == nil {
				return errors.New("swap: not ready")
			}
			n := swapCounts{}
			x.swap.config(c, "", pickOf(req.Items), n.add)
			x.swapped = n
			return nil
		}}
}

// swapPanelsAfter writes the panels of a saved Swap and returns the Settings saves field with every count.
func (a *App) swapPanelsAfter(x *saveCtx, edits map[string]fieldEdit, now config.Config) SavedField {
	req, _ := decodeJSON[swapReq](edits[swapKey].Value)
	n := maps.Clone(x.swapped)
	if n == nil {
		n = swapCounts{}
	}
	if err := x.swap.panels(a.panels, now, "", pickOf(req.Items), n.add); err != nil {
		log.Printf("swap: panels not all written: %v", err)
	}
	return x.swap.field(n)
}

type swapListResult struct {
	opResult
	Items []swapItem `json:"items"`
}

type swapMineResult struct {
	opResult
	Count int `json:"count"`
}

func (a *App) swapList(c *client, req swapReq) ([]swapItem, error) {
	mine := ""
	if req.Mine {
		if mine = c.userID(); mine == "" {
			return nil, errNoSuchUser
		}
	} else {
		a.mu.Lock()
		err := a.needRoleMu(c, config.RoleAdministrator)
		a.mu.Unlock()
		if err != nil {
			return nil, err
		}
	}
	cfg := a.r().cfg
	sw, err := a.newSwapper(cfg, req)
	if err != nil {
		return nil, err
	}
	out := []swapItem{}
	add := func(it swapItem) { out = append(out, it) }
	c2 := cfg.Clone()
	sw.config(&c2, mine, everyItem, add)
	for _, p := range a.panels.list() {
		if mine != "" && p.Owner != mine {
			continue
		}
		was := sw.brokenIn(p.Broken)
		if sw.panel(&p) {
			u, _ := cfg.FindUserByID(p.Owner)
			add(swapItem{ID: "panel/" + p.ID, Kind: "panel", Name: p.Name, Owner: u.Name, Broken: was})
		}
	}
	return out, nil
}

// swapMine: a signed-in user's Swap of their own presets, filter sets and panels. Like other My user changes it
// writes no Settings saves row.
func (a *App) swapMine(c *client, req swapReq) (int, error) {
	mine := c.userID()
	if mine == "" {
		return 0, errNoSuchUser
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	sw, err := a.newSwapper(a.r().cfg, req)
	if err != nil {
		return 0, err
	}
	pick := pickOf(req.Items)
	n := 0
	count := func(swapItem) { n++ }
	err = a.applyConfig(func(cur *config.Config) error {
		n = 0
		sw.config(cur, mine, pick, count)
		if n == 0 {
			return errNoCheckChange
		}
		return nil
	})
	if err != nil && !errors.Is(err, errNoCheckChange) {
		return 0, err
	}
	err = sw.panels(a.panels, a.r().cfg, mine, pick, count)
	return n, err
}

func (a *App) serveSwapList(c *client, m *inMsg) {
	var req swapReq
	if m.Swap != nil {
		req = *m.Swap
	}
	items, err := a.swapList(c, req)
	a.reply(c, swapListResult{opResult: opResultFor("swapList", &m.ID, err), Items: items})
}

func (a *App) serveSwapMine(c *client, m *inMsg) {
	defer m.finish()
	var req swapReq
	if m.Swap != nil {
		req = *m.Swap
	}
	n, err := a.swapMine(c, req)
	a.command(c, "swapMine", outcome(err), fmt.Sprintf("%d %s", n, plural(n, "item", "items")), err)
	a.reply(c, swapMineResult{opResult: opResultFor("swapMine", &m.ID, err), Count: n})
}
