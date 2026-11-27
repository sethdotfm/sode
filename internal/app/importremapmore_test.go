package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The check answers each filter set on a file router and the file's names, and the apply uses the page's text.
func TestImportRemapFilterSetTakesPageText(t *testing.T) {
	a, _, tok := importApp(t)
	a.mu.Lock()
	a.firstUnit().router.srcNames[23] = "CAM 3"
	a.mu.Unlock()
	uid := a.r().cfg.Routers[0].UID
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.FilterSets = []config.FilterSet{{ID: "cams1", Name: "Cams", Router: uid, Src: "23, 2", Sode: true}}
	})
	file := otherRouter(t, a, exported(t, a, tok, exportReq{Categories: []string{catFilterSets}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.FilterSets = nil })
	a.mu.Lock()
	a.firstUnit().router.srcNames[23], a.firstUnit().router.srcNames[40] = "VT 1", "CAM 3"
	a.mu.Unlock()

	check := postCheck(t, a, tok, file, importReq{Mode: importAdd})
	r := check.Plan.Remap.Routers[0]
	if len(r.Sets) != 1 || r.Sets[0].Key != "/cams1" || r.Sets[0].Src != "23, 2" {
		t.Fatalf("sets: %+v", r.Sets)
	}
	if r.Names == nil || !slices.ContainsFunc(r.Names.Sources, func(x namedItem) bool { return x.N == 23 && x.Name == "CAM 3" }) {
		t.Fatalf("file names: %+v", r.Names)
	}
	if r.Offset != a.r().cfg.Routers[0].NumberOffset {
		t.Errorf("offset %d", r.Offset)
	}

	text := `"CAM 3", 2`
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Filters: map[string]remapFilter{"/cams1": {Src: &text}}}))
	fs := a.r().cfg.FilterSets
	if len(fs) != 1 || fs[0].Src != text || fs[0].Router != uid || len(fs[0].Broken) != 0 {
		t.Fatalf("filter sets: %+v", fs)
	}
}

// Sode refuses text that does not parse: the set keeps the file's text.
func TestImportRemapFilterSetRefusesBadText(t *testing.T) {
	a, _, tok := importApp(t)
	uid := a.r().cfg.Routers[0].UID
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.FilterSets = []config.FilterSet{{ID: "cams1", Name: "Cams", Router: uid, Src: "2", Sode: true}}
	})
	file := otherRouter(t, a, exported(t, a, tok, exportReq{Categories: []string{catFilterSets}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.FilterSets = nil })
	bad := `"CAM`
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Filters: map[string]remapFilter{"/cams1": {Src: &bad}}}))
	if fs := a.r().cfg.FilterSets; len(fs) != 1 || fs[0].Src != "2" {
		t.Fatalf("filter sets: %+v", fs)
	}
}

func sessionOn(t *testing.T, a *App) string {
	t.Helper()
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 4096)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	return a.issueSessionTab(c, true)
}

// With the router offline the import plans against the saved names table. When the router answers, the first
// reads check the imported items again (the Broken check).
func TestImportOfflineUsesSavedNamesAndRechecks(t *testing.T) {
	a, _, tok := importApp(t)
	srcs := []string{"CAM 0", "CAM 1", "CAM 2", "CAM 3"}
	dsts := []string{"MON 0", "MON 1", "MON 2", "MON 3", "MON 4", "MON 5"}
	readNames(a, srcs, dsts)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = nil })

	hist, _ := OpenHistory("")
	b, err := New(a.r().cfg, a.cfgPath, hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	btok := sessionOn(t, b)
	check := postCheck(t, b, btok, file, importReq{Mode: importAdd})
	tg := check.Plan.Remap.Targets[0]
	if !tg.Offline || tg.SavedAt.IsZero() || tg.NoNames {
		t.Fatalf("target: %+v", tg)
	}
	applied(t, postApply(t, b, btok, file, importReq{Mode: importAdd}))
	p, ok := presetNamed(t, b, "Show")
	if !ok || len(p.Broken) != 0 {
		t.Fatalf("preset against the saved names: %+v", p)
	}

	// The router answers: MON 4 is gone. Two reads that agree mark the imported preset Broken.
	gone := with(dsts, 4, "")
	readNames(b, srcs, gone)
	readNames(b, srcs, gone)
	p, _ = presetNamed(t, b, "Show")
	if len(p.Broken) != 1 || p.Broken[0].Kind != config.MissingDestination || p.Broken[0].Ref.N != 4 {
		t.Fatalf("after the router answered: %+v", p.Broken)
	}
}

