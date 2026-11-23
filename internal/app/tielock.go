package app

import (
	"net/http"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/sethdotfm/sode/internal/tieplan"
)

// Tie locks and the stale check (docs/tielines-design.md, "The stale check"). A tie take locks every tie line that it
// names, and every tie line whose upstream destination it routes, before it sends anything. A plain take to the
// upstream destination of a tie line takes the same lock (takeAllowed). Each take locks its whole set in the order of
// the tie line ids and frees it when the take ends. Thus two takes never wait on each other. A take never asks for
// more locks while it holds some: the steps of a tie take go through runParts, which takes no lock.

// tieLocks holds one lock for each tie line id.
type tieLocks struct {
	mu      sync.Mutex
	m       map[string]chan struct{} // a full channel is a held lock
	waiting atomic.Int32             // takes that wait for a lock now; tests read it
}

// lock takes the lock of each id, in id order. The caller calls unlock when its take ends.
func (l *tieLocks) lock(ids []string) (unlock func()) {
	ids = slices.Compact(slices.Sorted(slices.Values(ids)))
	chs := make([]chan struct{}, len(ids))
	l.mu.Lock()
	if l.m == nil {
		l.m = map[string]chan struct{}{}
	}
	for i, id := range ids {
		if l.m[id] == nil {
			l.m[id] = make(chan struct{}, 1)
		}
		chs[i] = l.m[id]
	}
	l.mu.Unlock()
	for _, ch := range chs {
		select {
		case ch <- struct{}{}:
		default:
			l.waiting.Add(1)
			ch <- struct{}{}
			l.waiting.Add(-1)
		}
	}
	return func() {
		for i := len(chs) - 1; i >= 0; i-- {
			<-chs[i]
		}
	}
}

// upstreamTies gives the ids of the tie lines whose upstream destination is the destination of an item.
// A kept One-off tie line counts too. Caller does not hold mu.
func (a *App) upstreamTies(rt *runtime, items []routeReq) []string {
	if len(rt.cfg.TieLines) == 0 && len(a.oneOffs.froms()) == 0 {
		return nil
	}
	at := map[tieplan.End]bool{}
	for _, it := range items {
		if u, ok := a.unit(a.itemRouter(it)); ok {
			at[tieplan.End{Router: u.uid, N: it.Dst}] = true
		}
	}
	a.mu.Lock()
	ties, _ := a.tieLinesNow(rt)
	a.mu.Unlock()
	var ids []string
	for _, t := range ties {
		if at[t.From] {
			ids = append(ids, t.ID)
		}
	}
	return ids
}

// tieEnd gives the router unit end of a reference in a tie take: the expected source of a tie line, or an Affected
// destination. A bare number is not enough, as a tie take has two routers or more.
func (a *App) tieEnd(e apiEnd) (tieplan.End, *apiProblem) {
	switch {
	case e.Router == "":
		return tieplan.End{}, problem(http.StatusBadRequest, "router_needed", `give expect and affected as {"router": uid, "n": number}`)
	case !a.knownRouter(e.Router):
		return tieplan.End{}, problem(http.StatusNotFound, "unknown_router", "no router with UID %q; GET %s/routers lists them", e.Router, apiBase)
	}
	return tieplan.End{Router: a.unitOrFirst(e.Router).uid, N: e.N}, nil
}

