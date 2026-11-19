package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func refs(ns ...int) []Ref {
	out := make([]Ref, len(ns))
	for i, n := range ns {
		out[i] = Ref{N: n}
	}
	return out
}

func refItem(src int, levels []int, dsts ...int) PresetItem {
	return PresetItem{Src: Ref{N: src}, Dsts: refs(dsts...), Levels: levels}
}

func TestPresetRefsAndIDs(t *testing.T) {
	c := Defaults()
	c.Normalize()
	c.Presets = []Preset{
		{Name: "Show", Items: []PresetItem{{Src: Ref{N: 4, Shown: "5"}, Dsts: []Ref{{N: 1, Name: "MON 2"}}}}},
		{ID: "dup", Name: "B", Items: []PresetItem{refItem(1, nil, 2)}},
		{ID: "dup", Name: "C", Items: []PresetItem{refItem(1, nil, 3)}},
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	s := c.Presets[0]
	if s.ID == "" || s.Items[0].Src.Router != c.UID || s.Items[0].Dsts[0].Router != c.UID || s.Items[0].Src.Shown != "" {
		t.Fatalf("not filled: %+v", s)
	}
	if s.Items[0].Dsts[0].Name != "MON 2" {
		t.Errorf("name dropped: %+v", s.Items[0].Dsts[0])
	}
	if c.Presets[1].ID != "dup" || c.Presets[2].ID == "dup" || c.Presets[2].ID == "" {
		t.Errorf("ids: %q %q", c.Presets[1].ID, c.Presets[2].ID)
	}
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	for _, want := range []string{"id: " + s.ID, "router: " + c.UID, `"n": 4`, "name: MON 2"} {
		if !strings.Contains(string(b), want) {
			t.Errorf("file lacks %q:\n%s", want, b)
		}
	}
	got, _, err := Load(path)
	if err != nil || !reflect.DeepEqual(got.Presets, c.Presets) {
		t.Fatalf("round trip: %+v err=%v want %+v", got.Presets, err, c.Presets)
	}
	// One item is one route on one router. An unknown router is not an error (it is Broken).
	c.Presets[0].Items[0].Dsts = append(c.Presets[0].Items[0].Dsts, Ref{Router: "other", N: 2})
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "one router") {
		t.Errorf("mixed routers: %v", err)
	}
	c.Presets[0].Items[0] = PresetItem{Src: Ref{Router: "gone", N: 1}, Dsts: []Ref{{Router: "gone", N: 2}}}
	if err := c.Validate(); err != nil {
		t.Errorf("unknown router refused: %v", err)
	}
}

func TestPresetListSaveKeepsIDs(t *testing.T) {
	saved := []Preset{{ID: "a1", Name: "Show", Items: []PresetItem{refItem(1, nil, 2)}}, {ID: "b2", Name: "Late", Items: []PresetItem{refItem(1, nil, 3)}}}
	got := WithPresetExtras([]Preset{
		{ID: "a1", Name: "Renamed", Items: []PresetItem{refItem(1, nil, 2)}},
		{Name: "late", Items: []PresetItem{refItem(1, nil, 3)}},
		{Name: "Show", Items: []PresetItem{refItem(1, nil, 4)}},
	}, saved)
	if got[0].ID != "a1" || got[1].ID != "b2" || got[2].ID != "" {
		t.Fatalf("ids %q %q %q", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestLoadKeepsMadePresetIDs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	pin, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	text := "format: 2\nrouters: [{uid: r1, address: 10.0.0.1:8910}]\n" +
		"presets:\n  - name: Show\n    items: [{src: {router: r1, n: 1}, dsts: [{router: r1, n: 2}]}]\n" +
		"users:\n  - name: Ann\n    pin: " + pin + "\n    len: 4\n    profile:\n      presets:\n        - name: Mine\n          items: [{src: {router: r1, n: 1}, dsts: [{router: r1, n: 3}]}]\n"
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
	if first.Presets[0].ID == "" || again.Presets[0].ID != first.Presets[0].ID {
		t.Errorf("Everyone preset id %q then %q", first.Presets[0].ID, again.Presets[0].ID)
	}
	a, b := first.Users[0].Profile.Presets[0].ID, again.Users[0].Profile.Presets[0].ID
	if a == "" || a != b {
		t.Errorf("Me preset id %q then %q", a, b)
	}
}

func TestProfilePresetRefsFilled(t *testing.T) {
	c := Defaults()
	c.Normalize()
	c.Users = []User{{Name: "Ann", Profile: Profile{Presets: []Preset{{Name: "Mine", Items: []PresetItem{refItem(1, nil, 2)}}}}}}
	c.Normalize()
	p := c.Users[0].Profile.Presets[0]
	if p.ID == "" || p.Items[0].Src.Router != c.UID || p.Items[0].Dsts[0].Router != c.UID {
		t.Fatalf("profile preset not filled: %+v", p)
	}
}
