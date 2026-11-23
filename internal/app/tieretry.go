package app

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"sync"
	"sync/atomic"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// A Late hop and Retry of a tie take (docs/tielines-design.md, "Order and failures").
//
// A hop that the router answers late failed when its step gave up. Thus the hops after it were not sent, and Sode
// does not send them later. lateConfirm amends the hop entry as for any take. lateTieHop then marks the hop Late in
// each entry of the same Group that keeps it, so the row says "TIE 3 changed late" and offers the put-back.
//
// Retry of the entry of the destination asked for plans the same tie lines again. Each hop that held must still
// carry the typed source: the stale check compares it. A hop that failed is not compared. A hop whose tie line
// carries the source is no step (tieplan: Sends false). Thus Sode sends only the failed hops and the hops after
// them. If a hop no longer holds, the answer is 409 tie_line_changed with the tie lines as they are now: the caller
// opens the picker. Each step links to the entry that it retries.
//
// A page sends its Retry as a take item that names the failed entry. For the entry of a tie take, pageTieRetry runs
// the same plan; the item is not sent as a plain route. A hop entry has no Retry of its own.

// lateHopScan is how many entries after a hop entry lateTieHop reads to find the entries of its Group.
const lateHopScan = 256

// lateTieHop marks the late hop entry hop in each later entry of its Group that keeps it as a hop. Caller does not
// hold mu.
func (a *App) lateTieHop(hop Entry) {
	if hop.Group == "" {
		return
	}
	for id := hop.ID + 1; id <= hop.ID+lateHopScan; id++ {
		d, ok := a.hist.Get(id)
		if !ok {
			return
		}
		if d.Group != hop.Group || !slices.ContainsFunc(d.Hops, func(h TieHop) bool { return h.Entry != nil && *h.Entry == hop.ID }) {
			continue
		}
		e, ok := a.hist.Update(id, func(e *Entry) {
			for i := range e.Hops {
				if h := &e.Hops[i]; h.Entry != nil && *h.Entry == hop.ID {
					h.lateFrom(hop)
				}
			}
		})
		if ok {
			a.mu.Lock()
			a.broadcast(recentMsg{T: "entry", E: &e, Update: true})
			a.mu.Unlock()
		}
	}
}

// tieRedoProblem is the answer to a Redo of e, the entry of the destination asked for in a tie take. Redo opens the
// picker (docs/tielines-design.md, "Redo and Retry"). Thus Sode sends nothing. The answer is 409 with the paths as
// they are now, as for a route with no ties. The hops of e name the tie lines that it used last time.
func (a *App) tieRedoProblem(rt *runtime, rights config.Rights, v apiView, e Entry) *apiProblem {
	r := tieRoute{src: tieplan.End{Router: e.Origin.Router, N: e.Origin.N}, dst: tieplan.End{Router: e.Router, N: e.Dst}, levels: distinct(e.Levels)}
	for _, h := range e.Hops {
		a.hopOneOffAgain(rt, h, &r)
	}
	if prob := a.tieRouteRights(rt, rights, []tieRoute{r}); prob != nil {
		return prob
	}
	p := a.apiTieProblem(rt, v, []tieRoute{r})
	if p.Code == "tie_line_needed" {
		b := p.body.(apiTieBody)
		p.Msg = fmt.Sprintf("entry %d is a tie take: redo opens the picker. Send the route again to POST %s/takes with ties, from the paths in routes, cheapest first", e.ID, apiBase)
		b.Error.Message = p.Msg
		p.body = b
	}
	return p
}

