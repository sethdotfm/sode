package app

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"strings"
	"sync"

	"github.com/sethdotfm/sode/internal/tieplan"
)

// One-off tie lines (docs/tielines-design.md, "One-off tie lines"; GLOSSARY.md). A user with the Use tie lines right
// states that a destination on one router feeds a source on another, for one take. The API takes `ties: [{from,
// to}]` on POST /takes. The Tie line picker sends the same. The user needs no Rights on the stated upstream destination. The Protect check
// covers it, as it covers every hop (refuseHeld).
//
// Sode keeps a One-off tie line in memory while its downstream source is routed to a destination. It also keeps it
// while a take that uses it is in flight. It is never in config.yaml, GET /tielines or an export. While Sode keeps it, the planner,
// Via, Undo and Redo see it as a tie line with OneOff set. The planner then marks the destinations downstream of it
// unknown. An Administrator can save it as a tie line in Settings > Routers > Tie lines (the message oneOffTieLines
// lists them).
//
// The id of a One-off tie line comes from its two ends and a key that Sode makes at start. The same two ends give
// the same id until Sode starts again, so two routes of one take share it. The id does not give the numbers.

const oneOffPrefix = "oneoff-"

// isOneOff: id names a One-off tie line.
func isOneOff(id string) bool { return strings.HasPrefix(id, oneOffPrefix) }

type oneOffTie struct {
	tie  tieplan.Tie // OneOff set; the ends name router units
	name string
	by   string // who stated it last
	pins int    // takes in flight that use it
}

type oneOffStore struct {
	mu  sync.Mutex
	key []byte
	m   map[string]*oneOffTie
}

func (s *oneOffStore) id(from, to tieplan.End) string {
	s.mu.Lock()
	if s.key == nil {
		s.key = make([]byte, 32)
		_, _ = rand.Read(s.key)
	}
	h := hmac.New(sha256.New, s.key)
	s.mu.Unlock()
	fmt.Fprintf(h, "%s %d %s %d", from.Router, from.N, to.Router, to.N)
	return oneOffPrefix + hex.EncodeToString(h.Sum(nil))[:16]
}

// froms gives the upstream destination of each One-off tie line in the store, kept or not.
func (s *oneOffStore) froms() []tieplan.End {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []tieplan.End
	for _, k := range s.m {
		out = append(out, k.tie.From)
	}
	return out
}

// oneOffOf gives the One-off tie line from destination from to source to. Its name is "One-off A.12", after the
// stated upstream destination. Sode does not keep it until a take uses it (hold).
func (a *App) oneOffOf(rt *runtime, from, to tieplan.End) oneOffTie {
	rc := rt.routerOf(from.Router)
	return oneOffTie{
		tie:  tieplan.Tie{ID: a.oneOffs.id(from, to), From: from, To: to, OneOff: true},
		name: fmt.Sprintf("One-off %s.%d", rc.Prefix, from.N+rc.NumberOffset),
	}
}

// statedTie checks the two ends that a caller states. Ends of a saved tie line give that tie line: the id of the
// saved one, and no One-off tie line.
func (a *App) statedTie(rt *runtime, from, to apiEnd) (string, *oneOffTie, *apiProblem) {
	f, p := a.tieEnd(from)
	if p != nil {
		return "", nil, p
	}
	t, p := a.tieEnd(to)
	if p != nil {
		return "", nil, p
	}
	if f.Router == t.Router {
		return "", nil, problem(http.StatusBadRequest, "bad_request", "a One-off tie line goes from a destination on one router to a source on another router")
	}
	if f.N < 0 || t.N < 0 {
		return "", nil, problem(http.StatusBadRequest, "bad_request", "a One-off tie line needs numbers from 0")
	}
	for _, s := range rt.cfg.TieLines {
		fr, ok1 := a.unitUID(rt, s.From.Router)
		tr, ok2 := a.unitUID(rt, s.To.Router)
		if ok1 && ok2 && (tieplan.End{Router: fr, N: s.From.N}) == f && (tieplan.End{Router: tr, N: s.To.N}) == t {
			return s.ID, nil, nil
		}
	}
	o := a.oneOffOf(rt, f, t)
	return o.tie.ID, &o, nil
}

