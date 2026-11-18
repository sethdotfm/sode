package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testUID = "2c5bb739e7cf2f2a"

func writeStore(t *testing.T, dir, name, text string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
}

// A panel file reads no bare router number: a cell, Grid edge or fixed destination with no router skips the panel
// and says why. The same panel with its routers loads.
func TestPanelFileWithNoRouterIsSkipped(t *testing.T) {
	cases := map[string]string{
		"source cell":       "pages:\n  - id: a\n    cells: [{r: 1, c: 1, kind: source, n: 1}]\n",
		"destination cell":  "pages:\n  - id: a\n    cells: [{r: 1, c: 1, kind: destination, n: 2}]\n",
		"fixed destination": "settings: {fixedDests: [2]}\npages:\n  - id: a\n    cells: []\n",
		"fixed no router":   "settings: {fixedDests: [{n: 2}]}\npages:\n  - id: a\n    cells: []\n",
		"grid member":       "type: grid\nsources: {members: [{n: 1}]}\ndestinations: {members: all}\n",
		"grid hide":         "type: grid\nsources: {members: all, hide: [{n: 1}]}\ndestinations: {members: all}\n",
		"grid item":         "type: grid\nsources: {members: all, items: [{n: 1, pinned: true}]}\ndestinations: {members: all}\n",
		"grid plain number": "type: grid\nsources: {members: [1]}\ndestinations: {members: all}\n",
	}
	for name, body := range cases {
		dir := t.TempDir()
		writeStore(t, dir, "panels/p-0000000a.yaml", "id: p-0000000a\nname: Bad\n"+body)
		s := loadPanels(filepath.Join(dir, "config.yaml"))
		if _, ok := s.get("p-0000000a"); ok {
			t.Errorf("%s: loaded", name)
		}
		if p := strings.Join(s.problems(), " "); !strings.Contains(p, "p-0000000a") {
			t.Errorf("%s: problems %q", name, p)
		}
	}
	dir := t.TempDir()
	r := "router: " + testUID + ", "
	writeStore(t, dir, "panels/p-0000000b.yaml", "id: p-0000000b\nname: Good\nsettings: {fixedDests: [{"+r+"n: 2}]}\n"+
		"pages:\n  - id: a\n    cells: [{r: 1, c: 1, kind: source, "+r+"n: 1}, {r: 1, c: 2, kind: take}]\n")
	writeStore(t, dir, "panels/p-0000000c.yaml", "id: p-0000000c\nname: Grid\ntype: grid\n"+
		"sources: {members: all, hide: [{"+r+"n: 1}], items: [{"+r+"n: 2, pinned: true}]}\ndestinations: {members: [{"+r+"n: 3}]}\n")
	s := loadPanels(filepath.Join(dir, "config.yaml"))
	for _, id := range []string{"p-0000000b", "p-0000000c"} {
		if _, ok := s.get(id); !ok {
			t.Errorf("%s not loaded: %v", id, s.problems())
		}
	}
}

// Templates, checkpoints, Checkpoint sets and Default artwork read no bare router number either: the entry is
// skipped, and its neighbours with routers load.
func TestStoredLayoutsWithNoRouterAreSkipped(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	good := "{r: 1, c: 1, kind: source, router: " + testUID + ", n: 1}"
	bad := "{r: 1, c: 1, kind: source, n: 1}"
	tpl := func(id, cell string) string {
		return "id: " + id + "\nname: " + id + "\nscope: me\nowner: u1\nsources: true\nrows: 4\ncols: 8\npages:\n  - id: a\n    cells: [" + cell + "]\n"
	}
	writeStore(t, dir, "templates/me/t-0000000a.yaml", tpl("t-0000000a", bad))
	writeStore(t, dir, "templates/me/t-0000000b.yaml", tpl("t-0000000b", good))
	ts := loadTemplates(cfgPath)
	if _, ok := ts.get("t-0000000a"); ok {
		t.Error("template with no router loaded")
	}
	if _, ok := ts.get("t-0000000b"); !ok {
		t.Error("template with its router skipped")
	}

	layout := func(cell string) string {
		return "  layout: {rows: 4, cols: 8, settings: {}, pages: [{id: a, cells: [" + cell + "]}]}\n"
	}
	writeStore(t, dir, "panels/p-0000000a.yaml", "id: p-0000000a\nname: P\npages: [{id: a, cells: []}]\n")
	writeStore(t, dir, "panels/p-0000000a.checkpoints", "- id: c-0000000a\n  by: Ana\n  why: push\n"+layout(bad)+
		"- id: c-0000000b\n  by: Ana\n  why: push\n"+layout(good))
	ps := loadPanels(cfgPath)
	if cps, _ := ps.checkpoints("p-0000000a"); len(cps) != 1 || cps[0].ID != "c-0000000b" {
		t.Errorf("checkpoints %+v", cps)
	}

	writeStore(t, dir, "panelsets/everyone.yaml", "sets:\n"+
		"  - {id: s-0000000a, name: A, scope: everyone, owner: u1, entries: [{panel: p-0000000a, name: P, rows: 4, cols: 8, settings: {}, pages: [{id: a, cells: ["+bad+"]}]}]}\n"+
		"  - {id: s-0000000b, name: B, scope: everyone, owner: u1, entries: [{panel: p-0000000a, name: P, rows: 4, cols: 8, settings: {}, pages: [{id: a, cells: ["+good+"]}]}]}\n")
	sets := loadPanelSets(cfgPath)
	if _, ok := sets.get("s-0000000a"); ok {
		t.Error("set with no router loaded")
	}
	if _, ok := sets.get("s-0000000b"); !ok {
		t.Error("set with its router skipped")
	}

	writeStore(t, dir, "artdefaults.yaml", "rev: 1\ndefaults:\n  - {kind: source, n: 1, name: CAM 1, art: 'icon:camera'}\n"+
		"  - {kind: destination, router: "+testUID+", n: 2, name: MON 2, art: 'icon:camera'}\n")
	ds := loadArtDefaults(cfgPath)
	if got := ds.list(); len(got) != 1 || got[0].Kind != artKindDst {
		t.Errorf("defaults %+v", got)
	}
}