// tieTakeBegin locks the tie lines of t and reads the upstream destination of each from its router. Then it
// compares each tie line with what the caller saw. Sode planned t before the locks, so tieTakeBegin plans the take
// again and gives the new plan. The old plan can leave out a hop whose tie line carried the source. If the tie line
// changed during the wait, that hop must go. The new plan names the same tie lines, so the locks cover it. On a refusal it holds no lock and nothing went to a router.
func (a *App) tieTakeBegin(rt *runtime, v apiView, c *client, t *tieTake) (_ *tieTake, unlock func(), _ *apiProblem) {
	var ids []string
	reqs := make([]routeReq, 0, len(t.steps))
	for _, s := range t.steps {
		reqs = append(reqs, s.req)
	}
	for _, r := range t.routes {
		if r.mark != nil {
			for _, h := range r.mark.hops {
				ids = append(ids, h.Tie)
			}
		}
	}
	unlock = a.tieLocks.lock(append(ids, a.upstreamTies(rt, reqs)...))
	a.readTieLines(rt, c, t)
	if p := a.tieChanged(rt, v, t); p != nil {
		unlock()
		return nil, nil, p
	}
	if t.again != nil {
		now, p := t.again()
		if p != nil {
			unlock()
			return nil, nil, p
		}
		t = now
	}
	// Sode keeps each One-off tie line of the take while the take runs, and after it while its downstream source is
	// routed (tieoneoff.go). Each hop records who stated it.
	by := c.who()
	for _, r := range t.routes {
		if r.mark != nil {
			for i := range r.mark.hops {
				if o := r.mark.hops[i].OneOff; o != nil {
					o.By = by
				}
			}
		}
	}
	release := a.holdOneOffs(t.oneOffs(), by)
	tieLocks := unlock
	unlock = func() {
		release()
		tieLocks()
	}
	return t, unlock, nil
}

// readTieLines is the check before take on the upstream destination of each tie line of t, on all levels, before
// the stale check. The first wave then finds them fresh and does not read them again.
func (a *App) readTieLines(rt *runtime, c *client, t *tieTake) {
	type at struct {
		router string
		dst    int
	}
	seen := map[at]bool{}
	var parts []routerPart
	for _, r := range t.routes {
		if r.mark == nil {
			continue
		}
		for _, h := range r.mark.hops {
			u, ok := a.unit(h.Router)
			if !ok || seen[at{u.uid, h.Dst}] {
				continue
			}
			seen[at{u.uid, h.Dst}] = true
			i := slices.IndexFunc(parts, func(p routerPart) bool { return p.u == u })
			if i < 0 {
				parts = append(parts, routerPart{u: u})
				i = len(parts) - 1
			}
			parts[i].items = append(parts[i].items, routeReq{Src: h.Src, Dst: h.Dst, Levels: h.Levels, Router: u.uid})
		}
	}
	// What the read finds that Sode did not know is logged as for any check: a hop's entry logs it first.
	runLimited(len(parts), len(parts), func(i int) { a.checkRoutes(rt, parts[i].u, c, parts[i].items) })
}

// apiTieChangedRoute is one route of a 409 tie_line_changed body: the path through its tie lines as Sode has it now.
// Changed names each tie line that moved. NewAffected holds each Affected destination that the caller did not see.
type apiTieChangedRoute struct {
	apiTieRouteOut
	Changed     []apiTieRef    `json:"changed"`
	NewAffected []apiTieEndOut `json:"newAffected"`
}

type apiTieChangedBody struct {
	Error  apiErr               `json:"error"`
	Routes []apiTieChangedRoute `json:"routes"`
}

// tieWant is what the caller saw on a tie line. Without perLevel, one is the source on every level. With perLevel,
// levels has the source of each level of the upstream router, from level 1. A nil source is nothing. anything: Sode
// does not compare the tie line (Retry of a hop that failed, tieretry.go).
type tieWant struct {
	one      *tieplan.End
	levels   []*tieplan.End
	perLevel bool
	anything bool
}

func (a *App) tieWantOf(x apiExpect) (tieWant, *apiProblem) {
	w := tieWant{perLevel: x.PerLevel}
	end := func(e *apiEnd) (*tieplan.End, *apiProblem) {
		if e == nil {
			return nil, nil
		}
		te, p := a.tieEnd(*e)
		return &te, p
	}
	var p *apiProblem
	if w.one, p = end(x.One); p != nil {
		return w, p
	}
	for _, e := range x.Levels {
		te, p := end(e)
		if p != nil {
			return w, p
		}
		w.levels = append(w.levels, te)
	}
	return w, nil
}

