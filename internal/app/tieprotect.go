package app

import (
	"net/http"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// Protect in a tie take (docs/tielines-design.md, "Protect in a tie take"). A tie line of a route can carry protect.
// Sode then protects the upstream destination of that tie line, after every hop of the take. It cannot do it before,
// since Sode refuses its own take to a destination that it Protected. The Protect is its own entry in the Group of
// the take. A Protect that fails or that Sode does not send leaves the take a success: its entry has the error and
// its Still levels. Sode does not send the Protect when a route that asked for it failed. Then its entry says so,
// and the Group keeps its size.
//
// The Protect goes to the router of the tie line, by its unit (protectManyIn). Hops need no Rights, so only the
// Protect right counts, and an API caller needs the protect scope too.

// tieProtect is one tie line that the take protects. routes are the routes that asked for it.
type tieProtect struct {
	tie    string
	router string // the unit of the upstream router
	dst    int
	routes []*tieTakeRoute
	entry  *Entry // after the take
}

// apiTieProtectOut is one Protect of a tie take in the answer of POST /takes.
type apiTieProtectOut struct {
	Tie     string `json:"tie"`
	TieName string `json:"tieName"`
	Router  string `json:"router"`
	Dst     int    `json:"dst"`
	OK      bool   `json:"ok"`
	Error   string `json:"error,omitempty"`
	Entry   *int   `json:"entry,omitempty"`
	Still   []int  `json:"still,omitempty"`
}

// tieProtectAllowed refuses a take that asks for a Protect when the caller cannot set one. It runs before anything
// goes to a router.
func tieProtectAllowed(cl *apiCtx, protect bool) *apiProblem {
	switch {
	case !protect:
		return nil
	case cl.page == nil && !cl.has(config.ScopeProtect):
		return problem(http.StatusForbidden, "scope", "protect on a tie line needs the %s scope", config.ScopeProtect)
	case !cl.Rights.AllowsProtect():
		return problem(http.StatusForbidden, "outside_rights", "Protect is outside your Rights")
	}
	return nil
}

// protectHop notes that route tr asks for a Protect on the tie line of hop h.
func (t *tieTake) protectHop(h tieplan.Hop, tr *tieTakeRoute) {
	i := slices.IndexFunc(t.protects, func(p *tieProtect) bool { return p.tie == h.Tie })
	if i < 0 {
		t.protects = append(t.protects, &tieProtect{tie: h.Tie, router: h.Router, dst: h.Dst})
		i = len(t.protects) - 1
	}
	t.protects[i].routes = append(t.protects[i].routes, tr)
}

// protectHops sets each Protect of the take after its last wave, in the Group g of the take.
func (t *tieTake) protectHops(a *App, rt *runtime, c *client, g protGroup) {
	for _, p := range t.protects {
		why := ""
		if slices.ContainsFunc(p.routes, func(r *tieTakeRoute) bool { return !r.steps[len(r.steps)-1].ok() }) {
			why = "not sent: the take failed"
		} else {
			o := a.protectManyIn(c, []protectDst{{Dst: p.dst, Router: p.router}}, true, nil, protOpts{group: &g, hop: true})[0]
			if o.Err == nil {
				p.entry = o.Entry
				continue
			}
			why = o.Err.Error()
		}
		p.entry = a.protectNotSet(rt, c, p.router, p.dst, why, g)
	}
}

// protectNotSet records a failed Protect entry for a Protect that Sode did not send. Still names the levels that
// Sode holds on the destination all the same.
func (a *App) protectNotSet(rt *runtime, c *client, router string, dst int, why string, g protGroup) *Entry {
	u, ok := a.unit(router)
	if !ok {
		return nil
	}
	e := a.protectEntry(u, c, true, dst, uiLevels(rt.levelsOf(u.uid)), nil)
	g.mark(e)
	e.Err = why
	e.Failed = slices.Clone(e.Levels)
	a.protMu.Lock()
	_, own, _ := a.sodeHoldsProtLocked(rt, u, dst)
	a.protMu.Unlock()
	e.Still = own
	a.record(e)
	return e
}

// protectsOut is the Protects part of the answer.
func (t *tieTake) protectsOut() []apiTieProtectOut {
	var out []apiTieProtectOut
	for _, p := range t.protects {
		o := apiTieProtectOut{Tie: p.tie, TieName: t.tieName[p.tie], Router: p.router, Dst: p.dst, Error: "no entry"}
		if e := p.entry; e != nil {
			id := e.ID
			o.OK, o.Error, o.Entry, o.Still = e.OK, e.Err, &id, e.Still
		}
		out = append(out, o)
	}
	return out
}
