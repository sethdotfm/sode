package app

import (
	"cmp"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// Broken and Name moved counts for Settings > Routers and My user (GLOSSARY.md, Broken and Name moved). The page
// shows these numbers as sent and never counts them itself. An item with both kinds of mark counts as Broken only.

const (
	statusBroken = "broken"
	statusMoved  = "moved"
)

// brokenEntry: one Broken or Name moved item. ID is the swapItem id. Swap is for the Swap dialog: the side and
// number of the first marked source or destination, Broken first. It is nil for a filter set's name term.
type brokenEntry struct {
	ID     string    `json:"id"`
	Kind   string    `json:"kind"` // preset, filterSet or panel
	Name   string    `json:"name"`
	Owner  string    `json:"owner"` // the user's name, "" for Everyone
	Status string    `json:"status"`
	Swap   *swapFrom `json:"swap,omitempty"`
}

type swapFrom struct {
	Side string `json:"side"`
	From int    `json:"from"`
}

type brokenRouter struct {
	Router string        `json:"router"`
	Title  string        `json:"title"`
	Broken int           `json:"broken"`
	Moved  int           `json:"moved"`
	Items  []brokenEntry `json:"items"`
}

type brokenCountResult struct {
	opResult
	Routers []brokenRouter `json:"routers"`
}

func firstSwap(ls ...[]config.Missing) *swapFrom {
	for _, l := range ls {
		for _, m := range l {
			if (m.Kind == config.MissingSource || m.Kind == config.MissingDestination) && m.Ref.N >= 0 {
				return &swapFrom{Side: m.Kind, From: m.Ref.N}
			}
		}
	}
	return nil
}

func markedEntry(id, kind, name, owner string, broken, moved []config.Missing) (brokenEntry, bool) {
	e := brokenEntry{ID: id, Kind: kind, Name: name, Owner: owner, Swap: firstSwap(broken, moved)}
	switch {
	case len(broken) > 0:
		e.Status = statusBroken
	case len(moved) > 0:
		e.Status = statusMoved
	default:
		return e, false
	}
	return e, true
}

// mine: a user id, or "" for every owner. Sode drives one router, so every mark counts for it.
func brokenItems(cfg config.Config, panels []Panel, mine string) []brokenEntry {
	out := []brokenEntry{}
	add := func(e brokenEntry, ok bool) {
		if ok {
			out = append(out, e)
		}
	}
	for _, s := range itemScopes(&cfg, mine) {
		for _, p := range *s.Presets {
			add(markedEntry("preset/"+s.ID+"/"+p.ID, "preset", p.Name, s.Name, p.Broken, p.Moved))
		}
		for _, f := range *s.FilterSets {
			add(markedEntry("filterSet/"+s.ID+"/"+f.Name, "filterSet", f.Name, s.Name, f.Broken, nil))
		}
	}
	for _, p := range panels {
		if mine != "" && p.Owner != mine {
			continue
		}
		u, _ := cfg.FindUserByID(p.Owner)
		add(markedEntry("panel/"+p.ID, "panel", p.Name, u.Name, p.Broken, p.Moved))
	}
	slices.SortStableFunc(out, func(a, b brokenEntry) int {
		return cmp.Or(cmp.Compare(a.Status, b.Status), cmp.Compare(a.Owner, b.Owner), cmp.Compare(a.Name, b.Name))
	})
	return out
}

func (a *App) brokenCount(c *client, mine bool) ([]brokenRouter, error) {
	who := ""
	if mine {
		if who = c.userID(); who == "" {
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
	r := brokenRouter{Router: cfg.UID, Title: cfg.Title, Items: brokenItems(cfg, a.panels.list(), who)}
	for _, e := range r.Items {
		if e.Status == statusBroken {
			r.Broken++
		} else {
			r.Moved++
		}
	}
	return []brokenRouter{r}, nil
}

func (a *App) serveBrokenCount(c *client, m *inMsg) {
	routers, err := a.brokenCount(c, m.Mine)
	a.reply(c, brokenCountResult{opResult: opResultFor("brokenCount", &m.ID, err), Routers: routers})
}
