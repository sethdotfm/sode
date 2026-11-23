package tieplan

import (
	"encoding/json"
	"os"
	"testing"
)

// A change that Sode found at a re-read shows as "before", even when it is after the start of Sode.
func TestFoundChangeIsBefore(t *testing.T) {
	b, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	var cs cases
	if err := json.Unmarshal(b, &cs); err != nil {
		t.Fatal(err)
	}
	for _, c := range cs.Plans {
		if c.Name != "5-minute rule: t1 changed 2 minutes ago" {
			continue
		}
		before := func(found map[string]bool) bool {
			in := c.In
			in.Found = found
			res := Plan(in, c.Route)
			for _, p := range res.Paths {
				for _, h := range p.Hops {
					if h.Tie == "t1" {
						return h.Before
					}
				}
			}
			t.Fatal("no hop for t1")
			return false
		}
		if before(nil) {
			t.Error("a change after the start is before")
		}
		if !before(map[string]bool{"t1": true}) {
			t.Error("a found change is not before")
		}
		return
	}
	t.Fatal("fixture case missing")
}
