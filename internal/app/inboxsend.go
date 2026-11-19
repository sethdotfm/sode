package app

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Send a copy over the socket (docs/agents/profiles.md, Inbox). The Log gets one command row per send and answer.
// History gets nothing, since no route changes.

// Item scopes as the pages name them (web/presetstore.js): This browser, Me, Everyone.
const (
	scopeBrowser  = "local"
	scopeMe       = "user"
	scopeEveryone = "global"
)

var errGuestInbox = errors.New("sign in to use the Inbox")

// inboxReq is the one request body of every inbox message; each uses only its part.
type inboxReq struct {
	To         []string        `json:"to"` // inboxSend: user names
	Presets    []cardPresetReq `json:"presets"`
	FilterSets []cardSetReq    `json:"filterSets"`
	Panels     []string        `json:"panels"` // panel ids
	Card       string          `json:"card"`   // inboxReview, inboxAccept, inboxDecline, inboxWithdraw
	Plan       importReq       `json:"plan"`   // inboxReview, inboxAccept: pick and choices as in the import
}

// cardPresetReq names a Me or Everyone preset by id; a This browser preset comes whole.
type cardPresetReq struct {
	Scope  string         `json:"scope"`
	ID     string         `json:"id"`
	Preset *config.Preset `json:"preset"`
}

type cardSetReq struct {
	Scope string            `json:"scope"`
	Name  string            `json:"name"`
	Set   *config.FilterSet `json:"set"`
}

type inboxResult struct {
	T     string            `json:"t"` // "inboxResult"
	Op    string            `json:"op"`
	ID    int               `json:"id"`
	OK    bool              `json:"ok"`
	Err   string            `json:"err,omitempty"`
	Cards []string          `json:"cards,omitempty"` // inboxSend: the new cards
	Plan  *importPlan       `json:"plan,omitempty"`
	Fail  map[string]string `json:"failed,omitempty"`
}

// cardView is one line of the Inbox or the Sent list. From is "" for a removed sender ("from a removed
// user"); Items lists a waiting card's items for the review.
type cardView struct {
	ID       string     `json:"id"`
	From     string     `json:"from"`
	To       string     `json:"to"`
	At       int64      `json:"at"`      // unix ms
	Expires  int64      `json:"expires"` // unix ms; 0 after an answer
	State    string     `json:"state"`
	Count    int        `json:"count"`
	Accepted int        `json:"accepted,omitempty"`
	Items    []cardLine `json:"items,omitempty"`
}

type cardLine struct {
	Category string `json:"category"`
	Key      string `json:"key"`
	Name     string `json:"name"`
}

// Sode pushes inboxMsg to each connection signed in as the user. New lists the cards that arrived with this push, for
// one toast each; an answer never fills it.
type inboxMsg struct {
	T     string     `json:"t"` // "inbox"
	Cards []cardView `json:"cards"`
	Sent  []cardView `json:"sent"`
	New   []string   `json:"new,omitempty"`
}

func (a *App) inboxUser(c *client) (config.User, error) {
	u, err := a.ownUser(c)
	if err != nil || u.ID == "" {
		return u, errGuestInbox
	}
	return u, nil
}

func (a *App) replyInbox(c *client, op string, m *inMsg, res inboxResult, note string, err error) {
	res.T, res.Op, res.ID, res.OK = "inboxResult", op, m.ID, err == nil
	if err != nil {
		res.Err = err.Error()
	}
	a.command(c, op, outcome(err), note, err)
	a.reply(c, res)
}

