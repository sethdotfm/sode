package app

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// addRouter adds router prefix with this UID after the others, with its names. Its unit is made but not linked.
func addRouter(t *testing.T, a *App, uid, prefix string, src, dst map[int]string) {
	t.Helper()
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error {
		rc := c.Router
		rc.UID, rc.Prefix, rc.Title, rc.Address = uid, prefix, "Router "+prefix, "127.0.0.2:"+itoa(int64(len(c.MoreRouters)+2))
		c.MoreRouters = append(c.MoreRouters, rc)
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	u, ok := a.unit(uid)
	if !ok {
		t.Fatalf("no unit for router %s", prefix)
	}
	a.mu.Lock()
	u.router.srcNames, u.router.dstNames = src, dst
	a.mu.Unlock()
}

const (
	fileRouterB = "b0b0b0b0b0b0b0b0"
	liveRouterB = "c0c0c0c0c0c0c0c0"
)

// twoRouterFile: an install with routers A and B exports presets, filter sets, API clients and panels that use both.
func twoRouterFile(t *testing.T) []byte {
	t.Helper()
	a, _, tok := importApp(t)
	addRouter(t, a, fileRouterB, "B", map[int]string{7: "VT 7"}, map[int]string{8: "REC 8"})
	lifeCommit(t, a, func(c *config.Config) {
		b := config.Ref{Router: fileRouterB, N: 7, Name: "VT 7"}
		c.Presets = []config.Preset{
			{ID: "pr-a", Name: "On A", Items: []config.PresetItem{{Src: c.SourceRef(2, "CAM 2"), Dsts: []config.Ref{c.DestinationRef(4, "MON 4")}}}},
			{ID: "pr-b", Name: "On B", Items: []config.PresetItem{{Src: b, Dsts: []config.Ref{{Router: fileRouterB, N: 8, Name: "REC 8"}}}}},
		}
		c.FilterSets = []config.FilterSet{{ID: "fs-b", Name: "B cams", Router: fileRouterB, Src: "1-4"}}
		srcs := []config.Ref{c.SourceRef(2, "CAM 2"), b}
		c.APIClients = []config.APIClient{{Name: "Bot", Scopes: []string{config.ScopeNames},
			Rights: config.Rights{Sources: &srcs, Levels: config.RouterLevels{fileRouterB: {1}}}}}
	})
	if _, err := a.panels.create(Panel{Name: "Desk B", Owner: userID(t, a, "Ada"), Rows: 1, Cols: 2,
		Settings: map[string]any{panelLevelsKey: map[string]any{fileRouterB: []any{1}}},
		Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: []map[string]any{
			{"kind": "source", "n": 7, "router": fileRouterB, "name": "VT 7"},
			{"kind": "source", "n": 2, "router": a.r().cfg.UID, "name": "CAM 2"},
		}}}}); err != nil {
		t.Fatal(err)
	}
	return foreign(t, exported(t, a, tok, exportReq{Categories: []string{catPresets, catFilterSets, catAPIClients, catPanels}}))
}

// twoRouterLive: another install with routers A and B, with the same names and other UIDs.
func twoRouterLive(t *testing.T) (*App, string) {
	t.Helper()
	a, _, tok := importApp(t)
	addRouter(t, a, liveRouterB, "B", map[int]string{7: "VT 7"}, map[int]string{8: "REC 8"})
	return a, tok
}

func TestExportHoldsEveryRouter(t *testing.T) {
	files := unzipAll(t, twoRouterFile(t))
	var man exportManifest
	if err := yaml.Unmarshal(files["manifest.yaml"], &man); err != nil {
		t.Fatal(err)
	}
	if len(man.Routers) != 2 || man.Routers[1].UID != fileRouterB || man.Routers[1].Prefix != "B" {
		t.Fatalf("manifest routers: %+v", man.Routers)
	}
	if _, ok := files["names/"+fileRouterB+".yaml"]; !ok {
		t.Errorf("no names table for router B in %v", slicesSortedKeys(files))
	}
	if !bytes.Contains(files["routers.yaml"], []byte(fileRouterB)) {
		t.Errorf("routers.yaml: %s", files["routers.yaml"])
	}
	// Items write the prefix; with two routers a reference keeps it.
	if p := string(files["presets.yaml"]); !strings.Contains(p, "router: B") || !strings.Contains(p, "router: A") {
		t.Errorf("presets.yaml: %s", p)
	}
}