// With no names at all, an item that holds references waits and is not imported, and its category is not refused.
// Items without references import now.
func TestAddImportWaitsForNamesPerItem(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")}
		cfg.FilterSets = []config.FilterSet{{Name: "Cams", Src: "CAM", Sode: true}}
	})
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets, catFilterSets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets, cfg.FilterSets = nil, nil })
	a.mu.Lock()
	clear(a.firstUnit().router.srcNames)
	clear(a.firstUnit().router.dstNames)
	a.mu.Unlock()
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	cat := res.Plan.Categories[catPresets]
	if cat.Refused != "" {
		t.Fatalf("the whole category waits: %+v", cat)
	}
	show := slices.IndexFunc(cat.Items, func(it importItem) bool { return it.Name == "Show" })
	if show < 0 || cat.Items[show].Refused == "" {
		t.Fatalf("items: %+v", cat.Items)
	}
	if _, ok := presetNamed(t, a, "Show"); ok {
		t.Error("a preset with references imported with no names")
	}
	if fs := a.r().cfg.FilterSets; len(fs) != 1 {
		t.Errorf("filter sets: %+v", fs)
	}
	if rm := res.Plan.Remap; rm == nil || !rm.Targets[0].NoNames {
		t.Errorf("remap: %+v", rm)
	}
}

