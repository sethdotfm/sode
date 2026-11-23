package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func twoRouters() Config {
	c := Defaults()
	c.UID = "a1"
	b := c.Router
	b.UID, b.Prefix, b.Levels = "b2", "B", 1
	c.MoreRouters = []Router{b}
	c.Normalize()
	return c
}

// A tie line has a hidden id, a unique Tie line name and two router references on different routers.
func TestValidateTieLines(t *testing.T) {
	c := twoRouters()
	good := TieLine{Name: "TIE 1", From: Ref{Router: "a1", N: 10}, To: Ref{Router: "b2", N: 3}}
	c.TieLines = []TieLine{good}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("a good tie line refused: %v", err)
	}
	if c.TieLines[0].ID == "" {
		t.Error("no id made")
	}
	for _, tc := range []struct {
		list []TieLine
		want string
	}{
		{[]TieLine{{Name: "X", From: Ref{Router: "a1", N: 1}, To: Ref{Router: "a1", N: 2}}}, "to a different router"},
		{[]TieLine{{Name: "X", From: Ref{Router: "a1", N: 1}, To: Ref{Router: "zz", N: 2}}}, "a router that Sode does not have"},
		{[]TieLine{{Name: "X", From: Ref{N: 1}, To: Ref{Router: "b2", N: 2}}}, "the router of its destination"},
		{[]TieLine{{Name: "X", From: Ref{Router: "a1", N: -1}, To: Ref{Router: "b2", N: 2}}}, "a destination number"},
		{[]TieLine{good, {Name: "tie 1", From: Ref{Router: "a1", N: 11}, To: Ref{Router: "b2", N: 4}}}, `two tie lines are named "tie 1"`},
		{[]TieLine{good, {Name: "TIE 2", From: good.From, To: good.To}}, "the same two ends"},
		{[]TieLine{{From: Ref{Router: "a1", N: 1}, To: Ref{Router: "b2", N: 2}}}, "every tie line needs a name"},
	} {
		c := twoRouters()
		c.TieLines = tc.list
		c.Normalize()
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%+v: got %v, want %q", tc.list, err, tc.want)
		}
	}
}

// The page shows the same words (web/tielinerules.js).
func TestTieLineCases(t *testing.T) {
	raw, err := os.ReadFile("../../web/test/tieline_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Routers []string `json:"routers"`
		Cases   []struct {
			T       TieLine `json:"t"`
			Problem string  `json:"problem"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	c := twoRouters()
	if got := []string{c.UID, c.MoreRouters[0].UID}; !slices.Equal(got, doc.Routers) {
		t.Fatalf("routers %v, cases name %v", got, doc.Routers)
	}
	for _, tc := range doc.Cases {
		if got := c.TieLineProblem(tc.T); got != tc.Problem {
			t.Errorf("%+v: got %q, want %q", tc.T, got, tc.Problem)
		}
	}
}

// A blank Tie line name takes the name of the downstream source.
func TestTieLineNameFillsFromTheSource(t *testing.T) {
	c := twoRouters()
	c.TieLines = []TieLine{{Name: " ", From: Ref{Router: "a1", N: 1, Name: "TO STAGE 2"}, To: Ref{Router: "b2", N: 2, Name: " FROM ST1 "}}}
	c.Normalize()
	if got := c.TieLines[0].Name; got != "FROM ST1" {
		t.Errorf("name %q", got)
	}
}

// Tie lines survive a restart: Load writes back an id it made, and a second Load reads the same list.
func TestTieLinesSurviveARestart(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	body := "format: 2\nrouters:\n  - uid: a1\n  - uid: b2\n    prefix: B\n    levels: 1\n" +
		"tieLines:\n  - name: TIE 3\n    from: {router: a1, n: 40, name: TO B}\n    to: {router: b2, n: 2, name: FROM A}\n    later: kept\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	a, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	b, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.TieLines) != 1 || len(b.TieLines) != 1 || a.TieLines[0].ID == "" || a.TieLines[0].ID != b.TieLines[0].ID {
		t.Fatalf("tie lines %+v then %+v", a.TieLines, b.TieLines)
	}
	got := b.TieLines[0]
	if got.Name != "TIE 3" || got.From.Router != "a1" || got.From.N != 40 || got.From.Name != "TO B" || got.To.Router != "b2" || got.To.N != 2 {
		t.Errorf("after a restart %+v", got)
	}
	if got.Extra["later"] != "kept" {
		t.Errorf("unknown key lost: %+v", got.Extra)
	}
}

// The hop weight is a setting from 0 to MaxTieHopWeight. The default is one.
func TestTieHopWeight(t *testing.T) {
	c := Defaults()
	if c.TieHopWeight != DefaultTieHopWeight {
		t.Errorf("default %d", c.TieHopWeight)
	}
	for _, w := range []int{0, MaxTieHopWeight} {
		c.TieHopWeight = w
		if err := c.Validate(); err != nil && strings.Contains(err.Error(), "hop weight") {
			t.Errorf("weight %d: %v", w, err)
		}
	}
	for _, w := range []int{-1, MaxTieHopWeight + 1} {
		c.TieHopWeight = w
		if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "hop weight") {
			t.Errorf("weight %d: %v", w, err)
		}
	}
}

// A browser can neither drop nor add the unknown keys of a tie line.
func TestWithTieLineExtras(t *testing.T) {
	saved := []TieLine{{ID: "t1", Name: "A", Extra: map[string]any{"later": 1}}}
	got := WithTieLineExtras([]TieLine{{ID: "t1", Name: "B"}, {ID: "t2", Name: "C", Extra: map[string]any{"x": 1}}}, saved)
	if got[0].Extra["later"] != 1 || got[1].Extra != nil {
		t.Errorf("extras %+v", got)
	}
}