func TestExportListNamesEachRouter(t *testing.T) {
	a, _, _ := importApp(t)
	addRouter(t, a, fileRouterB, "B", map[int]string{7: "VT 7"}, nil)
	lifeCommit(t, a, func(c *config.Config) {
		c.Presets = []config.Preset{{ID: "pr-b", Name: "On B", Items: []config.PresetItem{{Src: config.Ref{Router: fileRouterB, N: 7, Name: "VT 7"},
			Dsts: []config.Ref{{Router: fileRouterB, N: 8, Name: "REC 8"}}}}}}
	})
	list, err := a.exportList()
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range list.Categories {
		switch c.Name {
		case catRouters:
			if len(c.Items) != 2 || c.Items[1].ID != fileRouterB {
				t.Errorf("routers: %+v", c.Items)
			}
		case catPresets:
			if len(c.Items) != 1 || !c.Items[0].Router || !slices.Equal(c.Items[0].Routers, []string{fileRouterB}) {
				t.Errorf("presets: %+v", c.Items)
			}
		}
	}
}

// Round trip with two routers: each file router goes to the live router with its prefix. Every reference, Rights
// level list and panel level subset then names the live UID.
func TestImportTwoRoutersMapsEveryReference(t *testing.T) {
	file := twoRouterFile(t)
	a, tok := twoRouterLive(t)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	cfg := a.r().cfg
	if rm := res.Plan.Remap; rm == nil || len(rm.Routers) != 2 || len(rm.Targets) != 2 || rm.Routers[1].Target != liveRouterB || len(rm.Routers[1].Candidates) != 2 {
		t.Fatalf("remap: %+v", res.Plan.Remap)
	}
	p, ok := presetNamed(t, a, "On B")
	if !ok {
		t.Fatalf("no preset On B: %+v", res.Plan.Categories[catPresets])
	}
	if it := p.Items[0]; it.Src.Router != liveRouterB || it.Src.N != 7 || it.Dsts[0].Router != liveRouterB || len(p.Broken) != 0 {
		t.Errorf("preset On B: %+v", p)
	}
	if p, _ := presetNamed(t, a, "On A"); p.Items[0].Src.Router != cfg.UID || len(p.Broken) != 0 {
		t.Errorf("preset On A: %+v", p)
	}
	if i := slices.IndexFunc(cfg.FilterSets, func(s config.FilterSet) bool { return s.Name == "B cams" }); i < 0 || cfg.FilterSets[i].Router != liveRouterB {
		t.Errorf("filter sets: %+v", cfg.FilterSets)
	}
	c, _ := cfg.FindAPIClient("Bot")
	if l, ok := c.Rights.Levels[liveRouterB]; !ok || len(c.Rights.Levels) != 1 || !slices.Equal(l, []int{1}) {
		t.Errorf("Rights levels: %v", c.Rights.Levels)
	}
	if c.Rights.Sources == nil || !slices.ContainsFunc(*c.Rights.Sources, func(r config.Ref) bool { return r.Router == liveRouterB && r.N == 7 }) {
		t.Errorf("Rights sources: %+v", c.Rights.Sources)
	}
	i := slices.IndexFunc(a.panels.list(), func(x Panel) bool { return x.Name == "Desk B" })
	if i < 0 {
		t.Fatalf("no panel: %+v", res.Plan.Categories[catPanels])
	}
	panel := a.panels.list()[i]
	if r := panel.Pages[0].Cells[0]["router"]; r != liveRouterB || len(panel.Broken) != 0 {
		t.Errorf("panel cell: %v, Broken %+v", panel.Pages[0].Cells[0], panel.Broken)
	}
	if by, _ := panel.Settings[panelLevelsKey].(map[string]any); len(by) != 1 || by[liveRouterB] == nil {
		t.Errorf("panel levels: %v", panel.Settings)
	}
}

