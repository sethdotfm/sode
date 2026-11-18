package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func defaultOf(t *testing.T, list []any, kind string) map[string]any {
	t.Helper()
	for _, d := range list {
		if m := d.(map[string]any); m["kind"] == kind {
			return m
		}
	}
	t.Fatalf("no %s default in %v", kind, list)
	return nil
}

// A preset's Default artwork is kept by the preset's hidden id, so a preset or user rename keeps it. A Source or
// Destination default names its router.
func TestArtDefaultSurvivesPresetAndUserRename(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	uid, pid := a.r().cfg.UID, a.r().cfg.Presets[0].ID

	rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"preset","scope":"everyone","name":"studio a","artB":"A"}`)
	if d := apiBody(t, rec); rec.Code != 200 || d["id"] != pid || d["name"] != "Studio A" {
		t.Fatalf("preset set: %d %s", rec.Code, rec.Body.String())
	}
	rec = apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":1,"art":"icon:camera"}`)
	if d := apiBody(t, rec); rec.Code != 200 || d["router"] != uid {
		t.Fatalf("source set: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"preset","scope":"everyone","name":"No such","artB":"N"}`), 404, "not_found")
	id := makePanel(t, a, ana, `{"name":"Wall","cells":[{"kind":"preset","scope":"everyone","name":"Studio A","r":1,"c":1}]}`)

	cfg := a.r().cfg
	cfg.Presets = config.ClonePresets(cfg.Presets)
	cfg.Presets[0].Name = "Studio B"
	cfg.Users = append([]config.User(nil), cfg.Users...)
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Cy" {
			cfg.Users[i].Name = "Cyd"
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}

	if d := defaultOf(t, artDefaultList(t, a, ana), "preset"); d["id"] != pid || d["name"] != "Studio B" || d["artB"] != "A" {
		t.Fatalf("after rename %v", d)
	}
	// The cell still holds the old name until its panel is saved again; the id finds the default.
	ds := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))["artDefaults"].([]any)
	if len(ds) != 1 || ds[0].(map[string]any)["id"] != pid {
		t.Fatalf("panel read %v", ds)
	}
	// Set by the new name replaces the same default; the renamed user can still change it.
	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"preset","scope":"everyone","name":"Studio B","artB":"B"}`); rec.Code != 200 {
		t.Fatalf("set by new name: %d %s", rec.Code, rec.Body.String())
	}
	if got := artDefaultList(t, a, ana); len(got) != 2 || defaultOf(t, got, "preset")["artB"] != "B" {
		t.Fatalf("defaults %v", got)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/default-artwork", cy, `{"kind":"preset","scope":"everyone","id":"`+pid+`"}`); rec.Code != 204 {
		t.Fatalf("clear by id: %d %s", rec.Code, rec.Body.String())
	}
}

// A preset default stored by name with no id still reaches a Preset cell, and a set or clear by id replaces it.
func TestArtDefaultStoredByNameStillApplies(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	pid := a.r().cfg.Presets[0].ID
	if _, err := a.artDefaults.set(ArtDefault{Kind: artKindPreset, Scope: "everyone", Name: "Studio A", ArtB: "L"}, nil); err != nil {
		t.Fatal(err)
	}
	id := makePanel(t, a, ana, `{"name":"Wall","cells":[{"kind":"preset","scope":"everyone","name":"Studio A","r":1,"c":1}]}`)
	ds := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))["artDefaults"].([]any)
	if len(ds) != 1 || ds[0].(map[string]any)["id"] != pid || ds[0].(map[string]any)["artB"] != "L" {
		t.Fatalf("panel read %v", ds)
	}
	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"preset","scope":"everyone","id":"`+pid+`","artB":"B"}`); rec.Code != 200 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	if got := a.artDefaults.list(); len(got) != 1 || got[0].ID != pid || got[0].ArtB != "B" {
		t.Fatalf("stored %+v", got)
	}
	if _, err := a.artDefaults.set(ArtDefault{Kind: artKindPreset, Scope: "everyone", Name: "Studio A", ArtB: "L"}, nil); err != nil {
		t.Fatal(err)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/default-artwork", cy, `{"kind":"preset","scope":"everyone","id":"`+pid+`"}`); rec.Code != 204 {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
	if got := a.artDefaults.list(); len(got) != 0 {
		t.Fatalf("after clear %+v", got)
	}
}
