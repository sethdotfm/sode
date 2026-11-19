package app

import (
	"cmp"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Accept a card: the Add import's review and clash choices (importplan.go), planned onto the recipient's
// own items. Presets and filter sets go to their Me lists, panels become theirs. Clashes are only with
// the recipient's own items, so an accept is never Critical. Routes outside the recipient's Rights still
// import; the review notes them and the Rights refuse them when the recipient fires the item.

const noteOutsideRights = "Outside your Rights"

type cardPlanner struct {
	live, next config.Config
	user       string // the recipient's id
	rights     config.Rights
	card       inboxCard
	req        importReq
	check      *refCheck
	allPanels  []Panel
	panels     []plannedPanel
	plan       importPlan
	// file: an export file, not a card (ownexport.go). An item equal to the importer's is No change, and
	// same (a file from this install) matches by hidden id and preselects Use new.
	file, same bool
}

// planCard plans card onto user toID of live. src, dst are the live router's names.
func planCard(live config.Config, src, dst map[int]string, allPanels []Panel, card inboxCard, toID string, req importReq) (*cardPlanner, error) {
	p, err := newCardPlanner(live, src, dst, allPanels, card, toID, req)
	if err != nil {
		return nil, err
	}
	p.run(nil)
	return p, nil
}

func newCardPlanner(live config.Config, src, dst map[int]string, allPanels []Panel, card inboxCard, toID string, req importReq) (*cardPlanner, error) {
	u, ok := live.FindUserByID(toID)
	if !ok {
		return nil, errNoSuchUser
	}
	_, rights := live.ActsAs(u)
	return &cardPlanner{
		live: live, next: live.Clone(), user: toID, rights: rights, card: card, req: req, allPanels: allPanels,
		check: &refCheck{uid: live.UID, levels: live.Levels, src: src, dst: dst, unknown: len(src) == 0 && len(dst) == 0},
		plan:  importPlan{SameInstall: true, Categories: map[string]*importCat{}},
	}, nil
}

// run plans profile (when given) and the card's items. Categories none in the request means all of them.
func (p *cardPlanner) run(profile func(*config.Config, *importCat) error) {
	steps := []struct {
		cat string
		n   int
		run func(*config.Config, *importCat) error
	}{
		{catProfile, 1, profile},
		{catPresets, len(p.card.Items.Presets), p.planPresets},
		{catFilterSets, len(p.card.Items.FilterSets), p.planFilterSets},
		{catPanels, len(p.card.Items.Panels), p.planPanels},
	}
	for _, s := range steps {
		if s.n == 0 || s.run == nil || len(p.req.Categories) > 0 && !slices.Contains(p.req.Categories, s.cat) {
			continue
		}
		cat := &importCat{Items: []importItem{}}
		p.plan.Categories[s.cat] = cat
		cand := p.next.Clone()
		p.check.waited = false
		err := s.run(&cand, cat)
		if err == nil && p.check.waited {
			err = errors.New("router names not read yet: try again after the re-read")
		}
		if err == nil && s.cat != catPanels {
			cand.Normalize()
			err = validateSave(p.live, cand)
		}
		if err != nil {
			cat.Refused = err.Error()
			if s.cat == catPanels {
				p.panels = nil
			}
			continue
		}
		p.next = cand
	}
}

func (p *cardPlanner) profile(cfg *config.Config) *config.Profile {
	i := slices.IndexFunc(cfg.Users, func(u config.User) bool { return u.ID == p.user })
	cfg.Users = slices.Clone(cfg.Users)
	cfg.Users[i].Profile = cfg.Users[i].Profile.Clone()
	return &cfg.Users[i].Profile
}

func (p *cardPlanner) picked(cat, key string) bool {
	keys, some := p.req.Pick[cat]
	return !some || slices.Contains(keys, key)
}

// choice: the page's choice, else Rename, since the clash is with the recipient's own item. A file from this
// install preselects Use new, as the Add import does.
func (p *cardPlanner) choice(cat, key string) string {
	switch c := p.req.Choices[cat][key]; c {
	case useNew, useCurrent, useSkip, useRename:
		return c
	}
	if p.same {
		return useNew
	}
	return useRename
}

// outside: the route is on this router and the recipient's Rights do not reach it.
func (p *cardPlanner) outside(kind string, r config.Ref) bool {
	if r.Router != "" && r.Router != p.live.UID {
		return false
	}
	switch kind {
	case config.MissingSource:
		return !p.rights.AllowsSource(r.N)
	case config.MissingDestination:
		return !p.rights.AllowsDst(r.N)
	}
	return !p.rights.AllowsLevel(r.N)
}

func (p *cardPlanner) presetOutside(s config.Preset) bool {
	for _, it := range s.Items {
		if p.outside(config.MissingSource, it.Src) || slices.ContainsFunc(it.Dsts, func(d config.Ref) bool { return p.outside(config.MissingDestination, d) }) ||
			slices.ContainsFunc(it.Levels, func(l int) bool { return p.outside(config.MissingLevel, config.Ref{Router: it.Src.Router, N: l}) }) {
			return true
		}
	}
	return false
}

func (p *cardPlanner) panelOutside(fp Panel) bool {
	for _, g := range fp.Pages {
		for _, c := range g.Cells {
			if r, ok := cellRef(c); ok {
				kind := config.MissingSource
				if c["kind"] == "destination" {
					kind = config.MissingDestination
				} else if c["kind"] != "source" {
					continue
				}
				if p.outside(kind, r) {
					return true
				}
			}
		}
	}
	for _, e := range []struct {
		edge *GridEdge
		kind string
	}{{fp.Sources, config.MissingSource}, {fp.Destinations, config.MissingDestination}} {
		if e.edge == nil {
			continue
		}
		for r := range edgeRefs(e.edge) {
			if p.outside(e.kind, r) {
				return true
			}
		}
	}
	return false
}

func withID(s config.Preset, id string) config.Preset {
	s.ID = id
	return s
}

func (p *cardPlanner) planPresets(cand *config.Config, cat *importCat) error {
	prof := p.profile(cand)
	ic := &importPlanner{check: p.check}
	for _, s := range ic.checkedPresets(p.card.Items.Presets) {
		if !p.picked(catPresets, s.ID) {
			continue
		}
		key := s.ID
		it := importItem{Key: key, Name: s.Name, Broken: s.Broken}
		if p.presetOutside(s) {
			it.Note = noteOutsideRights
		}
		taken := func(n string) bool {
			return slices.ContainsFunc(prof.Presets, func(x config.Preset) bool { return strings.EqualFold(x.Name, n) })
		}
		i := matchIdx(p.same, prof.Presets, s.ID, s.Name, func(x config.Preset) string { return x.ID }, func(x config.Preset) string { return x.Name })
		if i >= 0 && p.file && sameYAML(withID(s, prof.Presets[i].ID), prof.Presets[i]) {
			it.Same = true
			cat.Items = append(cat.Items, it)
			continue
		}
		if i >= 0 {
			it.Clash, it.Choice = prof.Presets[i].Name, p.choice(catPresets, key)
			switch it.Choice {
			case useNew:
				s.ID = prof.Presets[i].ID
				if !p.same { // matched by name: keep the live spelling
					s.Name = prof.Presets[i].Name
				}
				prof.Presets[i] = s
			case useRename:
				s.Name = nextName(s.Name, taken)
				it.As = s.Name
			}
			if it.Choice != useRename {
				cat.Items = append(cat.Items, it)
				continue
			}
		}
		if slices.ContainsFunc(prof.Presets, func(x config.Preset) bool { return x.ID == s.ID }) {
			s.ID = config.NewPresetID()
		}
		prof.Presets = append(prof.Presets, s)
		cat.Items = append(cat.Items, it)
	}
	return nil
}

func (p *cardPlanner) planFilterSets(cand *config.Config, cat *importCat) error {
	prof := p.profile(cand)
	for _, s := range p.card.Items.FilterSets {
		if !p.picked(catFilterSets, s.Name) {
			continue
		}
		key := s.Name
		s.Broken = p.check.filterSetBroken(s)
		it := importItem{Key: key, Name: s.Name, Broken: s.Broken}
		i := matchIdx(false, prof.FilterSets, "", s.Name, nil, func(x config.FilterSet) string { return x.Name })
		if i >= 0 && p.file {
			s.Extra = prof.FilterSets[i].Extra
			if sameYAML(s, prof.FilterSets[i]) {
				it.Same = true
				cat.Items = append(cat.Items, it)
				continue
			}
		}
		if i >= 0 {
			it.Clash, it.Choice = prof.FilterSets[i].Name, p.choice(catFilterSets, key)
			switch it.Choice {
			case useNew:
				s.Name, s.Extra = prof.FilterSets[i].Name, prof.FilterSets[i].Extra
				prof.FilterSets[i] = s
			case useRename:
				s.Name = nextName(s.Name, func(n string) bool {
					return slices.ContainsFunc(prof.FilterSets, func(x config.FilterSet) bool { return strings.EqualFold(x.Name, n) })
				})
				it.As = s.Name
				prof.FilterSets = append(prof.FilterSets, s)
			}
			cat.Items = append(cat.Items, it)
			continue
		}
		prof.FilterSets = append(prof.FilterSets, s)
		cat.Items = append(cat.Items, it)
	}
	return nil
}

// planPanels: a new panel belongs to the recipient, shared with nobody. Use new keeps the recipient's
// panel's id, sharing, short name, mode and client.
func (p *cardPlanner) planPanels(cand *config.Config, cat *importCat) error {
	var own []Panel
	for _, x := range p.allPanels {
		if x.Owner == p.user {
			own = append(own, x)
		}
	}
	ic := &importPlanner{check: p.check, presets: map[string]string{}, same: true}
	adds := 0
	for _, fp := range p.card.Items.Panels {
		if !p.picked(catPanels, fp.ID) {
			continue
		}
		key := fp.ID
		fp.Owner, fp.Share, fp.Client, fp.Slug, fp.Rev = p.user, PanelShare{Mode: panelShareMe}, "", "", 0
		fp.Broken = ic.panelBroken(&fp, *cand)
		it := importItem{Key: key, Name: fp.Name, Broken: fp.Broken}
		if p.panelOutside(fp) {
			it.Note = noteOutsideRights
		}
		if err := validatePanel(&fp); err != nil {
			it.Refused = err.Error()
		} else if fp.Acts == panelActsPublic && !cand.PublicPanels {
			it.Refused = errPublicOff.Error()
		}
		if it.Refused != "" {
			cat.Items = append(cat.Items, it)
			continue
		}
		i := matchIdx(false, own, "", fp.Name, nil, func(x Panel) string { return x.Name })
		if i >= 0 {
			lp := own[i]
			it.Clash, it.Choice = lp.Name, p.choice(catPanels, key)
			switch it.Choice {
			case useNew:
				fp.ID, fp.Name, fp.Acts, fp.Client, fp.Share, fp.Slug, fp.Rev = lp.ID, lp.Name, lp.Acts, lp.Client, lp.Share, lp.Slug, lp.Rev+1
				p.panels = append(p.panels, plannedPanel{P: fp, FileID: key, Live: &lp})
			case useRename:
				fp.Name = nextName(fp.Name, func(n string) bool {
					return slices.ContainsFunc(own, func(x Panel) bool { return strings.EqualFold(x.Name, n) })
				})
				it.As = fp.Name
			}
			if it.Choice != useRename {
				cat.Items = append(cat.Items, it)
				continue
			}
		}
		adds++
		p.panels = append(p.panels, plannedPanel{P: fp, FileID: key})
		cat.Items = append(cat.Items, it)
	}
	if len(p.allPanels)+adds > panelMax {
		return errPanelCap
	}
	return nil
}

// ReviewCard plans an accept and writes nothing.
func (a *App) ReviewCard(c *client, card string, req importReq) (*importPlan, error) {
	u, k, err := a.cardFor(c, card)
	if err != nil {
		return nil, err
	}
	src, dst := a.liveNames()
	p, err := planCard(a.r().cfg, src, dst, a.panels.list(), k, u.ID, req)
	if err != nil {
		return nil, err
	}
	return &p.plan, nil
}

// cardFor is a waiting card of c's user's Inbox.
func (a *App) cardFor(c *client, card string) (config.User, inboxCard, error) {
	u, err := a.inboxUser(c)
	if err != nil {
		return u, inboxCard{}, err
	}
	a.inboxSweep()
	k, ok := a.inbox.get(card)
	if !ok || k.To != u.ID {
		return u, k, errors.New("no such card")
	}
	if k.State != cardWaiting {
		return u, k, errCardGone
	}
	return u, k, nil
}

// AcceptCard plans again under cfgMu and writes the plan: the Me lists in one commit, then each panel.
// The card ends as accepted with the number of items written, so the Sent list can say "Accepted 4 of 5".
func (a *App) AcceptCard(c *client, card string, req importReq) (*importPlan, map[string]string, inboxCard, error) {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	u, k, err := a.cardFor(c, card)
	if err != nil {
		return nil, nil, k, err
	}
	src, dst := a.liveNames()
	p, err := planCard(a.r().cfg, src, dst, a.panels.list(), k, u.ID, req)
	if err != nil {
		return nil, nil, k, err
	}
	// Every ticked item refused: the card waits, so the person can fix it and accept again.
	planned, why := 0, ""
	for _, name := range slices.Sorted(maps.Keys(p.plan.Categories)) {
		cat := p.plan.Categories[name]
		why = cmp.Or(why, cat.Refused)
		for _, it := range cat.Items {
			why = cmp.Or(why, it.Refused)
			if cat.Refused == "" && it.writes() {
				planned++
			}
		}
	}
	if planned == 0 && why != "" {
		return &p.plan, nil, k, errors.New(why)
	}
	writes := func(cat string) bool {
		c := p.plan.Categories[cat]
		return c != nil && c.Refused == "" && slices.ContainsFunc(c.Items, importItem.writes)
	}
	if writes(catPresets) || writes(catFilterSets) {
		next := *p.profile(&p.next)
		err := a.applyConfig(func(cur *config.Config) error {
			i := slices.IndexFunc(cur.Users, func(x config.User) bool { return x.ID == u.ID })
			if i < 0 {
				return errNoSuchUser
			}
			cur.Users[i].Profile.Presets, cur.Users[i].Profile.FilterSets = next.Presets, next.FilterSets
			return nil
		})
		if err != nil {
			return &p.plan, nil, k, err
		}
	}
	failed := a.writeCardPanels(connActor(c), p, k.ID)
	n := 0
	for name, cat := range p.plan.Categories {
		for _, it := range cat.Items {
			if cat.Refused == "" && it.writes() && !(name == catPanels && failed[it.Key] != "") {
				n++
			}
		}
	}
	k, err = a.inbox.end(k.ID, cardAccepted, n, a.now())
	if err != nil {
		log.Printf("inbox: card %s accepted, then: %v", card, err)
	}
	a.pushInboxTo(map[string]bool{k.From: true, k.To: true}, nil)
	if len(failed) == 0 {
		failed = nil
	}
	return &p.plan, failed, k, nil
}

// writeCardPanels writes the planned panels and their frozen artwork. A new panel that is its own API
// client gets a new client with the recipient's Rights, as a panel made in the editor does.
func (a *App) writeCardPanels(by Actor, p *cardPlanner, card string) map[string]string {
	failed := map[string]string{}
	for _, pp := range p.panels {
		fp := pp.P
		arts := fp.Art
		fp.Art = nil
		var err error
		if pp.Live != nil {
			fp.Art = pp.Live.Art
			_, err = a.panels.replace(fp)
			for _, x := range pp.Live.Art {
				if err == nil {
					_, err = a.panels.delArt(fp.ID, x.ID)
				}
			}
		} else {
			fp, err = a.panels.create(fp)
			if err == nil && actsHasClient(fp.Acts) {
				rights := p.rights.Clone()
				fp.Client = panelClientName(fp.ID)
				err = a.putPanelClientLocked(by, fp.ID, &rights, nil)
				if err == nil {
					_, err = a.panels.replace(fp)
				}
				if err != nil {
					a.panels.delete(fp.ID)
					a.putPanelClientLocked(by, fp.ID, nil, nil)
				}
			}
		}
		var bad []string
		for _, x := range arts {
			if err != nil {
				break
			}
			b, ok := a.inbox.artData(card, pp.FileID, x.ID)
			if !ok || !a.panels.putArtAs(fp.ID, PanelArt{ID: x.ID, Type: x.Type}, b) {
				bad = append(bad, x.ID)
			}
		}
		if err == nil && len(bad) > 0 {
			err = errors.New("artwork not written: " + strings.Join(bad, ", "))
		}
		if err != nil {
			failed[pp.FileID] = err.Error()
			log.Printf("inbox: panel %s: %v", pp.P.Name, err)
		}
	}
	return failed
}

func (a *App) serveInboxReview(c *client, m *inMsg) {
	req := inboxBody(m)
	plan, err := a.ReviewCard(c, req.Card, req.Plan)
	res := inboxResult{T: "inboxResult", Op: "inboxReview", ID: m.ID, OK: err == nil, Plan: plan}
	if err != nil {
		res.Err = err.Error()
	}
	a.reply(c, res)
}

func (a *App) serveInboxAccept(c *client, m *inMsg) {
	defer m.finish()
	req := inboxBody(m)
	plan, failed, k, err := a.AcceptCard(c, req.Card, req.Plan)
	note := fmt.Sprintf("from %s: accepted %d of %d", a.senderName(k), k.Accepted, k.Count)
	a.replyInbox(c, "inboxAccept", m, inboxResult{Plan: plan, Fail: failed}, note, err)
}