func (a *App) cardView(cfg config.Config, c inboxCard) cardView {
	v := cardView{ID: c.ID, From: cfg.UserName(c.From), To: c.ToName, At: c.At.UnixMilli(), State: c.State, Count: c.Count, Accepted: c.Accepted}
	if n := cfg.UserName(c.To); n != "" {
		v.To = n
	}
	if c.State == cardWaiting {
		v.Expires = c.At.Add(a.inboxTTL).UnixMilli()
		for _, p := range c.Items.Presets {
			v.Items = append(v.Items, cardLine{catPresets, p.ID, p.Name})
		}
		for _, s := range c.Items.FilterSets {
			v.Items = append(v.Items, cardLine{catFilterSets, s.Name, s.Name})
		}
		for _, p := range c.Items.Panels {
			v.Items = append(v.Items, cardLine{catPanels, p.ID, p.Name})
		}
	}
	return v
}

// pushInboxMu sends c's user their Inbox and Sent list, or clears what a connection that lost its user
// holds. Caller holds mu.
func (a *App) pushInboxMu(c *client, fresh []string) {
	cfg := a.r().cfg
	if !a.userOKMu(c, cfg) || c.u.user.Load().ID == "" {
		if c.u.inbox {
			c.u.inbox = false
			b, _ := json.Marshal(inboxMsg{T: "inbox", Cards: []cardView{}, Sent: []cardView{}})
			a.deliver(c, b)
		}
		return
	}
	id := c.u.user.Load().ID
	m := inboxMsg{T: "inbox", Cards: []cardView{}, Sent: []cardView{}}
	for _, k := range a.inbox.cards(func(k inboxCard) bool { return k.To == id && k.State == cardWaiting || k.From == id }) {
		if k.To == id && k.State == cardWaiting {
			m.Cards = append(m.Cards, a.cardView(cfg, k))
			if slices.Contains(fresh, k.ID) {
				m.New = append(m.New, k.ID)
			}
		}
		if k.From == id {
			m.Sent = append(m.Sent, a.cardView(cfg, k))
		}
	}
	c.u.inbox = true
	b, _ := json.Marshal(m)
	a.deliver(c, b)
}

func (a *App) pushInboxTo(ids map[string]bool, fresh []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for cl := range a.clients {
		if ids[cl.userID()] {
			a.pushInboxMu(cl, fresh)
		}
	}
}

func (a *App) serveInboxList(c *client, m *inMsg) {
	if _, err := a.inboxUser(c); err != nil {
		a.reply(c, inboxResult{T: "inboxResult", Op: "inboxList", ID: m.ID, Err: err.Error()})
		return
	}
	a.inboxSweep()
	a.mu.Lock()
	a.pushInboxMu(c, nil)
	a.mu.Unlock()
	a.reply(c, inboxResult{T: "inboxResult", Op: "inboxList", ID: m.ID, OK: true})
}

func inboxBody(m *inMsg) inboxReq {
	if m.Inbox == nil {
		return inboxReq{}
	}
	return *m.Inbox
}

func itemCounts(it cardItems) string {
	var parts []string
	for _, p := range []struct {
		n         int
		one, many string
	}{
		{len(it.Presets), "preset", "presets"},
		{len(it.FilterSets), "filter set", "filter sets"},
		{len(it.Panels), "panel", "panels"},
	} {
		if p.n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", p.n, plural(p.n, p.one, p.many)))
		}
	}
	return strings.Join(parts, ", ")
}

func (a *App) serveInboxSend(c *client, m *inMsg) {
	defer m.finish()
	req := inboxBody(m)
	cards, items, err := a.SendCopy(c, req)
	var res inboxResult
	var to []string
	for _, k := range cards {
		res.Cards = append(res.Cards, k.ID)
		to = append(to, k.ToName)
	}
	a.replyInbox(c, "inboxSend", m, res, fmt.Sprintf("to %s: %s", strings.Join(to, ", "), itemCounts(items)), err)
}

