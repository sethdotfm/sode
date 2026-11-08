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
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// REST writes: a take by routes or by preset, and Undo, Redo and Retry of a History entry.
// Each request is synchronous: it returns once the router has answered or timed out every
// route, with a Result per route and the History entry ids. Late Results reach the stream
// (ticket F) as updates to those entries. Per ADR 0002 the request is its own Confirm.

func init() {
	registerAPI(
		apiRoute{"POST", apiBase + "/takes", config.ScopeTake, (*App).apiTakeRoutes},
		apiRoute{"POST", apiBase + "/presets/{scope}/{name}", config.ScopeTake, (*App).apiTakePreset},
		apiRoute{"POST", apiBase + "/history/{id}/undo", config.ScopeTake + " " + config.ScopeHistory, (*App).apiHistoryAction},
		apiRoute{"POST", apiBase + "/history/{id}/redo", config.ScopeTake + " " + config.ScopeHistory, (*App).apiHistoryAction},
		apiRoute{"POST", apiBase + "/history/{id}/retry", config.ScopeTake + " " + config.ScopeHistory, (*App).apiHistoryAction},
	)
}

const (
	apiTakeBody   = 64 << 10
	apiTakeRoutes = config.MaxPresetTotal // routes in one request
	// takeMargin is added to the computed worst case before the write deadline.
	takeMargin = 10 * time.Second
)

// apiResp is a finished answer, written by one place so Idempotency-Key can store and replay it.
type apiResp struct {
	Status int
	Body   []byte
	Keep   bool // an Idempotency-Key repeat gets this answer back
}

func respJSON(status int, v any, keep bool) apiResp {
	b, _ := json.Marshal(v)
	return apiResp{status, append(b, '\n'), keep}
}

func respProblem(p *apiProblem) apiResp {
	return respJSON(p.Status, apiErrBody{apiErr{p.Code, p.Msg}}, false)
}

func writeResp(w http.ResponseWriter, rs apiResp, replay bool) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	if replay {
		w.Header().Set("Idempotent-Replay", "true")
	}
	w.WriteHeader(rs.Status)
	w.Write(rs.Body)
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
		if i < 0 || !cl.view().Preset(list[i].Name) {
			return nil, problem(http.StatusNotFound, "not_found", "no preset %q in %s", name, scope)
		}
		return planPreset(a, a.r(), list[i])
	})
}

func (a *App) apiHistoryAction(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	id, err := strconv.Atoi(r.PathValue("id"))
	verb := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:] // undo, redo or retry
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

func subjectOf(cl *apiCtx) string {
	if cl.Key != nil {
		return "key:" + cl.Key.ID
	}
	return "ip:" + cl.IP
}

// apiTakeRun is every write's one path: body, Idempotency-Key, plan, rights, take cap, take,
// response. build turns the body into routes, or says why not.
func (a *App) apiTakeRun(w http.ResponseWriter, r *http.Request, cl *apiCtx, build func(body []byte) (*takePlan, *apiProblem)) {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, apiTakeBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeResp(w, respProblem(problem(http.StatusRequestEntityTooLarge, "too_large", "the request body is over %d bytes", apiTakeBody)), false)
		} else {
			writeResp(w, respProblem(problem(http.StatusBadRequest, "bad_request", "the request body could not be read")), false)
		}
		return
	}

	var slot *idemSlot
	idem := r.Header.Get("Idempotency-Key")
	if idem != "" {
		if !validIdemKey(idem) {
			writeResp(w, respProblem(problem(http.StatusBadRequest, "bad_request", "Idempotency-Key must be 1 to %d printable characters with no spaces", idemKeyMax)), false)
			return
		}
		sum := sha256.Sum256(append([]byte(r.Method+" "+r.URL.Path+"\n"), body...))
		var owner, mismatch bool
		slot, owner, mismatch = a.apiS.idemBegin(r.Context(), subjectOf(cl), idem, hex.EncodeToString(sum[:]), time.Now())
		switch {
		case mismatch:
			writeResp(w, respProblem(problem(http.StatusUnprocessableEntity, "idempotency_mismatch", "this Idempotency-Key was used for a different request")), false)
			return
		case slot == nil: // the repeat's own request ended while it waited
			return
		case !owner:
			writeResp(w, apiResp{slot.status, slot.body, true}, true)
			return
		}
	}

	rs := a.apiTakeExec(w, cl, body, build)
	if slot != nil {
		if rs.Keep {
			a.apiS.idemCommit(slot, rs.Status, rs.Body)
		} else {
			a.apiS.idemAbandon(subjectOf(cl), idem, slot)
		}
	}
	writeResp(w, rs, false)
}

func (a *App) apiTakeExec(w http.ResponseWriter, cl *apiCtx, body []byte, build func([]byte) (*takePlan, *apiProblem)) apiResp {
	refuse := func(p *apiProblem) apiResp {
		a.apiEvent(cl.Actor, "refused", "refused", p.Code, fmt.Errorf("%s", p.Msg))
		return respProblem(p)
	}
	plan, p := build(body)
	if p != nil {
		return refuse(p)
	}
	rt := a.r()
	c := cl.client()
	sink := &sinkResults{by: map[int]routeResult{}}
	c.api.sink = sink.put
	for i := range plan.Items {
		plan.Items[i].ID = i + 1
		plan.Items[i].Preset = plan.Preset
	}
	if msg := a.outsideRights(rt, cl.Rights, plan.Items); msg != "" {
		a.rightsPass(rt, c, plan.Items) // writes the refusal row
		return respJSON(http.StatusForbidden, apiErrBody{apiErr{"outside_rights", msg}}, true)
	}
	if !a.apiS.takeStart(rt.cfg.API.TakeCap) {
		a.apiEvent(cl.Actor, takeWhy(plan.Items), "refused", "take cap", fmt.Errorf("%d API takes already in flight", rt.cfg.API.TakeCap))
		return respJSON(http.StatusTooManyRequests, apiErrBody{apiErr{"take_cap",
			fmt.Sprintf("%d API takes are already in flight; the router handles one message at a time. Try again in a moment", rt.cfg.API.TakeCap)}}, false)
	}
	defer a.apiS.takeEnd()

	n := len(plan.Items)
	note := fmt.Sprintf("%d %s", n, plural(n, "route", "routes"))
	if plan.Preset != "" {
		note = fmt.Sprintf("preset %q, %s", plan.Preset, note)
	}
	a.apiEvent(cl.Actor, takeWhy(plan.Items), "sent", note, nil)
	// A write deadline for this request only; the recorder in tests has none.
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(a.takeDeadline(n)))
	a.take(c, plan.Items)

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
	return respJSON(http.StatusOK, out, true)
}

func distinct(ls []int) []int {
	s := slices.Clone(ls)
	slices.Sort(s)
	return slices.Compact(s)
}
