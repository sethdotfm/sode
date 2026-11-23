package app

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/sethdotfm/sode/internal/tieplan"
)

// Tie takes (docs/tielines-design.md, "Sending a tie take"). A take with a route across routers runs as steps. A step
// is one route on one router: a hop of a tie route, or a whole route on one router. Each step goes through runParts,
// so the check before take, the Protect guard, the waiter and Late answers work as for any route. Each step records
// its own History entry, and the whole take is one Group.
//
// The steps run in waves, upstream first: a step starts after the step before it on each route that needs it. A hop
// that two routes share (same router, source and destination) is one step, sent one time. If a step fails, the
// steps after it are not sent; each one records a failed entry, so the Group keeps its size. A hop whose tie line
// already carries the source (tieplan: Sends false) is not sent and has no entry.
//
// tielock.go locks the tie lines and runs the stale check before the first wave. Not here yet: the Protect check on
// every hop before sending. Also the Result
// across hops, put-back, One-off tie lines and the Tie line picker. The API is the only caller.

// TieEnd is a router reference with the name that Sode had for it.
type TieEnd struct {
	Router string `json:"router"`
	N      int    `json:"n"`
	Name   string `json:"name,omitempty"`
}

// TieHop is a hop before the last, as the entry of the destination asked for keeps it. Entry is the hop's own
// entry; there is none when the tie line already carried the source. Result is connected, already or failed. Prev is
// the Prev of the hop's entry: what the tie line carried before.
type TieHop struct {
	Router  string   `json:"router"`
	Src     int      `json:"src"`
	Dst     int      `json:"dst"`
	Levels  []int    `json:"levels"`
	Tie     string   `json:"tie"`
	TieName string   `json:"tieName"`
	Entry   *int     `json:"entry,omitempty"`
	Result  string   `json:"result"`
	Prev    [][2]int `json:"prev"`
}

// ViaLevel is the Via chain of one typed level of the destination asked for.
type ViaLevel struct {
	Level int `json:"level"`
	tieplan.Chain
}

// tieMark is what a step adds to its entry (planPhase). A hop before the last has tie only.
type tieMark struct {
	tie     string
	origin  *TieEnd
	hops    []TieHop
	viaPrev []ViaLevel
}

type tieStep struct {
	req   routeReq
	wave  int
	after []*tieStep // the steps that must succeed first
	res   routeResult
	got   bool // res holds the step's answer
}

func (s *tieStep) ok() bool { return s.got && s.res.OK }

// tieTakeRoute is one route of the request.
type tieTakeRoute struct {
	in    tieRoute
	steps []*tieStep // upstream first; the last is the destination asked for
	mark  *tieMark   // nil for a route on one router
	hops  []*tieStep // the step of each hop in mark.hops, nil for a hop that is not sent
}

type tieTake struct {
	routes  []*tieTakeRoute
	steps   []*tieStep
	waves   [][]*tieStep
	tieName map[string]string
}

type hopKey struct {
	router   string
	src, dst int
}

func (t *tieTake) step(req routeReq) *tieStep {
	req.ID = len(t.steps) + 1
	s := &tieStep{req: req}
	t.steps = append(t.steps, s)
	return s
}

