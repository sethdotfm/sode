package app

import (
	"net/http"
	"sort"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// Router state reads: names, tally, protected destinations, status, panel layouts.

func init() {
	registerAPI(
		apiRoute{"GET", apiBase + "/names", config.ScopeNames, (*App).apiNames},
		apiRoute{"GET", apiBase + "/tally", config.ScopeTally, (*App).apiTally},
		apiRoute{"GET", apiBase + "/protected", config.ScopeProtected, (*App).apiProtected},
		apiRoute{"GET", apiBase + "/status", config.ScopeStatus, (*App).apiStatus},
		apiRoute{"GET", apiBase + "/layouts", config.ScopeLayouts, (*App).apiLayouts},
	)
}

type apiName struct {
	N    int    `json:"n"`
	Name string `json:"name"`
}

type apiNames struct {
	NumberOffset int       `json:"numberOffset"`
	Sources      []apiName `json:"sources"`
	Destinations []apiName `json:"destinations"`
	NamesAt      time.Time `json:"namesAt"`
}

func namesOf(m map[int]string, ok func(int) bool) []apiName {
	out := []apiName{}
	for n, name := range m {
		if ok(n) {
			out = append(out, apiName{n, name})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].N < out[j].N })
	return out
}

// namesSnapshot is the names as v shows them. Caller holds mu.
func (a *App) namesSnapshot(v apiView) apiNames {
	return apiNames{
		NumberOffset: a.r().cfg.NumberOffset,
		Sources:      namesOf(a.router.srcNames, v.Source),
		Destinations: namesOf(a.router.dstNames, v.Destination),
		NamesAt:      a.router.namesAt,
	}
}

func (a *App) apiNames(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	a.mu.Lock()
	res := a.namesSnapshot(cl.view())
	a.mu.Unlock()
	apiJSON(w, res)
}

type apiRouteRow struct {
	Dst   int `json:"dst"`
	Level int `json:"level"` // UI level
	Src   int `json:"src"`   // -1 = nothing routed
}

// apiProtectedRow: router.protect maps dst and level to a device, which is not a source, so
// the row carries no value and visibility asks about the destination only.
type apiProtectedRow struct {
	Dst   int `json:"dst"`
	Level int `json:"level"` // UI level
}

// routeRows lists a dst to level to value map in a stable order, keeping what v shows.
func routeRows(m map[int]map[int]int, v apiView) []apiRouteRow {
	out := []apiRouteRow{}
	for d, t := range m {
		for l, s := range t {
			if v.Route(d, l, s) {
				out = append(out, apiRouteRow{d, l, s})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Dst != out[j].Dst {
			return out[i].Dst < out[j].Dst
		}
		return out[i].Level < out[j].Level
	})
	return out
}

type apiLevel struct {
	Level int    `json:"level"`
	Label string `json:"label"`
}

// tallySnapshot is the levels and routes as v shows them. Caller holds mu.
func (a *App) tallySnapshot(v apiView) map[string]any {
	levels := []apiLevel{}
	for _, l := range a.r().levels {
		levels = append(levels, apiLevel{l.UI, l.Label})
	}
	return map[string]any{"levels": levels, "routes": routeRows(a.router.tally, v)}
}

func (a *App) apiTally(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	a.mu.Lock()
	res := a.tallySnapshot(cl.view())
	a.mu.Unlock()
	apiJSON(w, res)
}

func (a *App) apiProtected(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	a.mu.Lock()
	v := cl.view()
	rows := []apiProtectedRow{}
	for d, t := range a.router.protect {
		if !v.Destination(d) {
			continue
		}
		for l := range t {
			rows = append(rows, apiProtectedRow{d, l})
		}
	}
	a.mu.Unlock()
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Dst != rows[j].Dst {
			return rows[i].Dst < rows[j].Dst
		}
		return rows[i].Level < rows[j].Level
	})
	apiJSON(w, map[string]any{"protected": rows})
}

// linkStatus is the Router link as any caller with the status scope sees it. The router
// address and the error text (which quotes it) go only to a caller with the log scope, the
// same rule as the Communication Log's raw router rows.
func linkStatus(s statusMsg, withDetail bool) map[string]any {
	m := map[string]any{"router": s.Router, "routerUp": s.Router == string(swp08.StateUp), "error": s.Err != ""}
	if withDetail {
		m["address"], m["err"] = s.Addr, s.Err
	}
	return m
}

// apiStatus: the Server link and the Router link, as /api/health reports them.
func (a *App) apiStatus(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	a.mu.Lock()
	st := a.status
	a.mu.Unlock()
	m := linkStatus(st, cl.has(config.ScopeLog))
	m["server"], m["version"], m["uptimeSeconds"] = "up", VersionLabel(), int64(time.Since(a.started).Seconds())
	apiJSON(w, m)
}

// apiLayouts: the content is not defined yet; the endpoint exists so clients can ask.
func (a *App) apiLayouts(w http.ResponseWriter, _ *http.Request, _ *apiCtx) {
	apiJSON(w, map[string]any{"layouts": []any{}})
}