// A file router with no target: the items that use it are not imported and say why; Rights lose that router.
func TestImportUnmappedRouterDropsItsItems(t *testing.T) {
	file := twoRouterFile(t)
	a, tok := twoRouterLive(t)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Routers: map[string]string{fileRouterB: ""}}))
	cfg := a.r().cfg
	refused := func(cat, name string) {
		t.Helper()
		items := res.Plan.Categories[cat].Items
		i := slices.IndexFunc(items, func(it importItem) bool { return it.Name == name })
		if i < 0 || !strings.Contains(items[i].Refused, "Router B, which has no target here") {
			t.Errorf("%s %s: %+v", cat, name, items)
		}
	}
	refused(catPresets, "On B")
	refused(catFilterSets, "B cams")
	refused(catPanels, "Desk B")
	if _, ok := presetNamed(t, a, "On B"); ok {
		t.Error("preset On B was imported")
	}
	if _, ok := presetNamed(t, a, "On A"); !ok {
		t.Error("preset On A was not imported")
	}
	c, ok := cfg.FindAPIClient("Bot")
	if !ok || c.Rights.Levels != nil || slices.ContainsFunc(*c.Rights.Sources, func(r config.Ref) bool { return r.Router != cfg.UID }) {
		t.Errorf("client Rights: %+v %v", c.Rights.Sources, c.Rights.Levels)
	}
	items := res.Plan.Categories[catAPIClients].Items
	if len(items) != 1 || !strings.Contains(items[0].Note, "Rights lose router Router B") {
		t.Errorf("client note: %+v", items)
	}
	for _, uid := range cfg.RouterUIDs() {
		if uid == fileRouterB {
			t.Error("the file's router UID is in the config")
		}
	}
}

func TestRemapDefaults(t *testing.T) {
	live := []config.Router{{UID: "l1", Prefix: "A"}, {UID: "l2", Prefix: "B"}}
	file := []exportRouter{{UID: "f1", Prefix: "B"}, {UID: "l1", Prefix: "C"}, {UID: "f3", Prefix: "D"}}
	got := remapDefaults(file, live, map[string]namesTable{})
	if got["l1"] != "l1" || got["f1"] != "l2" || got["f3"] != "" {
		t.Errorf("two live routers: %v", got)
	}
	one := remapDefaults([]exportRouter{{UID: "f1", Prefix: "X"}}, live[:1], map[string]namesTable{})
	if one["f1"] != "l1" {
		t.Errorf("the only router: %v", one)
	}
}