// planTieTake makes the steps of a take with a route across routers. routes holds every route of the request, in
// order. ties holds the tie line ids of each route across routers, by its place in the request.
func (a *App) planTieTake(rt *runtime, v apiView, routes []tieRoute, ties map[int][]string) (*tieTake, *apiProblem) {
	t := &tieTake{tieName: map[string]string{}}
	for _, tl := range rt.cfg.TieLines {
		t.tieName[tl.ID] = tl.Name
	}
	var cross []tieRoute
	down := false
	if prob := func() *apiProblem {
		a.mu.Lock()
		defer a.mu.Unlock()
		in := a.tieInput(rt)
		shared := map[hopKey]*tieStep{}
		carries := map[string]tieplan.End{} // tie line id → the source that this take puts on it
		for _, r := range routes {
			tr := &tieTakeRoute{in: r}
			t.routes = append(t.routes, tr)
			if r.src.Router == r.dst.Router {
				tr.steps = []*tieStep{t.step(routeReq{Src: r.src.N, Dst: r.dst.N, Levels: r.levels, Router: r.dst.Router})}
				continue
			}
			cross = append(cross, r)
			// Typed levels are levels of the destination's router (docs/tielines-design.md, Levels). Refuse a bad
			// one here, before any hop goes out.
			if _, err := rt.levelsOf(r.dst.Router).typed(r.levels); err != nil {
				return problem(http.StatusBadRequest, "bad_request", "route %d: %v", r.index, err)
			}
			path, why := tieplan.Through(in, tieplan.Route{Src: r.src, Dst: r.dst, Levels: r.levels}, ties[r.index])
			if why != nil {
				if why.Why == "router_down" {
					down = true
					continue
				}
				return problem(http.StatusBadRequest, "bad_request", "route %d: %s", r.index, t.whyText(why))
			}
			u, _ := a.unit(r.src.Router)
			m := &tieMark{origin: &TieEnd{Router: r.src.Router, N: r.src.N, Name: u.router.srcNames[r.src.N]}}
			for _, l := range r.levels {
				m.viaPrev = append(m.viaPrev, ViaLevel{l, tieplan.Via(in, r.dst, l)})
			}
			last := len(path.Hops) - 1
			for _, h := range path.Hops[:last] {
				if s, ok := carries[h.Tie]; ok && s != r.src {
					return problem(http.StatusConflict, "conflict", "route %d: %s carries another source in this take", r.index, t.name(h.Tie))
				}
				carries[h.Tie] = r.src
				m.hops = append(m.hops, TieHop{Router: h.Router, Src: h.Src, Dst: h.Dst, Levels: h.Levels, Tie: h.Tie, TieName: t.tieName[h.Tie], Result: "already", Prev: [][2]int{}})
				var s *tieStep
				if h.Sends {
					k := hopKey{h.Router, h.Src, h.Dst}
					if s = shared[k]; s == nil {
						s = t.step(routeReq{Src: h.Src, Dst: h.Dst, Levels: h.Levels, Router: h.Router, Tie: &tieMark{tie: h.Tie}})
						shared[k] = s
					}
					tr.steps = append(tr.steps, s)
				}
				tr.hops = append(tr.hops, s)
			}
			h := path.Hops[last]
			tr.mark = m
			tr.steps = append(tr.steps, t.step(routeReq{Src: h.Src, Dst: h.Dst, Levels: h.Levels, Router: h.Router, Tie: m}))
		}
		return nil
	}(); prob != nil {
		return nil, prob
	}
	if down {
		// The planner leaves out each path through a router that is down: the answer lists the paths that remain.
		return nil, a.apiTieProblem(rt, v, cross)
	}
	t.order()
	return t, nil
}

func (t *tieTake) name(id string) string {
	if n := t.tieName[id]; n != "" {
		return n
	}
	return "tie line " + strconv.Quote(id)
}

func (t *tieTake) whyText(why *tieplan.Reason) string {
	switch {
	case why.Why == "unknown_tie":
		return fmt.Sprintf("Sode has no tie line with id %q; GET %s/tielines lists them", why.Tie, apiBase)
	case why.Tie != "":
		return fmt.Sprintf("%s does not start on the router where the tie lines before it end, or goes back to a router", t.name(why.Tie))
	}
	return fmt.Sprintf("the tie lines end on router %s, not on the router of the destination", why.From)
}

// order puts each step in the first wave after every step before it, on each route that needs it.
func (t *tieTake) order() {
	for range t.steps {
		moved := false
		for _, r := range t.routes {
			for i := 1; i < len(r.steps); i++ {
				if w := r.steps[i-1].wave + 1; w > r.steps[i].wave {
					r.steps[i].wave, moved = w, true
				}
			}
		}
		if !moved {
			break
		}
	}
	for _, r := range t.routes {
		for i := 1; i < len(r.steps); i++ {
			if s := r.steps[i]; !slices.Contains(s.after, r.steps[i-1]) {
				s.after = append(s.after, r.steps[i-1])
			}
		}
	}
	for _, s := range t.steps {
		for len(t.waves) <= s.wave {
			t.waves = append(t.waves, nil)
		}
		t.waves[s.wave] = append(t.waves[s.wave], s)
	}
}

