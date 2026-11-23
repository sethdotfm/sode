// Package tieplan is the tie path planner (docs/tielines-design.md, "Routes through tie lines"). It is pure: it reads
// plain data and returns plain data. Thus the 409 bodies, the stale check, Via and the Tie line picker all read one
// result. Nothing here sends to a router or reads the clock.
//
// Plan gives the ranked paths for one route across routers. Via gives the Via chain of a destination. The caller
// holds the lock that guards the tally maps while it calls them.
package tieplan

import (
	"cmp"
	"slices"
	"time"
)

// End is one end of a crosspoint: a router UID and a number from the router.
type End struct {
	Router string `json:"router"`
	N      int    `json:"n"`
}

// Router is the state of one router as Sode has it now.
type Router struct {
	UID    string `json:"uid"`
	Levels int    `json:"levels"` // UI levels 1..Levels
	Up     bool   `json:"up"`
	// Tally: destination → UI level → source (-1: nothing routed). Nil: Sode has no state for this router. A
	// destination that is not in the map is unknown.
	Tally map[int]map[int]int `json:"tally"`
}

// Tie is a tie line: From is a destination, To a source on another router. OneOff is a One-off tie line that a user
// stated for this take only. Sode does not trust the tally downstream of it.
type Tie struct {
	ID     string `json:"id"`
	From   End    `json:"from"`
	To     End    `json:"to"`
	OneOff bool   `json:"oneOff,omitempty"`
}

// Hold is a Protect on a destination, on any level. By says who holds it.
type Hold struct {
	At End    `json:"at"`
	By string `json:"by"`
}

// Input is everything the planner reads.
type Input struct {
	Routers  []Router `json:"routers"`
	Ties     []Tie    `json:"ties"`
	Protects []Hold   `json:"protects,omitempty"`
	// Reserves: tie line id → the name of the panel that Reserved it.
	Reserves map[string]string `json:"reserves,omitempty"`
	// Changes: tie line id → the last change of the tie line.
	Changes map[string]time.Time `json:"changes,omitempty"`
	// Now is when the plan starts. Started is when Sode started: a change before it shows as "before HH:MM".
	Now     time.Time `json:"now,omitzero"`
	Started time.Time `json:"started,omitzero"`
	// HopWeight is the cost of each hop that sends upstream (config tieHopWeight).
	HopWeight int `json:"hopWeight"`
	// Taken: tie line id → the source that an earlier route of the same take puts on it. The planner does not offer
	// such a tie line for a different source, and shares it for the same source.
	Taken map[string]End `json:"taken,omitempty"`
}

// Route is the route asked for: Levels are the typed levels, levels of the router of Dst.
type Route struct {
	Src    End   `json:"src"`
	Dst    End   `json:"dst"`
	Levels []int `json:"levels"`
}

// Lately is the 5-minute rule of the ranking.
const Lately = 5 * time.Minute

// maxPaths stops the search in a large mesh. The ranking then sees only the first paths found.
const maxPaths = 500

// Result: Paths in rank order. Reasons says why each path that Sode left out is gone. A Plan with no Usable path is
// a no_path.
type Result struct {
	Paths   []Path   `json:"paths"`
	Reasons []Reason `json:"reasons"`
}

// Usable: the first path that a user can choose, or nil. A path with a Protected tie line that it must change is
// not usable.
func (r Result) Usable() *Path {
	for i := range r.Paths {
		if !r.Paths[i].Protected {
			return &r.Paths[i]
		}
	}
	return nil
}

// Path is one way from the source to the destination: one Hop for each router.
type Path struct {
	Hops []Hop `json:"hops"`
	// Cost is the number of Affected destinations plus HopWeight for each hop that sends upstream.
	Cost     int       `json:"cost"`
	Affected []End     `json:"affected"`
	Unknown  []Unknown `json:"unknown"`
	Loop     bool      `json:"loop"`
	// Protected and Reserved: a tie line that the path changes is Protected, or Reserved by a panel.
	Protected bool `json:"protected"`
	Reserved  bool `json:"reserved"`
	// ChangedLately: a tie line of the path changed in the last 5 minutes. Newest is its newest change, if Sode knows one.
	ChangedLately bool       `json:"changedLately"`
	Newest        *time.Time `json:"newest"`
}

