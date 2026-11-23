package app

import (
	"cmp"
	"errors"
	"fmt"
	"log"
	"maps"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// Accept a card: the Add import's review and clash choices (importplan.go), planned onto the recipient's
// own items. Presets and filter sets go to their Me lists, panels become theirs. Clashes are only with
// the recipient's own items, so an accept is never Critical. Routes outside the recipient's Rights still
// import; the review notes them and the Rights refuse them when the recipient fires the item.

const noteOutsideRights = "Outside your Rights"

// cardPlanner's choice preselects Rename, since the clash is with the recipient's own item, or Use new on a file from
// this install.
type cardPlanner struct {
	planBase
	user      string // the recipient's id
	rights    config.Rights
	card      inboxCard
	allPanels []Panel
	panels    []plannedPanel
	// file: an export file, not a card (ownexport.go). An item equal to the importer's is No change.
	file bool
}

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
	p := &cardPlanner{planBase: newPlanBase(live, src, dst, req, false), user: toID, rights: rights, card: card, allPanels: allPanels}
	p.plan.SameInstall = true
	return p, nil
}

// No categories in the request means all.
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
		if _, err := p.step(s.cat, s.cat != catPanels, s.run); err != nil && s.cat == catPanels {
			p.panels = nil
		}
	}
}

func (p *cardPlanner) profile(cfg *config.Config) *config.Profile {
	i := slices.IndexFunc(cfg.Users, func(u config.User) bool { return u.ID == p.user })
	cfg.Users = slices.Clone(cfg.Users)
	cfg.Users[i].Profile = cfg.Users[i].Profile.Clone()
	return &cfg.Users[i].Profile
}

// outside: the route is on a router of Sode and the recipient's Rights do not reach it on that router. A level is
// matched on its router too.
func (p *cardPlanner) outside(kind string, r config.Ref) bool {
	r.Router = cmp.Or(r.Router, p.live.UID)
	if _, ok := p.live.RouterByUID(r.Router); !ok {
		return false
	}
	switch kind {
	case config.MissingSource:
		return !p.rights.AllowsSource(r)
	case config.MissingDestination:
		return !p.rights.AllowsDst(r)
	}
	return !p.rights.AllowsLevel(r.Router, r.N)
}

func (p *cardPlanner) presetOutside(s config.Preset) bool {
	out := false
	config.RefWalk{Fn: func(kind string, r config.Ref) []config.Ref { out = out || p.outside(kind, r); return nil }}.Preset(&s)
	return out || slices.ContainsFunc(s.Items, func(it config.PresetItem) bool {
		return slices.ContainsFunc(it.Levels, func(l int) bool { return p.outside(config.MissingLevel, config.Ref{Router: it.Src.Router, N: l}) })
	})
}

func (p *cardPlanner) panelOutside(fp Panel) bool {
	out := false
	layoutOf(fp).readRefs(func(s refSlot, kind string, r config.Ref) { out = out || s != slotFixed && p.outside(kind, r) })
	return out
}

func withID(s config.Preset, id string) config.Preset {
	s.ID = id
	return s
}

func withSetID(s config.FilterSet, id string) config.FilterSet {
	s.ID = id
	return s
}

func (p *cardPlanner) planPresets(cand *config.Config, cat *importCat) error {
	prof := p.profile(cand)
	for _, s := range p.check.checked(p.card.Items.Presets) {
		if !p.picked(catPresets, s.ID) {
			continue
		}
		it := importItem{Key: s.ID, Name: s.Name, Broken: s.Broken}
		if p.presetOutside(s) {
			it.Note = noteOutsideRights
		}
		out := addItem
		if i := matchIdx(p.same, prof.Presets, s.ID, s.Name, func(x config.Preset) string { return x.ID }, presetName); i >= 0 {
			lp := prof.Presets[i]
			if out = p.settle(&it, catPresets, lp.Name, p.file && sameYAML(withID(s, lp.ID), lp), namesIn(prof.Presets, presetName)); out == overwrite {
				s.ID = lp.ID
				if !p.same { // matched by name: keep the live spelling
					s.Name = lp.Name
				}
				prof.Presets[i] = s
			}
		}
		if out == addItem {
			s.Name = cmp.Or(it.As, s.Name)
			if slices.ContainsFunc(prof.Presets, func(x config.Preset) bool { return x.ID == s.ID }) {
				s.ID = config.NewPresetID()
			}
			prof.Presets = append(prof.Presets, s)
		}
		cat.Items = append(cat.Items, it)
	}
	return nil
}

