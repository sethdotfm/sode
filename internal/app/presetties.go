package app

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// Presets with tie lines (docs/tielines-design.md, Presets). A preset item across routers stores the tie lines that
// the user chose when the item was added. A fire never chooses again by itself. It never takes a tie line on air with
// no look. Thus POST /presets/{scope}/{name} answers 409 tie_confirm_needed with the state of each stored tie line.
// It does so until the request names each one with the source that the caller expects on it. The page fires a preset
// through its tie confirm page (web/tieflow.js). That page sends the routes with their ties as a tieTake with the
// preset's name.

// apiPresetIn is the body of POST /presets/{scope}/{name}. It can be empty. Ties names each stored tie line of the
// preset with what the caller expects on it, as in the ties of POST /takes.
type apiPresetIn struct {
	Ties []apiTieIn `json:"ties"`
}

// presetTake plans a preset fire. A preset with no route across routers is planPreset's plan.
func (a *App) presetTake(rt *runtime, cl *apiCtx, body []byte, p config.Preset) (*takePlan, *apiProblem) {
	plan, prob := planPreset(a, rt, p)
	if prob != nil {
		return nil, prob
	}
	var in apiPresetIn
	if len(bytes.TrimSpace(body)) > 0 {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return nil, problem(http.StatusBadRequest, "bad_request", `send no body, or {"ties":[{"id":"…","expect":…}]}: %v`, err)
		}
	}
	if len(plan.TieRoutes) == 0 {
		if len(in.Ties) > 0 {
			return nil, problem(http.StatusBadRequest, "bad_request", "preset %q has no route across routers, so it takes no ties", p.Name)
		}
		return plan, nil
	}
	if !cl.Rights.AllowsPreset(p.ID) {
		if cl.Rights.Hidden() {
			return nil, problem(http.StatusForbidden, "outside_rights", "preset is outside your rights")
		}
		return nil, problem(http.StatusForbidden, "outside_rights", "preset %q is outside your rights", plan.Preset)
	}
	all := make([]tieRoute, 0, len(plan.Items)+len(plan.TieRoutes))
	for _, it := range plan.Items {
		all = append(all, tieRoute{index: len(all), src: tieplan.End{Router: it.Router, N: it.Src}, dst: tieplan.End{Router: it.Router, N: it.Dst}, levels: it.Levels})
	}
	for _, r := range plan.TieRoutes {
		r.index = len(all)
		all = append(all, r)
	}
	if prob := a.tieRouteRights(rt, cl.Rights, all); prob != nil {
		return nil, prob
	}
	pt, prob := a.panelTieRule(rt, cl, all)
	if prob != nil {
		return nil, prob
	}
	stored := map[string]bool{}
	for _, r := range plan.TieRoutes {
		for _, id := range r.ties {
			stored[id] = true
		}
	}
	expect := map[string]tieWant{}
	for _, t := range in.Ties {
		switch {
		case t.ID == "" || t.From != nil || t.To != nil || t.Protect:
			return nil, problem(http.StatusBadRequest, "bad_request", `each tie line of a preset fire is {"id", "expect"}: the preset stores its tie lines`)
		case !stored[t.ID]:
			return nil, problem(http.StatusBadRequest, "bad_request", "preset %q does not store tie line %q", p.Name, t.ID)
		case !t.Expect.Given:
			continue // not named: the confirm is still needed
		}
		w, prob := a.tieWantOf(t.Expect)
		if prob != nil {
			return nil, prob
		}
		expect[t.ID] = w
	}
	if len(expect) < len(stored) {
		return nil, a.presetTieProblem(rt, a.view(cl), all, pt)
	}
	ties := map[int][]string{}
	for i := range all {
		r := &all[i]
		if len(r.ties) == 0 {
			continue
		}
		ties[r.index] = r.ties
		r.expect = map[string]tieWant{}
		for _, id := range r.ties {
			r.expect[id] = expect[id]
		}
	}
	tt, prob := a.planTieTake(rt, a.view(cl), all, ties)
	if prob != nil {
		return nil, prob
	}
	return &takePlan{Ties: tt, Preset: plan.Preset, PresetID: plan.PresetID, LeftOut: plan.LeftOut}, nil
}

// presetTieProblem is the 409 tie_confirm_needed of a preset fire. It gives each route across routers with the path
// through its stored tie lines as Sode has it now. A hop gives what its tie line carries now, whether the take changes it, the
// last change, and a Protect or Reserve. A route whose stored tie lines make no path now has no path and the reason.
func (a *App) presetTieProblem(rt *runtime, v apiView, routes []tieRoute, pt *panelTies) *apiProblem {
	reserves := reserveNames(a.tieReserves())
	a.mu.Lock()
	defer a.mu.Unlock()
	in := a.tieInput(rt)
	in.Reserves = reserves
	if pt != nil {
		in.Reserves = maps.Clone(reserves)
		maps.DeleteFunc(in.Reserves, func(id, _ string) bool { return pt.only[id] })
	}
	body := apiTieBody{Routes: []apiTieRouteOut{}}
	for _, r := range routes {
		if r.src.Router == r.dst.Router {
			continue
		}
		var res tieplan.Result
		if p, why := tieplan.Through(in, tieplan.Route{Src: r.src, Dst: r.dst, Levels: r.levels}, r.ties); why != nil {
			res.Reasons = []tieplan.Reason{*why}
		} else {
			res.Paths = []tieplan.Path{p}
		}
		body.Routes = append(body.Routes, a.apiTieRoute(rt, v, r, res))
	}
	msg := fmt.Sprintf("this preset sends through %s. Look at each one in routes. Then send the preset again with ties: one {id, expect} for each tie line", tieCount(routes))
	body.Error = apiErr{Code: "tie_confirm_needed", Message: msg}
	return &apiProblem{Status: http.StatusConflict, Code: "tie_confirm_needed", Msg: msg, body: body}
}

func tieCount(routes []tieRoute) string {
	var ids []string
	for _, r := range routes {
		for _, id := range r.ties {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
		}
	}
	return fmt.Sprintf("%d %s", len(ids), plural(len(ids), "tie line", "tie lines"))
}
