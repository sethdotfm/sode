package app

import (
	"encoding/json"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestSavePresets(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	sv := []config.Preset{{Name: " Morning ", Items: []config.PresetItem{presetItem(3, []int{2, 1, 1}, 1, 2)}}}
	if err := a.SavePresets(c, sv); err != nil {
		t.Fatalf("open save: %v", err)
	}
	got := a.r().cfg.Presets
	if len(got) != 1 || got[0].Name != "Morning" || len(got[0].Items[0].Levels) != 2 || got[0].Items[0].Levels[0] != 1 {
		t.Fatalf("normalized: %+v", got)
	}
	sv[0].Items[0].Dsts[0].N = 99
	if a.r().cfg.Presets[0].Items[0].Dsts[0].N != 1 {
		t.Fatal("presets share memory with the caller")
	}
	if err := a.SavePresets(c, []config.Preset{{Name: "x"}}); err == nil {
		t.Fatal("preset with no takes accepted")
	}
	stale := a.r().cfg
	stale.Presets = nil
	if err := a.SaveSettings(c, stale, "", false); err != nil || len(a.r().cfg.Presets) != 1 {
		t.Fatalf("settings save dropped the presets: %v %v", err, a.r().cfg.Presets)
	}
	// An Advanced Settings PIN closes nothing: with no users, every browser is an Administrator.
	if err := a.SaveSettings(c, a.r().cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SavePresets(c, nil); err != nil || len(a.r().cfg.Presets) != 0 {
		t.Fatalf("save with a PIN set: %v %v", err, a.r().cfg.Presets)
	}
}

func TestPresetMessageShapes(t *testing.T) {
	b, _ := json.Marshal(presetsMsg{T: "presets", Presets: []config.Preset{}})
	if string(b) != `{"t":"presets","presets":[]}` {
		t.Fatalf("presets message %s", b)
	}
	var m struct {
		Presets []config.Preset `json:"presets"`
	}
	if err := json.Unmarshal([]byte(`{"presets":[{"name":"A","items":[{"src":{"n":1},"dsts":[{"n":2}],"levels":[]}]}]}`), &m); err != nil || m.Presets[0].Items[0].Dsts[0].N != 2 {
		t.Fatalf("decode %v %+v", err, m)
	}
}
