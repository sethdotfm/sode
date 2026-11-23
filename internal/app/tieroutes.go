package app

import (
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// Routes across routers (docs/tielines-design.md). internal/tieplan finds and ranks the paths. This file gives it the
// state of every router. It also turns its Result into the 409 bodies of POST /takes. A route across routers with
// no ties gets 409 tie_line_needed or 409 no_path, and nothing goes to a router. tietake.go sends a route with ties.

// apiTiePaths: the most paths one route of a 409 body lists.
const apiTiePaths = 20

// tieRoute is one route of a take with its ends on two routers. Src and Dst name router units.
type tieRoute struct {
	index    int // place in the request
	src, dst tieplan.End
	levels   []int // UI levels of the router of dst
	// expect: tie line id → what the caller saw on it. seen: the Affected destinations that the caller saw; nil
	// when it sent none, and then the stale check does not compare them.
	expect map[string]tieWant
	seen   []tieplan.End
	ties   []string // the tie line ids that the caller gave, upstream first
	// oneOffs: the One-off tie lines that the caller states (tieoneoff.go). On a route with ties, those of ties. On a
	// route with no ties, the plan gives paths through them too.
	oneOffs []oneOffTie
	// protect: the tie lines of ties whose upstream destination Sode protects after the take (tieprotect.go).
	protect []string
}

// unitUID gives the UID of the router unit that a router UID from the settings names.
func (a *App) unitUID(rt *runtime, uid string) (string, bool) {
	if u, ok := a.unit(uid); ok && uid != "" {
		return u.uid, true
	}
	if uid == rt.cfg.UID && a.first != nil {
		return a.first.uid, true
	}
	return "", false
}

// tieInput is the planner's input: every router, tie line and Protect as Sode has them now. The tie lines are the
// saved ones and the One-off tie lines that Sode keeps (tieLinesNow). The maps are the live ones, so the caller holds
// mu while it plans. The caller adds the Reserves (tieReserves), since the panel store is not read under mu. The last
// change times are the stamps of tiechanges.go.
func (a *App) tieInput(rt *runtime) tieplan.Input {
	in := tieplan.Input{Now: time.Now(), Started: a.started, HopWeight: rt.cfg.TieHopWeight}
	for _, u := range a.unitsInOrder(rt) {
		r := tieplan.Router{UID: u.uid, Levels: rt.routerOf(u.uid).Levels, Up: u.status.Router == string(swp08.StateUp)}
		// No route at all is a router that Sode has not read yet.
		if len(u.router.tally) > 0 {
			r.Tally = u.router.tally
		}
		in.Routers = append(in.Routers, r)
		dev := rt.routerOf(u.uid).ProtectDevice
		recs := map[int]protectRec{}
		for _, rec := range a.protects.list(rt.routerOf(u.uid).UID) {
			recs[rec.Dst] = rec
		}
		for _, dst := range slices.Sorted(maps.Keys(u.router.protect)) {
			for _, l := range slices.Sorted(maps.Keys(u.router.protect[dst])) {
				in.Protects = append(in.Protects, holdOf(rt, tieplan.End{Router: u.uid, N: dst}, u.router.protect[dst][l], dev, recs))
				break
			}
		}
	}
	in.Changes, in.Found = map[string]time.Time{}, map[string]bool{}
	for id, st := range a.tieChanges.snapshot(rt.cfg.TieLines) {
		in.Changes[id], in.Found[id] = st.At, st.Found
	}
	in.Ties, _ = a.tieLinesNow(rt)
	return in
}

// holdOf is one Protect for the planner. Sode's own Protect names who set it and when, from Sode's record
// (docs/tielines-design.md, Protect in a tie take, rule 4). Sode knows no time for a hold of another system.
func holdOf(rt *runtime, at tieplan.End, h protHold, dev int, recs map[int]protectRec) tieplan.Hold {
	hold := tieplan.Hold{At: at, By: holderText(h, dev)}
	if rec, ok := recs[at.N]; ok && sodesHold(h, dev, rec.Device) {
		if who := recName(rt.cfg, rec); who != "" {
			hold.By = who
		}
		since := rec.At
		hold.Since = &since
	}
	return hold
}

func holderText(h protHold, dev int) string {
	switch {
	case h.sodes(dev):
		return "Sode"
	case h.kind == swp08.KindRouterLock:
		return fmt.Sprintf("Router lock by device %d", h.dev)
	}
	return fmt.Sprintf("device %d", h.dev)
}

// planTieRoutes plans each route across routers with no ties, in order. A tie line that an earlier route changes is
// Taken for the later ones. Thus two destinations that want one source share it. No route gets a tie line that
// carries another source. The other routes of the take count too (docs/tielines-design.md, Takes with plain routes
// and tie routes). A route with ties takes the tie lines that it sends to, as the stale check sees them (tieChanged).
// A plain route to the upstream destination of a tie line takes that tie line for every route. A hop there would
// fight it (planTieTake gives 409 conflict). Its mark has no real source (N -1), so it never matches one.
// The result of a route with ties or on one router is empty. opts gives the Reserves. For a panel caller it also
// gives the Reserved tie lines of the panel. The search then uses only those, and they are not Reserved for it.
// Caller holds mu.
func (a *App) planTieRoutes(rt *runtime, routes []tieRoute, opts tiePlanOpts) []tieplan.Result {
	in := withOneOffs(a.tieInput(rt), nil, routes)
	in.Reserves = opts.reserves
	if opts.panel != nil {
		in.Only = opts.panel.only
		in.Reserves = maps.Clone(in.Reserves)
		maps.DeleteFunc(in.Reserves, func(id, _ string) bool { return opts.panel.only[id] })
	}
	in.Taken = map[string]tieplan.End{}
	for _, r := range routes {
		if r.src.Router != r.dst.Router {
			continue
		}
		for _, t := range in.Ties {
			if t.From == r.dst {
				in.Taken[t.ID] = tieplan.End{Router: r.src.Router, N: -1}
			}
		}
	}
	take := func(src tieplan.End, hops []tieplan.Hop) {
		for _, h := range hops {
			if _, ok := in.Taken[h.Tie]; h.Tie != "" && h.Sends && !ok {
				in.Taken[h.Tie] = src
			}
		}
	}
	out := make([]tieplan.Result, len(routes))
	for i, r := range routes {
		rr := tieplan.Route{Src: r.src, Dst: r.dst, Levels: r.levels}
		switch {
		case r.src.Router == r.dst.Router:
		case len(r.ties) > 0:
			if p, why := tieplan.Through(in, rr, r.ties); why == nil {
				take(r.src, p.Hops)
			}
		default:
			out[i] = tieplan.Plan(in, rr)
			if p := out[i].Usable(); p != nil {
				take(r.src, p.Hops)
			}
		}
	}
	return out
}

// planned: the route gets a plan in a 409 body, as a route across routers with no ties.
func (r tieRoute) planned() bool { return r.src.Router != r.dst.Router && len(r.ties) == 0 }

// apiTieProblem answers routes across routers: 409 no_path when a route has no usable path, else 409
// tie_line_needed with the ranked paths of each. The body lists only the routes that get a plan (planned); routes
// gives the whole take, so the other routes count (planTieRoutes).
func (a *App) apiTieProblem(rt *runtime, v apiView, routes []tieRoute) *apiProblem {
	return a.apiTieProblemFor(rt, v, routes, nil)
}

// tiePlanOpts: reserves is tie line id → the name of the panel that Reserves it. panel is the caller's, or nil.
type tiePlanOpts struct {
	reserves map[string]string
	panel    *panelTies
}

// apiTieProblemFor is apiTieProblem for a caller that can be a panel (panelTieRule). A panel gets the paths through
// its Reserved tie lines only, and a no_path message that says why.
func (a *App) apiTieProblemFor(rt *runtime, v apiView, routes []tieRoute, pt *panelTies) *apiProblem {
	opts := tiePlanOpts{reserves: reserveNames(a.tieReserves()), panel: pt}
	a.mu.Lock()
	defer a.mu.Unlock()
	plans := a.planTieRoutes(rt, routes, opts)
	body := apiTieBody{Routes: []apiTieRouteOut{}}
	code, msg := "tie_line_needed", "a route across routers needs a tie line for each hop: send the route again with ties. The paths are in routes, cheapest first"
	for i, r := range routes {
		if !r.planned() {
			continue
		}
		body.Routes = append(body.Routes, a.apiTieRoute(rt, v, r, plans[i]))
		if plans[i].Usable() == nil && code != "no_path" {
			code, msg = "no_path", "no tie line path is usable for a route across routers; reasons gives each reason"
			if pt != nil {
				msg = panelNoPath(rt, pt.panel, plans[i].Reasons)
			}
		}
	}
	body.Error = apiErr{Code: code, Message: msg}
	return &apiProblem{Status: http.StatusConflict, Code: code, Msg: msg, body: body}
}

type apiTieBody struct {
	Error  apiErr           `json:"error"`
	Routes []apiTieRouteOut `json:"routes"`
}

// apiTieEndOut is one end in a 409 body. Name is blank when the caller's hidden visibility does not show it.
type apiTieEndOut struct {
	Router string `json:"router"`
	N      int    `json:"n"`
	Name   string `json:"name"`
}

type apiTieRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	// OneOff: a One-off tie line (tieoneoff.go), which Sode keeps only while it is in use.
	OneOff bool `json:"oneOff,omitempty"`
}

