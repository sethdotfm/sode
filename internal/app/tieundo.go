package app

import (
	"fmt"
	"maps"
	"net/http"
	"slices"

	"github.com/sethdotfm/sode/internal/tieplan"
)

// Undo of a tie take (docs/tielines-design.md, Undo). Undo puts back the destination asked for. It never switches a
// tie line back by itself, since a tie line can have new users after the take. The entry of the destination keeps
// the Via chain of each typed level from before the take (viaPrev). tieUndoVerdict compares that chain with the
// chain that the Undo would give now:
//   - same: the Undo gives the destination what it had. It goes as a plain Undo.
//   - putBack: the chain differs only because a hop of this take switched a tie line that still carries what the
//     take set. The page's Undo confirm offers to put those tie lines back, with their Affected destinations.
//   - changed: any other difference. Undo sends nothing, says what changed and offers the picker.
// The page asks with the message tieUndo before its confirm opens. POST /history/{id}/undo and the panel Undo cell
// have no confirm that could offer a choice. So they refuse an Undo that is not same with 409 tie_line_changed.

// tieUndoPut is one tie line that the Undo confirm offers to put back. It has the hop entry and its Affected
// destinations now. To is the source that the tie line carried before, nil when the levels had different sources.
type tieUndoPut struct {
	Entry    int      `json:"entry"`
	Tie      string   `json:"tie"`
	TieName  string   `json:"tieName"`
	To       *TieEnd  `json:"to,omitempty"`
	Affected []TieEnd `json:"affected"`
}

// tieUndoCheck is the verdict on the Undo of one entry. Want (changed only) is the source that the destination had
// before, for the picker. Levels are the typed levels of the take.
type tieUndoCheck struct {
	Entry   int          `json:"entry"`
	Verdict string       `json:"verdict"` // same, putBack or changed
	Text    string       `json:"text,omitempty"`
	PutBack []tieUndoPut `json:"putBack,omitempty"`
	Want    *TieEnd      `json:"want,omitempty"`
	Levels  []int        `json:"levels,omitempty"`
}

// viaDiff is one typed level whose chain after the Undo would differ from the chain before the take.
type viaDiff struct {
	was ViaLevel
	now tieplan.Chain
}

// withTally gives a copy of in where destination at has the sources of prev ([level, source] pairs). The live tally
// maps stay as they are.
func withTally(in tieplan.Input, at tieplan.End, prev [][2]int) tieplan.Input {
	out := in
	out.Routers = slices.Clone(in.Routers)
	for i, r := range out.Routers {
		if r.UID != at.Router || r.Tally == nil {
			continue
		}
		t := maps.Clone(r.Tally)
		ls := maps.Clone(t[at.N])
		if ls == nil {
			ls = map[int]int{}
		}
		for _, p := range prev {
			ls[p[0]] = p[1]
		}
		t[at.N] = ls
		out.Routers[i].Tally = t
	}
	return out
}

func viaDiffs(in tieplan.Input, dst tieplan.End, was []ViaLevel) []viaDiff {
	var out []viaDiff
	for _, w := range was {
		if now := tieplan.Via(in, dst, w.Level); !now.Same(w.Chain) {
			out = append(out, viaDiff{w, now})
		}
	}
	return out
}

// hopHolds: the tie line of hop h still carries what the take set, on every level of the hop.
func hopHolds(in tieplan.Input, h TieHop) bool {
	for _, r := range in.Routers {
		if r.UID != h.Router {
			continue
		}
		for _, l := range h.Levels {
			if s, ok := r.Tally[h.Dst][l]; !ok || s != h.Src {
				return false
			}
		}
		return len(h.Levels) > 0
	}
	return false
}

