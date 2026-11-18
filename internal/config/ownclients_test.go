package config

import (
	"encoding/json"
	"strings"
	"testing"
)

func rightsJSON(r Rights) string {
	b, _ := json.Marshal(r)
	return string(b)
}

// Intersect is what both grant: an unset list is all, a router reference matches by number on
// the one router, and hidden wins.
func TestRightsIntersect(t *testing.T) {
	lv := func(l ...int) *[]int { return &l }
	ps := func(l ...string) *[]string { return &l }
	for _, c := range []struct {
		name string
		a, b Rights
		want string
	}{
		{"both open", Rights{}, Rights{}, `{}`},
		{"one open", Rights{Sources: nums(1, 2)}, Rights{}, `{"sources":[{"router":"","n":1},{"router":"","n":2}]}`},
		{"other open", Rights{}, Rights{Dsts: nums(4)}, `{"dsts":[{"router":"","n":4}]}`},
		{"common", Rights{Sources: nums(1, 2, 3)}, Rights{Sources: nums(2, 3, 9)}, `{"sources":[{"router":"","n":2},{"router":"","n":3}]}`},
		{"router blank matches", Rights{Sources: &[]Ref{{N: 2, Router: "r-1"}}}, Rights{Sources: nums(2)}, `{"sources":[{"router":"r-1","n":2}]}`},
		{"none left", Rights{Dsts: nums(1)}, Rights{Dsts: nums(2)}, `{"dsts":[]}`},
		{"levels and presets", Rights{Levels: lv(1, 2), Presets: ps("a", "b")}, Rights{Levels: lv(2), Presets: ps("b", "c")}, `{"levels":[2],"presets":["b"]}`},
		{"hidden wins", Rights{}, Rights{Visibility: VisHidden}, `{"visibility":"hidden"}`},
		{"read-only stays", Rights{Visibility: VisReadOnly}, Rights{}, `{"visibility":"readonly"}`},
	} {
		got := c.a.Intersect(c.b)
		if s := rightsJSON(got); s != c.want {
			t.Errorf("%s: %s, want %s", c.name, s, c.want)
		}
		if !got.Within(c.a) || !got.Within(c.b) {
			t.Errorf("%s: not within both", c.name)
		}
	}
}

// A user's own API clients: at most MaxAPIClients each, apart from the Administrator's 12.
func TestOwnClientsCountPerUser(t *testing.T) {
	c := Defaults()
	c.Users = []User{{ID: "u-1", Name: "Ana", Pin: "x"}}
	for i := range MaxAPIClients {
		c.APIClients = append(c.APIClients, APIClient{Name: "Bot " + string(rune('a'+i)), Scopes: []string{}})
		c.APIClients = append(c.APIClients, APIClient{Name: "Ana bot " + string(rune('a'+i)), Scopes: []string{}, Owner: "u-1"})
	}
	if errs := c.validateAPI(); len(errs) != 0 {
		t.Fatalf("12 own and 12 of Ana's refused: %v", errs)
	}
	c.APIClients = append(c.APIClients, APIClient{Name: "Ana bot z", Scopes: []string{}, Owner: "u-1"})
	errs := c.validateAPI()
	if len(errs) != 1 || !strings.Contains(errs[0].Error(), "Ana") {
		t.Fatalf("13th of Ana's: %v", errs)
	}
}
