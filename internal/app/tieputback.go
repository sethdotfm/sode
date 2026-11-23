package app

import (
	"fmt"
	"net/http"
	"slices"
	"strconv"

	"github.com/sethdotfm/sode/internal/config"
)

// Put back (docs/tielines-design.md, "Order and failures"). A tie take stops at the first failed hop. A hop before
// it can have changed a tie line for a route that did not get through. Put back is the Undo of that hop's entry (an
// entry with Tie). Its route carries putBack. After the check before take, prepare runs putBackHeld. The tie line
// must still carry what the take set, on each level that the hop changed. Else Sode sends nothing. takeAllowed locks
// the tie line, since its destination is the upstream destination of the tie line.
//
// Hops through saved tie lines need no Rights (docs/tielines-design.md, Rights), so a put-back that matches its hop
// entry skips the Rights check (putBackMatches). The page sends it as a take item with undo and putBack; the API has
// POST /history/{id}/putback. The hop entry keeps its Prev, which Undo of a tie take (later) also reads.

func init() {
	registerAPI(apiRoute{"POST", apiBase + "/history/{id}/putback", config.ScopeTake + " " + config.ScopeHistory, (*App).apiPutBack})
}

// hopChanged gives the levels on which the hop entry e changed its tie line. These are its levels, less the levels
// that were already on its source. Levels that failed are out too, unless the router confirmed them late.
func hopChanged(e Entry) []int {
	var out []int
	for _, l := range distinct(e.Levels) {
		if slices.Contains(e.Already, l) || slices.Contains(e.Failed, l) && !slices.Contains(e.Late, l) {
			continue
		}
		out = append(out, l)
	}
	return out
}

// putBackMatches: it is the put-back of a hop entry and restores what that entry had before (undoMatches).
func (a *App) putBackMatches(it routeReq) bool {
	if !it.PutBack || !a.undoMatches(it) {
		return false
	}
	e, _ := a.hist.Get(*it.Undo)
	return e.Tie != ""
}

// putBackHeld is the guard of a put-back route w, after the check before take: why Sode does not send it, or "". On
// a refusal it fills e.Prev from the cache.
func (a *App) putBackHeld(r *runtime, w *waiter, e *Entry, req routeReq) string {
	if req.Undo == nil {
		return "not sent: put back needs the entry of a hop"
	}
	hop, ok := a.hist.Get(*req.Undo)
	if !ok || hop.Tie == "" {
		return fmt.Sprintf("not sent: entry %d is not a hop of a tie take", *req.Undo)
	}
	name := tieNameOf(r, hop.Tie)
	a.mu.Lock()
	defer a.mu.Unlock()
	rs := w.u.router
	moved := map[int]bool{}
	var levels []int
	for _, l := range hopChanged(hop) {
		if s, ok := rs.tally[hop.Dst][l]; !ok || s != hop.Src {
			moved[s] = true
			if !ok {
				moved[-1] = true
			}
			levels = append(levels, l)
		}
	}
	if len(levels) == 0 {
		return ""
	}
	cachePrev(w, e)
	srcName := func(n int) string {
		switch {
		case n < 0:
			return "nothing"
		case rs.srcNames[n] != "":
			return rs.srcNames[n]
		}
		return fmt.Sprintf("source %d", n+r.routerOf(w.u.uid).NumberOffset)
	}
	now := "other sources"
	if len(moved) == 1 {
		for s := range moved {
			now = srcName(s)
		}
	}
	return fmt.Sprintf("not sent: %s now carries %s, not %s, on levels %s", name, now, srcName(hop.Src), levelList(r.levelsOf(w.u.uid), levels))
}

// tieNameOf names a tie line by its id, from the settings.
func tieNameOf(r *runtime, id string) string {
	for _, t := range r.cfg.TieLines {
		if t.ID == id && t.Name != "" {
			return t.Name
		}
	}
	return "tie line " + strconv.Quote(id)
}

// planPutBack is the Undo of the hop entry e, on the levels that the hop changed.
func planPutBack(r *runtime, v apiView, e Entry) (*takePlan, *apiProblem) {
	if e.Tie == "" {
		return nil, problem(http.StatusConflict, "conflict", "entry %d is not a hop of a tie take; use undo", e.ID)
	}
	changed := hopChanged(e)
	p := planUndo(r, v, e)
	items := p.Items[:0]
	for _, it := range p.Items {
		it.Levels = slices.DeleteFunc(it.Levels, func(l int) bool { return !slices.Contains(changed, l) })
		if len(it.Levels) > 0 {
			it.PutBack = true
			items = append(items, it)
		}
	}
	p.Items = items
	if len(items) == 0 {
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "entry %d changed no level of %s that had a source before it", e.ID, tieNameOf(r, e.Tie))
	}
	return p, nil
}

func (a *App) apiPutBack(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	id, err := strconv.Atoi(r.PathValue("id"))
	a.apiTakeRun(w, r, cl, func([]byte) (*takePlan, *apiProblem) {
		if err != nil || id < 0 {
			return nil, problem(http.StatusBadRequest, "bad_request", "the entry id must be a whole number")
		}
		e, ok := a.hist.Get(id)
		if ok {
			e, ok = a.view(cl).Entry(e)
		}
		if !ok {
			return nil, problem(http.StatusNotFound, "not_found", "no History entry %d", id)
		}
		return planPutBack(a.r(), a.view(cl), e)
	})
}
