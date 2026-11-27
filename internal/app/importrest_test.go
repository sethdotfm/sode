package app

import (
	"slices"
	"testing"
)

// This browser is the page's to apply: the check hands it the file's values, and the plan lists it.
func TestImportCheckHandsThisBrowserToThePage(t *testing.T) {
	a, _, tok := importApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catBrowser}, Browser: []byte(`{"theme":"dark","presets":[]}`)})
	check := postCheck(t, a, tok, file, importReq{Mode: importAdd, Categories: []string{catBrowser}})
	if check.Browser["theme"] != "dark" {
		t.Errorf("browser: %v", check.Browser)
	}
	if it := planItem(t, check.Plan, catBrowser, "This browser"); it.Key != catBrowser || !it.writes() {
		t.Errorf("item: %+v", it)
	}
}

const testSVG = `<svg xmlns="http://www.w3.org/2000/svg"/>`

// seedPanelExtras gives the install a panel with a Push checkpoint, a template and a Checkpoint set. All three
// use one Icon library icon, as does a Default artwork entry.
func seedPanelExtras(t *testing.T, a *App) (Panel, Template, PanelSet, LibIcon) {
	t.Helper()
	ana := userID(t, a, "Ana")
	ic, err := a.icons.add("Star", []byte(testSVG), "image/svg+xml")
	if err != nil {
		t.Fatal(err)
	}
	cells := []map[string]any{{"kind": "take", "art": "lib:" + ic.ID}}
	p, err := a.panels.create(Panel{Name: "Desk", Owner: ana, Rows: 1, Cols: 1, Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: cells}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.panels.pushLayout(p.ID, layoutOf(p), ana, "push", nil); err != nil {
		t.Fatal(err)
	}
	tpl, err := a.templates.create(Template{Name: "Row", Scope: templateEveryone, Owner: ana, Rows: 1, Cols: 1,
		Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: cells}}})
	if err != nil {
		t.Fatal(err)
	}
	set, err := a.panels.sets.create(PanelSet{Name: "Show A", Scope: panelShareAll, Owner: ana,
		Entries: []PanelSetEntry{{Panel: p.ID, Name: p.Name, panelLayout: layoutOf(p)}}})
	if err != nil {
		t.Fatal(err)
	}
	n := 2
	if _, err := a.artDefaults.set(ArtDefault{Kind: artKindSource, Router: a.r().cfg.Routers[0].UID, N: &n, Name: "CAM 2", Art: "lib:" + ic.ID}, nil); err != nil {
		t.Fatal(err)
	}
	return p, tpl, set, ic
}

func cellArt(pages []PanelPage) string {
	s, _ := pages[0].Cells[0]["art"].(string)
	return s
}

// A backup of this install brings templates, Checkpoint sets, Push checkpoints and Artwork back as they were.
func TestAddImportBringsTemplatesSetsCheckpointsAndArtwork(t *testing.T) {
	a, _, tok := importApp(t)
	p, tpl, set, ic := seedPanelExtras(t, a)
	file := exported(t, a, tok, exportReq{Categories: []string{catPanels, catArtwork}})
	if _, err := a.panels.delete(p.ID); err != nil {
		t.Fatal(err)
	}
	a.templates.delete(tpl.ID)
	a.panels.sets.delete(set.ID)
	a.icons.remove(ic.ID)
	a.artDefaults.clear(a.artDefaults.list()[0])

	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Categories: []string{catPanels, catArtwork}}))
	if len(res.Failed) != 0 {
		t.Errorf("failed: %v", res.Failed)
	}
	if ps := a.panels.list(); len(ps) != 1 || ps[0].ID != p.ID {
		t.Errorf("the panel did not keep its id, so its links break: %+v", ps)
	}
	if got := a.icons.list(); len(got) != 1 || got[0].ID != ic.ID || got[0].Name != "Star" {
		t.Fatalf("icons: %+v", got)
	}
	if d := a.artDefaults.list(); len(d) != 1 || d[0].Art != "lib:"+ic.ID || *d[0].N != 2 {
		t.Fatalf("defaults: %+v", d)
	}
	ps := a.panels.list()
	if len(ps) != 1 {
		t.Fatalf("panels: %+v", ps)
	}
	if cps, _ := a.panels.checkpoints(ps[0].ID); len(cps) != 1 || cellArt(cps[0].Layout.Pages) != "lib:"+ic.ID {
		t.Fatalf("checkpoints: %+v", cps)
	}
	if ts := a.templates.list(); len(ts) != 1 || ts[0].ID != tpl.ID || ts[0].Name != "Row" || cellArt(ts[0].Pages) != "lib:"+ic.ID {
		t.Fatalf("templates: %+v", ts)
	}
	sets := a.panels.sets.sorted(nil)
	if len(sets) != 1 || sets[0].Name != "Show A" || len(sets[0].Entries) != 1 || sets[0].Entries[0].Panel != ps[0].ID {
		t.Fatalf("sets: %+v", sets)
	}
}

// A foreign file's icon that clashes by name arrives renamed with its own id, and the cells that used it follow.
func TestAddImportForeignIconRenameKeepsCells(t *testing.T) {
	a, _, tok := importApp(t)
	_, tpl, set, ic := seedPanelExtras(t, a)
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catPanels, catArtwork}}))
	// Here the live Star is a different file under a different id.
	a.icons.remove(ic.ID)
	live, err := a.icons.add("Star", []byte(`<svg xmlns="http://www.w3.org/2000/svg"><g/></svg>`), "image/svg+xml")
	if err != nil {
		t.Fatal(err)
	}
	a.templates.delete(tpl.ID)
	a.panels.sets.delete(set.ID)
	a.artDefaults.clear(a.artDefaults.list()[0])

	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Categories: []string{catPanels, catArtwork}}))
	if res.Plan.Critical {
		t.Error("a foreign Rename is not Critical")
	}
	icons := a.icons.list()
	i := slices.IndexFunc(icons, func(x LibIcon) bool { return x.Name == "Star 2" })
	if len(icons) != 2 || i < 0 || icons[i].ID == live.ID {
		t.Fatalf("icons: %+v", icons)
	}
	want := "lib:" + icons[i].ID
	if ts := a.templates.list(); len(ts) != 1 || cellArt(ts[0].Pages) != want {
		t.Fatalf("templates: %+v, want %s", ts, want)
	}
	for _, p := range a.panels.list() {
		if p.Name == "Desk 2" && cellArt(p.Pages) != want {
			t.Fatalf("panel: %+v, want %s", p, want)
		}
	}
}
