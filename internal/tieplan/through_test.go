package tieplan

import (
	"encoding/json"
	"os"
	"testing"
)

// Through gives the path that Plan gives for the same chain of tie lines, so a take sends what the caller saw.
func TestThroughMatchesPlan(t *testing.T) {
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var cs cases
	if err := json.Unmarshal(b, &cs); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, c := range cs.Plans {
		if len(c.In.Taken) > 0 {
			continue
		}
		for _, want := range Plan(c.In, c.Route).Paths {
			var ids []string
			for _, h := range want.Hops[:len(want.Hops)-1] {
				ids = append(ids, h.Tie)
			}
			got, why := Through(c.In, c.Route, ids)
			g, _ := json.Marshal(got)
			w, _ := json.Marshal(want)
			if why != nil || string(g) != string(w) {
				t.Errorf("%s %v: %+v\n got %s\nwant %s", c.Name, ids, why, g, w)
			}
			n++
		}
	}
	if n < 10 {
		t.Errorf("checked %d paths", n)
	}
}

// Through refuses a chain that is no path, with the reason.
func TestThroughRefuses(t *testing.T) {
	up := func(uid string) Router {
		return Router{UID: uid, Levels: 1, Up: true, Tally: map[int]map[int]int{}}
	}
	in := Input{Routers: []Router{up("a"), up("b"), up("c")}, Ties: []Tie{
		{ID: "t1", From: End{"a", 10}, To: End{"b", 5}},
		{ID: "t2", From: End{"b", 30}, To: End{"c", 8}},
		{ID: "back", From: End{"b", 31}, To: End{"a", 6}},
	}}
	rt := Route{Src: End{"a", 1}, Dst: End{"c", 9}, Levels: []int{1}}
	for _, c := range []struct {
		ids  []string
		want Reason
	}{
		{[]string{"t1", "nope"}, Reason{Why: "unknown_tie", Tie: "nope"}},
		{[]string{"t2"}, Reason{Why: "not_joined", Tie: "t2"}},
		{[]string{"t1"}, Reason{Why: "not_joined", From: "b", To: "c"}},
		{[]string{"t1", "back"}, Reason{Why: "not_joined", Tie: "back"}},
		{nil, Reason{Why: "not_joined", From: "a", To: "c"}},
	} {
		if _, why := Through(in, rt, c.ids); why == nil || *why != c.want {
			t.Errorf("%v: %+v, want %+v", c.ids, why, c.want)
		}
	}
	in.Routers[1].Up = false
	if _, why := Through(in, rt, []string{"t1", "t2"}); why == nil || *why != (Reason{Why: "router_down", Router: "b"}) {
		t.Errorf("down: %+v", why)
	}
	in.Routers[1].Up = true
	if p, why := Through(in, rt, []string{"t1", "t2"}); why != nil || len(p.Hops) != 3 || p.Hops[1].Src != 5 || p.Hops[1].Dst != 30 || p.Hops[2].Src != 8 {
		t.Errorf("path %+v, %+v", p, why)
	}
}
