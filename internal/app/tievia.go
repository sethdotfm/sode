package app

import (
	"cmp"
	"maps"
	"slices"

	"github.com/sethdotfm/sode/internal/tieplan"
)

// Via in GET /tally and the stream (docs/tielines-design.md, "Via" and "API tally and stream"). A tally row keeps
// src, the real crosspoint on the router of the destination. A row whose source is the far end of a tie line also
// carries its Via chain, worked out for its own level. When something upstream of a tie line changes, the stream
// sends the rows of every destination downstream of it again. An External on the upstream end of a tie line notes
// the destinations that it also changes (Entry.Also).

// viaTrace fills the Via of tally rows from one tieplan.Input. Caller holds mu while it uses one.
type viaTrace struct {
	in    tieplan.Input
	tr    *tieplan.Tracer
	names map[string]string // tie line id → Tie line name
}

// newViaTrace is nil when there are no tie lines, so a router with none pays nothing. One-off tie lines that Sode
// keeps count (tieoneoff.go). Caller holds mu.
func (a *App) newViaTrace(rt *runtime) *viaTrace {
	if len(rt.cfg.TieLines) == 0 && len(a.liveOneOffs(rt)) == 0 {
		return nil
	}
	in := a.tieInput(rt)
	_, names := a.tieLinesNow(rt)
	return &viaTrace{in: in, tr: tieplan.NewTracer(in), names: names}
}

// fill adds the Via chain to a row of router rv.uid when a tie line feeds its source. Hidden visibility blanks an
// origin that the caller cannot see and keeps the Tie line names.
func (x *viaTrace) fill(rv routerView, row *apiRouteRow) {
	if x == nil || row.Src < 0 || !x.tr.Fed(tieplan.End{Router: rv.uid, N: row.Src}) {
		return
	}
	c := x.tr.Via(tieplan.End{Router: rv.uid, N: row.Dst}, row.Level)
	if len(c.Via) == 0 {
		return
	}
	seen := func(o *tieplan.End) *tieplan.End {
		if o == nil || !rv.v.on(o.Router).Source(o.N) {
			return nil
		}
		e := *o
		return &e
	}
	row.Origin, row.Mixed, row.Nothing, row.Unknown, row.Loop = seen(c.Origin), c.Mixed, c.Nothing, c.Unknown, c.Loop
	for _, id := range c.Via {
		row.Via = append(row.Via, tieRef(id, x.names))
	}
	if c.Mixed {
		// The level pie of the nearest tie line: what each level of its upstream end carries.
		near := c.Via[len(c.Via)-1]
		for _, o := range x.tr.UpLevels(near) {
			row.Levels = append(row.Levels, seen(o))
		}
		for _, t := range x.in.Ties {
			if t.ID == near {
				row.LevelsRouter = t.From.Router
			}
		}
	}
}

// tieFrom: destination dst of unit u is the upstream end of a tie line, a kept One-off tie line too. Caller holds mu.
// It runs for each crosspoint change, so it reads the One-off tie lines without letting any go.
func (a *App) tieFrom(u *routerUnit, rt *runtime, dst int) bool {
	for _, t := range rt.cfg.TieLines {
		if t.From.N == dst {
			if uid, ok := a.unitUID(rt, t.From.Router); ok && uid == u.uid() {
				return true
			}
		}
	}
	return slices.Contains(a.oneOffs.froms(), tieplan.End{Router: u.uid(), N: dst})
}

// viaTouch notes a change on destination dst of unit u. Any level counts, the first read too: the Via downstream
// goes from unknown to known. Caller holds mu.
func (a *App) viaTouch(u *routerUnit, rt *runtime, dst int) {
	if a.tieFrom(u, rt, dst) {
		a.viaTouchAt(u, dst)
	}
}

// viaTouchAt notes a change of the Via downstream of destination dst of unit u. Caller holds mu.
func (a *App) viaTouchAt(u *routerUnit, dst int) {
	if a.viaPend == nil {
		a.viaPend = map[tieplan.End]bool{}
	}
	a.viaPend[tieplan.End{Router: u.uid(), N: dst}] = true
	a.viaGen++
}

