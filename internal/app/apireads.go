package app

import (
	"encoding/json"
	"net/http"
	"sort"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/internal/tieplan"
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
	out := []apiRouter{}
	for _, rc := range a.r().cfg.Routers() {
		out = append(out, apiRouter{rc.UID, rc.Prefix, rc.Title})
	}
	apiJSONTagged(w, r, map[string][]apiRouter{"routers": out})
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
func (a *App) namesSnapshot(u *routerUnit, v apiView) apiNames {
	return apiNames{
		NumberOffset: a.r().routerOf(u.uid).NumberOffset,
		Sources:      namesOf(u.router.srcNames, v.on(u.uid).Source),
		Destinations: namesOf(u.router.dstNames, v.on(u.uid).Destination),
		NamesAt:      u.router.namesAt,
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
	v := a.view(cl)
	u, ok := a.apiReadUnit(w, r)
	if !ok {
		return
	}
	if v.Hidden() {
		a.mu.Lock()
		res := a.namesSnapshot(u, v)
		a.mu.Unlock()
		apiJSONTagged(w, r, res)
		return
	}
	a.mu.Lock()
	key := namesBody{rs: u.router, rt: a.r(), gen: u.router.namesGen}
	if c := u.namesCache; c.rs == key.rs && c.rt == key.rt && c.gen == key.gen && c.body != nil {
		a.mu.Unlock()
		apiWriteTagged(w, r, c.body, c.tag)
		return
	}
	res := a.namesSnapshot(u, v)
	a.mu.Unlock()
	body, err := json.Marshal(res)
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the answer could not be built")
		return
	}
	key.body, key.tag = body, etagOf(body)
	a.mu.Lock()
	if u.router == key.rs && a.r() == key.rt && u.router.namesGen == key.gen {
		u.namesCache = key
	}
	a.mu.Unlock()
	apiWriteTagged(w, r, key.body, key.tag)
}

type apiRouteRow struct {
	Dst   int `json:"dst"`
	Level int `json:"level"` // UI level
	Src   int `json:"src"`   // -1 = nothing routed
	// The Via chain of this level, only when a tie line feeds Src (tievia.go). Origin is nil when Nothing, Unknown or
	// Loop is set, or when the caller's hidden visibility does not show it. Levels: only when Mixed, the origin of
	// each level upstream of the nearest tie line, from level 1.
	Via     []apiTieRef    `json:"via,omitempty"`
	Origin  *tieplan.End   `json:"origin,omitempty"`
	Mixed   bool           `json:"mixed,omitempty"`
	Nothing bool           `json:"nothing,omitempty"`
	Unknown string         `json:"unknown,omitempty"`
	Loop    bool           `json:"loop,omitempty"`
	Levels  []*tieplan.End `json:"levels,omitempty"`
}

// router.protect maps dst and level to a device, not a source. So visibility asks about dst only.
type apiProtectedRow struct {
	Dst      int    `json:"dst"`
	Level    int    `json:"level"` // UI level
	Kind     string `json:"kind"`  // protect or routerLock
	Device   int    `json:"device"`
	SodeUser string `json:"sodeUser,omitempty"` // on a Protect Sode set: who set it
}

// Stable order.
func routeRows(m map[int]map[int]int, v routerView, x *viaTrace) []apiRouteRow {
	out := []apiRouteRow{}
	for d, t := range m {
		for l, s := range t {
			if v.Route(d, l, s) {
				row := apiRouteRow{Dst: d, Level: l, Src: s}
				x.fill(v, &row)
				out = append(out, row)
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
func (a *App) tallySnapshot(u *routerUnit, v apiView) map[string]any {
	levels := []apiLevel{}
	r := a.r()
	for _, l := range r.levelsOf(u.uid).levels {
		levels = append(levels, apiLevel{l.UI, l.Label, l.Swatch, l.Color})
	}
	rc := r.routerOf(u.uid)
	return map[string]any{
		"levels": levels, "levelNumbering": rc.LevelNumbering, "defaultLevels": rc.DefaultLevels,
		"routes": routeRows(u.router.tally, v.on(u.uid), a.newViaTrace(r)),
	}
}

// Built one time per change for every caller that is not hidden. Each panel page polls it every 2 s, and on a
// 2500 by 750 router a build is about 400 KB of JSON. Guarded by App.mu.
type tallyBody struct {
	rs   *routerState // a router change replaces it
	rt   *runtime     // levels and level numbering
	gen  int
	via  int // App.viaGen
	body []byte
	tag  string
}

func (a *App) apiTally(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	v := a.view(cl)
	u, ok := a.apiReadUnit(w, r)
	if !ok {
		return
	}
	if v.Hidden() {
		a.mu.Lock()
		res := a.tallySnapshot(u, v)
		a.mu.Unlock()
		apiJSONTagged(w, r, res)
		return
	}
	a.mu.Lock()
	key := tallyBody{rs: u.router, rt: a.r(), gen: u.router.tallyGen, via: a.viaGen}
	if c := u.tallyCache; c.rs == key.rs && c.rt == key.rt && c.gen == key.gen && c.via == key.via && c.body != nil {
		a.mu.Unlock()
		apiWriteTagged(w, r, c.body, c.tag)
		return
	}
	res := a.tallySnapshot(u, v)
	a.mu.Unlock()
	body, err := json.Marshal(res)
	if err != nil {
		apiFail(w, http.StatusInternalServerError, "internal", "the answer could not be built")
		return
	}
	key.body, key.tag = body, etagOf(body)
	a.mu.Lock()
	if u.router == key.rs && a.r() == key.rt && u.router.tallyGen == key.gen && a.viaGen == key.via {
		u.tallyCache = key
	}
	a.mu.Unlock()
	apiWriteTagged(w, r, key.body, key.tag)
}

func (a *App) apiProtected(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	rt := a.r()
	by := map[int]protectBy{}
	for _, b := range a.protectByList(rt) {
		by[b.Dst] = b
	}
	a.mu.Lock()
	v := a.view(cl)
	rows := []apiProtectedRow{}
	// Only the first router, as in the protected message.
	for d, t := range a.firstUnit().router.protect {
		if !v.on(a.firstUnit().uid).Destination(d) {
			continue
		}
		for l, h := range t {
			row := apiProtectedRow{Dst: d, Level: l, Kind: holdKindWord(h.kind), Device: h.dev}
			// The rule of protectLevelsMu. A Sode hold with no record has no name, so sodeUser stays empty.
			if b, ok := by[d]; ok && sodesHold(h, rt.cfg.ProtectDevice, b.Device) {
				row.SodeUser = b.By
			}
			rows = append(rows, row)
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

func (a *App) apiStatus(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	u, ok := a.apiReadUnit(w, r)
	if !ok {
		return
	}
	a.mu.Lock()
	st := u.status
	a.mu.Unlock()
	m := linkStatus(st, cl.has(config.ScopeLog))
	m["server"], m["version"], m["uptimeSeconds"] = "up", VersionLabel(), int64(time.Since(a.started).Seconds())
	apiJSON(w, m)
}

func (a *App) apiLayouts(w http.ResponseWriter, _ *http.Request, _ *apiCtx) {
	apiJSON(w, map[string]any{"layouts": []any{}})
}
