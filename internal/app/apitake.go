package app

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// REST writes. Each request returns after the router answered or timed out every route. Late Results reach the stream
// as updates to the entries. ADR 0002: the request is its own Confirm.

func init() {
	registerAPI(
		apiRoute{"POST", apiBase + "/takes", config.ScopeTake, (*App).apiTakeRoutes},
		apiRoute{"POST", apiBase + "/presets/{scope}/{name}", config.ScopeTake, (*App).apiTakePreset},
		apiRoute{"POST", apiBase + "/history/{id}/undo", config.ScopeTake + " " + config.ScopeHistory, historyVerb("undo")},
		apiRoute{"POST", apiBase + "/history/{id}/redo", config.ScopeTake + " " + config.ScopeHistory, historyVerb("redo")},
		apiRoute{"POST", apiBase + "/history/{id}/retry", config.ScopeTake + " " + config.ScopeHistory, historyVerb("retry")},
	)
}

const (
	apiTakeBody   = 64 << 10
	apiTakeRoutes = config.MaxPresetTotal // routes in one request
	takeMargin    = 10 * time.Second
)

// Marshaled one time, so Idempotency-Key can store and replay the same bytes.
type apiAnswer struct {
	Status int
	Body   []byte
}

func jsonAnswer(status int, v any) apiAnswer {
	b, _ := json.Marshal(v)
	return apiAnswer{status, append(b, '\n')}
}

func (x apiAnswer) write(w http.ResponseWriter, replay bool) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if x.Status == http.StatusUnauthorized {
		w.Header().Set("WWW-Authenticate", "Bearer")
	}
	if replay {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.WriteHeader(x.Status)
	w.Write(x.Body)
}

// The worst case of n routes: each wave of routeLimit routes adds check, queue wait, route and grace (about 35 s by
// default). It is this request's write deadline. Never set a server-wide WriteTimeout: it would cut /ws and the stream.
// The routers of a take run side by side, so the slowest router's timers count.
func (a *App) takeDeadline(n int) time.Duration {
	var one time.Duration
	for _, u := range a.routers {
		one = max(one, u.checkTimeout+u.queueMax+u.routeTimeout+u.routeGrace)
	}
	return one*time.Duration((n+routeLimit-1)/routeLimit) + takeMargin
}

type apiRouteIn struct {
	Src    apiEnd `json:"src"`
	Dst    apiEnd `json:"dst"`
	Levels []int  `json:"levels"`
	// Ties: a route across routers names one tie line for each hop before the last, upstream first (tietake.go).
	Ties []apiTieIn `json:"ties"`
}

// apiTieIn is one tie line of a route. Expect is the source that the caller saw on it (null: nothing routed). Sode
// does not compare it yet.
type apiTieIn struct {
	ID     string  `json:"id"`
	Expect *apiEnd `json:"expect"`
}

// apiEnd is one end of a route: a router reference {router, n}, or a bare number, which is enough while Sode has one
// router.
type apiEnd struct {
	Router string
	N      int
}

func (e *apiEnd) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] != '{' {
		*e = apiEnd{}
		return json.Unmarshal(b, &e.N)
	}
	var ref struct {
		Router string `json:"router"`
		N      *int   `json:"n"`
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&ref); err != nil {
		return err
	}
	if ref.Router == "" || ref.N == nil {
		return errors.New(`a router reference needs "router" and "n"`)
	}
	*e = apiEnd{ref.Router, *ref.N}
	return nil
}

// routeUnits gives the router units of the two ends of a route. A bare end is the one router; with two or more
// routers it is 400 router_needed. Ends on two routers give two units: the route needs tie lines (tieroutes.go).
func (a *App) routeUnits(ro apiRouteIn) (src, dst *routerUnit, _ *apiProblem) {
	var us [2]*routerUnit
	for i, e := range []apiEnd{ro.Src, ro.Dst} {
		switch {
		case e.Router == "" && len(a.routers) > 1:
			return nil, nil, problem(http.StatusBadRequest, "router_needed", `Sode has %d routers: give each end as {"router": uid, "n": number}, with a uid from GET %s/routers`, len(a.routers), apiBase)
		case !a.knownRouter(e.Router):
			return nil, nil, problem(http.StatusNotFound, "unknown_router", "no router with UID %q; GET %s/routers lists them", e.Router, apiBase)
		}
		us[i] = a.unitOrFirst(e.Router)
	}
	return us[0], us[1], nil
}

