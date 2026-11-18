package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// makeGridPanel is a Grid panel owned by the key's user, every edge "all".
func makeGridPanel(t *testing.T, a *App, key, name string) string {
	t.Helper()
	return makePanel(t, a, key, `{"name":"`+name+`","type":"grid"}`)
}

func TestGridPanelReadsAsGridWithAllEdgesAndNoPages(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	p := readPanel(t, a, ana, makeGridPanel(t, a, ana, "Studio grid"))
	if p["type"] != "grid" {
		t.Errorf("type %v", p["type"])
	}
	for _, edge := range []string{"sources", "destinations"} {
		e, _ := p[edge].(map[string]any)
		if e["members"] != "all" {
			t.Errorf("%s %v, want members all", edge, p[edge])
		}
	}
	if pages, ok := p["pages"].([]any); !ok || len(pages) != 0 {
		t.Errorf("pages %#v, want []", p["pages"])
	}
}

func TestPanelWithNoTypeIsAButtonsPanel(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	p := readPanel(t, a, ana, makePanel(t, a, ana, `{"name":"Old style"}`))
	if p["type"] != "buttons" {
		t.Errorf("type %v", p["type"])
	}
	if _, has := p["sources"]; has {
		t.Errorf("a Buttons panel read has sources: %v", p["sources"])
	}
}

