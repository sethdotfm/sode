package app

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
)

// postCheck sends the import check as the review does: the page's plan, then the file.
func postCheck(t *testing.T, a *App, tok string, zipBytes []byte, req importReq) importCheck {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	pw, _ := mw.CreateFormField("plan")
	json.NewEncoder(pw).Encode(req)
	fw, _ := mw.CreateFormFile("file", "export.zip")
	fw.Write(zipBytes)
	mw.Close()
	r := httptest.NewRequest("POST", "/api/import", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set(sessionHeader, tok)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	var check importCheck
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &check) != nil {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	return check
}

func planItem(t *testing.T, p importPlan, cat, name string) importItem {
	t.Helper()
	if c := p.Categories[cat]; c != nil {
		for _, it := range c.Items {
			if it.Name == name {
				return it
			}
		}
	}
	t.Fatalf("no %s %q in the plan: %+v", cat, name, p.Categories[cat])
	return importItem{}
}

// reviewApp: a preset Show and a panel Desk with a Show button, in a same-install file. The live install has
// since changed both, so each clashes.
func reviewApp(t *testing.T) (*App, *client, string, []byte, string, string) {
	t.Helper()
	a, c, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	show := a.r().cfg.Presets[0].ID
	desk, err := a.panels.create(Panel{Name: "Desk", Owner: userID(t, a, "Ada"), Rows: 1, Cols: 1, Pages: []PanelPage{{ID: "p1", Enabled: true,
		Cells: []map[string]any{{"kind": "preset", "scope": "everyone", "id": show, "name": "Show", "r": 1, "c": 1}}}}})
	if err != nil {
		t.Fatal(err)
	}
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets, catPanels}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(5, "MON 5")} })
	desk.Name = "Desk live"
	if _, err := a.panels.replace(desk); err != nil {
		t.Fatal(err)
	}
	return a, c, tok, file, show, desk.ID
}

// The review's check plans the page's choices and writes nothing. Skip on Show leaves Desk's button Broken.
// With no Use new the import is not Critical. Items carry their sizes.
func TestImportCheckPlansThePageChoices(t *testing.T) {
	a, _, tok, file, show, desk := reviewApp(t)
	pre := postCheck(t, a, tok, file, importReq{Mode: importAdd})
	if !pre.Plan.Critical || planItem(t, pre.Plan, catPresets, "Show").Choice != useNew || len(planItem(t, pre.Plan, catPanels, "Desk").Broken) != 0 {
		t.Fatalf("preselected plan: %+v", pre.Plan)
	}
	if planItem(t, pre.Plan, catPresets, "Show").Size == 0 || planItem(t, pre.Plan, catPanels, "Desk").Size == 0 {
		t.Errorf("no sizes: %+v", pre.Plan.Categories)
	}

	check := postCheck(t, a, tok, file, importReq{Mode: importAdd, Choices: map[string]map[string]string{catPresets: {show: useSkip}, catPanels: {desk: useRename}}})
	d := planItem(t, check.Plan, catPanels, "Desk")
	if check.Plan.Critical || d.Choice != useRename || d.As != "Desk 2" || len(d.Broken) != 1 || d.Broken[0].Kind != config.MissingPreset {
		t.Fatalf("plan with choices: %+v %+v", check.Plan, d)
	}
	if len(a.panels.list()) != 1 || a.r().cfg.Presets[0].Items[0].Dsts[0].N != 5 {
		t.Fatal("the check wrote something")
	}
}

// Clash choices change what the apply writes. Skip keeps the live preset. Rename adds a second panel whose
// Show button is Broken. Nothing waits for a PIN recheck.
func TestAddImportAppliesMixedChoices(t *testing.T) {
	a, _, tok, file, show, desk := reviewApp(t)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Choices: map[string]map[string]string{catPresets: {show: useSkip}, catPanels: {desk: useRename}}}))
	if res.Plan.Critical {
		t.Error("Skip and Rename made the import Critical")
	}
	if ps := a.r().cfg.Presets; len(ps) != 1 || ps[0].Items[0].Dsts[0].N != 5 {
		t.Fatalf("presets: %+v", ps)
	}
	var live, two Panel
	for _, p := range a.panels.list() {
		switch p.Name {
		case "Desk live":
			live = p
		case "Desk 2":
			two = p
		}
	}
	if live.ID != desk || len(live.Broken) != 0 || two.ID == "" || two.ID == desk || len(two.Broken) != 1 {
		t.Fatalf("panels: %+v", a.panels.list())
	}
}
