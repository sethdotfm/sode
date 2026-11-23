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
func (a *App) upstreamTies(rt *runtime, items []routeReq) []string {
	if len(rt.cfg.TieLines) == 0 {
		return nil
	}
	at := map[tieplan.End]bool{}
	for _, it := range items {
		if u, ok := a.unit(a.itemRouter(it)); ok {
			at[tieplan.End{Router: u.uid, N: it.Dst}] = true
		}
	}
	var ids []string
	for _, t := range rt.cfg.TieLines {
		if uid, ok := a.unitUID(rt, t.From.Router); ok && at[tieplan.End{Router: uid, N: t.From.N}] {
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
// compares each tie line with what the caller saw. On a refusal it holds no lock and nothing went to a router.
func (a *App) tieTakeBegin(rt *runtime, v apiView, c *client, t *tieTake) (unlock func(), _ *apiProblem) {
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
		return nil, p
	}
	return unlock, nil
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

// sameSource: chain c carries expect (nil: nothing routed). Mixed levels count as the source of the lowest level,
// which is the origin that Sode shows.
func sameSource(c *tieplan.Chain, expect *tieplan.End) bool {
	if c == nil {
		return false
	}
	if expect == nil {
		return c.Nothing
	}
	return c.Origin != nil && *c.Origin == *expect
}

// tieChanged is the stale check. Each tie line of t must carry the source that the caller expects. The path must
// have no Affected destination that the caller did not see. An Affected destination that the caller's hidden
// visibility leaves out is not compared, as the caller cannot name it. Routes go in order, with the tie lines of
// the routes before them Taken, as the planner showed them (planTieRoutes).
func (a *App) tieChanged(rt *runtime, v apiView, t *tieTake) *apiProblem {
	a.mu.Lock()
	defer a.mu.Unlock()
	in := a.tieInput(rt)
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
				if !sameSource(h.Now, r.in.expect[h.Tie]) {
					out.Changed = append(out.Changed, apiTieRef{h.Tie, t.tieName[h.Tie]})
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
