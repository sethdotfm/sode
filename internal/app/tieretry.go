package app

import (
	"net/http"
	"slices"

	"github.com/sethdotfm/sode/internal/tieplan"
)

// A Late hop and Retry of a tie take (docs/tielines-design.md, "Order and failures").
//
// A hop that the router answers late failed when its step gave up, so the hops after it were not sent and are not
// sent later. lateConfirm amends the hop entry as for any take. lateTieHop then marks the hop Late in each entry of the
// same Group that keeps it, so the row says "TIE 3 changed late" and offers the put-back.
//
// Retry of the entry of the destination asked for plans the same tie lines again. Each hop that held must still
// carry the typed source: the stale check compares it. A hop that failed is not compared. A hop whose tie line
// carries the source is no step (tieplan: Sends false), so Sode sends only the failed hops and the hops after them.
// If a hop no longer holds, the answer is 409 tie_line_changed with the tie lines as they are now: the caller opens
// the picker. Each step links to the entry that it retries.

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

// planTieRetry is the Retry of e, the entry of the destination asked for in a tie take.
func (a *App) planTieRetry(rt *runtime, cl *apiCtx, e Entry) (*takePlan, *apiProblem) {
	if e.OK {
		return nil, problem(http.StatusConflict, "conflict", "entry %d succeeded; redo repeats it, retry is for failed takes", e.ID)
	}
	src := tieplan.End{Router: e.Origin.Router, N: e.Origin.N}
	r := tieRoute{src: src, dst: tieplan.End{Router: e.Router, N: e.Dst}, levels: distinct(e.Levels), expect: map[string]tieWant{}}
	var ids []string
	for _, h := range e.Hops {
		ids = append(ids, h.Tie)
		w := tieWant{one: &src}
		if h.Result == "failed" {
			w = tieWant{anything: true}
		}
		r.expect[h.Tie] = w
	}
	if prob := a.tieRouteRights(rt, cl.Rights, []tieRoute{r}); prob != nil {
		return nil, prob
	}
	t, prob := a.planTieTake(rt, a.view(cl), []tieRoute{r}, map[int][]string{0: ids})
	if prob != nil {
		return nil, prob
	}
	for _, s := range t.steps {
		m := s.req.Tie
		switch {
		case m == nil:
		case m.origin != nil:
			id := e.ID
			s.req.Retry = &id
		default:
			if i := slices.IndexFunc(e.Hops, func(h TieHop) bool { return h.Tie == m.tie && h.Entry != nil }); i >= 0 {
				s.req.Retry = e.Hops[i].Entry
			}
		}
	}
	return &takePlan{Ties: t}, nil
}
