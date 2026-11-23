package tieplan

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"testing"
)

// The cases are shared with web/test/tieplan.test.mjs, which checks the shape of each plan that the page reads.
const fixture = "../../web/test/tieplan_cases.json"

var update = flag.Bool("update", false, "write the planner's output to the fixture as the new want")

type planCase struct {
	Name  string          `json:"name"`
	In    Input           `json:"in"`
	Route Route           `json:"route"`
	Want  json.RawMessage `json:"want"`
}

type viaCase struct {
	Name  string          `json:"name"`
	In    Input           `json:"in"`
	Dst   End             `json:"dst"`
	Level int             `json:"level"`
	Want  json.RawMessage `json:"want"`
}

type cases struct {
	Plans []planCase `json:"plans"`
	Vias  []viaCase  `json:"vias"`
}

func compact(t *testing.T, b []byte) string {
	t.Helper()
	var out bytes.Buffer
	if err := json.Compact(&out, b); err != nil {
		t.Fatal(err)
	}
	return out.String()
}

// Table tests: line, mesh, loop, chain, down router, unknown tally, mixed levels, Protect and Reserve. Also the
// 5-minute rule, a One-off tie line and two routes of one take. `go test ./internal/tieplan -update` writes the
// output back as the new want. Read the diff before you commit it.
func TestPlannerCases(t *testing.T) {
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var cs cases
	if err := json.Unmarshal(b, &cs); err != nil {
		t.Fatal(err)
	}
	check := func(name string, got any, want *json.RawMessage) {
		g, err := json.Marshal(got)
		if err != nil {
			t.Fatal(err)
		}
		if *update {
			*want = g
			return
		}
		if len(*want) == 0 || compact(t, g) != compact(t, *want) {
			t.Errorf("%s:\n got %s\nwant %s", name, g, *want)
		}
	}
	for i := range cs.Plans {
		c := &cs.Plans[i]
		check(c.Name, Plan(c.In, c.Route), &c.Want)
	}
	for i := range cs.Vias {
		c := &cs.Vias[i]
		check(c.Name, Via(c.In, c.Dst, c.Level), &c.Want)
	}
	if *update {
		out, err := json.MarshalIndent(cs, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fixture, append(out, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Usable skips a path with a Protected tie line that it must change.
func TestUsableSkipsProtected(t *testing.T) {
	r := Result{Paths: []Path{{Protected: true, Cost: 1}, {Reserved: true, Cost: 2}}}
	if p := r.Usable(); p == nil || p.Cost != 2 {
		t.Errorf("usable %+v", p)
	}
	if p := (Result{Paths: []Path{{Protected: true}}}).Usable(); p != nil {
		t.Errorf("usable %+v", p)
	}
}
