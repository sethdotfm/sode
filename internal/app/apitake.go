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
)

// REST writes: a take by routes or by preset, and Undo, Redo and Retry of a History entry.
// Each request is synchronous: it returns once the router has answered or timed out every
// route, with a Result per route and the History entry ids. Late Results reach the stream
// as updates to those entries. Per ADR 0002 the request is its own Confirm.

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
	// takeMargin is added to the computed worst case before the write deadline.
	takeMargin = 10 * time.Second
)

// apiAnswer is a finished JSON answer. It is marshaled once so Idempotency-Key can store and
// replay the same bytes.
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

// takeDeadline is the worst case of n routes: per route check, queue wait, route and grace
// (about 35 s by default), repeated for each wave of routeLimit routes. It becomes this
// request's write deadline, so no server-wide WriteTimeout is needed (and none must be set:
// it would cut /ws and the stream).
func (a *App) takeDeadline(n int) time.Duration {
	one := a.checkTimeout + a.queueMax + a.routeTimeout + a.routeGrace
	return one*time.Duration((n+routeLimit-1)/routeLimit) + takeMargin
}

type apiRouteIn struct {
	Src    int   `json:"src"`
	Dst    int   `json:"dst"`
	Levels []int `json:"levels"`
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
		defaults, _ := config.ParseLevels(rt.cfg.DefaultLevels, rt.cfg.Levels)
		p := &takePlan{}
		for _, ro := range in.Routes {
			ls := ro.Levels
			if len(ls) == 0 {
				ls = slices.Clone(defaults)
			}
			p.Items = append(p.Items, routeReq{Src: ro.Src, Dst: ro.Dst, Levels: ls})
		}
		return p, nil
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
			if u, ok := cfg.FindUser(cl.Actor.Name); ok {
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
}

type apiTakeOut struct {
	OK bool `json:"ok"` // every route succeeded
	// CheckSkipped: the take had bigTake or more routes, so Sode did not read each destination
	// from the router first. Entries then take Prev from the cache, which an unannounced change
	// can make stale, so a later Undo may restore less accurately.
	CheckSkipped bool          `json:"checkSkipped"`
	Results      []apiRouteOut `json:"results"`
	LeftOut      []string      `json:"leftOut,omitempty"`
	Skipped      []int         `json:"skippedLevels,omitempty"`
	Untouched    []int         `json:"untouchedLevels,omitempty"`
}

// sinkResults collects the takeResult replies of one request.
type sinkResults struct {
	mu sync.Mutex
	by map[int]routeResult
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

// apiTakeRun is every write's one path: body, Idempotency-Key, plan, take cap, take with its
// rights check, response. build turns the body into routes, or says why not.
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

// apiTakeExec runs one write and returns its answer. keep says a repeat with the same
// Idempotency-Key gets that answer back: only requests that reached the take or its rights check.
func (a *App) apiTakeExec(w http.ResponseWriter, cl *apiCtx, body []byte, build func([]byte) (*takePlan, *apiProblem)) (ans apiAnswer, keep bool) {
	plan, p := build(body)
	if p != nil {
		a.apiEvent(cl.Actor, "refused", "refused", p.Code, fmt.Errorf("%s", p.Msg))
		return p.answer(), false
	}
	rt := a.r()
	for i := range plan.Items {
		plan.Items[i].ID = i + 1
		plan.Items[i].Preset = plan.Preset
	}
	n := len(plan.Items)
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
	// A write deadline for this request only; the recorder in tests has none.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(a.takeDeadline(n)))
	a.takeAllowed(c, plan.Items)

	out := apiTakeOut{OK: true, CheckSkipped: n >= bigTake, LeftOut: plan.LeftOut, Skipped: plan.Skipped, Untouched: plan.Untouched, Results: make([]apiRouteOut, n)}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	for i, it := range plan.Items {
		rr, ok := sink.by[it.ID]
		o := apiRouteOut{ID: it.ID, Src: it.Src, Dst: it.Dst, Levels: it.Levels, OK: rr.OK, Already: rr.Already, Error: rr.Err, Entry: rr.Entry}
		switch {
		case !ok:
			o.Error = "no result"
			fallthrough
		case !rr.OK:
			o.Result = "failed"
			out.OK = false
		case len(rr.Already) > 0 && len(rr.Already) >= len(distinct(it.Levels)):
			o.Result = "already"
		default:
			o.Result = "connected"
		}
		out.Results[i] = o
	}
	return jsonAnswer(http.StatusOK, out), true
}

func distinct(ls []int) []int {
	s := slices.Clone(ls)
	slices.Sort(s)
	return slices.Compact(s)
}
