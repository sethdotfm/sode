package app

import (
	"encoding/json"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

const refCells = `[{"kind":"source","n":1,"r":1,"c":1},{"kind":"destination","n":2,"r":1,"c":2},` +
	`{"kind":"destination","n":7,"r":1,"c":3,"name":"OLD 7"},{"kind":"preset","scope":"everyone","name":"studio a","r":1,"c":4},{"kind":"take","r":1,"c":5}]`

func cellsOf(t *testing.T, body map[string]any) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, c := range page1Cells(body) {
		out = append(out, c.(map[string]any))
	}
	return out
}

// A saved Buttons panel holds {router, n, name} on Source and Destination cells and the preset's id on a Preset
// cell. A preset rename keeps the cell, and the cell numbers fire the same route as before references.
func TestPanelCellsStoreRouterReferencesAndFireSameRoutes(t *testing.T) {
	a, f, bot := takerApp(t, nil)
	uid := a.r().cfg.UID
	pid := a.r().cfg.Presets[0].ID
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Refs","cells":`+refCells+`}`)

	rec := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil)
	cells := cellsOf(t, apiBody(t, rec))
	want := []map[string]any{
		{"kind": "source", "router": uid, "n": float64(1), "name": "CAM 1"},
		{"kind": "destination", "router": uid, "n": float64(2), "name": "MON 2"},
		// The router has no name for 7, so the cell keeps the one it came with.
		{"kind": "destination", "router": uid, "n": float64(7), "name": "OLD 7"},
		{"kind": "preset", "scope": "everyone", "id": pid, "name": "Studio A"},
		{"kind": "take"},
	}
	for i, w := range want {
		for k, v := range w {
			if cells[i][k] != v {
				t.Fatalf("cell %d %s = %v, want %v (cells %v)", i, k, cells[i][k], v, cells)
			}
		}
	}
	// An id this list does not hold (another user's Me preset) stays, even when the name matches.
	other := makePanel(t, a, ana, `{"name":"Other","cells":[{"kind":"preset","scope":"everyone","id":"feedfeedfeedfeed","name":"Studio A","r":1,"c":1}]}`)
	if o, _ := a.panels.get(other); o.Pages[0].Cells[0]["id"] != "feedfeedfeedfeed" {
		t.Fatalf("unknown id replaced: %v", o.Pages[0].Cells[0])
	}
	if _, has := cells[4]["router"]; has {
		t.Fatalf("a Take cell got a router: %v", cells[4])
	}

	cfg := a.r().cfg
	cfg.Presets = config.ClonePresets(cfg.Presets)
	cfg.Presets[0].Name = "Studio B"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	// The editor sends back what it read, so the cell still says the old name.
	body, _ := json.Marshal(map[string]any{"name": "Refs", "cells": cells})
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, string(body)); rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	p, _ := a.panels.get(id)
	if c := p.Pages[0].Cells[3]; c["id"] != pid || c["name"] != "Studio B" {
		t.Fatalf("preset cell after rename %v", c)
	}

	f.seed(2, 3, 0)
	src, dst := p.Pages[0].Cells[0]["n"], p.Pages[0].Cells[1]["n"]
	route, _ := json.Marshal(map[string]any{"routes": []any{map[string]any{"src": src, "dst": dst, "levels": []int{1}}}})
	rec = apiPost(a, "/api/v1/takes", bot, string(route), nil)
	r0 := apiBody(t, rec)["results"].([]any)[0].(map[string]any)
	if rec.Code != 200 || r0["src"] != float64(1) || r0["dst"] != float64(2) || r0["ok"] != true {
		t.Fatalf("fired %d %s", rec.Code, rec.Body.String())
	}
}

// Templates store the same shape. A template that leaves contents out drops the references too.
func TestTemplateCellsStoreRouterReferences(t *testing.T) {
	a := panelsApp(t)
	uid := a.r().cfg.UID
	pid := a.r().cfg.Presets[0].ID
	ana := userKey(t, a, "Ana")

	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"With","scope":"me","sources":true,"dests":true,"cells":`+refCells+`}`)
	if rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	cells := cellsOf(t, apiBody(t, rec))
	if c := cells[0]; c["router"] != uid || c["n"] != float64(1) || c["name"] != "CAM 1" {
		t.Fatalf("source cell %v", c)
	}
	if c := cells[1]; c["router"] != uid || c["n"] != float64(2) || c["name"] != "MON 2" {
		t.Fatalf("destination cell %v", c)
	}
	if c := cells[3]; c["id"] != pid || c["name"] != "Studio A" {
		t.Fatalf("preset cell %v", c)
	}

	rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Without","scope":"me","cells":`+refCells+`}`)
	if rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	for _, c := range cellsOf(t, apiBody(t, rec)) {
		for _, k := range []string{"router", "n", "name", "id", "scope"} {
			if _, has := c[k]; has {
				t.Fatalf("a template without contents kept %s: %v", k, c)
			}
		}
	}
}