// Hop is one crosspoint on one router. Every hop but the last feeds the tie line Tie and switches all levels of its
// router. The last hop switches the typed levels of the destination asked for.
type Hop struct {
	Router string `json:"router"`
	Src    int    `json:"src"`
	Dst    int    `json:"dst"`
	Levels []int  `json:"levels"`
	Tie    string `json:"tie"`
	// Sends is false when the tie line already carries the wanted source, or the last hop is already routed.
	Sends bool `json:"sends"`
	// Now is what the tie line carries now. Blank on the last hop.
	Now     *Chain     `json:"now"`
	Protect string     `json:"protect"`
	Reserve string     `json:"reserve"`
	Changed *time.Time `json:"changed"`
	// Before: Sode started after Changed. Thus it is possible that the tie line changed again while Sode was down.
	Before bool `json:"before"`
}

// Unknown is a part of the downstream that Sode cannot count. Why is "down" (a router that is down), "no_state" (a
// router with no state) or "one_off" (the far side of a One-off tie line).
type Unknown struct {
	Router string `json:"router,omitempty"`
	Tie    string `json:"tie,omitempty"`
	Why    string `json:"why"`
}

// Reason says why a path was left out, or why there is none. Why is one of these:
//   - "no_tie_line": no tie lines join From and To.
//   - "router_down": Router is down.
//   - "protected": By holds a Protect on Tie.
//   - "taken": Tie carries another source in this take.
type Reason struct {
	Why    string `json:"why"`
	From   string `json:"from,omitempty"`
	To     string `json:"to,omitempty"`
	Router string `json:"router,omitempty"`
	Tie    string `json:"tie,omitempty"`
	By     string `json:"by,omitempty"`
}

// Chain is the Via chain of a destination: the tie lines from the origin down, upstream first. Origin is nil when
// Nothing, Unknown or Loop is set.
type Chain struct {
	Origin *End     `json:"origin"`
	Via    []string `json:"via"`
	// Mixed: the levels upstream of a tie line have different sources.
	Mixed   bool   `json:"mixed"`
	Nothing bool   `json:"nothing"`
	Unknown string `json:"unknown"` // the router UID with no state, or ""
	Loop    bool   `json:"loop"`
}

func (c Chain) same(o Chain) bool {
	return c.Nothing == o.Nothing && c.Unknown == o.Unknown && c.Loop == o.Loop &&
		(c.Origin == nil) == (o.Origin == nil) && (c.Origin == nil || *c.Origin == *o.Origin) && slices.Equal(c.Via, o.Via)
}

// carries: the chain brings src on every level, and nothing is unknown.
func (c Chain) carries(src End) bool {
	return c.Origin != nil && *c.Origin == src && !c.Mixed && c.Unknown == "" && !c.Loop
}

type planner struct {
	in      Input
	routers map[string]*Router
	byTo    map[End][]int // source → the ties that feed it
	byFrom  map[End][]int // destination → the ties it feeds
	holds   map[End]string
	wholes  map[End]Chain // whole, as a loop upstream is a loop from every end on it
}

func newPlanner(in Input) *planner {
	p := &planner{in: in, routers: map[string]*Router{}, byTo: map[End][]int{}, byFrom: map[End][]int{}, holds: map[End]string{}, wholes: map[End]Chain{}}
	for i := range in.Routers {
		p.routers[in.Routers[i].UID] = &in.Routers[i]
	}
	for i, t := range in.Ties {
		p.byTo[t.To] = append(p.byTo[t.To], i)
		p.byFrom[t.From] = append(p.byFrom[t.From], i)
	}
	for _, h := range in.Protects {
		if _, ok := p.holds[h.At]; !ok {
			p.holds[h.At] = h.By
		}
	}
	return p
}

