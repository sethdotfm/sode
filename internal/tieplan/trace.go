package tieplan

import "slices"

// Tracer gives the Via chains of many destinations from one Input, as GET /tally and the stream need. It keeps the
// chains that it worked out, so it is good for one Input only. Via(in, ...) is the same for one destination.
type Tracer struct {
	p    *planner
	fed  map[End]bool // sources that a tie line feeds
	tieN map[string]int
}

// NewTracer reads in. The caller holds the lock that guards the tally maps while it uses the Tracer.
func NewTracer(in Input) *Tracer {
	t := &Tracer{p: newPlanner(in), fed: map[End]bool{}, tieN: map[string]int{}}
	for i, ti := range in.Ties {
		t.fed[ti.To] = true
		t.tieN[ti.ID] = i
	}
	return t
}

// Fed: a tie line feeds source src. Only a destination that routes such a source has a Via chain.
func (t *Tracer) Fed(src End) bool { return t.fed[src] }

// Via gives the Via chain of dst on one UI level of its router.
func (t *Tracer) Via(dst End, level int) Chain { return t.p.via(dst, level, map[int]bool{}) }

// UpLevels gives the origin of each level of the upstream end of tie line id, from level 1. Nil is nothing,
// unknown or a loop on that level. A mixed chain shows them in its level pie (docs/tielines-design.md, Via).
func (t *Tracer) UpLevels(id string) []*End {
	i, ok := t.tieN[id]
	if !ok {
		return nil
	}
	from := t.p.in.Ties[i].From
	r, why := t.p.state(from.Router)
	if why != "" {
		return nil
	}
	out := make([]*End, max(r.Levels, 1))
	for l := range out {
		if c := t.Via(from, l+1); c.Origin != nil {
			o := *c.Origin
			out[l] = &o
		}
	}
	return out
}

// Reach is a destination that takes its source through tie lines, and the tie lines on the way, upstream first.
type Reach struct {
	At  End      `json:"at"`
	Via []string `json:"via"`
}

// Downstream gives every destination downstream of destination at. That is each destination that routes the far end
// of a tie line from at, on any level. Then it goes on through the tie lines from those destinations. Each tie line is
// passed one time, so a loop ends. A router with no state gives nothing, since Sode cannot say what it routes. The
// order is by distance, then router and number.
func Downstream(in Input, at End) []Reach {
	p := newPlanner(in)
	out := []Reach{}
	seen := map[End]bool{at: true}
	passed := map[int]bool{}
	front := []Reach{{At: at}}
	for len(front) > 0 {
		var next []Reach
		for _, f := range front {
			for _, ti := range p.byFrom[f.At] {
				if passed[ti] {
					continue
				}
				passed[ti] = true
				tie := p.in.Ties[ti]
				r, why := p.state(tie.To.Router)
				if why != "" {
					continue
				}
				via := append(slices.Clone(f.Via), tie.ID)
				for _, d := range sortedKeys(r.Tally) {
					e := End{tie.To.Router, d}
					if seen[e] || !slices.Contains(mapValues(r.Tally[d]), tie.To.N) {
						continue
					}
					seen[e] = true
					next = append(next, Reach{At: e, Via: via})
				}
			}
		}
		slices.SortStableFunc(next, func(a, b Reach) int {
			if a.At.Router != b.At.Router {
				if a.At.Router < b.At.Router {
					return -1
				}
				return 1
			}
			return a.At.N - b.At.N
		})
		out = append(out, next...)
		front = next
	}
	return out
}
