package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// twoRouterConfig: A (the first router, 4 levels) and B (2 levels).
func twoRouterConfig() (Config, string, string) {
	c := Defaults()
	c.Routers[0].Address, c.Routers[0].Levels = "127.0.0.1:1", 4
	b := c.Routers[0]
	b.UID, b.Prefix, b.Address, b.Levels = "b2b2b2b2", "B", "127.0.0.1:2", 2
	c.Routers = append(c.Routers[:1:1], []Router{b}...)
	c.Normalize()
	return c, c.Routers[0].UID, b.UID
}

// Rights levels name their router: a limit on router A does not limit router B.
func TestRightsLevelsNameTheirRouter(t *testing.T) {
	_, a, b := twoRouterConfig()
	r := Rights{Levels: RouterLevels{a: {1}}}
	if !r.AllowsLevel(a, 1) || r.AllowsLevel(a, 2) {
		t.Error("the limit on A is wrong on A")
	}
	if !r.AllowsLevel(b, 2) || !r.AllowsLevel(b, 1) {
		t.Error("a limit on A limits B")
	}
	none := Rights{Levels: RouterLevels{b: {}}}
	if none.AllowsLevel(b, 1) || !none.AllowsLevel(a, 3) {
		t.Error("an empty list on B is wrong")
	}

	onA, onB := Rights{Levels: RouterLevels{a: {1}}}, Rights{Levels: RouterLevels{b: {1}}}
	if onA.Within(onB) || onB.Within(onA) || !onA.Within(Rights{}) || (Rights{}).Within(onA) {
		t.Error("Within compares levels of two routers")
	}
	both := onA.Intersect(onB).Levels
	if len(both) != 2 || len(both[a]) != 1 || len(both[b]) != 1 {
		t.Errorf("Intersect of a limit on A and a limit on B: %v", both)
	}
	if got := (Rights{Levels: RouterLevels{a: {1, 2}}}).Intersect(Rights{Levels: RouterLevels{a: {2, 3}}}).Levels; len(got[a]) != 1 || got[a][0] != 2 {
		t.Errorf("Intersect on one router: %v", got)
	}
}

// A request can send a plain list, which is the first router's. Normalize fills it.
func TestRightsLevelsFromARequest(t *testing.T) {
	c, a, b := twoRouterConfig()
	var r Rights
	if err := json.Unmarshal([]byte(`{"levels":[1,3]}`), &r); err != nil {
		t.Fatal(err)
	}
	c.GuestRights = r
	var more Rights
	if err := json.Unmarshal([]byte(`{"levels":{"`+b+`":[2],"`+a+`":null}}`), &more); err != nil {
		t.Fatal(err)
	}
	c.APIClients = []APIClient{{Name: "Bot", Scopes: []string{ScopeTake}, Rights: more}}
	c.Normalize()
	if got := c.GuestRights.Levels; len(got) != 1 || len(got[a]) != 2 {
		t.Errorf("a plain list is not the first router's: %v", got)
	}
	if got := c.APIClients[0].Rights.Levels; len(got[b]) != 1 || got[a] == nil || len(got[a]) != 0 {
		t.Errorf("levels by router: %v", got)
	}
	var none Rights
	if err := json.Unmarshal([]byte(`{"levels":null}`), &none); err != nil || none.Levels != nil {
		t.Errorf("null levels: %v %v", none.Levels, err)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

// Validate checks each list against the level count of its router, and refuses a router Sode does not have.
func TestRightsLevelsValidatePerRouter(t *testing.T) {
	c, a, b := twoRouterConfig()
	c.GuestRights.Levels = RouterLevels{a: {4}, b: {2}}
	if err := c.Validate(); err != nil {
		t.Fatalf("levels in range: %v", err)
	}
	c.GuestRights.Levels = RouterLevels{b: {3}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "levels must be 1-2") {
		t.Errorf("level 3 on a router with 2: %v", err)
	}
	c.GuestRights.Levels = RouterLevels{"c3c3c3c3": {1}}
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "levels name a router Sode does not have") {
		t.Errorf("an unknown router: %v", err)
	}
}

// The file stores levels under their router; a plain list in a file is refused, as a reference with no router is.
func TestRightsLevelsStoredByRouter(t *testing.T) {
	c, a, b := twoRouterConfig()
	c.GuestRights.Levels = RouterLevels{a: {1}, b: {2}}
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if l := got.GuestRights.Levels; len(l) != 2 || l[a][0] != 1 || l[b][0] != 2 {
		t.Errorf("levels came back %v", l)
	}
	raw, _ := os.ReadFile(path)
	var file map[string]any
	if err := yaml.Unmarshal(raw, &file); err != nil {
		t.Fatal(err)
	}
	g := file["guestRights"].(map[string]any)
	g["levels"] = []int{1}
	out, _ := yaml.Marshal(file)
	if _, err := Parse(out); err == nil || !strings.Contains(err.Error(), "has no router") {
		t.Errorf("a plain list in the file: %v", err)
	}
}