// state: the tally of r, or why it is unknown.
func (p *planner) state(uid string) (*Router, string) {
	r, ok := p.routers[uid]
	switch {
	case !ok || r.Tally == nil && r.Up:
		return nil, "no_state"
	case !r.Up:
		return nil, "down"
	}
	return r, ""
}

// Via gives the Via chain of dst on one UI level of its router.
func Via(in Input, dst End, level int) Chain {
	return newPlanner(in).via(dst, level, map[int]bool{})
}

func (p *planner) via(dst End, level int, seen map[int]bool) Chain {
	r, why := p.state(dst.Router)
	if why != "" {
		return Chain{Unknown: dst.Router}
	}
	ls, ok := r.Tally[dst.N]
	src, ok2 := ls[level]
	if !ok || !ok2 {
		return Chain{Unknown: dst.Router}
	}
	if src < 0 {
		return Chain{Nothing: true}
	}
	at := End{dst.Router, src}
	ties := p.byTo[at]
	if len(ties) == 0 {
		return Chain{Origin: &at}
	}
	// Two tie lines into one source would be two cables on one input; the first one wins.
	ti := ties[0]
	if seen[ti] {
		return Chain{Loop: true}
	}
	seen[ti] = true
	defer delete(seen, ti)
	up := p.whole(p.in.Ties[ti].From, seen)
	up.Via = append(slices.Clone(up.Via), p.in.Ties[ti].ID)
	return up
}

// whole is the chain of dst on all levels of its router, as a tie line carries every level. Levels with different
// chains make it Mixed; the lowest level gives the origin.
func (p *planner) whole(dst End, seen map[int]bool) Chain {
	if c, ok := p.wholes[dst]; ok {
		return c
	}
	r, why := p.state(dst.Router)
	if why != "" {
		return Chain{Unknown: dst.Router}
	}
	var out Chain
	for l := 1; l <= max(r.Levels, 1); l++ {
		c := p.via(dst, l, seen)
		if l == 1 {
			out = c
			continue
		}
		if !c.same(out) {
			out.Mixed = true
		}
		out.Mixed = out.Mixed || c.Mixed
	}
	p.wholes[dst] = out
	return out
}

// Plan gives the ranked paths for rt.
func Plan(in Input, rt Route) Result {
	p := newPlanner(in)
	res := Result{Paths: []Path{}, Reasons: []Reason{}}
	var found [][]int
	p.search(rt.Src.Router, rt.Dst.Router, map[string]bool{rt.Src.Router: true}, nil, &found)
	if len(found) == 0 {
		res.Reasons = append(res.Reasons, Reason{Why: "no_tie_line", From: rt.Src.Router, To: rt.Dst.Router})
		return res
	}
	reason := func(r Reason) {
		if !slices.Contains(res.Reasons, r) {
			res.Reasons = append(res.Reasons, r)
		}
	}
	for _, ties := range found {
		if down := p.downOn(rt, ties); down != "" {
			reason(Reason{Why: "router_down", Router: down})
			continue
		}
		if ti := p.takenOther(rt, ties); ti >= 0 {
			reason(Reason{Why: "taken", Tie: in.Ties[ti].ID})
			continue
		}
		path := p.path(rt, ties)
		for _, h := range path.Hops {
			if h.Sends && h.Protect != "" {
				reason(Reason{Why: "protected", Tie: h.Tie, By: h.Protect})
			}
		}
		res.Paths = append(res.Paths, path)
	}
	slices.SortStableFunc(res.Paths, rank)
	return res
}

