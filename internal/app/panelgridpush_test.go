package app

import (
	"encoding/json"
	"errors"
	"testing"
)

// Push, checkpoints and checkpoint sets of Grid panels (ADR 0011).

func gridWithEdges(t *testing.T, a *App, key, name string) string {
	t.Helper()
	id := makeGridPanel(t, a, key, name)
	body := `{"name":"` + name + `","sources":{"members":"all","hide":[3],"items":{"4":{"pinned":true}}},"destinations":{"members":[9,2],"items":{"9":{"art":"icon:camera"}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, key, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	return id
}

const (
	gridSourcesWant = `{"hide":[3],"items":{"4":{"pinned":true}},"members":"all"}`
	gridDestsWant   = `{"items":{"9":{"art":"icon:camera"}},"members":[9,2]}`
)

func TestPushCarriesGridEdges(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := gridWithEdges(t, a, ana, "Src")
	dst := makeGridPanel(t, a, ana, "Dst")
	if code, body := pushTo(a, ana, src, dst); code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	got := readPanel(t, a, ana, dst)
	if s := gridEdgeJSON(t, got, "sources"); s != gridSourcesWant {
		t.Errorf("sources %s", s)
	}
	if s := gridEdgeJSON(t, got, "destinations"); s != gridDestsWant {
		t.Errorf("destinations %s", s)
	}
	if got["type"] != "grid" || got["name"] != "Dst" {
		t.Errorf("identity %v %v", got["type"], got["name"])
	}
}

func TestRestoreCheckpointPutsGridEdgesBack(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makeGridPanel(t, a, ana, "Src")
	dst := gridWithEdges(t, a, ana, "Dst")
	pushTo(a, ana, src, dst)
	if s := gridEdgeJSON(t, readPanel(t, a, ana, dst), "destinations"); s != `{"members":"all"}` {
		t.Fatalf("push did not land: %s", s)
	}
	cps := checkpointsOf(t, a, ana, dst)
	rec := apiSend(a, "POST", "/api/v1/panels/"+dst+"/checkpoints/"+cps[0]["id"].(string)+"/restore", ana, ``)
	if rec.Code != 200 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	got := readPanel(t, a, ana, dst)
	if s := gridEdgeJSON(t, got, "destinations"); s != gridDestsWant {
		t.Errorf("destinations %s", s)
	}
	if got["type"] != "grid" {
		t.Errorf("type %v", got["type"])
	}
}

func TestPushBetweenTypesIs409BothWays(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	g := makeGridPanel(t, a, ana, "G")
	b := makePanel(t, a, ana, `{"name":"B","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	for _, pair := range [][2]string{{g, b}, {b, g}} {
		if code, _ := pushTo(a, ana, pair[0], pair[1]); code != 409 {
			t.Errorf("push %s to %s: %d", pair[0], pair[1], code)
		}
	}
	if len(checkpointsOf(t, a, ana, b)) != 0 || len(checkpointsOf(t, a, ana, g)) != 0 {
		t.Error("a refused push kept a checkpoint")
	}
}

func TestCheckpointSetCarriesGridEdges(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	g := gridWithEdges(t, a, ana, "G")
	rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Show", "me", g))
	if rec.Code != 201 {
		t.Fatalf("create set: %d %s", rec.Code, rec.Body.String())
	}
	sid := apiBody(t, rec)["id"].(string)
	apiSend(a, "PUT", "/api/v1/panels/"+g, ana, `{"name":"G","sources":{"members":[1]},"destinations":{"members":"all"}}`)
	if rec := apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, `{}`); rec.Code != 200 {
		t.Fatalf("push set: %d %s", rec.Code, rec.Body.String())
	}
	got := readPanel(t, a, ana, g)
	if s := gridEdgeJSON(t, got, "sources"); s != gridSourcesWant {
		t.Errorf("sources %s", s)
	}
	if s := gridEdgeJSON(t, got, "destinations"); s != gridDestsWant {
		t.Errorf("destinations %s", s)
	}
}