type apiTakeIn struct {
	Routes []apiRouteIn `json:"routes"`
}

func (a *App) apiTakeRoutes(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	a.apiTakeRun(w, r, cl, func(body []byte) (*takePlan, *apiProblem) {
		var in apiTakeIn
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return nil, problem(http.StatusBadRequest, "bad_request", `send {"routes":[{"src":0,"dst":0,"levels":[1]}]}: %v`, err)
		}
		if len(in.Routes) == 0 || len(in.Routes) > apiTakeRoutes {
			return nil, problem(http.StatusBadRequest, "bad_request", "routes must hold 1 to %d routes", apiTakeRoutes)
		}
		rt := a.r()
		p := &takePlan{}
		var all, bare []tieRoute
		ties := map[int][]string{}
		for i, ro := range in.Routes {
			su, u, prob := a.routeUnits(ro)
			if prob != nil {
				return nil, prob
			}
			ls := ro.Levels
			if len(ls) == 0 {
				rc := rt.routerOf(u.uid)
				ls, _ = config.ParseLevels(rc.DefaultLevels, rc.Levels)
			}
			r := tieRoute{index: i, src: tieplan.End{Router: su.uid, N: ro.Src.N}, dst: tieplan.End{Router: u.uid, N: ro.Dst.N}, levels: ls}
			all = append(all, r)
			switch {
			case su == u && len(ro.Ties) > 0:
				return nil, problem(http.StatusBadRequest, "bad_request", "route %d: ties is only for a route across routers", i)
			case su != u && len(ro.Ties) == 0:
				bare = append(bare, r)
			case su != u:
				for _, t := range ro.Ties {
					ties[i] = append(ties[i], t.ID)
				}
			}
			p.Items = append(p.Items, routeReq{Src: ro.Src.N, Dst: ro.Dst.N, Levels: ls, Router: u.uid})
		}
		if len(ties) == 0 && len(bare) == 0 {
			return p, nil
		}
		// Rights cover the typed ends only, and come first, so a caller learns no path for ends that it must not patch.
		if prob := a.tieRouteRights(rt, cl.Rights, all); prob != nil {
			return nil, prob
		}
		if len(bare) > 0 {
			return nil, a.apiTieProblem(rt, cl.view(), bare)
		}
		tt, prob := a.planTieTake(rt, cl.view(), all, ties)
		if prob != nil {
			return nil, prob
		}
		return &takePlan{Ties: tt}, nil
	})
}

func (a *App) apiTakePreset(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	scope, name := r.PathValue("scope"), r.PathValue("name")
	a.apiTakeRun(w, r, cl, func([]byte) (*takePlan, *apiProblem) {
		cfg := a.r().cfg
		var list []config.Preset
		switch {
		case scope == "everyone":
			list = cfg.Presets
		case scope == "me" && cl.Actor.Kind == actorUser:
			if u, ok := cfg.FindUserByID(cl.Actor.User); ok {
				list = u.Profile.Presets
			}
		default:
			return nil, problem(http.StatusNotFound, "not_found", `no preset scope %q; use "everyone", or "me" with a user's key`, scope)
		}
		i := slices.IndexFunc(list, func(p config.Preset) bool { return cleanPresetName(p.Name) == cleanPresetName(name) })
		if i < 0 || !cl.view().WholePreset(list[i]) {
			return nil, problem(http.StatusNotFound, "not_found", "no preset %q in %s", name, scope)
		}
		return planPreset(a, a.r(), list[i])
	})
}

func historyVerb(verb string) apiHandler {
	return func(a *App, w http.ResponseWriter, r *http.Request, cl *apiCtx) {
		a.apiHistoryAction(verb, w, r, cl)
	}
}