// Through gives the path of rt through the tie lines ids. The ids go in order from the router of the source to the
// router of the destination. It is the path that Plan gives for that chain, so the take sends what the caller saw.
// A nil Reason means the chain is a path. Else Why is one of these:
//   - "unknown_tie": Tie is not a tie line.
//   - "not_joined": Tie does not start on the router where the chain is, or goes back to a router. With no Tie, the
//     chain ends on router From, not on To.
//   - "router_down": Router is down.
func Through(in Input, rt Route, ids []string) (Path, *Reason) {
	p := newPlanner(in)
	at, visited := rt.Src.Router, map[string]bool{rt.Src.Router: true}
	ties := make([]int, len(ids))
	for i, id := range ids {
		ti := slices.IndexFunc(in.Ties, func(t Tie) bool { return t.ID == id })
		switch {
		case ti < 0:
			return Path{}, &Reason{Why: "unknown_tie", Tie: id}
		case in.Ties[ti].From.Router != at || visited[in.Ties[ti].To.Router]:
			return Path{}, &Reason{Why: "not_joined", Tie: id}
		}
		ties[i] = ti
		at = in.Ties[ti].To.Router
		visited[at] = true
	}
	if at != rt.Dst.Router {
		return Path{}, &Reason{Why: "not_joined", From: at, To: rt.Dst.Router}
	}
	if down := p.downOn(rt, ties); down != "" {
		return Path{}, &Reason{Why: "router_down", Router: down}
	}
	return p.path(rt, ties), nil
}

// search finds each chain of tie lines from router at to router to that visits no router two times.
func (p *planner) search(at, to string, visited map[string]bool, ties []int, found *[][]int) {
	if len(*found) >= maxPaths {
		return
	}
	if at == to {
		*found = append(*found, slices.Clone(ties))
		return
	}
	for i, t := range p.in.Ties {
		if t.From.Router != at || visited[t.To.Router] {
			continue
		}
		visited[t.To.Router] = true
		p.search(t.To.Router, to, visited, append(ties, i), found)
		delete(visited, t.To.Router)
	}
}

// downOn: the first router of the path that is down, or "".
func (p *planner) downOn(rt Route, ties []int) string {
	for _, uid := range p.routersOf(rt, ties) {
		if r, ok := p.routers[uid]; !ok || !r.Up {
			return uid
		}
	}
	return ""
}

func (p *planner) routersOf(rt Route, ties []int) []string {
	out := []string{rt.Src.Router}
	for _, ti := range ties {
		out = append(out, p.in.Ties[ti].To.Router)
	}
	return out
}

func (p *planner) takenOther(rt Route, ties []int) int {
	for _, ti := range ties {
		if s, ok := p.in.Taken[p.in.Ties[ti].ID]; ok && s != rt.Src {
			return ti
		}
	}
	return -1
}

func allLevels(r *Router) []int {
	out := make([]int, max(r.Levels, 1))
	for i := range out {
		out[i] = i + 1
	}
	return out
}

func (p *planner) path(rt Route, ties []int) Path {
	path := Path{Affected: []End{}, Unknown: []Unknown{}}
	// The most downstream tie line that already carries the source: nothing upstream of it needs a send.
	carried := -1
	nows := make([]Chain, len(ties))
	for i, ti := range ties {
		t := p.in.Ties[ti]
		nows[i] = p.whole(t.From, map[int]bool{})
		if s, ok := p.in.Taken[t.ID]; (ok && s == rt.Src) || (!t.OneOff && nows[i].carries(rt.Src)) {
			carried = i
		}
	}
	src := rt.Src.N
	var newest time.Time
	for i, ti := range ties {
		t := p.in.Ties[ti]
		now := nows[i]
		h := Hop{Router: t.From.Router, Src: src, Dst: t.From.N, Levels: allLevels(p.routers[t.From.Router]), Tie: t.ID,
			Sends: i > carried, Now: &now, Protect: p.holds[t.From], Reserve: p.in.Reserves[t.ID]}
		if at, ok := p.in.Changes[t.ID]; ok {
			h.Changed = &at
			h.Before = at.Before(p.in.Started)
			path.ChangedLately = path.ChangedLately || p.in.Now.Sub(at) < Lately
			if at.After(newest) {
				newest = at
			}
		}
		if h.Sends {
			path.Protected = path.Protected || h.Protect != ""
			path.Reserved = path.Reserved || h.Reserve != ""
			path.Cost += p.in.HopWeight
		}
		path.Hops = append(path.Hops, h)
		src = t.To.N
	}
	if !newest.IsZero() {
		path.Newest = &newest
	}
	last := Hop{Router: rt.Dst.Router, Src: src, Dst: rt.Dst.N, Levels: slices.Clone(rt.Levels), Sends: carried < len(ties)-1}
	if !last.Sends {
		last.Sends = !p.routed(rt.Dst, src, rt.Levels)
	}
	path.Hops = append(path.Hops, last)
	p.affect(&path, rt, ties)
	path.Cost += len(path.Affected)
	return path
}

