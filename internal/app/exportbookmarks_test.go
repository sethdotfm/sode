package app

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

func everyoneBookmarks(presetID string) []config.Bookmark {
	return []config.Bookmark{
		{Key: "preset:global:" + presetID, Label: "Show", InPalette: true},
		{URL: "https://example.com/run", Label: "Run sheet", NewTab: true},
	}
}

func TestExportCarriesEveryoneBookmarks(t *testing.T) {
	a, tok := exportApp(t)
	lifeCommit(t, a, func(c *config.Config) { c.Bookmarks = everyoneBookmarks("pr-1") })
	file := exported(t, a, tok, exportReq{Categories: []string{catBookmarks}})
	files := unzipAll(t, file)
	if _, ok := files["bookmarks.yaml"]; !ok {
		t.Fatalf("no bookmarks.yaml in %v", slicesSortedKeys(files))
	}
	var man exportManifest
	if err := yaml.Unmarshal(files["manifest.yaml"], &man); err != nil || !slices.Equal(man.Categories, []string{catBookmarks}) {
		t.Fatalf("manifest categories: %v %v", err, man.Categories)
	}
	b, done, err := a.stageImport(bytes.NewReader(file))
	defer done()
	if err != nil {
		t.Fatal(err)
	}
	if !sameBookmarks(b.Bookmarks, everyoneBookmarks("pr-1")) {
		t.Fatalf("bookmarks read back: %+v", b.Bookmarks)
	}

	list, err := a.exportList()
	if err != nil {
		t.Fatal(err)
	}
	i := slices.IndexFunc(list.Categories, func(c exportListCat) bool { return c.Name == catBookmarks })
	if i < 0 || len(list.Categories[i].Items) != 1 || list.Categories[i].Items[0].ID != catBookmarks || list.Categories[i].Items[0].Size == 0 {
		t.Fatalf("export list: %+v", list.Categories)
	}
	// Not picked: the file holds an empty list.
	none := unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catBookmarks}, Pick: map[string][]string{catBookmarks: {}}}))
	if bytes.Contains(none["bookmarks.yaml"], []byte("example.com")) {
		t.Fatalf("an unpicked list went:\n%s", none["bookmarks.yaml"])
	}
}

// The list is one item, written whole. A live list that differs is a clash; Use new is Critical.
func TestAddImportEveryoneBookmarksIsOneItem(t *testing.T) {
	a, c, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Bookmarks = everyoneBookmarks("pr-1") })
	file := exported(t, a, tok, exportReq{Categories: []string{catBookmarks}})
	live := []config.Bookmark{{URL: "https://example.org/", Label: "Live"}}
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Bookmarks = live })

	keep := importReq{Mode: importAdd, Choices: map[string]map[string]string{catBookmarks: {catBookmarks: useCurrent}}}
	res := applied(t, postApply(t, a, tok, file, keep))
	if it := planItem(t, res.Plan, catBookmarks, everyoneBookmarksName); it.Key != catBookmarks || it.Clash == "" || it.writes() || res.Plan.Critical {
		t.Fatalf("Use current: %+v critical %v", it, res.Plan.Critical)
	}
	if !sameBookmarks(a.r().cfg.Bookmarks, live) {
		t.Fatalf("Use current changed the list: %+v", a.r().cfg.Bookmarks)
	}

	if rec := postApply(t, a, tok, file, importReq{Mode: importAdd}); rec.Code != 403 {
		t.Fatalf("Use new without a recheck: %d %s", rec.Code, rec.Body)
	}
	grantRecheck(a, c)
	res = applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if it := planItem(t, res.Plan, catBookmarks, everyoneBookmarksName); it.Choice != useNew || !res.Plan.Critical {
		t.Fatalf("Use new: %+v", it)
	}
	if !sameBookmarks(a.r().cfg.Bookmarks, everyoneBookmarks("pr-1")) {
		t.Fatalf("after Use new: %+v", a.r().cfg.Bookmarks)
	}
	// Equal now: No change.
	res = applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if it := planItem(t, res.Plan, catBookmarks, everyoneBookmarksName); !it.Same {
		t.Fatalf("equal list: %+v", it)
	}
}

// An empty live list takes the file's with no clash and no recheck.
func TestAddImportBookmarksIntoEmptyList(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Bookmarks = everyoneBookmarks("pr-1") })
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catBookmarks}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Bookmarks = nil })
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if it := planItem(t, res.Plan, catBookmarks, everyoneBookmarksName); it.Clash != "" || !it.writes() || res.Plan.Critical {
		t.Fatalf("plan: %+v", res.Plan.Categories[catBookmarks])
	}
	if !sameBookmarks(a.r().cfg.Bookmarks, everyoneBookmarks("pr-1")) {
		t.Fatalf("bookmarks: %+v", a.r().cfg.Bookmarks)
	}
}

// A file with an unsafe web address is refused by the bookmark rules. The other categories still import.
func TestImportRefusesUnsafeBookmark(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")}
		cfg.Bookmarks = []config.Bookmark{{URL: "https://example.com/", Label: "Docs"}}
	})
	files := map[string]string{}
	for name, b := range unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catPresets, catBookmarks}})) {
		files[name] = string(b)
	}
	files["bookmarks.yaml"] = strings.Replace(files["bookmarks.yaml"], "https://example.com/", "https://example.com/#key=abc", 1)
	file := foreign(t, zipOf(t, files))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets, cfg.Bookmarks = nil, nil })

	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if c := res.Plan.Categories[catBookmarks]; c == nil || !strings.Contains(c.Refused, "panel key") {
		t.Fatalf("bookmarks not refused: %+v", c)
	}
	if len(a.r().cfg.Bookmarks) != 0 {
		t.Fatalf("an unsafe list arrived: %+v", a.r().cfg.Bookmarks)
	}
	if _, ok := presetNamed(t, a, "Show"); !ok {
		t.Fatal("the presets did not import")
	}
}

// A preset that arrives with a new id takes its Everyone bookmark with it.
func TestAddImportBookmarkFollowsImportedPreset(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	id := a.r().cfg.Presets[0].ID
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Bookmarks = everyoneBookmarks(id) })
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catPresets, catBookmarks}}))
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(5, "MON 5")}
		cfg.Bookmarks = nil
	})
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	two, ok := presetNamed(t, a, "Show 2")
	if !ok || two.ID == id {
		t.Fatalf("presets: %+v", a.r().cfg.Presets)
	}
	if got := a.r().cfg.Bookmarks; len(got) != 2 || got[0].Key != "preset:global:"+two.ID || got[1].URL != "https://example.com/run" {
		t.Fatalf("bookmarks: %+v", got)
	}
}

// Replace import with a file whose list is empty empties the live list.
func TestReplaceImportEmptiesBookmarks(t *testing.T) {
	a, tok := replaceApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catBookmarks}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Bookmarks = everyoneBookmarks("pr-1") })
	res := applied(t, postApply(t, a, tok, file, replaceReq(catBookmarks)))
	if it := planItem(t, res.Plan, catBookmarks, everyoneBookmarksName); !it.Remove {
		t.Fatalf("plan: %+v", it)
	}
	if len(a.r().cfg.Bookmarks) != 0 {
		t.Fatalf("bookmarks: %+v", a.r().cfg.Bookmarks)
	}
}