func (a *App) apiHistoryAction(verb string, w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	id, err := strconv.Atoi(r.PathValue("id"))
	a.apiTakeRun(w, r, cl, func([]byte) (*takePlan, *apiProblem) {
		if err != nil || id < 0 {
			return nil, problem(http.StatusBadRequest, "bad_request", "the entry id must be a whole number")
		}
		e, ok := a.hist.Get(id)
		if ok {
			e, ok = cl.view().Entry(e)
		}
		if !ok {
			return nil, problem(http.StatusNotFound, "not_found", "no History entry %d", id)
		}
		if e.Protect != "" {
			return nil, problem(http.StatusConflict, "conflict", "entry %d is a Protect: use POST or DELETE /protected/%d", id, e.Dst)
		}
		rt := a.r()
		if verb == "undo" {
			p := planUndo(rt, cl.view(), e)
			if len(p.Items) == 0 {
				return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "entry %d has nothing to restore: no level had a route before it", id)
			}
			return p, nil
		}
		return planAgain(rt, cl.view(), e, verb == "retry")
	})
}

type apiRouteOut struct {
	ID      int    `json:"id"`
	Src     int    `json:"src"`
	Dst     int    `json:"dst"`
	Levels  []int  `json:"levels"`
	Result  string `json:"result"` // connected, already or failed
	OK      bool   `json:"ok"`
	Already []int  `json:"already,omitempty"`
	Error   string `json:"error,omitempty"`
	Entry   *int   `json:"entry,omitempty"`
	// Hops: a route across routers, each hop before the last, upstream first.
	Hops []TieHop `json:"hops,omitempty"`
}

type apiTakeOut struct {
	OK bool `json:"ok"` // every route succeeded
	// CheckSkipped: the check before take did not finish for some route (no answer in time, or the link was down).
	// Its Prev came from the cache. After a change that the router did not announce, an Undo can restore a stale route.
	CheckSkipped bool          `json:"checkSkipped"`
	Results      []apiRouteOut `json:"results"`
	LeftOut      []string      `json:"leftOut,omitempty"`
	Skipped      []int         `json:"skippedLevels,omitempty"`
	Untouched    []int         `json:"untouchedLevels,omitempty"`
}

type sinkResults struct {
	mu sync.Mutex
	by map[int]routeResult
}

func (s *sinkResults) get(id int) (routeResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rr, ok := s.by[id]
	return rr, ok
}

func (s *sinkResults) put(b []byte) {
	var rr routeResult
	if json.Unmarshal(b, &rr) != nil || rr.T != "takeResult" {
		return
	}
	s.mu.Lock()
	s.by[rr.ID] = rr
	s.mu.Unlock()
}

// The one path of every write: body, Idempotency-Key, plan, take cap, rights check, take, response.
func (a *App) apiTakeRun(w http.ResponseWriter, r *http.Request, cl *apiCtx, build func(body []byte) (*takePlan, *apiProblem)) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, apiTakeBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			problem(http.StatusRequestEntityTooLarge, "too_large", "the request body is over %d bytes", apiTakeBody).write(w)
		} else {
			problem(http.StatusBadRequest, "bad_request", "the request body could not be read").write(w)
		}
		return
	}

	var slot *idemSlot
	idem := r.Header.Get("Idempotency-Key")
	if idem != "" {
		switch {
		case !validIdemKey(idem):
			problem(http.StatusBadRequest, "bad_request", "Idempotency-Key must be 1 to %d printable characters with no spaces", idemKeyMax).write(w)
			return
		case cl.Key == nil:
			// Guests behind one NAT share an address, so there is no caller to tie the key to.
			problem(http.StatusBadRequest, "bad_request", "Idempotency-Key needs an API key; a call without one has no caller of its own to keep the answer for").write(w)
			return
		}
		sum := sha256.Sum256(append([]byte(r.Method+" "+r.URL.Path+"\n"), body...))
		var owner, mismatch bool
		slot, owner, mismatch = a.apiS.idemBegin(r.Context(), cl.Key.subject(), idem, hex.EncodeToString(sum[:]), time.Now())
		switch {
		case mismatch:
			problem(http.StatusUnprocessableEntity, "idempotency_mismatch", "this Idempotency-Key was used for a different request").write(w)
			return
		case slot == nil: // the repeat's own request ended while it waited
			return
		case !owner:
			apiAnswer{slot.status, slot.body}.write(w, true)
			return
		}
	}

	ans, keep := a.apiTakeExec(w, cl, body, build)
	if slot != nil {
		if keep {
			a.apiS.idemCommit(slot, ans.Status, ans.Body)
		} else {
			a.apiS.idemAbandon(cl.Key.subject(), idem, slot)
		}
	}
	ans.write(w, false)
}