// routed: dst routes src on every level in ls now.
func (p *planner) routed(dst End, src int, ls []int) bool {
	r, why := p.state(dst.Router)
	if why != "" {
		return false
	}
	for _, l := range ls {
		if s, ok := r.Tally[dst.N][l]; !ok || s != src {
			return false
		}
	}
	return true
}

// affect traces the change downstream of each tie line that the path changes, and of the destination asked for. It
// goes through every tie line, with a visited set. A trace that comes back to a tie line it passed gives the loop mark.
func (p *planner) affect(path *Path, rt Route, ties []int) {
	visited := map[int]bool{}
	onPath := map[End]bool{}
	for _, ti := range ties {
		visited[ti] = true
		onPath[p.in.Ties[ti].From] = true
	}
	seen := map[End]bool{}
	unknown := func(u Unknown) {
		if !slices.Contains(path.Unknown, u) {
			path.Unknown = append(path.Unknown, u)
		}
	}
	var trace func(ti int)
	trace = func(ti int) {
		t := p.in.Ties[ti]
		if t.OneOff {
			unknown(Unknown{Tie: t.ID, Why: "one_off"})
			return
		}
		r, why := p.state(t.To.Router)
		if why != "" {
			unknown(Unknown{Router: t.To.Router, Why: why})
			return
		}
		for _, d := range sortedKeys(r.Tally) {
			if !slices.Contains(mapValues(r.Tally[d]), t.To.N) {
				continue
			}
			at := End{t.To.Router, d}
			if at == rt.Dst {
				continue
			}
			// The take routes the upstream end of each tie line of the path again, so it is a hop, not an
			// Affected destination.
			if onPath[at] {
				continue
			}
			if !seen[at] {
				seen[at] = true
				path.Affected = append(path.Affected, at)
			}
			for _, tj := range p.byFrom[at] {
				if visited[tj] {
					path.Loop = true
					continue
				}
				visited[tj] = true
				trace(tj)
			}
		}
	}
	for i, ti := range ties {
		if path.Hops[i].Sends {
			trace(ti)
		}
	}
	if path.Hops[len(path.Hops)-1].Sends {
		for _, tj := range p.byFrom[rt.Dst] {
			if visited[tj] {
				path.Loop = true
				continue
			}
			visited[tj] = true
			trace(tj)
		}
	}
}

func sortedKeys[V any](m map[int]V) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

func mapValues(m map[int]int) []int {
	out := make([]int, 0, len(m))
	for _, v := range m {
		out = append(out, v)
	}
	return out
}

func bit(b bool) int {
	if b {
		return 1
	}
	return 0
}

// rank: no unknown part; no Protected or Reserved tie line; lower cost; no change in the last 5 minutes; the older
// newest change. Then fewer hops and the tie line ids, so the order never depends on the search.
func rank(a, b Path) int {
	newest := func(p Path) time.Time {
		if p.Newest == nil {
			return time.Time{}
		}
		return *p.Newest
	}
	return cmp.Or(
		cmp.Compare(bit(len(a.Unknown) > 0), bit(len(b.Unknown) > 0)),
		cmp.Compare(bit(a.Protected || a.Reserved), bit(b.Protected || b.Reserved)),
		cmp.Compare(a.Cost, b.Cost),
		cmp.Compare(bit(a.ChangedLately), bit(b.ChangedLately)),
		newest(a).Compare(newest(b)),
		cmp.Compare(len(a.Hops), len(b.Hops)),
		slices.CompareFunc(a.Hops, b.Hops, func(x, y Hop) int { return cmp.Compare(x.Tie, y.Tie) }),
	)
}
