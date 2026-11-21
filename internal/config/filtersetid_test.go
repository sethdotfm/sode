package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A filter set has a hidden id, as a preset has. Bookmarks and active sets name a set by it, so a rename moves
// nothing.
func TestNormalizeGivesFilterSetsIDs(t *testing.T) {
	c := Defaults()
	c.FilterSets = []FilterSet{{Name: "A"}, {ID: " k1 ", Name: "B"}, {ID: "k1", Name: "C"}}
	c.Users = []User{{Name: "Ana", Profile: Profile{FilterSets: []FilterSet{{Name: "Mine"}, {ID: "m", Name: "Two"}, {ID: "m", Name: "Three"}}}}}
	c.Normalize()
	for _, l := range [][]FilterSet{c.FilterSets, c.Users[0].Profile.FilterSets} {
		seen := map[string]bool{}
		for _, s := range l {
			if s.ID == "" || seen[s.ID] {
				t.Fatalf("set %q has id %q in %+v", s.Name, s.ID, l)
			}
			seen[s.ID] = true
		}
	}
	if c.FilterSets[1].ID != "k1" || c.FilterSets[2].ID == "k1" {
		t.Errorf("Everyone ids %+v", c.FilterSets)
	}
	if c.Users[0].Profile.FilterSets[1].ID != "m" {
		t.Errorf("Me ids %+v", c.Users[0].Profile.FilterSets)
	}
}

func TestFilterSetListSaveKeepsIDs(t *testing.T) {
	saved := []FilterSet{
		{ID: "a1", Name: "Show", Broken: []Missing{{Kind: MissingSource}}, Extra: map[string]any{"x": 1}},
		{ID: "b2", Name: "Late", Extra: map[string]any{"y": 2}},
	}
	got := WithFilterSetExtras([]FilterSet{
		{ID: "a1", Name: "Renamed"},
		{Name: "late"},
		{Name: "Show"},
	}, saved)
	if got[0].ID != "a1" || got[1].ID != "b2" || got[2].ID != "" {
		t.Fatalf("ids %q %q %q", got[0].ID, got[1].ID, got[2].ID)
	}
	if got[0].Extra["x"] != 1 || len(got[0].Broken) != 1 || got[1].Extra["y"] != 2 || got[2].Extra != nil || got[2].Broken != nil {
		t.Fatalf("extras follow the id: %+v", got)
	}
}

func TestRenamedSetBookmarks(t *testing.T) {
	was := []FilterSet{{ID: "s1", Name: "Cams"}, {ID: "s2", Name: "Same"}}
	now := []FilterSet{{ID: "s1", Name: "Cameras"}, {ID: "s2", Name: "Same"}}
	bm := []Bookmark{{Key: "filterSet:global:s1", Label: "Cams"}, {Key: "filterSet:user:s1", Label: "Cams"}}
	got := RenamedSetBookmarks(bm, "global", was, now)
	if got[0].Key != "filterSet:global:s1" || got[0].Label != "Cameras" || got[1].Label != "Cams" {
		t.Fatalf("set rename %+v", got)
	}
}

func TestLoadKeepsMadeFilterSetIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	text := "format: 2\nrouters: [{uid: r1, address: 10.0.0.1:8910}]\nfilterSets:\n  - name: Cams\n    src: CAM\n"
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	first, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	again, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if first.FilterSets[0].ID == "" || again.FilterSets[0].ID != first.FilterSets[0].ID {
		t.Errorf("filter set id %q then %q", first.FilterSets[0].ID, again.FilterSets[0].ID)
	}
}