// SendCopy freezes the items c's user can see into one card per recipient.
func (a *App) SendCopy(c *client, req inboxReq) ([]inboxCard, cardItems, error) {
	from, err := a.inboxUser(c)
	if err != nil {
		return nil, cardItems{}, err
	}
	cfg := a.r().cfg
	items, art, err := a.freezeItems(cfg, from, req)
	if err != nil {
		return nil, items, err
	}
	if items.count() == 0 {
		return nil, items, errors.New("pick at least one preset, filter set or panel to send")
	}
	var cards []inboxCard
	seen := map[string]bool{}
	for _, name := range req.To {
		u, ok := cfg.FindUserByName(strings.TrimSpace(name))
		if !ok {
			return nil, items, fmt.Errorf("there is no user named %q", strings.TrimSpace(name))
		}
		if seen[u.ID] {
			continue
		}
		seen[u.ID] = true
		cards = append(cards, inboxCard{From: from.ID, FromName: from.Name, To: u.ID, ToName: u.Name, At: a.now(), State: cardWaiting, Count: items.count(), Items: items})
	}
	if len(cards) == 0 {
		return nil, items, errors.New("pick at least one user to send to")
	}
	a.inboxSweep()
	stored, full, err := a.inbox.add(cards, art, func(id string) bool {
		_, ok := a.r().cfg.FindUserByID(id)
		return ok
	})
	if full != "" {
		return nil, items, fmt.Errorf("%s's Inbox is full", full)
	}
	if err != nil {
		log.Printf("inbox: send from %s: %v", from.Name, err)
		err = errors.New("the copy could not be stored: " + err.Error())
	}
	ids, fresh := map[string]bool{from.ID: true}, []string{}
	for _, k := range stored {
		ids[k.To] = true
		fresh = append(fresh, k.ID)
	}
	a.pushInboxTo(ids, fresh)
	return stored, items, err
}

// freezeItems reads Me and Everyone items and panels here, and from must be able to see them. Only This browser items
// come in the request.
func (a *App) freezeItems(cfg config.Config, from config.User, req inboxReq) (cardItems, map[string][]byte, error) {
	var it cardItems
	art := map[string][]byte{}
	for _, p := range req.Presets {
		var s config.Preset
		var ok bool
		switch p.Scope {
		case scopeMe:
			s, ok = findPreset(from.Profile.Presets, p.ID)
		case scopeEveryone:
			s, ok = findPreset(cfg.Presets, p.ID)
		case scopeBrowser:
			if p.Preset != nil {
				s, ok = *p.Preset, true
			}
		}
		if !ok {
			return it, nil, fmt.Errorf("no %s preset %q to send", scopeWord(p.Scope), p.ID)
		}
		s = config.ClonePresets([]config.Preset{s})[0]
		s.Name, s.Broken = cleanPresetName(s.Name), nil
		if s.ID == "" || p.Scope == scopeBrowser {
			s.ID = config.NewPresetID()
		}
		it.Presets = append(it.Presets, s)
	}
	for _, f := range req.FilterSets {
		var s config.FilterSet
		ok := false
		find := func(list []config.FilterSet) {
			i := slices.IndexFunc(list, func(x config.FilterSet) bool { return strings.EqualFold(x.Name, f.Name) })
			if i >= 0 {
				s, ok = list[i], true
			}
		}
		switch f.Scope {
		case scopeMe:
			find(from.Profile.FilterSets)
		case scopeEveryone:
			find(cfg.FilterSets)
		case scopeBrowser:
			if f.Set != nil {
				s, ok = *f.Set, true
			}
		}
		if !ok {
			return it, nil, fmt.Errorf("no %s filter set %q to send", scopeWord(f.Scope), f.Name)
		}
		s.Broken = nil
		if slices.ContainsFunc(it.FilterSets, func(x config.FilterSet) bool { return strings.EqualFold(x.Name, s.Name) }) {
			continue
		}
		it.FilterSets = append(it.FilterSets, s)
	}
	actor := &apiCtx{Actor: Actor{Kind: actorUser, Name: from.Name, User: from.ID}}
	for _, id := range req.Panels {
		p, ok := a.panels.get(id)
		if ok {
			ok, _ = a.panelReach(actor, p)
		}
		if !ok {
			return it, nil, fmt.Errorf("no panel %q to send", id)
		}
		if slices.ContainsFunc(it.Panels, func(x Panel) bool { return x.ID == p.ID }) {
			continue
		}
		// What a copy never brings: sharing, keys (the client), the short name and the stored counts.
		p.Share, p.Client, p.Slug, p.Rev, p.Broken, p.Owner = PanelShare{Mode: panelShareMe}, "", "", 0, nil, ""
		var kept []PanelArt
		for _, x := range p.Art {
			if b, _, ok := a.panels.artData(p.ID, x.ID); ok {
				art[p.ID+"/"+x.ID] = b
				kept = append(kept, x)
			}
		}
		p.Art = kept
		it.Panels = append(it.Panels, p)
	}
	return it, art, nil
}