// tieUndoVerdict judges the Undo of entry e. An entry that is not the destination of a tie take that succeeded is
// always same. Caller does not hold mu.
func (a *App) tieUndoVerdict(rt *runtime, e Entry) tieUndoCheck {
	out := tieUndoCheck{Entry: e.ID, Verdict: "same", Levels: distinct(e.Levels)}
	if e.Origin == nil || !e.OK || len(e.ViaPrev) == 0 {
		return out
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	in := a.tieInput(rt)
	_, names := a.tieLinesNow(rt)
	dst := tieplan.End{Router: e.Router, N: e.Dst}
	back := withTally(in, dst, e.Prev)
	diffs := viaDiffs(back, dst, e.ViaPrev)
	if len(diffs) == 0 {
		return out
	}
	// The hops of this take whose tie line an old chain went through, and that still carry what the take set.
	var puts []TieHop
	for _, h := range e.Hops {
		if h.Entry == nil || h.Result != "connected" && !h.Late || !hopHolds(in, h) {
			continue
		}
		if slices.ContainsFunc(diffs, func(d viaDiff) bool { return slices.Contains(d.was.Via, h.Tie) }) {
			puts = append(puts, h)
		}
	}
	if len(puts) > 0 {
		all := back
		for _, h := range puts {
			all = withTally(all, tieplan.End{Router: h.Router, N: h.Dst}, h.Prev)
		}
		if len(viaDiffs(all, dst, e.ViaPrev)) == 0 {
			out.Verdict = "putBack"
			for _, h := range puts {
				out.PutBack = append(out.PutBack, a.tieUndoPutOf(in, names, h))
			}
			return out
		}
	}
	d := diffs[0]
	out.Verdict = "changed"
	out.Text = a.viaChangedText(rt, names, e, d)
	if o := d.was.Origin; o != nil {
		out.Want = &TieEnd{Router: o.Router, N: o.N, Name: a.srcNameOf(rt, *o)}
	}
	return out
}

// tieUndoPutOf names the put-back of hop h and its Affected destinations now. Caller holds mu.
func (a *App) tieUndoPutOf(in tieplan.Input, names map[string]string, h TieHop) tieUndoPut {
	p := tieUndoPut{Entry: *h.Entry, Tie: h.Tie, TieName: h.TieName, Affected: []TieEnd{}}
	if n := names[h.Tie]; n != "" {
		p.TieName = n
	}
	srcs := map[int]bool{}
	for _, pv := range h.Prev {
		srcs[pv[1]] = true
	}
	if len(srcs) == 1 {
		for s := range srcs {
			if s >= 0 {
				p.To = &TieEnd{Router: h.Router, N: s, Name: a.srcNameLocked(h.Router, s)}
			}
		}
	}
	for _, r := range tieplan.Downstream(in, tieplan.End{Router: h.Router, N: h.Dst}) {
		end := TieEnd{Router: r.At.Router, N: r.At.N}
		if u, ok := a.unit(r.At.Router); ok {
			end.Name = u.router.dstNames[r.At.N]
		}
		p.Affected = append(p.Affected, end)
	}
	return p
}

// viaChangedText says why Undo sends nothing: "TIE 5 now carries CAM 2, not CAM 9". Without a tie line on the chain
// now, it names the destination. Caller holds mu.
func (a *App) viaChangedText(rt *runtime, names map[string]string, e Entry, d viaDiff) string {
	word := func(c tieplan.Chain) string {
		switch {
		case c.Loop:
			return "a loop"
		case c.Unknown != "":
			return "an unknown source"
		case c.Nothing || c.Origin == nil:
			return "nothing"
		}
		return a.srcNameOf(rt, *c.Origin)
	}
	if n := len(d.now.Via); n > 0 {
		id := d.now.Via[n-1]
		name := names[id]
		if name == "" {
			name = tieNameOf(rt, id)
		}
		return fmt.Sprintf("%s now carries %s, not %s", name, word(d.now), word(d.was.Chain))
	}
	dst := e.DstName
	if dst == "" {
		dst = fmt.Sprintf("destination %d", e.Dst+rt.routerOf(e.Router).NumberOffset)
	}
	return fmt.Sprintf("%s would get %s, not %s", dst, word(d.now), word(d.was.Chain))
}

// srcNameOf names a source by its router name, else its number. Caller holds mu.
func (a *App) srcNameOf(rt *runtime, at tieplan.End) string {
	if n := a.srcNameLocked(at.Router, at.N); n != "" {
		return n
	}
	return fmt.Sprintf("source %d", at.N+rt.routerOf(at.Router).NumberOffset)
}

func (a *App) srcNameLocked(uid string, n int) string {
	if u, ok := a.unit(uid); ok {
		return u.router.srcNames[n]
	}
	return ""
}

// tieUndoProblem is the refusal of an Undo with no confirm, as from the API and the panel Undo cell. It is nil when
// the Undo of e can go.
func (a *App) tieUndoProblem(rt *runtime, e Entry) *apiProblem {
	c := a.tieUndoVerdict(rt, e)
	switch c.Verdict {
	case "putBack":
		p := c.PutBack[0]
		return problem(http.StatusConflict, "tie_line_changed",
			"entry %d: the Undo would not give the source from before, since %s still carries what this take set. Put it back first: POST /history/%d/putback",
			e.ID, p.TieName, p.Entry)
	case "changed":
		return problem(http.StatusConflict, "tie_line_changed", "entry %d: %s. Sode sent nothing; take the route again with the tie lines that you choose", e.ID, c.Text)
	}
	return nil
}

// serveTieUndo answers the page's tieUndo: the verdict on the Undo of each entry in m.Entries that this caller can
// see. Not a write: it changes nothing.
func (a *App) serveTieUndo(c *client, m *inMsg) {
	rt := a.r()
	act := a.caller(c)
	v := a.view(&apiCtx{Actor: act.Actor, Rights: act.rights, IP: c.ip, page: c})
	checks := []tieUndoCheck{}
	for _, id := range m.Entries {
		e, ok := a.hist.Get(id)
		if ok {
			e, ok = v.Entry(e)
		}
		if ok {
			checks = append(checks, a.tieUndoVerdict(rt, e))
		}
	}
	a.reply(c, tieUndoMsg{T: "tieUndoResult", ID: m.ID, Checks: checks})
}

type tieUndoMsg struct {
	T      string         `json:"t"`
	ID     int            `json:"id"`
	Checks []tieUndoCheck `json:"checks"`
}
