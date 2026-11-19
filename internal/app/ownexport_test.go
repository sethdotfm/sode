package app

import (
	"bytes"
	"encoding/json"
	"maps"
	"mime/multipart"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
)

// ownApp signs Ana, a Standard user, in on a connection and gives her a session token. Names are read.
func ownApp(t *testing.T) (*App, string) {
	t.Helper()
	a, _, _ := importApp(t)
	c := &client{ip: "10.0.0.10", send: make(chan []byte, 4096)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" {
		t.Fatal("no session token for Ana")
	}
	return a, tok
}

func ownPost(t *testing.T, a *App, tok, path string, req any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", path, bytes.NewReader(b))
	if tok != "" {
		r.Header.Set(sessionHeader, tok)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func ownExported(t *testing.T, a *App, tok string, req exportReq) []byte {
	t.Helper()
	rec := ownPost(t, a, tok, "/api/own/export", req)
	if rec.Code != 200 {
		t.Fatalf("own export: %d %s", rec.Code, rec.Body)
	}
	return rec.Body.Bytes()
}

// postMultipart sends an import or restore request: the plan part, then the file part.
func postMultipart(t *testing.T, a *App, path string, plan any, zipBytes []byte, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	pw, _ := mw.CreateFormField("plan")
	json.NewEncoder(pw).Encode(plan)
	fw, _ := mw.CreateFormFile("file", "export.zip")
	fw.Write(zipBytes)
	mw.Close()
	r := httptest.NewRequest("POST", path, &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func ownMultipart(t *testing.T, a *App, tok, path string, zipBytes []byte, req importReq) *httptest.ResponseRecorder {
	t.Helper()
	return postMultipart(t, a, path, req, zipBytes, map[string]string{sessionHeader: tok})
}

func ownChecked(t *testing.T, a *App, tok string, zipBytes []byte, req importReq) ownCheck {
	t.Helper()
	rec := ownMultipart(t, a, tok, "/api/own/import", zipBytes, req)
	if rec.Code != 200 {
		t.Fatalf("own check: %d %s", rec.Code, rec.Body)
	}
	var out ownCheck
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out
}

func ownApplied(t *testing.T, a *App, tok string, zipBytes []byte, req importReq) importResult {
	t.Helper()
	return applied(t, ownMultipart(t, a, tok, "/api/own/import/apply", zipBytes, req))
}

func ownSetup(t *testing.T, a *App) {
	t.Helper()
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = []config.Preset{showPreset(*cfg, "Everyone show", "MON 4")}
		for i := range cfg.Users {
			if cfg.Users[i].Name != "Ana" {
				continue
			}
			cfg.Users[i].Profile = config.Profile{
				Prefs:      map[string]string{"theme": "dark", "clock": "12"},
				Presets:    []config.Preset{showPreset(*cfg, "Mine", "MON 4"), otherPreset(*cfg)},
				FilterSets: []config.FilterSet{{Name: "Cams", Src: "CAM", Sode: true, Ext: true}},
			}
		}
	})
}

func otherPreset(c config.Config) config.Preset {
	return config.Preset{Name: "Other", Items: []config.PresetItem{{Src: c.SourceRef(3, "CAM 3"), Dsts: []config.Ref{c.DestinationRef(5, "MON 5")}}}}
}

// A Standard user exports their own presets and imports them again. The file holds only their own user, with no
// PIN and no global items, and the import is never Critical.
func TestOwnExportReimportsPresets(t *testing.T) {
	a, tok := ownApp(t)
	ownSetup(t, a)
	file := ownExported(t, a, tok, exportReq{Categories: []string{catProfile, catPresets, catFilterSets}})
	files := unzipAll(t, file)
	if _, ok := files["presets.yaml"]; ok {
		t.Error("the own export holds global presets")
	}
	users := string(files["users.yaml"])
	if !strings.Contains(users, "Mine") || strings.Contains(users, "Bo") || strings.Contains(users, "Ada") || strings.Contains(users, "pin:") {
		t.Fatalf("users.yaml:\n%s", users)
	}
	mine := profileOf(t, a, "Ana").Presets[0]

	// Changed since the export: Mine is gone, Other changed.
	lifeCommit(t, a, func(cfg *config.Config) {
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Ana" {
				p := &cfg.Users[i].Profile
				p.Presets = p.Presets[1:]
				p.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(4, "MON 4")}
			}
		}
	})
	check := ownChecked(t, a, tok, file, importReq{Mode: importAdd})
	if !check.SameInstall || check.From != "Ana" {
		t.Errorf("check: same %v, from %q", check.SameInstall, check.From)
	}
	if c := check.Plan.Categories[catPresets]; c == nil || len(c.Items) != 2 {
		t.Fatalf("presets plan: %+v", c)
	} else if it := c.Items[1]; it.Name != "Other" || it.Choice != useNew {
		t.Errorf("a clash from this install preselects Use new: %+v", it)
	}

	res := ownApplied(t, a, tok, file, importReq{Mode: importAdd})
	if res.Plan.Critical {
		t.Error("an own import is Critical")
	}
	got := profileOf(t, a, "Ana").Presets
	if len(got) != 2 {
		t.Fatalf("presets after import: %+v", got)
	}
	i := slices.IndexFunc(got, func(p config.Preset) bool { return p.Name == "Mine" })
	if i < 0 || got[i].ID != mine.ID {
		t.Errorf("Mine did not come back with its id: %+v", got)
	}
	if o := got[slices.IndexFunc(got, func(p config.Preset) bool { return p.Name == "Other" })]; o.Items[0].Dsts[0].N != 5 {
		t.Errorf("Use new did not update Other: %+v", o)
	}
	if ps := a.r().cfg.Presets; len(ps) != 1 || ps[0].Name != "Everyone show" {
		t.Errorf("global presets changed: %+v", ps)
	}
}

