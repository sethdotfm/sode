package tieplan

import "testing"

// Only limits the search to a panel's Reserved tie lines. Via and the Affected destinations still read every tie
// line, so a tie line that is not Reserved still counts downstream.
func TestOnlySearchesReservedTieLines(t *testing.T) {
	up := func(uid string, tally map[int]map[int]int) Router {
		return Router{UID: uid, Levels: 1, Up: true, Tally: tally}
	}
	in := Input{Routers: []Router{
		up("a", map[int]map[int]int{10: {1: 0}, 11: {1: 0}}),
		up("b", map[int]map[int]int{2: {1: 5}, 3: {1: 6}, 9: {1: 6}}),
	}, Ties: []Tie{
		{ID: "t1", From: End{"a", 10}, To: End{"b", 5}},
		{ID: "t2", From: End{"a", 11}, To: End{"b", 6}},
	}}
	rt := Route{Src: End{"a", 1}, Dst: End{"b", 2}, Levels: []int{1}}
	if got := Plan(in, rt).Paths; len(got) != 2 {
		t.Fatalf("paths without Only: %d", len(got))
	}
	in.Only = map[string]bool{"t2": true}
	res := Plan(in, rt)
	if len(res.Paths) != 1 || res.Paths[0].Hops[0].Tie != "t2" {
		t.Fatalf("paths with Only t2: %+v", res.Paths)
	}
	// t2 feeds B.3 and B.9 now, so both are Affected.
	if got := res.Paths[0].Affected; len(got) != 2 {
		t.Errorf("affected: %+v", got)
	}
	in.Only = map[string]bool{}
	if res := Plan(in, rt); len(res.Paths) != 0 || len(res.Reasons) != 1 || res.Reasons[0].Why != "no_tie_line" {
		t.Errorf("empty Only: %+v", res)
	}
}