// keep: a repeat with the same Idempotency-Key gets this answer back. Only a request that reached the rights check is
// kept.
func (a *App) apiTakeExec(w http.ResponseWriter, cl *apiCtx, body []byte, build func([]byte) (*takePlan, *apiProblem)) (ans apiAnswer, keep bool) {
	plan, p := build(body)
	if p != nil {
		a.apiEvent(cl.Actor, "refused", "refused", p.Code, fmt.Errorf("%s", p.Msg))
		return p.answer(), false
	}
	rt := a.r()
	for i := range plan.Items {
		plan.Items[i].ID = i + 1
		plan.Items[i].Preset, plan.Items[i].PresetID = plan.Preset, plan.PresetID
	}
	n := len(plan.Items)
	if plan.Ties != nil {
		n = len(plan.Ties.routes)
	}
	sink := &sinkResults{by: map[int]routeResult{}}
	cl.sink = sink.put
	c := cl.client()
	// Rights first, so a refused take holds no slot and logs no `sent` row.
	if msg := a.rightsPass(rt, c, plan.Items); msg != "" {
		return problem(http.StatusForbidden, "outside_rights", "%s", msg).answer(), true
	}
	if !a.apiS.takeStart(rt.cfg.API.TakeCap) {
		a.apiEvent(cl.Actor, takeWhy(plan.Items), "refused", "take cap", fmt.Errorf("%d API takes already in flight", rt.cfg.API.TakeCap))
		return problem(http.StatusTooManyRequests, "take_cap",
			"%d API takes are already in flight; the router handles one message at a time. Try again in a moment", rt.cfg.API.TakeCap).answer(), false
	}
	defer a.apiS.takeEnd()
	note := fmt.Sprintf("%d %s", n, plural(n, "route", "routes"))
	if plan.Preset != "" {
		note = fmt.Sprintf("preset %q, %s", plan.Preset, note)
	}
	a.apiEvent(cl.Actor, takeWhy(plan.Items), "sent", note, nil)
	if t := plan.Ties; t != nil {
		_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(t.deadline(a)))
		return jsonAnswer(http.StatusOK, t.out(a.runTieTake(c, t, sink.get))), true
	}
	// The recorder in tests has no write deadline.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(a.takeDeadline(n)))
	unread := a.takeAllowed(c, plan.Items)

	out := apiTakeOut{OK: true, CheckSkipped: unread, LeftOut: plan.LeftOut, Skipped: plan.Skipped, Untouched: plan.Untouched, Results: make([]apiRouteOut, n)}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for i, it := range plan.Items {
		rr, ok := sink.by[it.ID]
		o := apiRouteOut{ID: it.ID, Src: it.Src, Dst: it.Dst, Levels: it.Levels, OK: rr.OK, Already: rr.Already, Error: rr.Err, Entry: rr.Entry,
			Result: resultWord(rr, ok, it.Levels)}
		if !ok {
			o.Error = "no result"
		}
		out.OK = out.OK && o.Result != "failed"
		out.Results[i] = o
	}
	return jsonAnswer(http.StatusOK, out), true
}

func distinct(ls []int) []int {
	s := slices.Clone(ls)
	slices.Sort(s)
	return slices.Compact(s)
}