// An own import of an Administrator's file takes only the importer's own profile: other users, global items and
// the importer's Role and Rights stay. Profile is overwritten whole.
func TestOwnImportTouchesOnlyOwnProfile(t *testing.T) {
	a, tok := ownApp(t)
	ownSetup(t, a)
	ada := adaToken(t, a)
	lifeCommit(t, a, func(cfg *config.Config) {
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Ana" {
				cfg.Users[i].Profile.Prefs = map[string]string{"theme": "light"}
				cfg.Users[i].Profile.FilterSets = append(cfg.Users[i].Profile.FilterSets, config.FilterSet{Name: "Mons", Dst: "MON", Sode: true})
			}
		}
	})
	file := exported(t, a, ada, exportReq{Categories: exportCategoryNames(), IncludePins: true})
	wasUsers := slices.Clone(a.r().cfg.Users)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = nil
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Ana" {
				cfg.Users[i].Profile.Prefs = map[string]string{"theme": "dark", "clock": "12", "flip": "on"}
			}
		}
	})

	check := ownChecked(t, a, tok, file, importReq{Mode: importAdd})
	for cat := range check.Plan.Categories {
		if !slices.Contains([]string{catProfile, catPresets, catFilterSets}, cat) {
			t.Errorf("an own import plans %s", cat)
		}
	}
	for _, cat := range []string{catSettings, catUsers, catPresets} {
		if !slices.Contains(check.Admin, cat) {
			t.Errorf("admin list %v lacks %s", check.Admin, cat)
		}
	}
	if it := check.Plan.Categories[catProfile].Items; len(it) != 1 || it[0].Note != noteReplacesProfile {
		t.Errorf("profile item: %+v", it)
	}

	ownApplied(t, a, tok, file, importReq{Mode: importAdd})
	cfg := a.r().cfg
	if len(cfg.Presets) != 0 {
		t.Errorf("an own import wrote global presets: %+v", cfg.Presets)
	}
	ana := profileOf(t, a, "Ana")
	if !maps.Equal(ana.Prefs, map[string]string{"theme": "light"}) {
		t.Errorf("profile not overwritten whole: %v", ana.Prefs)
	}
	if len(ana.FilterSets) != 2 {
		t.Errorf("filter sets: %+v", ana.FilterSets)
	}
	for _, was := range wasUsers {
		now, _ := cfg.FindUserByID(was.ID)
		if now.Role != was.Role || now.Pin != was.Pin || !sameYAML(now.Rights, was.Rights) {
			t.Errorf("%s changed beyond the profile", was.Name)
		}
		if was.Name != "Ana" && !sameYAML(now.Profile, was.Profile) {
			t.Errorf("%s's profile changed", was.Name)
		}
	}
}

func adaToken(t *testing.T, a *App) string {
	t.Helper()
	c := &client{ip: "10.0.0.11", send: make(chan []byte, 4096)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	return a.issueSessionTab(c, true)
}

// Global categories are not offered: the own routes refuse them, refuse a guest and refuse Replace import.
// The Administrator routes still refuse a Standard user.
func TestOwnRoutesRefuseGlobalItems(t *testing.T) {
	a, tok := ownApp(t)
	ownSetup(t, a)
	for _, cat := range []string{catSettings, catUsers, catPanels, catBrowser, catRouters} {
		if rec := ownPost(t, a, tok, "/api/own/export", exportReq{Categories: []string{cat}}); rec.Code != 400 {
			t.Errorf("own export of %s: %d %s", cat, rec.Code, rec.Body)
		}
	}
	if rec := ownPost(t, a, "", "/api/own/export", exportReq{Categories: []string{catPresets}}); rec.Code != 401 {
		t.Errorf("a guest's own export: %d", rec.Code)
	}
	if rec := postExport(t, a, tok, exportReq{Categories: []string{catPresets}}); rec.Code != 403 {
		t.Errorf("a Standard user on the Administrator export: %d", rec.Code)
	}
	file := ownExported(t, a, tok, exportReq{Categories: []string{catPresets}})
	if rec := ownMultipart(t, a, tok, "/api/own/import/apply", file, importReq{Mode: "replace"}); rec.Code != 400 {
		t.Errorf("an own Replace import: %d %s", rec.Code, rec.Body)
	}
}

// The items list offers only the user's own profile, presets and filter sets.
func TestOwnExportItems(t *testing.T) {
	a, tok := ownApp(t)
	ownSetup(t, a)
	r := httptest.NewRequest("GET", "/api/own/export/items", nil)
	r.Header.Set(sessionHeader, tok)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	if rec.Code != 200 {
		t.Fatalf("items: %d %s", rec.Code, rec.Body)
	}
	var list exportList
	json.Unmarshal(rec.Body.Bytes(), &list)
	var names []string
	for _, c := range list.Categories {
		names = append(names, c.Name)
		for _, it := range c.Items {
			if it.Name == "Everyone show" {
				t.Error("a global preset is offered")
			}
		}
	}
	if !slices.Equal(names, []string{catProfile, catPresets, catFilterSets}) {
		t.Errorf("categories: %v", names)
	}
}

// A Standard user's page gets the session token for the own routes.
func TestExportSessionForStandardUser(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	if got := p.ask("exportSession", 3, nil); got["ok"] == true {
		t.Errorf("a guest got a session: %v", got)
	}
	p.signIn("Ana", "1234")
	if tok, _ := p.ask("exportSession", 4, nil)["token"].(string); tok == "" {
		t.Error("no token for a Standard user")
	}
}
