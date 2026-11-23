package app

import (
	"bytes"
	"cmp"
	"encoding/json"
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// Via for the page (docs/tielines-design.md, Via). The hello gives each router its Via rows. A `viaRows` message sends
// the whole set of one router again when it changes. Browsers see everything (GLOSSARY.md, Visibility), so no row is
// left out and no origin is blanked. web/core.js keeps them (`routerData(uid).via`).

// viaMsg replaces the Via rows of one router on the page.
type viaMsg struct {
	T         string        `json:"t"` // "viaRows"
	RouterUID string        `json:"routerUid"`
	Rows      []apiRouteRow `json:"rows"`
}

// pageViaRows gives each tally row of unit u whose source a tie line feeds, with its Via, in dst and level order.
// x nil (no tie lines) gives none. Caller holds mu.
func (a *App) pageViaRows(u *routerUnit, x *viaTrace) []apiRouteRow {
	rows := []apiRouteRow{}
	if x == nil {
		return rows
	}
	rv := a.viewOf(config.Rights{}).on(u.uid())
	for d, t := range u.router.tally {
		for l, s := range t {
			row := apiRouteRow{Dst: d, Level: l, Src: s}
			if x.fill(rv, &row); len(row.Via) > 0 {
				rows = append(rows, row)
			}
		}
	}
	slices.SortFunc(rows, func(p, q apiRouteRow) int { return cmp.Or(cmp.Compare(p.Dst, q.Dst), cmp.Compare(p.Level, q.Level)) })
	return rows
}

// pageViaState: what flushPageVia last sent, so it sends a router only when its rows change. Guarded by App.mu.
type pageViaState struct {
	rt   *runtime
	gen  int
	sent map[string][]byte // router unit UID → the rows as sent
}

// flushPageVia sends a `viaRows` message for each router whose rows changed. tallied: a tally changed on some router since
// the last flush. Caller holds mu.
func (a *App) flushPageVia(tallied bool) {
	st := &a.pageVia
	rt := a.r()
	if !tallied && st.rt == rt && st.gen == a.viaGen {
		return
	}
	st.rt, st.gen = rt, a.viaGen
	if st.sent == nil {
		st.sent = map[string][]byte{}
	}
	x := a.newViaTrace(rt)
	for _, u := range a.unitsInOrder(rt) {
		rows := a.pageViaRows(u, x)
		b, _ := json.Marshal(rows)
		was, ok := st.sent[u.uid()]
		if (!ok && len(rows) == 0) || (ok && bytes.Equal(was, b)) {
			continue
		}
		st.sent[u.uid()] = b
		a.broadcastPage(viaMsg{T: "viaRows", RouterUID: u.uid(), Rows: rows})
	}
}

// broadcastPage sends v to the pages only: the API streams carry Via on their own tally frames. Caller holds mu.
func (a *App) broadcastPage(v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	for c := range a.clients {
		a.deliver(c, b)
	}
}