type apiTieChain struct {
	Origin  *apiTieEndOut `json:"origin"`
	Via     []apiTieRef   `json:"via"`
	Mixed   bool          `json:"mixed"`
	Nothing bool          `json:"nothing"`
	Unknown string        `json:"unknown,omitempty"`
	Loop    bool          `json:"loop"`
	// Levels: a mixed tie line, the origin of each level of its upstream router from level 1 (null: nothing or
	// unknown). A caller sends it back as expect.
	Levels []*apiTieEndOut `json:"levels,omitempty"`
}

type apiTieHop struct {
	Router string `json:"router"`
	// Src and Dst are null for a caller with hidden visibility that cannot see that end, as in GET /tielines.
	Src     *apiTieEndOut `json:"src"`
	Dst     *apiTieEndOut `json:"dst"`
	Levels  []int         `json:"levels"`
	Tie     *apiTieRef    `json:"tie"`
	Sends   bool          `json:"sends"`
	Now     *apiTieChain  `json:"now,omitempty"`
	Protect string        `json:"protect,omitempty"`
	Reserve string        `json:"reserve,omitempty"`
	// Changed, Before and ProtectSince are left out when the caller cannot see Dst, the upstream destination of the
	// tie line. ProtectSince is when Sode set the Protect.
	ProtectSince *time.Time `json:"protectSince,omitempty"`
	Changed      *time.Time `json:"changed,omitempty"`
	Before       bool       `json:"before,omitempty"`
}