// tieInID gives the tie line id of one entry of ties: id, or the two ends of a One-off tie line. A One-off tie line
// goes into r.oneOffs.
func (a *App) tieInID(rt *runtime, t apiTieIn, r *tieRoute) (string, *apiProblem) {
	switch {
	case t.ID != "" && (t.From != nil || t.To != nil):
		return "", problem(http.StatusBadRequest, "bad_request", "route %d: give a tie line as {id} or as {from, to}, not both", r.index)
	case t.ID != "":
		return t.ID, nil
	case t.From == nil || t.To == nil:
		return "", problem(http.StatusBadRequest, "bad_request", "route %d: give a tie line as {id}, or a One-off tie line as {from, to}", r.index)
	}
	id, o, p := a.statedTie(rt, *t.From, *t.To)
	if p != nil {
		return "", p
	}
	if o != nil && !slices.ContainsFunc(r.oneOffs, func(x oneOffTie) bool { return x.tie.ID == id }) {
		r.oneOffs = append(r.oneOffs, *o)
	}
	return id, nil
}

// hopOneOffAgain gives the tie line id of hop h for a Retry or a Redo. A One-off tie line goes into r.oneOffs from
// the two ends that History keeps, so it works after Sode let it go. Its id can differ after a restart.
func (a *App) hopOneOffAgain(rt *runtime, h TieHop, r *tieRoute) string {
	if h.OneOff == nil {
		return h.Tie
	}
	o := a.oneOffOf(rt, tieplan.End{Router: h.OneOff.From.Router, N: h.OneOff.From.N}, tieplan.End{Router: h.OneOff.To.Router, N: h.OneOff.To.N})
	if !slices.ContainsFunc(r.oneOffs, func(x oneOffTie) bool { return x.tie.ID == o.tie.ID }) {
		r.oneOffs = append(r.oneOffs, o)
	}
	return o.tie.ID
}

// holdOneOffs keeps each of list in memory until release. by is who stated them. A new one changes the Via of what its
// upstream destination feeds, so the streams and the page get it again. Caller does not hold mu.
func (a *App) holdOneOffs(list []oneOffTie, by string) (release func()) {
	if len(list) == 0 {
		return func() {}
	}
	s := &a.oneOffs
	a.mu.Lock()
	s.mu.Lock()
	if s.m == nil {
		s.m = map[string]*oneOffTie{}
	}
	var ids []string
	for _, o := range list {
		if slices.Contains(ids, o.tie.ID) {
			continue
		}
		ids = append(ids, o.tie.ID)
		k := s.m[o.tie.ID]
		if k == nil {
			k = &oneOffTie{tie: o.tie, name: o.name}
			s.m[o.tie.ID] = k
		}
		k.by = by
		k.pins++
	}
	s.mu.Unlock()
	for _, o := range list {
		if u, ok := a.unit(o.tie.From.Router); ok {
			a.viaTouchAt(u, o.tie.From.N)
		}
	}
	a.mu.Unlock()
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		for _, id := range ids {
			if k := s.m[id]; k != nil {
				k.pins--
			}
		}
	}
}

// liveOneOffs gives each One-off tie line that Sode keeps now, in id order. It first lets go of each one that is now
// a saved tie line. It also lets go of each one that no take uses and whose downstream source is routed to no
// destination.
// A router with no state keeps them: Sode cannot tell. Caller holds mu.
func (a *App) liveOneOffs(rt *runtime) []oneOffTie {
	s := &a.oneOffs
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.m) == 0 {
		return nil
	}
	saved := map[[2]tieplan.End]bool{}
	for _, t := range rt.cfg.TieLines {
		fr, ok1 := a.unitUID(rt, t.From.Router)
		tr, ok2 := a.unitUID(rt, t.To.Router)
		if ok1 && ok2 {
			saved[[2]tieplan.End{{Router: fr, N: t.From.N}, {Router: tr, N: t.To.N}}] = true
		}
	}
	var out []oneOffTie
	for _, id := range slices.Sorted(maps.Keys(s.m)) {
		k := s.m[id]
		if saved[[2]tieplan.End{k.tie.From, k.tie.To}] || k.pins <= 0 && !a.routedAnywhere(k.tie.To) {
			delete(s.m, id)
			continue
		}
		out = append(out, *k)
	}
	return out
}

// routedAnywhere: source src is routed to a destination on some level, or its router has no state. Caller holds mu.
func (a *App) routedAnywhere(src tieplan.End) bool {
	u, ok := a.unit(src.Router)
	if !ok {
		return false
	}
	if len(u.router.tally) == 0 {
		return true
	}
	for _, ls := range u.router.tally {
		for _, s := range ls {
			if s == src.N {
				return true
			}
		}
	}
	return false
}

