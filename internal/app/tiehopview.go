package app

import (
	"encoding/json"
	"slices"
)

// A caller with hidden visibility gets null for a hop end that it cannot see, as in GET /tielines and the 409 bodies.
// The hop keeps its tie line id and name. apiView.hop marks what to leave out, and only MarshalJSON reads the mark, so
// Retry and Redo (planTieRetry, tieRedoProblem) still read the whole hop.
type hopHide struct {
	src, dst, from, to bool
	// prev: the levels of Prev whose source the caller cannot see.
	prev []int
}

func (h TieHop) MarshalJSON() ([]byte, error) {
	type plain TieHop
	if h.hide == nil {
		return json.Marshal(plain(h))
	}
	type oneOffOut struct {
		From *TieEnd `json:"from"`
		To   *TieEnd `json:"to"`
		By   string  `json:"by"`
	}
	// These fields are less deep than the ones of plain, so they take their place.
	out := struct {
		plain
		Src    *int       `json:"src"`
		Dst    *int       `json:"dst"`
		Prev   [][2]int   `json:"prev"`
		OneOff *oneOffOut `json:"oneOff,omitempty"`
	}{plain: plain(h), Prev: [][2]int{}}
	if !h.hide.src {
		out.Src = &h.Src
	}
	if !h.hide.dst {
		out.Dst = &h.Dst
	}
	for _, p := range h.Prev {
		if !slices.Contains(h.hide.prev, p[0]) {
			out.Prev = append(out.Prev, p)
		}
	}
	if o := h.OneOff; o != nil {
		out.OneOff = &oneOffOut{By: o.By}
		if !h.hide.from {
			out.OneOff.From = &o.From
		}
		if !h.hide.to {
			out.OneOff.To = &o.To
		}
	}
	return json.Marshal(out)
}

// hop gives h as the caller sees it. A source below zero is nothing routed, which is always visible.
func (v apiView) hop(h TieHop) TieHop {
	if !v.Hidden() {
		return h
	}
	rv := v.on(h.Router)
	hd := &hopHide{src: h.Src >= 0 && !rv.Source(h.Src), dst: !rv.Destination(h.Dst)}
	for _, p := range h.Prev {
		if p[1] >= 0 && !rv.Source(p[1]) {
			hd.prev = append(hd.prev, p[0])
		}
	}
	if o := h.OneOff; o != nil {
		hd.from = !v.on(o.From.Router).Destination(o.From.N)
		hd.to = !v.on(o.To.Router).Source(o.To.N)
	}
	h.hide = hd
	return h
}

// hops gives each hop of hs as the caller sees it, in a new slice.
func (v apiView) hops(hs []TieHop) []TieHop {
	if hs == nil || !v.Hidden() {
		return hs
	}
	out := make([]TieHop, len(hs))
	for i, h := range hs {
		out[i] = v.hop(h)
	}
	return out
}
