package app

import (
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The check answers each filter set on a file router and the file's names, and the apply takes the page's text.
func TestImportRemapFilterSetTakesPageText(t *testing.T) {
	a, _, tok := importApp(t)
	a.mu.Lock()
	a.router.srcNames[23] = "CAM 3"
	a.mu.Unlock()
	uid := a.r().cfg.UID
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.FilterSets = []config.FilterSet{{Name: "Cams", Router: uid, Src: "23, 2", Sode: true}}
	})
	file := otherRouter(t, a, exported(t, a, tok, exportReq{Categories: []string{catFilterSets}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.FilterSets = nil })
	a.mu.Lock()
	a.router.srcNames[23], a.router.srcNames[40] = "VT 1", "CAM 3"
	a.mu.Unlock()

	check := postCheck(t, a, tok, file, importReq{Mode: importAdd})
	r := check.Plan.Remap.Routers[0]
	if len(r.Sets) != 1 || r.Sets[0].Key != "/Cams" || r.Sets[0].Src != "23, 2" {
		t.Fatalf("sets: %+v", r.Sets)
	}
	if r.Names == nil || !slices.ContainsFunc(r.Names.Sources, func(x namedItem) bool { return x.N == 23 && x.Name == "CAM 3" }) {
		t.Fatalf("file names: %+v", r.Names)
	}
	if r.Offset != a.r().cfg.NumberOffset {
		t.Errorf("offset %d", r.Offset)
	}

	text := `"CAM 3", 2`
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Filters: map[string]remapFilter{"/Cams": {Src: &text}}}))
	fs := a.r().cfg.FilterSets
	if len(fs) != 1 || fs[0].Src != text || fs[0].Router != uid || len(fs[0].Broken) != 0 {
		t.Fatalf("filter sets: %+v", fs)
	}
}

// Text that does not parse is not taken: the set keeps the file's text.
func TestImportRemapFilterSetRefusesBadText(t *testing.T) {
	a, _, tok := importApp(t)
	uid := a.r().cfg.UID
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.FilterSets = []config.FilterSet{{Name: "Cams", Router: uid, Src: "2", Sode: true}}
	})
	file := otherRouter(t, a, exported(t, a, tok, exportReq{Categories: []string{catFilterSets}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.FilterSets = nil })
	bad := `"CAM`
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Filters: map[string]remapFilter{"/Cams": {Src: &bad}}}))
	if fs := a.r().cfg.FilterSets; len(fs) != 1 || fs[0].Src != "2" {
		t.Fatalf("filter sets: %+v", fs)
	}
}

// sessionOn gives an App made from importApp's config a session token for Ada.
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

	// Sode starts again with the router offline: its names are the saved names table.
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

	// The router answers: MON 4 is gone. Two agreeing reads mark the imported preset Broken.
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
	clear(a.router.srcNames)
	clear(a.router.dstNames)
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