func findPreset(list []config.Preset, id string) (config.Preset, bool) {
	i := slices.IndexFunc(list, func(x config.Preset) bool { return id != "" && x.ID == id })
	if i < 0 {
		return config.Preset{}, false
	}
	return list[i], true
}

func scopeWord(scope string) string {
	switch scope {
	case scopeMe:
		return "Me"
	case scopeEveryone:
		return "Everyone"
	case scopeBrowser:
		return "This browser"
	}
	return "such"
}

// answerCard ends a waiting card that c's user can answer: the recipient declines, the sender withdraws.
func (a *App) answerCard(c *client, card, state string) (inboxCard, error) {
	u, err := a.inboxUser(c)
	if err != nil {
		return inboxCard{}, err
	}
	a.cfgMu.Lock() // an accept of the same card holds it
	defer a.cfgMu.Unlock()
	a.inboxSweep()
	k, ok := a.inbox.get(card)
	mine := ok && (state == cardDeclined && k.To == u.ID || state == cardWithdrawn && k.From == u.ID)
	if !mine {
		return k, errors.New("no such card")
	}
	k, err = a.inbox.end(card, state, 0, a.now())
	if err != nil {
		return k, err
	}
	a.pushInboxTo(map[string]bool{k.From: true, k.To: true}, nil)
	return k, nil
}

func (a *App) serveInboxDecline(c *client, m *inMsg) {
	k, err := a.answerCard(c, inboxBody(m).Card, cardDeclined)
	a.replyInbox(c, "inboxDecline", m, inboxResult{}, fmt.Sprintf("from %s: %d %s", a.senderName(k), k.Count, plural(k.Count, "item", "items")), err)
}

func (a *App) serveInboxWithdraw(c *client, m *inMsg) {
	k, err := a.answerCard(c, inboxBody(m).Card, cardWithdrawn)
	a.replyInbox(c, "inboxWithdraw", m, inboxResult{}, fmt.Sprintf("to %s: %d %s", k.ToName, k.Count, plural(k.Count, "item", "items")), err)
}

func (a *App) senderName(k inboxCard) string {
	if n := a.r().cfg.UserName(k.From); n != "" {
		return n
	}
	return "a removed user"
}

func (a *App) inboxSweep() {
	a.inboxEnded(a.inbox.sweep(a.now(), a.inboxTTL, inboxKeep))
}

func (a *App) inboxEnded(cards []inboxCard) {
	if len(cards) == 0 {
		return
	}
	ids := map[string]bool{}
	for _, k := range cards {
		e := event(kindCommand, "inboxExpired", sodeActor.Name, "ok")
		e.Note = fmt.Sprintf("from %s to %s: %d %s", cmp.Or(a.r().cfg.UserName(k.From), k.FromName), k.ToName, k.Count, plural(k.Count, "item", "items"))
		a.logEvent(e)
		ids[k.From], ids[k.To] = true, true
	}
	a.pushInboxTo(ids, nil)
}

// inboxFollowUsers runs after a commit that can remove users (panelsFollowUsers).
func (a *App) inboxFollowUsers(removed map[string]bool) {
	if len(removed) > 0 {
		a.inboxEnded(a.inbox.usersRemoved(removed, a.now()))
	}
}

// A read or an answer also sweeps.
const inboxSweepEvery = time.Minute
