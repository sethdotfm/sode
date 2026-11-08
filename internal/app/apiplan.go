package app

import (
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// The server-side planner: what the browser builds in web/plans.js (undoItem, againItem,
// resolvePreset) and sends as plain routes, built here from a History entry or a preset.
// Keep the rules in step with plans.js.

// takePlan is routes ready for a.take plus what the plan left out, for the response.
type takePlan struct {
	Items     []routeReq
	Preset    string
	LeftOut   []string // preset parts the router no longer has
	Skipped   []int    // undo: levels with nothing routed before, left as is
	Untouched []int    // undo: levels a failed take never changed, left as is
}

// apiProblem is every API error: a status, a code listed in openapi.yaml and a message.
type apiProblem struct {
	Status    int
	Code, Msg string
}

func problem(status int, code, format string, args ...any) *apiProblem {
	return &apiProblem{status, code, fmt.Sprintf(format, args...)}
}

func (p *apiProblem) answer() apiAnswer {
	return jsonAnswer(p.Status, apiErrBody{apiErr{p.Code, p.Msg}})
}

func (p *apiProblem) write(w http.ResponseWriter) { p.answer().write(w, false) }

// Get returns one entry by id.
func (h *History) Get(id int) (Entry, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if id < 0 || (h.f == nil && id >= len(h.mem)) || (h.f != nil && id >= len(h.offs)) {
		return Entry{}, false
	}
	e, err := h.at(id)
	if err != nil {
		return Entry{}, false
	}
	e.ID = id
	return e, true
}

// planUndo restores Prev. Levels the take did not change (a failed take the router answered
// for, unless the answer came late) and levels with nothing routed or no answer before stay
// as they are. Rights skip the Before source (rights.go); visibility drops routes the
// caller may not see.
func planUndo(r *runtime, v apiView, e Entry) *takePlan {
	untouched := map[int]bool{}
	if !e.OK {
		for _, p := range e.Now {
			if !slices.Contains(e.Late, p[0]) {
				untouched[p[0]] = true
			}
		}
	}
	p := &takePlan{}
	groups := map[int][]int{}
	known := map[int]bool{}
	for _, pv := range e.Prev {
		u, s := pv[0], pv[1]
		known[u] = true
		if _, ok := r.byUI[u]; !ok || untouched[u] {
			continue
		}
		if s < 0 {
			p.Skipped = append(p.Skipped, u)
			continue
		}
		if !v.Route(e.Dst, u, s) {
			continue
		}
		groups[s] = append(groups[s], u)
	}
	for _, u := range e.Levels {
		if !known[u] && !untouched[u] {
			p.Skipped = append(p.Skipped, u)
		}
	}
	for u := range untouched {
		if _, ok := r.byUI[u]; ok {
			p.Untouched = append(p.Untouched, u)
		}
	}
	sort.Ints(p.Skipped)
	sort.Ints(p.Untouched)
	srcs := make([]int, 0, len(groups))
	for s := range groups {
		srcs = append(srcs, s)
	}
	sort.Ints(srcs)
	id := e.ID
	for _, s := range srcs {
		ls := groups[s]
		sort.Ints(ls)
		p.Items = append(p.Items, routeReq{Src: s, Dst: e.Dst, Levels: ls, Undo: &id})
	}
	return p
}

// planAgain repeats an entry's take: a Retry when it failed (linked to the entry), else a
// Redo sent as a plain new take. Needs a real source and a level that still exists.
func planAgain(r *runtime, v apiView, e Entry, retry bool) (*takePlan, *apiProblem) {
	switch {
	case retry && e.OK:
		return nil, problem(http.StatusConflict, "conflict", "entry %d succeeded; redo repeats it, retry is for failed takes", e.ID)
	case !retry && !e.OK:
		return nil, problem(http.StatusConflict, "conflict", "entry %d failed; use retry to take it again", e.ID)
	case e.Src < 0:
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "entry %d has no source to repeat", e.ID)
	}
	var ls []int
	for _, u := range e.Levels {
		if _, ok := r.byUI[u]; ok && v.Route(e.Dst, u, e.Src) {
			ls = append(ls, u)
		}
	}
	if len(ls) == 0 {
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "none of entry %d's levels exist or are visible to you", e.ID)
	}
	sort.Ints(ls)
	req := routeReq{Src: e.Src, Dst: e.Dst, Levels: ls}
	if retry {
		id := e.ID
		req.Retry = &id
	}
	return &takePlan{Items: []routeReq{req}}, nil
}

// planPreset resolves a preset the way resolvePreset does: parts the router lacks are left
// out and listed, one route per source and destination with levels merged. Order follows the
// preset.
func planPreset(a *App, r *runtime, p config.Preset) (*takePlan, *apiProblem) {
	defaults, _ := config.ParseLevels(r.cfg.DefaultLevels, r.cfg.Levels)
	a.mu.Lock()
	hasSrc := func(n int) bool { _, ok := a.router.srcNames[n]; return ok }
	hasDst := func(n int) bool { _, ok := a.router.dstNames[n]; return ok }
	type key struct{ dst, src int }
	var order []key
	levels := map[key][]int{}
	var left []string
	miss := func(s string) {
		if !slices.Contains(left, s) {
			left = append(left, s)
		}
	}
	off := r.cfg.NumberOffset
	for _, it := range p.Items {
		if !hasSrc(it.Src) {
			miss(fmt.Sprintf("source %d", it.Src+off))
			continue
		}
		want := it.Levels
		if len(want) == 0 {
			want = defaults
		}
		var have []int
		for _, u := range want {
			if _, ok := r.byUI[u]; ok {
				have = append(have, u)
			} else {
				miss("level " + r.cfg.LevelLabel(u))
			}
		}
		if len(have) == 0 {
			continue
		}
		for _, d := range it.Dsts {
			if !hasDst(d) {
				miss(fmt.Sprintf("destination %d", d+off))
				continue
			}
			k := key{d, it.Src}
			if _, ok := levels[k]; !ok {
				order = append(order, k)
			}
			levels[k] = append(levels[k], have...)
		}
	}
	a.mu.Unlock()
	plan := &takePlan{Preset: cleanPresetName(p.Name), LeftOut: left}
	for _, k := range order {
		ls := levels[k]
		sort.Ints(ls)
		plan.Items = append(plan.Items, routeReq{Src: k.src, Dst: k.dst, Levels: slices.Compact(ls)})
	}
	if len(plan.Items) == 0 {
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "nothing in preset %q exists on the router now: %s", p.Name, strings.Join(left, ", "))
	}
	return plan, nil
}