// A file saved before Grid panels has no type: it loads as a Buttons panel with its Pages as they were.
func TestPanelFileFromBeforeGridLoadsAsButtons(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Fixture"}`)
	old := "id: " + id + `
name: Fixture
owner: Ana
acts: session
share:
  mode: me
rows: 2
cols: 3
pages:
  - id: aa11
    enabled: true
    cells:
      - {r: 1, c: 1, kind: source, n: 4}
settings: {}
rev: 3
`
	if err := os.WriteFile(fixPanelFile(a, id), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	b := fixRestart(t, a, func(*config.Config) {})
	p, ok := b.panels.get(id)
	if !ok {
		t.Fatal("the old file did not load")
	}
	if p.Type != panelTypeButtons || p.Sources != nil || p.Destinations != nil {
		t.Errorf("type %q sources %v destinations %v", p.Type, p.Sources, p.Destinations)
	}
	got, _ := json.Marshal(p.Pages)
	if string(got) != `[{"id":"aa11","enabled":true,"cells":[{"c":1,"kind":"source","n":4,"r":1}]}]` {
		t.Errorf("pages %s", got)
	}
}

// settings.mode is free-form to the server (the page reads it, gridpanelstage.js createStage). Each of its three
// values, and a Grid file saved before takeOnly with mode take, must come back as stored after a restart.
func TestGridPanelTakeModesSurviveARestart(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	ids := map[string]string{}
	for _, mode := range []string{"instant", "take", "takeOnly"} {
		id := makeGridPanel(t, a, ana, "G "+mode)
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"G `+mode+`","settings":{"mode":"`+mode+`"}}`); rec.Code != 200 {
			t.Fatalf("PUT mode %s: %d %s", mode, rec.Code, rec.Body.String())
		}
		ids[mode] = id
	}
	old := makeGridPanel(t, a, ana, "Old grid")
	file := "id: " + old + `
name: Old grid
owner: Ana
acts: session
share:
  mode: me
type: grid
sources:
  members: all
destinations:
  members: [3, 1]
settings:
  mode: take
rev: 2
`
	if err := os.WriteFile(fixPanelFile(a, old), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	ids["take (old file)"] = old
	b := fixRestart(t, a, func(*config.Config) {})
	for mode, id := range ids {
		p := readPanel(t, b, ana, id)
		s, _ := p["settings"].(map[string]any)
		want := strings.TrimSuffix(mode, " (old file)")
		if p["type"] != "grid" || s["mode"] != want {
			t.Errorf("%s: type %v, settings %v", mode, p["type"], p["settings"])
		}
	}
}

func TestGridPanelReadSentBackIsAccepted(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "Round trip")
	body := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.String()
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT of the read: %d %s", rec.Code, rec.Body.String())
	}
	if p := readPanel(t, a, ana, id); p["type"] != "grid" {
		t.Errorf("type after round trip %v", p["type"])
	}
}

func TestGridPanelRefusesCellsAndPages(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	for _, body := range []string{
		`{"name":"G","type":"grid","pages":[{"cells":[]}]}`,
		`{"name":"G","type":"grid","cells":[{"r":1,"c":1,"kind":"take"}]}`,
	} {
		if rec := apiSend(a, "POST", "/api/v1/panels", ana, body); rec.Code != 400 {
			t.Errorf("create %s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	id := makeGridPanel(t, a, ana, "G")
	before := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.String()
	for _, body := range []string{
		`{"name":"G","type":"grid","cells":[{"r":1,"c":1,"kind":"take"}]}`,
		`{"name":"G","cells":[{"r":1,"c":1,"kind":"take"}]}`,
		`{"name":"G","pages":[{"cells":[]}]}`,
	} {
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 400 {
			t.Errorf("PUT %s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
	if after := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.String(); after != before {
		t.Errorf("a refused PUT changed the panel:\n%s\nwas\n%s", after, before)
	}
}

func TestPanelTypeIsFixedWhenMade(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	grid := makeGridPanel(t, a, ana, "G")
	buttons := makePanel(t, a, ana, `{"name":"B"}`)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+grid, ana, `{"name":"G","type":"buttons"}`); rec.Code != 409 {
		t.Errorf("grid to buttons: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+buttons, ana, `{"name":"B","type":"grid"}`); rec.Code != 409 {
		t.Errorf("buttons to grid: %d %s", rec.Code, rec.Body.String())
	}
	// An older client sends no type: the stored one stays.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+grid, ana, `{"name":"G renamed"}`); rec.Code != 200 {
		t.Fatalf("PUT with no type: %d %s", rec.Code, rec.Body.String())
	}
	p := readPanel(t, a, ana, grid)
	if p["type"] != "grid" || p["name"] != "G renamed" {
		t.Errorf("after a PUT with no type: type %v name %v", p["type"], p["name"])
	}
}

func TestPanelBodyRefusesBadTypeAndEdges(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	for _, body := range []string{
		`{"name":"X","type":"knobs"}`,
		`{"name":"X","type":"grid","sources":{"members":"some"}}`,
		`{"name":"X","type":"grid","sources":{"members":7}}`,
		`{"name":"X","sources":{"members":"all"}}`,
	} {
		rec := apiSend(a, "POST", "/api/v1/panels", ana, body)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "bad_request") {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body.String())
		}
	}
}

// A Push between a Grid panel and a Buttons panel changes neither.
func TestPushRefusesTheOtherType(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	grid := makeGridPanel(t, a, ana, "G")
	buttons := makePanel(t, a, ana, `{"name":"B","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	before := cellCount(readPanel(t, a, ana, buttons))
	for _, c := range [][2]string{{grid, buttons}, {buttons, grid}} {
		rec := apiSend(a, "POST", "/api/v1/panels/"+c[0]+"/push", ana, `{"targets":["`+c[1]+`"]}`)
		if rec.Code != 409 {
			t.Errorf("push %s to %s: %d %s", c[0], c[1], rec.Code, rec.Body.String())
		}
	}
	if got := cellCount(readPanel(t, a, ana, buttons)); got != before {
		t.Errorf("the Buttons target has %d cells, want %d", got, before)
	}
}

// Copy keeps the type and the edges.
func TestGridPanelCopyIsAGridPanel(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", ana, `{}`)
	if rec.Code != 201 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body.String())
	}
	c := apiBody(t, rec)
	if c["type"] != "grid" || c["sources"].(map[string]any)["members"] != "all" {
		t.Errorf("copy %v", c)
	}
}