// Every kind of reference follows the remap: Rights, fixed destinations, templates, Push checkpoints,
// Checkpoint sets, Default artwork, the History filter's router and This browser's presets.
func TestImportRemapMovesEveryReferenceKind(t *testing.T) {
	a, c, tok := importApp(t)
	uid := a.r().cfg.Routers[0].UID
	a.mu.Lock()
	a.firstUnit().router.srcNames[23] = "CAM 3"
	a.mu.Unlock()
	lifeCommit(t, a, func(cfg *config.Config) {
		srcs := []config.Ref{cfg.SourceRef(23, "CAM 3")}
		cfg.APIClients = []config.APIClient{{Name: "Cue", Scopes: []string{config.ScopeNames}, Rights: config.Rights{Sources: &srcs}}}
		cfg.GuestRights = config.Rights{Sources: &srcs}
		cfg.HiddenDsts, cfg.HiddenDstsRouter = "4", uid
	})
	ada := userID(t, a, "Ada")
	cells := []map[string]any{{"kind": "source", "router": uid, "n": 23, "name": "CAM 3", "r": 1, "c": 1}}
	desk, err := a.panels.create(Panel{Name: "Desk", Owner: ada, Rows: 1, Cols: 1, Pages: []PanelPage{{ID: "p1", Enabled: true, Cells: cells}},
		Settings: map[string]any{panelFixedKey: []config.Ref{{Router: uid, N: 4, Name: "MON 4"}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.panels.pushLayout(desk.ID, layoutOf(desk), ada, "push", nil); err != nil {
		t.Fatal(err)
	}
	tpl, err := a.templates.create(Template{Name: "Row", Scope: templateEveryone, Owner: ada, Rows: 1, Cols: 1, Contents: true,
		Pages: []PanelPage{{ID: "p1", Enabled: true, Cells: cells}}})
	if err != nil {
		t.Fatal(err)
	}
	set, err := a.panels.sets.create(PanelSet{Name: "Show A", Scope: panelShareAll, Owner: ada,
		Entries: []PanelSetEntry{{Panel: desk.ID, Name: desk.Name, panelLayout: layoutOf(desk)}}})
	if err != nil {
		t.Fatal(err)
	}
	n := 23
	def := ArtDefault{Kind: artKindSource, Router: uid, N: &n, Name: "CAM 3", Art: "icon:camera"}
	if _, err := a.artDefaults.set(def, nil); err != nil {
		t.Fatal(err)
	}
	browser := `{"presets":[{"name":"Here","items":[{"src":{"n":23},"dsts":[{"n":4}]}]}]}`
	file := otherRouter(t, a, exported(t, a, tok, exportReq{Categories: []string{catSettings, catPanels, catArtwork, catAPIClients, catBrowser},
		Sections: []string{"history", "api"}, Browser: []byte(browser)}))

	a.panels.delete(desk.ID)
	a.templates.delete(tpl.ID)
	a.panels.sets.delete(set.ID)
	a.artDefaults.clear(def)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.APIClients, cfg.GuestRights, cfg.HiddenDsts, cfg.HiddenDstsRouter = nil, config.Rights{}, "", ""
	})
	a.mu.Lock()
	a.firstUnit().router.srcNames[23], a.firstUnit().router.srcNames[40] = "VT 1", "CAM 3"
	a.mu.Unlock()
	grantRecheck(a, c)
	req := importReq{Mode: importAdd, Categories: []string{catSettings, catPanels, catArtwork, catAPIClients, catBrowser},
		Names: map[string][]int{remapKey(config.MissingSource, 23): {40}}, Pick: map[string][]string{catArtwork: {"source/23"}}}
	moved := func(what string, r config.Ref) {
		t.Helper()
		if r.Router != uid || r.N != 40 {
			t.Errorf("%s: %+v", what, r)
		}
	}
	check := postCheck(t, a, tok, file, req)
	if !strings.Contains(fmt.Sprint(check.Browser), "src:map[n:40 name:CAM 3 router:"+uid+"]") {
		t.Errorf("This browser: %v", check.Browser)
	}
	res := applied(t, postApply(t, a, tok, file, req))
	if len(res.Failed) != 0 {
		t.Fatalf("failed: %v", res.Failed)
	}
	cfg := a.r().cfg
	cue, _ := cfg.FindAPIClient("Cue")
	moved("API client Rights", (*cue.Rights.Sources)[0])
	moved("guestRights", (*cfg.GuestRights.Sources)[0])
	if cfg.HiddenDstsRouter != uid {
		t.Errorf("hiddenDstsRouter %q", cfg.HiddenDstsRouter)
	}
	ps := a.panels.list()
	if len(ps) != 1 {
		t.Fatalf("panels: %+v", ps)
	}
	if fixed, _ := fixedList(ps[0].Settings[panelFixedKey]); len(fixed) != 1 || fixed[0].Router != uid {
		t.Errorf("fixed destinations: %v", ps[0].Settings)
	}
	cps, _ := a.panels.checkpoints(ps[0].ID)
	for _, cp := range cps {
		r, _ := cellRef(cp.Layout.Pages[0].Cells[0])
		moved("Push checkpoint", r)
	}
	for _, tp := range a.templates.list() {
		r, _ := cellRef(tp.Pages[0].Cells[0])
		moved("template", r)
	}
	for _, s := range a.panels.sets.sorted(nil) {
		r, _ := cellRef(s.Entries[0].Pages[0].Cells[0])
		moved("Checkpoint set", r)
	}
	if ds := a.artDefaults.list(); len(ds) != 1 || ds[0].Router != uid || *ds[0].N != 40 || ds[0].Name != "CAM 3" {
		t.Errorf("Default artwork: %+v", ds)
	}
}