// planTieRetry is the Retry of e, the entry of the destination asked for in a tie take.
func (a *App) planTieRetry(rt *runtime, rights config.Rights, v apiView, e Entry) (*takePlan, *apiProblem) {
	if e.OK {
		return nil, problem(http.StatusConflict, "conflict", "entry %d succeeded; redo repeats it, retry is for failed takes", e.ID)
	}
	src := tieplan.End{Router: e.Origin.Router, N: e.Origin.N}
	r := tieRoute{src: src, dst: tieplan.End{Router: e.Router, N: e.Dst}, levels: distinct(e.Levels), expect: map[string]tieWant{}}
	var ids []string
	for _, h := range e.Hops {
		id := a.hopOneOffAgain(rt, h, &r)
		ids = append(ids, id)
		w := tieWant{one: &src}
		if h.Result == "failed" {
			w = tieWant{anything: true}
		}
		r.expect[id] = w
	}
	if prob := a.tieRouteRights(rt, rights, []tieRoute{r}); prob != nil {
		return nil, prob
	}
	t, prob := a.planTieTake(rt, v, []tieRoute{r}, map[int][]string{0: ids})
	if prob != nil {
		return nil, prob
	}
	// Each step links to the entry that it retries, also in the plan that tieTakeBegin makes again.
	link := func(t *tieTake) {
		for _, s := range t.steps {
			m := s.req.Tie
			switch {
			case m == nil:
			case m.origin != nil:
				id := e.ID
				s.req.Retry = &id
			default:
				if i := slices.IndexFunc(ids, func(id string) bool { return id == m.tie }); i >= 0 && e.Hops[i].Entry != nil {
					s.req.Retry = e.Hops[i].Entry
				}
			}
		}
	}
	link(t)
	again := t.again
	t.again = func() (*tieTake, *apiProblem) {
		t, prob := again()
		if prob == nil {
			link(t)
		}
		return t, prob
	}
	return &takePlan{Ties: t}, nil
}

// stepTaps sends the results of the steps of a tie take that a page started to that take, not to the page
// (pageTieRetry). The steps have IDs below zero, so they never meet the IDs of a page.
type stepTaps struct {
	seq atomic.Int64
	m   sync.Map // stepTap → *sinkResults
}

type stepTap struct {
	c  *client
	id int
}

// take gives rr to its tie take, if it is the result of a step that a page started.
func (s *stepTaps) take(c *client, rr *routeResult) bool {
	if rr.ID >= 0 {
		return false
	}
	v, ok := s.m.Load(stepTap{c, rr.ID})
	if !ok {
		return false
	}
	sink := v.(*sinkResults)
	sink.mu.Lock()
	sink.by[rr.ID] = *rr
	sink.mu.Unlock()
	return true
}

// tieRetries takes out of items each Retry of an entry of a tie take: a hop entry or the entry of the destination.
func (a *App) tieRetries(items []routeReq) (rest, ties []routeReq) {
	for _, it := range items {
		if it.Retry != nil {
			if e, ok := a.hist.Get(*it.Retry); ok && (e.Tie != "" || e.Origin != nil) {
				ties = append(ties, it)
				continue
			}
		}
		rest = append(rest, it)
	}
	return rest, ties
}

// pageTieRetry is a Retry from a page of the entry of a tie take. It plans, locks and sends as the API Retry does.
// The page gets one result for its item: the result of the route.
func (a *App) pageTieRetry(c *client, it routeReq) {
	act, rights := a.callerOf(c)
	fail := func(code, msg string) {
		a.apiEvent(act, "retry", "refused", code, errors.New(msg))
		a.reply(c, &routeResult{T: "takeResult", ID: it.ID, Err: msg})
	}
	rt, v := a.r(), a.viewOf(rights)
	e, ok := a.hist.Get(*it.Retry)
	if ok {
		e, ok = v.Entry(e)
	}
	switch {
	case !ok:
		fail("not_found", fmt.Sprintf("not sent: no History entry %d", *it.Retry))
		return
	case e.Tie != "":
		fail("conflict", fmt.Sprintf("not sent: entry %d is a hop of a tie take. Retry its destination entry", e.ID))
		return
	}
	plan, p := a.planTieRetry(rt, rights, v, e)
	var t *tieTake
	var unlock func()
	if p == nil {
		t, unlock, p = a.tieTakeBegin(rt, v, c, plan.Ties)
	}
	if p != nil {
		msg := "not sent: " + p.Msg
		if p.Code == "tie_line_changed" {
			msg = "not sent: a tie line changed after the take. Choose the path again"
		}
		fail(p.Code, msg)
		return
	}
	defer unlock()
	sink := &sinkResults{by: map[int]routeResult{}}
	for _, s := range t.steps {
		s.req.ID = -int(a.stepTaps.seq.Add(1))
		a.stepTaps.m.Store(stepTap{c, s.req.ID}, sink)
	}
	defer func() {
		for _, s := range t.steps {
			a.stepTaps.m.Delete(stepTap{c, s.req.ID})
		}
	}()
	r := t.out(a.runTieTake(c, t, sink.get)).Results[0]
	a.reply(c, &routeResult{T: "takeResult", ID: it.ID, OK: r.OK, Err: r.Error, Already: r.Already, Entry: r.Entry})
}