// viaTouchRouter notes that unit u went up or down, or lost its state: the Via of every tie line from it changes.
// Caller holds mu.
func (a *App) viaTouchRouter(u *routerUnit, rt *runtime) {
	for _, t := range rt.cfg.TieLines {
		if uid, ok := a.unitUID(rt, t.From.Router); ok && uid == u.uid() {
			a.viaTouchAt(u, t.From.N)
		}
	}
	for _, e := range a.oneOffs.froms() {
		if e.Router == u.uid() {
			a.viaTouchAt(u, e.N)
		}
	}
}

// flushVia sends the tally rows of every destination downstream of a changed tie line to the API streams of its
// router. Each row is sent again whole, with its new Via. Caller holds mu.
func (a *App) flushVia() {
	pend := a.viaPend
	a.viaPend = nil
	if len(pend) == 0 || len(a.streams) == 0 {
		return
	}
	rt := a.r()
	x := a.newViaTrace(rt)
	if x == nil {
		return
	}
	down := map[string]map[int]bool{} // router unit UID → destinations
	for _, at := range slices.SortedFunc(maps.Keys(pend), func(p, q tieplan.End) int {
		return cmp.Or(cmp.Compare(p.Router, q.Router), cmp.Compare(p.N, q.N))
	}) {
		for _, r := range tieplan.Downstream(x.in, at) {
			if down[r.At.Router] == nil {
				down[r.At.Router] = map[int]bool{}
			}
			down[r.At.Router][r.At.N] = true
		}
	}
	for uid, dsts := range down {
		u, ok := a.unit(uid)
		if !ok {
			continue
		}
		for st := range a.streams {
			if !st.topics["tally"] || st.unit != u {
				continue
			}
			rv := st.view.on(u.uid())
			rows := []apiRouteRow{}
			for _, d := range slices.Sorted(maps.Keys(dsts)) {
				for _, l := range slices.Sorted(maps.Keys(u.router.tally[d])) {
					s := u.router.tally[d][l]
					if !rv.Route(d, l, s) || !x.tr.Fed(tieplan.End{Router: u.uid(), N: s}) {
						continue
					}
					row := apiRouteRow{Dst: d, Level: l, Src: s}
					x.fill(rv, &row)
					rows = append(rows, row)
				}
			}
			if len(rows) > 0 {
				a.streamQueue(st, streamMsg{T: "tally", Topic: "tally", Data: map[string]any{"routes": rows}})
			}
		}
	}
}

// TieAlso is a destination that an External on the upstream end of a tie line also changes. Name is its name at that
// time. Via is the tie lines on the way, upstream first.
type TieAlso struct {
	Router string      `json:"router"`
	N      int         `json:"n"`
	Name   string      `json:"name,omitempty"`
	Via    []apiTieRef `json:"via"`
}

// noteAlso looks at each External route entry of unit u on the upstream end of a tie line. It adds the destinations
// that the entry also changes, as they are now. Caller holds mu.
func (a *App) noteAlso(u *routerUnit, es []*Entry) {
	rt := a.r()
	var x *viaTrace
	for _, e := range es {
		if e.Src < 0 || !a.tieFrom(u, rt, e.Dst) {
			continue
		}
		if x == nil {
			x = a.newViaTrace(rt)
		}
		for _, r := range tieplan.Downstream(x.in, tieplan.End{Router: u.uid(), N: e.Dst}) {
			al := TieAlso{Router: r.At.Router, N: r.At.N, Via: []apiTieRef{}}
			if du, ok := a.unit(r.At.Router); ok {
				al.Name = du.router.dstNames[r.At.N]
			}
			for _, id := range r.Via {
				al.Via = append(al.Via, tieRef(id, x.names))
			}
			e.Also = append(e.Also, al)
		}
	}
}