type apiTiePath struct {
	Cost          int               `json:"cost"`
	Hops          []apiTieHop       `json:"hops"`
	Affected      []apiTieEndOut    `json:"affected"`
	Others        int               `json:"others"` // Affected destinations that the caller's hidden visibility leaves out
	Unknown       []tieplan.Unknown `json:"unknown"`
	Loop          bool              `json:"loop"`
	Protected     bool              `json:"protected"`
	Reserved      bool              `json:"reserved"`
	ChangedLately bool              `json:"changedLately"`
	Newest        *time.Time        `json:"newest,omitempty"`
}

type apiTieReason struct {
	tieplan.Reason
	TieName string `json:"tieName,omitempty"`
}

type apiTieRouteOut struct {
	Route   int            `json:"route"` // place in the request, from 0
	Src     apiTieEndOut   `json:"src"`
	Dst     apiTieEndOut   `json:"dst"`
	Levels  []int          `json:"levels"`
	Paths   []apiTiePath   `json:"paths"`
	More    int            `json:"more"` // paths past the first apiTiePaths
	Reasons []apiTieReason `json:"reasons"`
	// OneOffs: the One-off tie lines that the request states for this route, with the id that its paths use.
	OneOffs []apiOneOffOut `json:"oneOffs,omitempty"`
}