// carries: the tie line with upstream destination from carries w on each level. The stale check compares every
// level, so a move on any level is a change. A level whose Via chain is unknown or a loop never matches. A mixed
// chain never matches one source. A tie line downstream of a mixed tie line has a mixed chain too. For each level,
// the 409 body gives the origin of that chain (apiTieChain.Levels). A per-level w compares that origin.
func carries(in tieplan.Input, from tieplan.End, levels []int, w tieWant) bool {
	if w.anything {
		return true
	}
	if w.perLevel && len(w.levels) != len(levels) {
		return false
	}
	for i, l := range levels {
		want := w.one
		if w.perLevel {
			want = w.levels[i]
		}
		c := tieplan.Via(in, from, l)
		switch {
		case c.Unknown != "" || c.Loop || c.Mixed && !w.perLevel:
			return false
		case want == nil && !c.Nothing:
			return false
		case want != nil && (c.Origin == nil || *c.Origin != *want):
			return false
		}
	}
	return true
}

// tieChanged is the stale check. Each tie line of t must carry the source that the caller expects. The path must
// have no Affected destination that the caller did not see. An Affected destination that the caller's hidden
// visibility leaves out is not compared, as the caller cannot name it. Routes go in order, with the tie lines of
// the routes before them Taken, as the planner showed them (planTieRoutes).
func (a *App) tieChanged(rt *runtime, v apiView, t *tieTake) *apiProblem {
	a.mu.Lock()
	defer a.mu.Unlock()
	var routes []tieRoute
	for _, r := range t.routes {
		routes = append(routes, r.in)
	}
	in := withOneOffs(a.tieInput(rt), nil, routes)
	in.Taken = map[string]tieplan.End{}
	body := apiTieChangedBody{Routes: []apiTieChangedRoute{}}
	changed := false
	for _, r := range t.routes {
		if r.mark == nil {
			continue
		}
		ids := make([]string, len(r.mark.hops))
		for i, h := range r.mark.hops {
			ids[i] = h.Tie
		}
		res := tieplan.Result{Paths: []tieplan.Path{}, Reasons: []tieplan.Reason{}}
		out := apiTieChangedRoute{Changed: []apiTieRef{}, NewAffected: []apiTieEndOut{}}
		path, why := tieplan.Through(in, tieplan.Route{Src: r.in.src, Dst: r.in.dst, Levels: r.in.levels}, ids)
		if why != nil {
			// A router of the path went down after the plan.
			changed = true
			res.Reasons = append(res.Reasons, *why)
		} else {
			res.Paths = append(res.Paths, path)
			for _, h := range path.Hops[:len(path.Hops)-1] {
				if h.Sends {
					in.Taken[h.Tie] = r.in.src
				}
				if !carries(in, tieplan.End{Router: h.Router, N: h.Dst}, h.Levels, r.in.expect[h.Tie]) {
					out.Changed = append(out.Changed, tieRef(h.Tie, t.tieName))
				}
			}
			for _, e := range path.Affected {
				if r.in.seen != nil && v.on(e.Router).Destination(e.N) && !slices.Contains(r.in.seen, e) {
					out.NewAffected = append(out.NewAffected, apiTieEndOut{Router: e.Router, N: e.N})
				}
			}
		}
		out.apiTieRouteOut = a.apiTieRoute(rt, v, r.in, res)
		for i, e := range out.NewAffected {
			if u, ok := a.unit(e.Router); ok {
				out.NewAffected[i].Name = u.router.name(true, e.N)
			}
		}
		changed = changed || len(out.Changed) > 0 || len(out.NewAffected) > 0
		body.Routes = append(body.Routes, out)
	}
	if !changed {
		return nil
	}
	msg := "a tie line changed after the caller read it; routes gives each tie line as it is now. Nothing was sent"
	body.Error = apiErr{Code: "tie_line_changed", Message: msg}
	return &apiProblem{Status: http.StatusConflict, Code: "tie_line_changed", Msg: msg, body: body}
}