func (p *cardPlanner) planFilterSets(cand *config.Config, cat *importCat) error {
	prof := p.profile(cand)
	for _, s := range p.card.Items.FilterSets {
		if !p.picked(catFilterSets, s.ID) {
			continue
		}
		s.Broken = p.check.filterSetBroken(s)
		it := importItem{Key: s.ID, Name: s.Name, Broken: s.Broken}
		out := addItem
		if i := matchSet(p.same, prof.FilterSets, s); i >= 0 {
			lf, same := prof.FilterSets[i], false
			if p.file {
				s.Extra = lf.Extra
				same = sameYAML(withSetID(s, lf.ID), lf)
			}
			if out = p.settle(&it, catFilterSets, lf.Name, same, namesIn(prof.FilterSets, setName)); out == overwrite {
				s.ID, s.Extra = lf.ID, lf.Extra
				if !p.same { // matched by name: keep the live spelling
					s.Name = lf.Name
				}
				prof.FilterSets[i] = s
			}
		}
		if out == addItem {
			s.Name = cmp.Or(it.As, s.Name)
			if s.ID == "" || slices.ContainsFunc(prof.FilterSets, func(x config.FilterSet) bool { return x.ID == s.ID }) {
				s.ID = config.NewFilterSetID()
			}
			prof.FilterSets = append(prof.FilterSets, s)
		}
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
	adds := 0
	for _, fp := range p.card.Items.Panels {
		if !p.picked(catPanels, fp.ID) {
			continue
		}
		key := fp.ID
		fp.Owner, fp.Share, fp.Client, fp.Slug, fp.Rev = p.user, PanelShare{Mode: panelShareMe}, "", "", 0
		// A card comes from this install, so its preset buttons keep their ids.
		fp.Broken = p.check.panelBroken(&fp, *cand, func(id string) string { return id })
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
		out := addItem
		if i := matchIdx(false, own, "", fp.Name, nil, panelName); i >= 0 {
			lp := own[i]
			if out = p.settle(&it, catPanels, lp.Name, false, namesIn(own, panelName)); out == overwrite {
				fp.ID, fp.Name, fp.Acts, fp.Client, fp.Share, fp.Slug, fp.Rev = lp.ID, lp.Name, lp.Acts, lp.Client, lp.Share, lp.Slug, lp.Rev+1
				p.panels = append(p.panels, plannedPanel{P: fp, FileID: key, Live: &lp})
			}
		}
		if out == addItem {
			fp.Name = cmp.Or(it.As, fp.Name)
			adds++
			p.panels = append(p.panels, plannedPanel{P: fp, FileID: key})
		}
		cat.Items = append(cat.Items, it)
	}
	if len(p.allPanels)+adds > panelMax {
		return errPanelCap
	}
	return nil
}

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
	why := ""
	for _, name := range slices.Sorted(maps.Keys(p.plan.Categories)) {
		cat := p.plan.Categories[name]
		why = cmp.Or(why, cat.Refused)
		for _, it := range cat.Items {
			why = cmp.Or(why, it.Refused)
		}
	}
	if p.plan.written(nil) == 0 && why != "" {
		return &p.plan, nil, k, errors.New(why)
	}
	if p.plan.writes(catPresets) || p.plan.writes(catFilterSets) {
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
	k, err = a.inbox.end(k.ID, cardAccepted, p.plan.written(failed), a.now())
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
	create := func(fp Panel) (Panel, error) {
		fp, err := a.panels.create(fp)
		if err != nil || !actsHasClient(fp.Acts) {
			return fp, err
		}
		rights := p.rights.Clone()
		fp.Client = panelClientName(fp.ID)
		if err = a.putPanelClientLocked(by, fp.ID, &rights, nil); err == nil {
			_, err = a.panels.replace(fp)
		}
		if err != nil {
			a.panels.delete(fp.ID)
			a.putPanelClientLocked(by, fp.ID, nil, nil)
		}
		return fp, err
	}
	for _, pp := range p.panels {
		_, err := a.storePanel(pp, create, func(x PanelArt) ([]byte, bool) { return a.inbox.artData(card, pp.FileID, x.ID) })
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
