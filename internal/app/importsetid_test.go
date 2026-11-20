package app

import (
	"encoding/json"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func checkPlan(t *testing.T, a *App, tok string, file []byte) importPlan {
	t.Helper()
	rec := postImport(t, a, tok, file)
	var check importCheck
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &check) != nil {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	return check.Plan
}

// A filter set matches by its hidden id in a file from this install, then by name, as a preset does.
func TestImportMatchesFilterSetByID(t *testing.T) {
	a, c, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.FilterSets = []config.FilterSet{{ID: "set1", Name: "Cams", Src: "CAM"}} })
	file := exported(t, a, tok, exportReq{Categories: []string{catFilterSets}})
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.FilterSets = []config.FilterSet{{ID: "set1", Name: "Cameras", Src: "CAM 2"}, {ID: "set2", Name: "Cams", Src: "VT"}}
	})
	if it := checkPlan(t, a, tok, file).Categories[catFilterSets].Items[0]; it.Key != "set1" || it.Clash != "Cameras" {
		t.Fatalf("this install: %+v", it)
	}
	if it := checkPlan(t, a, tok, foreign(t, file)).Categories[catFilterSets].Items[0]; it.Clash != "Cams" {
		t.Fatalf("another install matches by name: %+v", it)
	}

	// Use new on a renamed set writes the file's set over it and keeps its id, so bookmarks on it still work.
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.FilterSets = []config.FilterSet{{ID: "set1", Name: "Cameras", Src: "CAM 2"}, {ID: "set2", Name: "Desk", Src: "VT"}}
	})
	grantRecheck(a, c)
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Choices: map[string]map[string]string{catFilterSets: {"set1": useNew}}}))
	got := a.r().cfg.FilterSets
	if len(got) != 2 || got[0].ID != "set1" || got[0].Name != "Cams" || got[0].Src != "CAM" || got[1].ID != "set2" {
		t.Fatalf("sets: %+v", got)
	}
}
