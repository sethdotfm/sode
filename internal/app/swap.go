package app

import (
	"cmp"
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

// GLOSSARY.md, Swap; docs/agents/settings-saves.md, Swap. A user's own Swap (swapMine) saves at once, outside the one
// Save.

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

// A name term's mark has N -1.
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

// answer is To for a reference to From on the Swap's side, as a RefWalk answer.
func (sw *swapper) answer(kind string, r config.Ref) []config.Ref {
	if kind != sw.kind() || !sw.ref(&r) {
		return nil
	}
	return []config.Ref{r}
}

// preset changes p in place; callers pass a copy (config.ClonePresets). A list that held To and From holds To once.
func (sw *swapper) preset(p *config.Preset) bool {
	changed := false
	config.RefWalk{Dedupe: true, Fn: func(kind string, r config.Ref) []config.Ref {
		out := sw.answer(kind, r)
		changed = changed || out != nil
		return out
	}}.Preset(p)
	if changed {
		p.Broken, p.Moved = sw.unmark(p.Broken), sw.unmark(p.Moved)
	}
	return changed
}

// The number as shown, else its exact name. Negated terms swap too.
func (sw *swapper) filterText(text string) (string, bool) {
	terms, ok := filterTerms(text)
	if !ok {
		return text, false
	}
	r := []rune(text)
	for i := len(terms) - 1; i >= 0; i-- {
		t := terms[i]
		var put string
		switch {
		case t.Kind == "num" && t.A-sw.off == sw.from:
			put = strconv.Itoa(sw.to + sw.off)
		case t.Kind == "exact" && sw.named(t.Text) && !sameName(t.Text, sw.toName):
			put = strconv.Itoa(sw.to + sw.off)
			if strings.TrimSpace(sw.toName) != "" {
				put = `"` + strings.ReplaceAll(strings.TrimSpace(sw.toName), `"`, `""`) + `"`
			}
		default:
			continue
		}
		r = slices.Concat(r[:t.at], []rune(put), r[t.end:])
	}
	// An exact term that already names To (Name moved) is no change.
	return string(r), string(r) != text
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

// panel copies what it changes, since a stored panel shares its lists. A Grid item that already has settings for To
// keeps them; From's go.
func (sw *swapper) panel(p *Panel) bool {
	edge := p.Sources
	if sw.dst {
		edge = p.Destinations
	}
	hasTo := edge != nil && slices.ContainsFunc(edge.Items, func(it GridItem) bool { return sw.ours(it.Router) && it.N == sw.to })
	changed := false
	l := layoutOf(*p).walkRefs(func(s refSlot, kind string, r config.Ref) []config.Ref {
		out := sw.answer(kind, r)
		if changed = changed || out != nil; out != nil && s == slotItem && hasTo {
			return []config.Ref{}
		}
		return out
	}, true)
	if !changed {
		return false
	}
	p.Pages, p.Settings, p.Sources, p.Destinations = l.Pages, l.Settings, l.Sources, l.Destinations
	p.Broken, p.Moved = sw.unmark(p.Broken), sw.unmark(p.Moved)
	return true
}

// For a list, an item the Swap would change. Broken is from before the Swap.
type swapHit func(it swapItem)

// mine is a user id and limits the Swap to the items of that user; "" is every owner.
func (sw *swapper) config(c *config.Config, mine string, pick func(id string) bool, hit swapHit) {
	for _, s := range itemScopes(c, mine) {
		ps, fs := config.ClonePresets(*s.Presets), slices.Clone(*s.FilterSets)
		for i := range ps {
			id, was := "preset/"+s.ID+"/"+ps[i].ID, sw.brokenIn(ps[i].Broken)
			if pick(id) && sw.preset(&ps[i]) {
				hit(swapItem{ID: id, Kind: "preset", Name: ps[i].Name, Owner: s.Name, Broken: was})
			}
		}
		for i := range fs {
			id, was := "filterSet/"+s.ID+"/"+fs[i].Name, sw.brokenIn(fs[i].Broken)
			if pick(id) && sw.filterSet(&fs[i]) {
				hit(swapItem{ID: id, Kind: "filterSet", Name: fs[i].Name, Owner: s.Name, Broken: was})
			}
		}
		*s.Presets, *s.FilterSets = ps, fs
	}
}

// onPanel swaps p, a copy, when it is picked and mine.
func (sw *swapper) onPanel(p *Panel, cfg config.Config, mine string, pick func(id string) bool, hit swapHit) bool {
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
}

func (sw *swapper) panels(s *panelStore, cfg config.Config, mine string, pick func(id string) bool, hit swapHit) error {
	return s.rewrite(func(p *Panel) bool { return sw.onPanel(p, cfg, mine, pick, hit) })
}

var errSwapSide = errors.New("swap: side is source or destination")

// Caller does not hold mu.
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
	note := func(kind string, r config.Ref) {
		if kind == sw.kind() && sw.ours(r.Router) && r.N == sw.from && strings.TrimSpace(r.Name) != "" && !sw.named(r.Name) {
			sw.fromNames = append(sw.fromNames, r.Name)
		}
	}
	w := config.RefWalk{Fn: func(kind string, r config.Ref) []config.Ref { note(kind, r); return nil }}
	for _, s := range itemScopes(&cfg, "") {
		for _, p := range *s.Presets {
			w.Preset(&p)
			for _, m := range slices.Concat(p.Broken, p.Moved) {
				note(m.Kind, m.Ref)
			}
		}
	}
	for _, p := range a.panels.list() {
		layoutOf(p).readRefs(func(s refSlot, kind string, r config.Ref) {
			if s == slotCell {
				note(kind, r)
			}
		})
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

// "Swap destination 23 for 40", numbers as shown.
func (sw *swapper) label() string {
	side := "source"
	if sw.dst {
		side = "destination"
	}
	return fmt.Sprintf("Swap %s %d for %d", side, sw.from+sw.off, sw.to+sw.off)
}

// swapCounts: items changed per owner, Everyone for shared items.
type swapCounts map[string]int

func (n swapCounts) add(it swapItem) { n[cmp.Or(it.Owner, swapEveryone)]++ }

// Counts per owner are the new value.
func (sw *swapper) field(n swapCounts) SavedField {
	f := newSavedField(swapKey, "routers", sw.label())
	f.New, _ = json.Marshal(n)
	// A Broken number has no name on the router now, so the first stored one names it.
	from := "no name"
	if i := slices.IndexFunc(sw.fromNames, func(n string) bool { return strings.TrimSpace(n) != "" }); i >= 0 {
		from = strings.TrimSpace(sw.fromNames[i])
	}
	f.Note = from + " to " + cmp.Or(sw.toName, "no name")
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
				return errors.New("swap: internal error, save again")
			}
			n := swapCounts{}
			x.swap.config(c, "", pickOf(req.Items), n.add)
			x.swapped = n
			return nil
		}}
}

func (a *App) swapCtx(live config.Config, edits map[string]fieldEdit) (*saveCtx, error) {
	x := &saveCtx{}
	e, ok := edits[swapKey]
	if !ok {
		return x, nil
	}
	req, err := decodeJSON[swapReq](e.Value)
	if err == nil {
		x.swap, err = a.newSwapper(live, req)
	}
	return x, err
}

// The Swap already ran on config. write false only counts, for a Request approval card.
func (a *App) swapPanelsField(x *saveCtx, edits map[string]fieldEdit, now config.Config, write bool) SavedField {
	req, _ := decodeJSON[swapReq](edits[swapKey].Value)
	pick := pickOf(req.Items)
	n := swapCounts{}
	maps.Copy(n, x.swapped)
	if write {
		if err := x.swap.panels(a.panels, now, "", pick, n.add); err != nil {
			log.Printf("swap: panels not all written: %v", err)
		}
		return x.swap.field(n)
	}
	for _, p := range a.panels.list() {
		x.swap.onPanel(&p, now, "", pick, n.add)
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
		sw.onPanel(&p, cfg, mine, everyItem, add)
	}
	return out, nil
}

// Like other My user changes it writes no Settings saves row.
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
