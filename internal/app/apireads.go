package app

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

func init() {
	registerAPI(
		apiRoute{"GET", apiBase + "/names", config.ScopeNames, (*App).apiNames},
		apiRoute{"GET", apiBase + "/tally", config.ScopeTally, (*App).apiTally},
		apiRoute{"GET", apiBase + "/protected", config.ScopeProtected, (*App).apiProtected},
		apiRoute{"GET", apiBase + "/status", config.ScopeStatus, (*App).apiStatus},
		apiRoute{"GET", apiBase + "/layouts", config.ScopeLayouts, (*App).apiLayouts},
		apiRoute{"GET", apiBase + "/routers", "", (*App).apiRouters},
	)
}

// The router parameter is the UID, never Prefix.
type apiRouter struct {
	UID    string `json:"uid"`
	Prefix string `json:"prefix"`
	Title  string `json:"title"`
}

func (a *App) apiRouters(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	c := a.r().cfg
	apiJSONTagged(w, r, map[string][]apiRouter{"routers": {{c.UID, c.Prefix, c.Title}}})
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

// Caller holds mu.
func (a *App) namesSnapshot(v apiView) apiNames {
	return apiNames{
		NumberOffset: a.r().cfg.NumberOffset,
		Sources:      namesOf(a.router.srcNames, v.Source),
		Destinations: namesOf(a.router.dstNames, v.Destination),
		NamesAt:      a.router.namesAt,
	}
}

// Built one time per change for every caller that is not hidden, as tallyBody is. A panel page asks every 30 s, and
// on a 2500 by 750 router a build is about 455 KB of garbage. Guarded by App.mu.
type namesBody struct {
	rs   *routerState // a router change replaces it
	rt   *runtime     // numberOffset
	gen  int
	body []byte
	tag  string
}

func (a *App) apiNames(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	v := cl.view()
	if v.Hidden() {
		a.mu.Lock()
		res := a.namesSnapshot(v)
		a.mu.Unlock()
		apiJSONTagged(w, r, res)
		return
	}
	a.mu.Lock()
	key := namesBody{rs: a.router, rt: a.r(), gen: a.router.namesGen}
	if c := a.namesCache; c.rs == key.rs && c.rt == key.rt && c.gen == key.gen && c.body != nil {
		a.mu.Unlock()
		apiWriteTagged(w, r, c.body, c.tag)
		return
	}
	res := a.namesSnapshot(v)
	a.mu.Unlock()
	body, err := json.Marshal(res)
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the answer could not be built")
		return
	}
	key.body, key.tag = body, etagOf(body)
	a.mu.Lock()
	if a.router == key.rs && a.r() == key.rt && a.router.namesGen == key.gen {
		a.namesCache = key
	}
	a.mu.Unlock()
	apiWriteTagged(w, r, key.body, key.tag)
}

type apiRouteRow struct {
	Dst   int `json:"dst"`
	Level int `json:"level"` // UI level
	Src   int `json:"src"`   // -1 = nothing routed
}

// router.protect maps dst and level to a device, not a source. So the row has no value and visibility asks about dst
// only.
type apiProtectedRow struct {
	Dst   int `json:"dst"`
	Level int `json:"level"` // UI level
}

// Stable order.
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
	Level  int    `json:"level"`
	Label  string `json:"label"`
	Swatch string `json:"swatch"`
	Color  string `json:"color,omitempty"` // wins over Swatch
}

// Caller holds mu.
func (a *App) tallySnapshot(v apiView) map[string]any {
	levels := []apiLevel{}
	for _, l := range a.r().levels {
		levels = append(levels, apiLevel{l.UI, l.Label, l.Swatch, l.Color})
	}
	cfg := a.r().cfg
	return map[string]any{
		"levels": levels, "levelNumbering": cfg.LevelNumbering, "defaultLevels": cfg.DefaultLevels,
		"routes": routeRows(a.router.tally, v),
	}
}

// Built one time per change for every caller that is not hidden. Each panel page polls it every 2 s, and on a
// 2500 by 750 router a build is about 400 KB of JSON. Guarded by App.mu.
type tallyBody struct {
	rs   *routerState // a router change replaces it
	rt   *runtime     // levels and level numbering
	gen  int
	body []byte
	tag  string
}

func (a *App) apiTally(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	v := cl.view()
	if v.Hidden() {
		a.mu.Lock()
		res := a.tallySnapshot(v)
		a.mu.Unlock()
		apiJSONTagged(w, r, res)
		return
	}
	a.mu.Lock()
	key := tallyBody{rs: a.router, rt: a.r(), gen: a.router.tallyGen}
	if c := a.tallyCache; c.rs == key.rs && c.rt == key.rt && c.gen == key.gen && c.body != nil {
		a.mu.Unlock()
		apiWriteTagged(w, r, c.body, c.tag)
		return
	}
	res := a.tallySnapshot(v)
	a.mu.Unlock()
	body, err := json.Marshal(res)
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the answer could not be built")
		return
	}
	key.body, key.tag = body, etagOf(body)
	a.mu.Lock()
	if a.router == key.rs && a.r() == key.rt && a.router.tallyGen == key.gen {
		a.tallyCache = key
	}
	a.mu.Unlock()
	apiWriteTagged(w, r, key.body, key.tag)
}

func (a *App) apiProtected(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
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
	apiJSONTagged(w, r, map[string]any{"protected": rows})
}

// The router address and the error text (which quotes it) go only to a caller with the log scope.
// The raw router rows of the Communication Log do the same.
func linkStatus(s statusMsg, withDetail bool) map[string]any {
	m := map[string]any{"router": s.Router, "routerUp": s.Router == string(swp08.StateUp), "error": s.Err != ""}
	if withDetail {
		m["address"], m["err"] = s.Addr, s.Err
	}
	return m
}

func (a *App) apiStatus(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	a.mu.Lock()
	st := a.status
	a.mu.Unlock()
	m := linkStatus(st, cl.has(config.ScopeLog))
	m["server"], m["version"], m["uptimeSeconds"] = "up", VersionLabel(), int64(time.Since(a.started).Seconds())
	apiJSON(w, m)
}

func (a *App) apiLayouts(w http.ResponseWriter, _ *http.Request, _ *apiCtx) {
	apiJSON(w, map[string]any{"layouts": []any{}})
}
