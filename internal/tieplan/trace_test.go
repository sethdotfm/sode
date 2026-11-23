package tieplan

import (
	"slices"
	"testing"
)

// Three routers in a chain: A.10 feeds TIE 3 into B.5, and B.30 feeds TIE 9 into C.7. B.20 and B.30 route B.5; C.1
// routes C.7. A has two levels.
func chainInput() Input {
	return Input{
		Routers: []Router{
			{UID: "A", Levels: 2, Up: true, Tally: map[int]map[int]int{10: {1: 3, 2: 3}}},
			{UID: "B", Levels: 1, Up: true, Tally: map[int]map[int]int{20: {1: 5}, 30: {1: 5}, 40: {1: 6}}},
			{UID: "C", Levels: 1, Up: true, Tally: map[int]map[int]int{1: {1: 7}, 2: {1: 8}}},
		},
		Ties: []Tie{
			{ID: "t3", From: End{"A", 10}, To: End{"B", 5}},
			{ID: "t9", From: End{"B", 30}, To: End{"C", 7}},
		},
	}
}

func TestDownstreamFollowsTieLines(t *testing.T) {
	got := Downstream(chainInput(), End{"A", 10})
	want := []Reach{{End{"B", 20}, []string{"t3"}}, {End{"B", 30}, []string{"t3"}}, {End{"C", 1}, []string{"t3", "t9"}}}
	if !slices.EqualFunc(got, want, func(a, b Reach) bool { return a.At == b.At && slices.Equal(a.Via, b.Via) }) {
		t.Errorf("got %+v", got)
	}
	if got := Downstream(chainInput(), End{"B", 20}); len(got) != 0 {
		t.Errorf("a destination that feeds no tie line: %+v", got)
	}
}

// A loop ends, and a router with no state gives nothing.
func TestDownstreamLoopAndUnknown(t *testing.T) {
	in := chainInput()
	in.Ties = append(in.Ties, Tie{ID: "t5", From: End{"C", 1}, To: End{"A", 9}})
	in.Routers[0].Tally[10] = map[int]int{1: 9, 2: 9}
	// A.10 routes A.9, the far end of TIE 5 from C.1: the trace comes back to its start and ends.
	if got := Downstream(in, End{"A", 10}); len(got) != 3 || got[2].At != (End{"C", 1}) {
		t.Errorf("loop %+v", got)
	}
	in = chainInput()
	in.Routers[1].Up = false
	if got := Downstream(in, End{"A", 10}); len(got) != 0 {
		t.Errorf("router down: %+v", got)
	}
}

// The Tracer gives the same chains as Via, and the level pie of a mixed tie line.
func TestTracerMixed(t *testing.T) {
	in := chainInput()
	in.Routers[0].Tally[10] = map[int]int{1: 3, 2: 4}
	tr := NewTracer(in)
	c := tr.Via(End{"C", 1}, 1)
	if !c.Mixed || c.Origin == nil || *c.Origin != (End{"A", 3}) || !slices.Equal(c.Via, []string{"t3", "t9"}) {
		t.Errorf("chain %+v", c)
	}
	if !c.same(Via(in, End{"C", 1}, 1)) {
		t.Errorf("Tracer and Via differ")
	}
	ls := tr.UpLevels("t3")
	if len(ls) != 2 || *ls[0] != (End{"A", 3}) || *ls[1] != (End{"A", 4}) {
		t.Errorf("levels %v", ls)
	}
	if !tr.Fed(End{"B", 5}) || tr.Fed(End{"B", 6}) {
		t.Error("Fed")
	}
	if tr.UpLevels("none") != nil {
		t.Error("an unknown tie line has no levels")
	}
}