// A layout of the other type is refused whatever the way in, so a Checkpoint set entry never blanks a panel.
func TestApplyLayoutRefusesTheOtherType(t *testing.T) {
	a := panelsApp(t)
	b, err := a.panels.create(Panel{Name: "B", Owner: "Ana"})
	if err != nil {
		t.Fatal(err)
	}
	g, err := a.panels.create(Panel{Name: "G", Owner: "Ana", Type: panelTypeGrid})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.panels.applyLayout(b.ID, layoutOf(g), "Ana", "t"); !errors.Is(err, errLayoutType) {
		t.Errorf("grid layout on buttons panel: %v", err)
	}
	if _, _, err := a.panels.applyLayout(g.ID, layoutOf(b), "Ana", "t"); !errors.Is(err, errLayoutType) {
		t.Errorf("buttons layout on grid panel: %v", err)
	}
	if cps, _ := a.panels.checkpoints(g.ID); len(cps) != 0 {
		t.Error("a refused layout kept a checkpoint")
	}
}

func TestSetPushOfTheOtherTypeIs409AndChangesNothing(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	g := makeGridPanel(t, a, ana, "G")
	b := makePanel(t, a, ana, `{"name":"B","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Mixed", "me", b, g))
	sid := apiBody(t, rec)["id"].(string)
	// The entries name panels by id, so swap them to simulate a panel id that now holds the other type.
	a.panels.mu.Lock()
	set, _ := a.panels.sets.get(sid)
	set.Entries[0].Panel, set.Entries[1].Panel = g, b
	a.panels.sets.by[sid] = set
	a.panels.mu.Unlock()
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, `{}`), 409, "conflict")
	if cellCount(readPanel(t, a, ana, b)) != 1 || len(checkpointsOf(t, a, ana, g)) != 0 {
		t.Error("a refused set push changed a panel")
	}
}

func TestOldLayoutWithNoTypeAppliesToButtonsPanel(t *testing.T) {
	a := panelsApp(t)
	p, err := a.panels.create(Panel{Name: "B", Owner: "Ana"})
	if err != nil {
		t.Fatal(err)
	}
	q, _, err := a.panels.applyLayout(p.ID, panelLayout{Rows: 1, Cols: 1, LegacyCells: []map[string]any{{"r": 1, "c": 1, "kind": "take"}}}, "Ana", "t")
	if err != nil || q.Type != panelTypeButtons {
		t.Fatalf("%+v %v", q, err)
	}
}

func TestGridIconUsesAndDefaultsWalkItems(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","sources":{"members":[1,5],"items":{"5":{"art":"lib:i-1"}}},"destinations":{"members":"all","hide":[7]}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT %d %s", rec.Code, rec.Body.String())
	}
	var uses map[string]int
	rec := apiSend(a, "GET", "/api/v1/icon-library/i-1/uses", ana, ``)
	json.Unmarshal(rec.Body.Bytes(), &uses)
	if uses["cells"] != 1 || uses["panels"] != 1 {
		t.Errorf("uses %s", rec.Body.String())
	}

	for _, d := range []string{
		`{"kind":"source","n":1,"art":"icon:camera"}`,
		`{"kind":"source","n":2,"art":"icon:camera"}`,
		`{"kind":"destination","n":6,"art":"icon:mic"}`,
		`{"kind":"destination","n":7,"art":"icon:mic"}`,
	} {
		if rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, d); rec.Code != 200 {
			t.Fatalf("default %s: %d %s", d, rec.Code, rec.Body.String())
		}
	}
	got := map[string]bool{}
	for _, x := range readPanel(t, a, ana, id)["artDefaults"].([]any) {
		m := x.(map[string]any)
		n := int(m["n"].(float64))
		got[m["kind"].(string)+"/"+string(rune('0'+n))] = true
	}
	if len(got) != 2 || !got["source/1"] || !got["destination/6"] {
		t.Errorf("artDefaults %v, want source 1 and destination 6", got)
	}
}
