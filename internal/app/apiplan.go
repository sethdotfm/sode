package app

import (
	"cmp"
	"fmt"
	"net/http"
	"slices"
	"sort"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// The server-side planner. Keep the rules in step with web/plans.js (undoItem, againItem, resolvePreset).

type takePlan struct {
	Items     []routeReq
	Preset    string
	PresetID  string
	LeftOut   []string // preset parts the router does not have
	Skipped   []int    // undo: levels with nothing routed before, left as is
	Untouched []int    // undo: levels a failed take never changed, left as is
	// Ties: a take with a route across routers runs as steps (tietake.go); Items is then empty.
	Ties *tieTake
	// TieRoutes: planPreset's routes across routers, each with its stored tie lines. presetTieTake makes Ties of
	// them.
	TieRoutes []tieRoute
}

// The code is one listed in openapi.yaml.
type apiProblem struct {
	Status    int
	Code, Msg string
	// body replaces the plain error body; it carries its own error object (the 409 bodies of tieroutes.go).
	body any
}

func problem(status int, code, format string, args ...any) *apiProblem {
	return &apiProblem{Status: status, Code: code, Msg: fmt.Sprintf(format, args...)}
}

func (p *apiProblem) answer() apiAnswer {
	if p.body != nil {
		return jsonAnswer(p.Status, p.body)
	}
	return jsonAnswer(p.Status, apiErrBody{apiErr{Code: p.Code, Message: p.Msg}})
}

func (p *apiProblem) write(w http.ResponseWriter) { p.answer().write(w, false) }

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

// Levels the take did not change stay: a failed take the router answered for, unless the answer came late.
// So do levels with nothing routed or no answer before. Rights skip the Before source (rights.go).
func planUndo(r *runtime, v apiView, e Entry) *takePlan {
	untouched := map[int]bool{}
	if !e.OK {
		for _, p := range e.Now {
			if !slices.Contains(e.Late, p[0]) {
				untouched[p[0]] = true
			}
		}
	}
	lv := r.levelsOf(e.Router)
	p := &takePlan{}
	groups := map[int][]int{}
	known := map[int]bool{}
	for _, pv := range e.Prev {
		u, s := pv[0], pv[1]
		known[u] = true
		if _, ok := lv.byUI[u]; !ok || untouched[u] {
			continue
		}
		if s < 0 {
			p.Skipped = append(p.Skipped, u)
			continue
		}
		if !v.on(e.Router).Route(e.Dst, u, s) {
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
		if _, ok := lv.byUI[u]; ok {
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
		p.Items = append(p.Items, routeReq{Src: s, Dst: e.Dst, Levels: ls, Undo: &id, Router: e.Router})
	}
	return p
}

// A Retry is linked to the failed entry; a Redo is a plain new take. Both need a real source and a level that still
// exists, and both go to the router of the entry.
func planAgain(r *runtime, v apiView, e Entry, retry bool) (*takePlan, *apiProblem) {
	switch {
	case retry && e.OK:
		return nil, problem(http.StatusConflict, "conflict", "entry %d succeeded; redo repeats it, retry is for failed takes", e.ID)
	case !retry && !e.OK:
		return nil, problem(http.StatusConflict, "conflict", "entry %d failed; use retry to take it again", e.ID)
	case e.Src < 0:
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "entry %d has no source to repeat", e.ID)
	}
	lv := r.levelsOf(e.Router)
	var ls []int
	for _, u := range e.Levels {
		if _, ok := lv.byUI[u]; ok && v.on(e.Router).Route(e.Dst, u, e.Src) {
			ls = append(ls, u)
		}
	}
	if len(ls) == 0 {
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "none of entry %d's levels exist or are visible to you", e.ID)
	}
	sort.Ints(ls)
	req := routeReq{Src: e.Src, Dst: e.Dst, Levels: ls, Router: e.Router}
	if retry {
		id := e.ID
		req.Retry = &id
	}
	return &takePlan{Items: []routeReq{req}}, nil
}

// Keep in step with resolvePreset: one route per source and destination with levels merged, in the preset's order.
// Parts the routers lack are left out and listed. A route across routers goes to TieRoutes with its stored tie lines
// (presetties.go). A tie line that Sode no longer has (a tie mark) leaves its route out.
func planPreset(a *App, r *runtime, p config.Preset) (*takePlan, *apiProblem) {
	type end struct {
		cfg, unit string // the router UID in the settings, and the UID of its unit
		n         int
	}
	type key struct{ src, dst end }
	var order []key
	levels := map[key][]int{}
	ties := map[key][]string{}
	var left []string
	miss := func(s string) {
		if !slices.Contains(left, s) {
			left = append(left, s)
		}
	}
	// "CAM 3 (source 13)": the stored name says what was meant, the number says which one is gone. With two or more
	// routers the number has its prefix.
	gone := func(side string, f config.Ref) string {
		rc := r.routerOf(f.Router)
		num := fmt.Sprint(f.N + rc.NumberOffset)
		if len(r.cfg.RouterUIDs()) > 1 && rc.Prefix != "" {
			num = rc.Prefix + "." + num
		}
		if name := strings.TrimSpace(f.Name); name != "" {
			return fmt.Sprintf("%s (%s %s)", name, side, num)
		}
		return fmt.Sprintf("%s %s", side, num)
	}
	a.mu.Lock()
	_, tieName := a.tieLinesNow(r)
	// A reference to a router that Sode does not have, or one the Broken check marked, is left out like a gone number.
	has := func(f config.Ref, kind string) (end, bool) {
		uid := cmp.Or(f.Router, r.cfg.DefaultRouter().UID)
		u, ok := a.unitOfRouter(r.cfg, uid)
		if !ok || hasMissing(p.Broken, kind, f) {
			return end{}, false
		}
		_, named := sideNames(kind, u.router.srcNames, u.router.dstNames)[f.N]
		return end{uid, u.uid(), f.N}, named
	}
	for _, it := range p.Items {
		src, ok := has(it.Src, config.MissingSource)
		if !ok {
			miss(gone("source", it.Src))
			continue
		}
		lost := slices.ContainsFunc(it.Ties, func(id string) bool {
			_, ok := tieName[id]
			return !ok || slices.ContainsFunc(p.Broken, func(m config.Missing) bool { return m.Kind == config.MissingTie && m.Tie == id })
		})
		if lost {
			miss(fmt.Sprintf("the route from %s: a tie line that Sode no longer has", gone("source", it.Src)))
			continue
		}
		for _, d := range it.Dsts {
			dst, ok := has(d, config.MissingDestination)
			if !ok {
				miss(gone("destination", d))
				continue
			}
			rc := r.routerOf(dst.cfg)
			want := it.Levels
			if len(want) == 0 {
				want, _ = config.ParseLevels(rc.DefaultLevels, rc.Levels)
			}
			lv := r.levelsOf(dst.cfg)
			var have []int
			for _, u := range want {
				if _, ok := lv.byUI[u]; ok {
					have = append(have, u)
				} else {
					miss("level " + lv.label(u))
				}
			}
			if len(have) == 0 {
				continue
			}
			k := key{src, dst}
			if _, ok := levels[k]; !ok {
				order = append(order, k)
			}
			levels[k] = append(levels[k], have...)
			if src.unit != dst.unit {
				ties[k] = it.Ties
			}
		}
	}
	a.mu.Unlock()
	plan := &takePlan{Preset: cleanPresetName(p.Name), PresetID: p.ID, LeftOut: left}
	for _, k := range order {
		ls := levels[k]
		sort.Ints(ls)
		ls = slices.Compact(ls)
		if k.src.unit != k.dst.unit {
			plan.TieRoutes = append(plan.TieRoutes, tieRoute{src: tieplan.End{Router: k.src.unit, N: k.src.n}, dst: tieplan.End{Router: k.dst.unit, N: k.dst.n},
				levels: ls, ties: slices.Clone(ties[k])})
			continue
		}
		plan.Items = append(plan.Items, routeReq{Src: k.src.n, Dst: k.dst.n, Levels: ls, Router: k.dst.unit})
	}
	if len(plan.Items) == 0 && len(plan.TieRoutes) == 0 {
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "nothing in preset %q exists on the routers now: %s", p.Name, strings.Join(left, ", "))
	}
	return plan, nil
}