// deadline is the worst case of every wave, one after the other.
func (t *tieTake) deadline(a *App) time.Duration {
	var d time.Duration
	for _, w := range t.waves {
		d += a.takeDeadline(len(w))
	}
	return d
}

// runTieTake sends the steps wave by wave. got gives the answer of a step by its ID. unread: the check before take
// did not finish for some step.
func (a *App) runTieTake(c *client, t *tieTake, got func(id int) (routeResult, bool)) (unread bool) {
	r := a.r()
	if len(t.steps) > 1 {
		g := strconv.FormatInt(time.Now().UnixNano(), 36)
		for _, s := range t.steps {
			s.req.Group, s.req.GroupN = g, len(t.steps)
		}
	}
	for _, wave := range t.waves {
		var items []routeReq
		for _, s := range wave {
			if s.req.Tie != nil && s.req.Tie.origin != nil {
				t.fillHops(a, s)
			}
			if i := slices.IndexFunc(s.after, func(x *tieStep) bool { return !x.ok() }); i >= 0 {
				a.notSent(r, c, s.req, "not sent: the hop to "+t.name(s.after[i].req.Tie.tie)+" failed")
				continue
			}
			items = append(items, s.req)
		}
		if len(items) > 0 && a.runParts(r, c, items) {
			unread = true
		}
		for _, s := range wave {
			s.res, s.got = got(s.req.ID)
		}
	}
	return unread
}

// fillHops writes the answer of each hop before the last into the mark of the destination step s, before s runs.
func (t *tieTake) fillHops(a *App, s *tieStep) {
	i := slices.IndexFunc(t.routes, func(r *tieTakeRoute) bool { return r.mark == s.req.Tie })
	r := t.routes[i]
	for j, hs := range r.hops {
		if hs == nil {
			continue
		}
		h := &r.mark.hops[j]
		h.Result, h.Entry = resultWord(hs.res, hs.got, hs.req.Levels), hs.res.Entry
		if h.Entry != nil {
			if e, ok := a.hist.Get(*h.Entry); ok {
				h.Prev = e.Prev
			}
		}
	}
}

// notSent records a failed entry for a step that Sode did not send.
func (a *App) notSent(r *runtime, c *client, req routeReq, why string) {
	res := &routeResult{T: "takeResult", ID: req.ID}
	u, ok := a.unit(req.Router)
	if !ok {
		res.Err = "unknown router"
		a.reply(c, res)
		return
	}
	w, e, err := a.planPhase(r, u, c, req)
	if err != nil {
		res.Err = err.Error()
		a.reply(c, res)
		return
	}
	e.Failed = slices.Sorted(slices.Values(e.Levels))
	e.Unsent = slices.Clone(e.Failed)
	a.mu.Lock()
	e.SrcName, e.DstName = u.router.srcNames[req.Src], u.router.dstNames[req.Dst]
	cachePrev(w, e)
	a.mu.Unlock()
	a.finish(c, res, e, errors.New(why))
}

// out is the answer: one result for each route of the request, from the step of the destination asked for.
func (t *tieTake) out(unread bool) apiTakeOut {
	out := apiTakeOut{OK: true, CheckSkipped: unread, Results: make([]apiRouteOut, len(t.routes))}
	for i, r := range t.routes {
		s := r.steps[len(r.steps)-1]
		o := apiRouteOut{ID: i + 1, Src: r.in.src.N, Dst: r.in.dst.N, Levels: r.in.levels, OK: s.ok(), Already: s.res.Already, Error: s.res.Err,
			Entry: s.res.Entry, Result: resultWord(s.res, s.got, s.req.Levels)}
		if !s.got {
			o.Error = "no result"
		}
		if r.mark != nil {
			o.Hops = r.mark.hops
		}
		out.OK = out.OK && o.OK
		out.Results[i] = o
	}
	return out
}

// resultWord is the Result of one route: connected, already or failed.
func resultWord(rr routeResult, got bool, levels []int) string {
	switch {
	case !got || !rr.OK:
		return "failed"
	case len(rr.Already) > 0 && len(rr.Already) >= len(distinct(levels)):
		return "already"
	}
	return "connected"
}