// Setup restore with two routers: both come back with their UIDs and names, a tie line to router B comes back, and
// a second backup is the same.
func TestRestoreTwoRoutersRoundTrip(t *testing.T) {
	a, tok := exportApp(t)
	seedEverything(t, a)
	seedPanelExtras(t, a)
	addRouter(t, a, fileRouterB, "B", map[int]string{7: "VT 7"}, map[int]string{8: "REC 8"})
	lifeCommit(t, a, func(c *config.Config) {
		c.Presets = append(c.Presets, config.Preset{ID: "pr-b", Name: "On B", Items: []config.PresetItem{{
			Src: config.Ref{Router: fileRouterB, N: 7, Name: "VT 7"}, Dsts: []config.Ref{{Router: fileRouterB, N: 8, Name: "REC 8"}}}}})
		c.TieLines = append(c.TieLines, config.TieLine{ID: "tie-ab", Name: "A TO B", From: c.DestinationRef(4, "MON 4"),
			To: config.Ref{Router: fileRouterB, N: 7, Name: "VT 7"}})
	})
	file := exported(t, a, tok, backupReq(true))
	b := setupApp(t)
	_, key := restoreKey(t, b)
	if res := restored(t, postRestore(t, b, key, file, "")); len(res.Failed) != 0 {
		t.Fatalf("failed: %v", res.Failed)
	}
	cfg := b.r().cfg
	if !slices.Equal(cfg.RouterUIDs(), a.r().cfg.RouterUIDs()) {
		t.Fatalf("routers: %v, want %v", cfg.RouterUIDs(), a.r().cfg.RouterUIDs())
	}
	if p, ok := presetNamed(t, b, "On B"); !ok || len(p.Broken) != 0 {
		t.Errorf("preset On B: %+v", p)
	}
	if i := slices.IndexFunc(cfg.TieLines, func(x config.TieLine) bool { return x.ID == "tie-ab" }); i < 0 ||
		cfg.TieLines[i].From.Router != cfg.UID || cfg.TieLines[i].To.Router != fileRouterB {
		t.Errorf("tie line to router B: %+v", cfg.TieLines)
	}
	u, ok := b.unit(fileRouterB)
	if !ok {
		t.Fatal("no unit for router B")
	}
	b.mu.Lock()
	name := u.router.srcNames[7]
	b.mu.Unlock()
	if name != "VT 7" {
		t.Errorf("router B names: %q", name)
	}
	again := exported(t, b, signInToken(t, b, "Ada", "2468"), backupReq(true))
	want, got := unzipAll(t, file), unzipAll(t, again)
	for name, w := range want {
		if g := got[name]; !bytes.Equal(w, g) {
			t.Errorf("%s differs:\n--- first\n%s\n--- second\n%s", name, w, g)
		}
	}
	if len(got) != len(want) {
		t.Errorf("files: %v, want %v", slicesSortedKeys(got), slicesSortedKeys(want))
	}
}

// twoRouterTieFile: an install with routers A and B exports one tie line A.4 → B.7.
func twoRouterTieFile(t *testing.T) []byte {
	t.Helper()
	a, _, tok := importApp(t)
	addRouter(t, a, fileRouterB, "B", map[int]string{7: "VT 7"}, map[int]string{8: "REC 8"})
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{{ID: "tie-ab", Name: "A TO B", From: c.DestinationRef(4, "MON 4"), To: config.Ref{Router: fileRouterB, N: 7, Name: "VT 7"}}}
	})
	return foreign(t, exported(t, a, tok, exportReq{Categories: []string{catTieLines}}))
}

// A tie line to the second router names it by prefix as other items do, and the file holds that router. Import into
// another install with two routers maps both ends.
func TestImportTieLineTwoRoutersRoundTrip(t *testing.T) {
	file := twoRouterTieFile(t)
	files := unzipAll(t, file)
	var man exportManifest
	if err := yaml.Unmarshal(files["manifest.yaml"], &man); err != nil || len(man.Routers) != 2 {
		t.Fatalf("manifest routers: %v %+v", err, man.Routers)
	}
	if ties := string(files["tielines.yaml"]); strings.Contains(ties, fileRouterB) || !strings.Contains(ties, "router: B\n") {
		t.Fatalf("tielines.yaml does not name router B by prefix:\n%s", ties)
	}
	a, tok := twoRouterLive(t)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if it := planItem(t, res.Plan, catTieLines, "A TO B"); it.Refused != "" || !it.writes() {
		t.Fatalf("A TO B: %+v", it)
	}
	cfg := a.r().cfg
	if len(cfg.TieLines) != 1 || cfg.TieLines[0].From.Router != cfg.UID || cfg.TieLines[0].To.Router != liveRouterB || cfg.TieLines[0].To.N != 7 {
		t.Fatalf("tie lines: %+v", cfg.TieLines)
	}
}

// A tie line to a file router with no target is dropped with the same note as other items.
func TestImportTieLineUnmappedRouterDropped(t *testing.T) {
	file := twoRouterTieFile(t)
	a, tok := twoRouterLive(t)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Routers: map[string]string{fileRouterB: ""}}))
	if it := planItem(t, res.Plan, catTieLines, "A TO B"); it.Refused != "it uses router Router B, which has no target here" || it.writes() {
		t.Fatalf("A TO B: %+v", it)
	}
	if got := a.r().cfg.TieLines; len(got) != 0 {
		t.Fatalf("tie lines arrived: %+v", got)
	}
}