// Names follow apiView, which matches numbers only until Rights name routers (docs/tielines-design.md, Rights
// matching). Caller holds mu.
func (a *App) apiTieRoute(rt *runtime, v apiView, r tieRoute, res tieplan.Result) apiTieRouteOut {
	end := func(e tieplan.End, dst bool) apiTieEndOut {
		out := apiTieEndOut{Router: e.Router, N: e.N}
		u, ok := a.unit(e.Router)
		if !ok {
			return out
		}
		if (dst && v.on(u.uid).Destination(e.N)) || (!dst && v.on(u.uid).Source(e.N)) {
			out.Name = u.router.name(dst, e.N)
		}
		return out
	}
	_, tieName := a.tieLinesNow(rt)
	withOneOffs(tieplan.Input{}, tieName, []tieRoute{r})
	ref := func(id string) apiTieRef { return tieRef(id, tieName) }
	var in *tieplan.Input // read when a tie line is mixed
	out := apiTieRouteOut{Route: r.index, Src: end(r.src, false), Dst: end(r.dst, true), Levels: r.levels, Paths: []apiTiePath{}, Reasons: []apiTieReason{}}
	for _, o := range r.oneOffs {
		out.OneOffs = append(out.OneOffs, apiOneOffOut{ID: o.tie.ID, Name: o.name, From: apiTieEnd{o.tie.From.Router, o.tie.From.N}, To: apiTieEnd{o.tie.To.Router, o.tie.To.N}})
	}
	for i, p := range res.Paths {
		if i == apiTiePaths {
			out.More = len(res.Paths) - i
			break
		}
		ap := apiTiePath{Cost: p.Cost, Affected: []apiTieEndOut{}, Unknown: p.Unknown, Loop: p.Loop, Protected: p.Protected,
			Reserved: p.Reserved, ChangedLately: p.ChangedLately, Newest: p.Newest}
		for _, h := range p.Hops {
			ah := apiTieHop{Router: h.Router, Levels: h.Levels, Sends: h.Sends, Protect: h.Protect, Reserve: h.Reserve}
			if v.on(h.Router).Source(h.Src) {
				e := end(tieplan.End{Router: h.Router, N: h.Src}, false)
				ah.Src = &e
			}
			if v.on(h.Router).Destination(h.Dst) {
				e := end(tieplan.End{Router: h.Router, N: h.Dst}, true)
				ah.Dst, ah.Changed, ah.Before, ah.ProtectSince = &e, h.Changed, h.Before, h.ProtectSince
			} else if h.Changed != nil {
				// Newest would give the change time of this hidden end.
				ap.Newest = nil
			}
			if h.Tie != "" {
				t := ref(h.Tie)
				ah.Tie = &t
			}
			if c := h.Now; c != nil {
				ac := &apiTieChain{Via: []apiTieRef{}, Mixed: c.Mixed, Nothing: c.Nothing, Unknown: c.Unknown, Loop: c.Loop}
				if c.Origin != nil {
					o := end(*c.Origin, false)
					ac.Origin = &o
				}
				for _, id := range c.Via {
					ac.Via = append(ac.Via, ref(id))
				}
				if c.Mixed {
					if in == nil {
						x := withOneOffs(a.tieInput(rt), nil, []tieRoute{r})
						in = &x
					}
					for _, l := range h.Levels {
						var o *apiTieEndOut
						if lc := tieplan.Via(*in, tieplan.End{Router: h.Router, N: h.Dst}, l); lc.Origin != nil {
							e := end(*lc.Origin, false)
							o = &e
						}
						ac.Levels = append(ac.Levels, o)
					}
				}
				ah.Now = ac
			}
			ap.Hops = append(ap.Hops, ah)
		}
		for _, e := range p.Affected {
			if v.on(e.Router).Destination(e.N) {
				ap.Affected = append(ap.Affected, end(e, true))
			} else {
				ap.Others++
			}
		}
		out.Paths = append(out.Paths, ap)
	}
	for _, why := range res.Reasons {
		out.Reasons = append(out.Reasons, apiTieReason{why, tieName[why.Tie]})
	}
	return out
}

// tieRouteRights: rights cover the typed source and destination only (docs/tielines-design.md, Rights). Hops
// through saved tie lines need none. The upstream destination of a One-off tie line needs none either (owner
// decision); the Protect check still covers it.
func (a *App) tieRouteRights(rt *runtime, rights config.Rights, routes []tieRoute) *apiProblem {
	// Each typed end is matched with its own router: a route across routers has its source on another router.
	out := func(what string, e tieplan.End) *apiProblem {
		if rights.Hidden() {
			return problem(http.StatusForbidden, "outside_rights", "%s is outside your rights", what)
		}
		return problem(http.StatusForbidden, "outside_rights", "%s %d is outside your rights", what, e.N+rt.routerOf(a.rightsUID(rt, e.Router)).NumberOffset)
	}
	for _, r := range routes {
		if !rights.AllowsDst(config.Ref{Router: a.rightsUID(rt, r.dst.Router), N: r.dst.N}) {
			return out("destination", r.dst)
		}
		for _, ui := range r.levels {
			if !rights.AllowsLevel(a.rightsUID(rt, r.dst.Router), ui) {
				return problem(http.StatusForbidden, "outside_rights", "level %s is outside your rights", rt.cfg.LevelLabel(ui))
			}
		}
		if !rights.AllowsSource(config.Ref{Router: a.rightsUID(rt, r.src.Router), N: r.src.N}) {
			return out("source", r.src)
		}
	}
	// After the ends, so a caller outside both learns of the ends first.
	if !rights.AllowsTieLines() {
		return problem(http.StatusForbidden, "tie_lines_off", "you do not have the Use tie lines right, so Sode cannot send a route across routers")
	}
	return nil
}
