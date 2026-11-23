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

// tieInput is the planner's input: every router, saved tie line and Protect as Sode has them now. The maps are
// the live ones, so the caller holds mu while it plans. Reserves come with the ticket that adds them; until then
// nothing is Reserved. The last change times are the stamps of tiechanges.go.
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
		for _, dst := range slices.Sorted(maps.Keys(u.router.protect)) {
			for _, l := range slices.Sorted(maps.Keys(u.router.protect[dst])) {
				in.Protects = append(in.Protects, tieplan.Hold{At: tieplan.End{Router: u.uid, N: dst}, By: holderText(u.router.protect[dst][l], dev)})
				break
			}
		}
	}
	in.Changes, in.Found = map[string]time.Time{}, map[string]bool{}
	for id, st := range a.tieChanges.snapshot(rt.cfg.TieLines) {
		in.Changes[id], in.Found[id] = st.At, st.Found
	}
	for _, t := range rt.cfg.TieLines {
		fr, ok1 := a.unitUID(rt, t.From.Router)
		to, ok2 := a.unitUID(rt, t.To.Router)
		if ok1 && ok2 {
			in.Ties = append(in.Ties, tieplan.Tie{ID: t.ID, From: tieplan.End{Router: fr, N: t.From.N}, To: tieplan.End{Router: to, N: t.To.N}})
		}
	}
	return in
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

// planTieRoutes plans each route in order. A tie line that an earlier route changes is Taken for the later ones.
// Thus two destinations that want one source share it. No route gets a tie line that carries another source. Caller
// holds mu.
func (a *App) planTieRoutes(rt *runtime, routes []tieRoute) []tieplan.Result {
	in := a.tieInput(rt)
	in.Taken = map[string]tieplan.End{}
	out := make([]tieplan.Result, len(routes))
	for i, r := range routes {
		out[i] = tieplan.Plan(in, tieplan.Route{Src: r.src, Dst: r.dst, Levels: r.levels})
		if p := out[i].Usable(); p != nil {
			for _, h := range p.Hops {
				if h.Tie != "" && h.Sends {
					in.Taken[h.Tie] = r.src
				}
			}
		}
	}
	return out
}

// apiTieProblem answers routes across routers: 409 no_path when a route has no usable path, else 409
// tie_line_needed with the ranked paths of each.
func (a *App) apiTieProblem(rt *runtime, v apiView, routes []tieRoute) *apiProblem {
	a.mu.Lock()
	defer a.mu.Unlock()
	plans := a.planTieRoutes(rt, routes)
	body := apiTieBody{Routes: make([]apiTieRouteOut, len(routes))}
	code, msg := "tie_line_needed", "a route across routers needs a tie line for each hop: send the route again with ties. The paths are in routes, cheapest first"
	for i, r := range routes {
		body.Routes[i] = a.apiTieRoute(rt, v, r, plans[i])
		if plans[i].Usable() == nil {
			code, msg = "no_path", "no tie line path is usable for a route across routers; reasons gives each reason"
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
}

type apiTieChain struct {
	Origin  *apiTieEndOut `json:"origin"`
	Via     []apiTieRef   `json:"via"`
	Mixed   bool          `json:"mixed"`
	Nothing bool          `json:"nothing"`
	Unknown string        `json:"unknown,omitempty"`
	Loop    bool          `json:"loop"`
}

type apiTieHop struct {
	Router  string       `json:"router"`
	Src     apiTieEndOut `json:"src"`
	Dst     apiTieEndOut `json:"dst"`
	Levels  []int        `json:"levels"`
	Tie     *apiTieRef   `json:"tie"`
	Sends   bool         `json:"sends"`
	Now     *apiTieChain `json:"now,omitempty"`
	Protect string       `json:"protect,omitempty"`
	Reserve string       `json:"reserve,omitempty"`
	Changed *time.Time   `json:"changed,omitempty"`
	Before  bool         `json:"before,omitempty"`
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
	tieName := map[string]string{}
	for _, t := range rt.cfg.TieLines {
		tieName[t.ID] = t.Name
	}
	ref := func(id string) apiTieRef { return apiTieRef{id, tieName[id]} }
	out := apiTieRouteOut{Route: r.index, Src: end(r.src, false), Dst: end(r.dst, true), Levels: r.levels, Paths: []apiTiePath{}, Reasons: []apiTieReason{}}
	for i, p := range res.Paths {
		if i == apiTiePaths {
			out.More = len(res.Paths) - i
			break
		}
		ap := apiTiePath{Cost: p.Cost, Affected: []apiTieEndOut{}, Unknown: p.Unknown, Loop: p.Loop, Protected: p.Protected,
			Reserved: p.Reserved, ChangedLately: p.ChangedLately, Newest: p.Newest}
		for _, h := range p.Hops {
			ah := apiTieHop{Router: h.Router, Src: end(tieplan.End{Router: h.Router, N: h.Src}, false), Dst: end(tieplan.End{Router: h.Router, N: h.Dst}, true),
				Levels: h.Levels, Sends: h.Sends, Protect: h.Protect, Reserve: h.Reserve, Changed: h.Changed, Before: h.Before}
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
// through saved tie lines need none.
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
			if !rights.AllowsLevel(ui) {
				return problem(http.StatusForbidden, "outside_rights", "level %s is outside your rights", rt.cfg.LevelLabel(ui))
			}
		}
		if !rights.AllowsSource(config.Ref{Router: a.rightsUID(rt, r.src.Router), N: r.src.N}) {
			return out("source", r.src)
		}
	}
	return nil
}