// tieLinesNow gives every tie line that Sode routes through now, saved first, then each One-off tie line that it
// keeps, with the name of each. The ends name router units. Caller holds mu.
func (a *App) tieLinesNow(rt *runtime) ([]tieplan.Tie, map[string]string) {
	var out []tieplan.Tie
	names := map[string]string{}
	for _, t := range rt.cfg.TieLines {
		names[t.ID] = t.Name
		fr, ok1 := a.unitUID(rt, t.From.Router)
		to, ok2 := a.unitUID(rt, t.To.Router)
		if ok1 && ok2 {
			out = append(out, tieplan.Tie{ID: t.ID, From: tieplan.End{Router: fr, N: t.From.N}, To: tieplan.End{Router: to, N: t.To.N}})
		}
	}
	for _, o := range a.liveOneOffs(rt) {
		out = append(out, o.tie)
		names[o.tie.ID] = o.name
	}
	return out, names
}

// tieRef names tie line id for an answer: a One-off tie line says so.
func tieRef(id string, names map[string]string) apiTieRef {
	return apiTieRef{ID: id, Name: names[id], OneOff: isOneOff(id)}
}

// withOneOffs adds the One-off tie lines of routes that in does not have yet: those that a take or a plan states.
func withOneOffs(in tieplan.Input, names map[string]string, routes []tieRoute) tieplan.Input {
	for _, r := range routes {
		for _, o := range r.oneOffs {
			if names != nil {
				names[o.tie.ID] = o.name
			}
			if !slices.ContainsFunc(in.Ties, func(t tieplan.Tie) bool { return t.ID == o.tie.ID }) {
				in.Ties = append(in.Ties, o.tie)
			}
		}
	}
	return in
}

// oneOffs gives the One-off tie lines that the routes of t send through.
func (t *tieTake) oneOffs() []oneOffTie {
	var out []oneOffTie
	for _, r := range t.routes {
		out = append(out, r.in.oneOffs...)
	}
	return out
}

// hopOneOff gives the History record of tie line id when route r states it as a One-off tie line, or nil. Caller
// holds mu. tieTakeBegin sets By.
func (a *App) hopOneOff(r tieRoute, id string) *TieOneOff {
	for _, o := range r.oneOffs {
		if o.tie.ID == id {
			return &TieOneOff{From: TieEnd(a.namedEnd(o.tie.From, true)), To: TieEnd(a.namedEnd(o.tie.To, false))}
		}
	}
	return nil
}

// apiOneOffOut is a One-off tie line that a route of a 409 body states: its id in the paths, and its two ends.
type apiOneOffOut struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	From apiTieEnd `json:"from"`
	To   apiTieEnd `json:"to"`
}

// TieOneOff is the One-off tie line of a hop, as History keeps it. It has the two stated ends, with the names that
// Sode had for them, and who stated them.
type TieOneOff struct {
	From TieEnd `json:"from"`
	To   TieEnd `json:"to"`
	By   string `json:"by"`
}

// oneOffListMsg is the reply to oneOffTieLines (Settings > Routers > Tie lines, Administrator only). It has each
// One-off tie line that Sode keeps now, with the names of its ends.
type oneOffListMsg struct {
	T       string       `json:"t"` // "oneOffTieLines"
	ID      int          `json:"id"`
	OneOffs []oneOffItem `json:"oneOffs"`
}

type oneOffItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	From TieEnd `json:"from"`
	To   TieEnd `json:"to"`
	By   string `json:"by"`
}

func (a *App) oneOffList(id int) oneOffListMsg {
	out := oneOffListMsg{T: "oneOffTieLines", ID: id, OneOffs: []oneOffItem{}}
	rt := a.r()
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, o := range a.liveOneOffs(rt) {
		out.OneOffs = append(out.OneOffs, oneOffItem{ID: o.tie.ID, Name: o.name, From: a.namedEnd(o.tie.From, true), To: a.namedEnd(o.tie.To, false), By: o.by})
	}
	return out
}

// namedEnd gives e with the name that Sode has for it now. Caller holds mu.
func (a *App) namedEnd(e tieplan.End, dst bool) TieEnd {
	out := TieEnd{Router: e.Router, N: e.N}
	if u, ok := a.unit(e.Router); ok {
		out.Name = u.router.name(dst, e.N)
	}
	return out
}
