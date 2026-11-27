package app

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

const otherRouterUID = "0123456789abcdef"

// otherRouter makes a file look like another install's, made on another router.
func otherRouter(t *testing.T, a *App, file []byte) []byte {
	t.Helper()
	uid := a.r().cfg.Routers[0].UID
	files := map[string]string{}
	for name, b := range unzipAll(t, foreign(t, file)) {
		files[strings.ReplaceAll(name, uid, otherRouterUID)] = strings.ReplaceAll(string(b), uid, otherRouterUID)
	}
	return zipOf(t, files)
}

// A file made where CAM 3 is source 23, imported where CAM 3 is source 40.
func cam3File(t *testing.T) (*App, string, []byte) {
	t.Helper()
	a, _, tok := importApp(t)
	a.mu.Lock()
	a.first.router.srcNames[23] = "CAM 3"
	a.mu.Unlock()
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = []config.Preset{{Name: "Show", Items: []config.PresetItem{{Src: cfg.SourceRef(23, "CAM 3"), Dsts: []config.Ref{cfg.DestinationRef(4, "MON 4")}}}}}
	})
	file := otherRouter(t, a, exported(t, a, tok, exportReq{Categories: []string{catPresets}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = nil })
	a.mu.Lock()
	a.first.router.srcNames[23] = "VT 1"
	a.first.router.srcNames[40] = "CAM 3"
	a.mu.Unlock()
	return a, tok, file
}

func remapKey(kind string, n int) string { return otherRouterUID + ":" + kind + ":" + itoa(int64(n)) }

func TestImportRemapMovesPresetToTargetNumber(t *testing.T) {
	a, tok, file := cam3File(t)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Names: map[string][]int{remapKey(config.MissingSource, 23): {40}}}))
	p, ok := presetNamed(t, a, "Show")
	if !ok {
		t.Fatalf("no preset: %+v %+v", res.Plan.Categories[catPresets], res.Plan.Remap)
	}
	src := p.Items[0].Src
	if src.Router != a.r().cfg.Routers[0].UID || src.N != 40 || src.Name != "CAM 3" || len(p.Broken) != 0 {
		t.Fatalf("preset: %+v", p)
	}
	if d := p.Items[0].Dsts[0]; d.Router != a.r().cfg.Routers[0].UID || d.N != 4 {
		t.Errorf("destination: %+v", d)
	}
}

// With no choice from the page, an unmatched name is Skip: the item keeps its number and arrives Broken.
func TestImportRemapUnmatchedDefaultsToSkip(t *testing.T) {
	a, tok, file := cam3File(t)
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	p, _ := presetNamed(t, a, "Show")
	want := []config.Missing{{Kind: config.MissingSource, Ref: config.Ref{Router: a.r().cfg.Routers[0].UID, N: 23, Name: "CAM 3"}}}
	if p.Items[0].Src.N != 23 || len(p.Broken) != 1 || p.Broken[0].Kind != want[0].Kind || p.Broken[0].Ref.N != 23 {
		t.Fatalf("preset: %+v", p)
	}
}

// The check answers the file router with its default target and one row per distinct reference.
func TestImportCheckAnswersRemapRows(t *testing.T) {
	a, tok, file := cam3File(t)
	rec := postImport(t, a, tok, file)
	var check importCheck
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &check) != nil {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	rm := check.Plan.Remap
	if rm == nil || len(rm.Routers) != 1 {
		t.Fatalf("remap: %+v", rm)
	}
	r := rm.Routers[0]
	if r.UID != otherRouterUID || r.Target != a.r().cfg.Routers[0].UID || len(r.Candidates) != 1 || r.Candidates[0].UID != a.r().cfg.Routers[0].UID {
		t.Fatalf("router: %+v", r)
	}
	if c := r.Candidates[0]; c.Total == 0 || c.Found == 0 || c.Found > c.Total {
		t.Errorf("found: %+v", c)
	}
	i := slices.IndexFunc(r.Refs, func(x remapRef) bool { return x.Kind == config.MissingSource && x.N == 23 })
	if i < 0 || r.Refs[i].Name != "CAM 3" || r.Refs[i].Presets != 1 || r.Refs[i].Key != remapKey(config.MissingSource, 23) {
		t.Fatalf("refs: %+v", r.Refs)
	}
	if len(rm.Targets) != 1 || !slices.ContainsFunc(rm.Targets[0].Sources, func(x namedItem) bool { return x.N == 40 && x.Name == "CAM 3" }) {
		t.Errorf("targets: %+v", rm.Targets)
	}
}

func TestImportRemapNoTargetDropsItsItems(t *testing.T) {
	a, tok, file := cam3File(t)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Routers: map[string]string{otherRouterUID: ""}}))
	if _, ok := presetNamed(t, a, "Show"); ok {
		t.Fatal("a preset on a router with no target was imported")
	}
	items := res.Plan.Categories[catPresets].Items
	if len(items) != 1 || !strings.Contains(items[0].Refused, "has no target here") || items[0].writes() {
		t.Fatalf("plan: %+v", items)
	}
}

func TestImportRemapRefusesBadTargets(t *testing.T) {
	live := config.Config{Routers: []config.Router{{UID: "live"}}}
	for _, bad := range []map[string]string{{"a": "nope"}, {"a": "live", "b": "live"}} {
		if err := checkRouterTargets(importReq{Routers: bad}, live); err == nil {
			t.Errorf("%v: no error", bad)
		}
	}
	if err := checkRouterTargets(importReq{Routers: map[string]string{"a": "live", "b": ""}}, live); err != nil {
		t.Error(err)
	}
}

// A panel cell follows its row too, and Add both gives a preset both destinations.
func TestImportRemapMovesPanelCellsAndAddsBoth(t *testing.T) {
	a, _, tok := importApp(t)
	uid := a.r().cfg.Routers[0].UID
	a.mu.Lock()
	a.first.router.srcNames[23] = "CAM 3"
	a.mu.Unlock()
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	desk, err := a.panels.create(Panel{Name: "Desk", Owner: userID(t, a, "Ada"), Rows: 1, Cols: 1, Pages: []PanelPage{{ID: "p1", Enabled: true,
		Cells: []map[string]any{{"kind": "source", "router": uid, "n": 23, "name": "CAM 3", "r": 1, "c": 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	file := otherRouter(t, a, exported(t, a, tok, exportReq{Categories: []string{catPresets, catPanels}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = nil })
	if _, err := a.panels.delete(desk.ID); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.first.router.srcNames[23], a.first.router.srcNames[40] = "VT 1", "CAM 3"
	a.first.router.dstNames[4], a.first.router.dstNames[6], a.first.router.dstNames[7] = "OUT", "MON 4", "MON 4"
	a.mu.Unlock()
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Names: map[string][]int{
		remapKey(config.MissingSource, 23):     {40},
		remapKey(config.MissingDestination, 4): {6, 7},
	}}))
	p, _ := presetNamed(t, a, "Show")
	if d := p.Items[0].Dsts; len(d) != 2 || d[0].N != 6 || d[1].N != 7 || len(p.Broken) != 0 {
		t.Fatalf("preset: %+v", p)
	}
	ps := a.panels.list()
	if len(ps) != 1 {
		t.Fatalf("panels: %+v", ps)
	}
	if r, _ := cellRef(ps[0].Pages[0].Cells[0]); r.N != 40 || r.Router != uid || len(ps[0].Broken) != 0 {
		t.Fatalf("cell: %+v broken %+v", ps[0].Pages[0].Cells[0], ps[0].Broken)
	}
}
