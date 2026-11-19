package app

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// exportApp is a file-backed app with Ana (Standard), Bo and Ada (Administrator), signed in as Ada.
func exportApp(t *testing.T) (*App, string) {
	t.Helper()
	made := time.Date(2026, 11, 19, 4, 0, 0, 0, time.UTC)
	exportNow = func() time.Time { return made }
	t.Cleanup(func() { exportNow = time.Now })
	a := userApp(t, false)
	c := &client{ip: "10.0.0.9"}
	register(a, c)
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" {
		t.Fatal("no session token for Ada")
	}
	return a, tok
}

func userID(t *testing.T, a *App, name string) string {
	t.Helper()
	u, ok := a.r().cfg.FindUserByName(name)
	if !ok {
		t.Fatalf("no user %s", name)
	}
	return u.ID
}

func postExport(t *testing.T, a *App, tok string, req any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/export", bytes.NewReader(b))
	if tok != "" {
		r.Header.Set(sessionHeader, tok)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func postImport(t *testing.T, a *App, tok string, zipBytes []byte) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("POST", "/api/import", bytes.NewReader(zipBytes))
	r.Header.Set(sessionHeader, tok)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func unzipAll(t *testing.T, b []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatal(err)
	}
	out := map[string][]byte{}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name], _ = io.ReadAll(rc)
		rc.Close()
	}
	return out
}

func zipOf(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range slicesSorted(files) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(files[name]))
	}
	zw.Close()
	return buf.Bytes()
}

func slicesSorted(m map[string]string) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}

var everyCategory = []string{catSettings, catRouters, catUsers, catPresets, catFilterSets, catPanels, catArtwork, catAPIClients, catBrowser}

// seedEverything puts one item in every category. Each item names the router where it can.
func seedEverything(t *testing.T, a *App) {
	t.Helper()
	cfg := a.r().cfg
	uid, ana := cfg.UID, userID(t, a, "Ana")
	a.mu.Lock()
	a.router.srcNames[2] = "CAM 2"
	a.router.dstNames[4] = "MON 4"
	a.mu.Unlock()
	lifeCommit(t, a, func(c *config.Config) {
		c.Presets = []config.Preset{{ID: "pr-1", Name: "Show", Items: []config.PresetItem{{Src: c.SourceRef(2, "CAM 2"), Dsts: []config.Ref{c.DestinationRef(4, "MON 4")}}}}}
		c.FilterSets = []config.FilterSet{{Name: "Cams", Router: uid, Src: "1-4"}}
		c.HiddenDsts, c.HiddenDstsRouter = "9", uid
		c.APIClients = []config.APIClient{{Name: "Bot", Scopes: []string{config.ScopeNames}, Owner: ana}}
	})
	p, err := a.panels.create(Panel{Name: "Desk", Owner: ana, Rows: 1, Cols: 1,
		Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: []map[string]any{{"kind": "source", "n": 2, "router": uid, "name": "CAM 2"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.panels.addArt(p.ID, imgBytes("png", 1, 1), "image/png"); err != nil {
		t.Fatal(err)
	}
	if _, _, err := a.panels.pushLayout(p.ID, layoutOf(p), ana, "push", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := a.icons.add("Star", []byte(`<svg xmlns="http://www.w3.org/2000/svg"/>`), "image/svg+xml"); err != nil {
		t.Fatal(err)
	}
	n := 2
	if _, err := a.artDefaults.set(ArtDefault{Kind: artKindSource, Router: uid, N: &n, Name: "CAM 2"}, &artImage{[]byte("\x89PNG\r\n\x1a\ndefault"), "image/png"}); err != nil {
		t.Fatal(err)
	}
}

func TestExportEveryCategoryReadsBack(t *testing.T) {
	a, tok := exportApp(t)
	seedEverything(t, a)
	cfg := a.r().cfg
	rec := postExport(t, a, tok, exportReq{Categories: everyCategory, IncludePins: true, Browser: json.RawMessage(`{"theme":"dark"}`)})
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Content-Type") != "application/zip" || rec.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("headers: %v", rec.Header())
	}
	files := unzipAll(t, rec.Body.Bytes())
	for _, want := range []string{"manifest.yaml", "settings.yaml", "routers.yaml", "users.yaml", "presets.yaml", "filtersets.yaml",
		"panels.yaml", "artwork.yaml", "apiclients.yaml", "browser.yaml", "names/" + cfg.UID + ".yaml"} {
		if _, ok := files[want]; !ok {
			t.Errorf("no %s in %v", want, slices.Sorted(maps.Keys(files)))
		}
	}
	var art []string
	for name := range files {
		if !exportEntryRe.MatchString(name) {
			t.Errorf("entry %q is not one the import accepts", name)
		}
		if strings.HasPrefix(name, "art/") {
			art = append(art, name)
		}
		// Items name the router by prefix; only the router's own files name its UID.
		if name != "manifest.yaml" && name != "routers.yaml" && !strings.HasPrefix(name, "names/") && bytes.Contains(files[name], []byte(cfg.UID)) {
			t.Errorf("%s holds the router UID:\n%s", name, files[name])
		}
	}
	slices.Sort(art)
	if len(art) != 3 || !strings.HasPrefix(art[0], "art/defaults/d-") || !strings.HasPrefix(art[1], "art/icons/") || !strings.HasPrefix(art[2], "art/panels/p-") {
		t.Errorf("art entries: %v", art)
	}

	var man exportManifest
	if err := yaml.Unmarshal(files["manifest.yaml"], &man); err != nil {
		t.Fatal(err)
	}
	if man.Format != 1 || man.Install != cfg.InstallUID || man.Install == "" || man.Sode != BuildLabel() {
		t.Errorf("manifest: %+v", man)
	}
	r := man.Routers
	if len(r) != 1 || r[0].UID != cfg.UID || r[0].Prefix != cfg.Prefix || r[0].Matrix != cfg.Matrix || r[0].Levels != cfg.Levels || r[0].NamesHash == "" {
		t.Errorf("manifest routers: %+v", r)
	}
	if !slices.Equal(man.Categories, everyCategory) {
		t.Errorf("categories: %v", man.Categories)
	}
	if !bytes.Contains(files["users.yaml"], []byte("pin: ")) {
		t.Errorf("Include PINs left the hashes out:\n%s", files["users.yaml"])
	}

	imp := postImport(t, a, tok, rec.Body.Bytes())
	if imp.Code != 200 {
		t.Fatalf("import check: %d %s", imp.Code, imp.Body)
	}
	var check importCheck
	json.Unmarshal(imp.Body.Bytes(), &check)
	if !check.SameInstall || len(check.Unknown) != 0 || check.Counts[catPresets] != 1 || check.Counts[catUsers] != 3 ||
		check.Counts[catPanels] != 1 || check.Counts["icons"] != 1 || check.Counts["artDefaults"] != 1 || check.Counts[catAPIClients] != 1 {
		t.Errorf("import check: %+v", check)
	}
	b, err := a.stageAndLoad(bytes.NewReader(rec.Body.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if got := b.Presets[0].Items[0]; got.Src.Router != cfg.UID || got.Src.N != 2 || got.Src.Name != "CAM 2" || got.Dsts[0].Router != cfg.UID {
		t.Errorf("preset reference: %+v", got)
	}
	if got := b.Panels.Panels[0].Pages[0].Cells[0]["router"]; got != cfg.UID {
		t.Errorf("panel cell router: %v", got)
	}
	if len(b.Panels.Checkpoints[b.Panels.Panels[0].ID]) != 1 {
		t.Errorf("checkpoints: %+v", b.Panels.Checkpoints)
	}
	if b.APIClients[0].Owner != userID(t, a, "Ana") {
		t.Errorf("API client owner: %q", b.APIClients[0].Owner)
	}
	if b.Settings == nil || b.Settings.HiddenDsts != "9" || b.Settings.HiddenDstsRouter != cfg.UID || b.FilterSets[0].Router != cfg.UID {
		t.Errorf("settings: %+v", b.Settings)
	}
	if b.Browser["theme"] != "dark" || len(b.Names[cfg.UID].Sources) != 1 || len(b.Art) != 3 {
		t.Errorf("browser %v, names %+v, art %v", b.Browser, b.Names, b.Art)
	}
	if b.Routers[0].UID != cfg.UID || b.Routers[0].Address != cfg.Address {
		t.Errorf("routers: %+v", b.Routers)
	}
}

func TestExportPicksItemsAndLeavesPinsOut(t *testing.T) {
	a, tok := exportApp(t)
	seedEverything(t, a)
	ana := userID(t, a, "Ana")
	rec := postExport(t, a, tok, exportReq{Categories: []string{catUsers, catPresets, catSettings},
		Pick: map[string][]string{catUsers: {ana}, catPresets: {"nope"}}, Sections: []string{"branding"}})
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	files := unzipAll(t, rec.Body.Bytes())
	var users exportUsers
	yaml.Unmarshal(files["users.yaml"], &users)
	if len(users.Users) != 1 || users.Users[0].ID != ana || users.Users[0].Pin != "" || users.Users[0].Len != 0 {
		t.Errorf("users: %+v", users.Users)
	}
	if bytes.Contains(files["presets.yaml"], []byte("Show")) {
		t.Errorf("an unpicked preset went:\n%s", files["presets.yaml"])
	}
	var settings map[string]any
	yaml.Unmarshal(files["settings.yaml"], &settings)
	if len(settings) != 1 || settings["branding"] == nil {
		t.Errorf("settings sections: %v", settings)
	}
	// Nothing here names the router, so it does not go.
	if _, ok := files["routers.yaml"]; ok {
		t.Error("routers.yaml went with nothing that names the router")
	}
}

func TestExportRoutesNeedAnAdministrator(t *testing.T) {
	a, _ := exportApp(t)
	if rec := postExport(t, a, "", exportReq{Categories: []string{catSettings}}); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", rec.Code)
	}
	c := &client{ip: "10.0.0.8"}
	register(a, c)
	if err := a.UserUnlockAs(c, "Bo", "567890"); err != nil {
		t.Fatal(err)
	}
	bo := a.issueSessionTab(c, true)
	if rec := postExport(t, a, bo, exportReq{Categories: []string{catSettings}}); rec.Code != http.StatusForbidden {
		t.Errorf("Standard user export: %d", rec.Code)
	}
	if rec := postImport(t, a, bo, zipOf(t, map[string]string{"manifest.yaml": "format: 1\n"})); rec.Code != http.StatusForbidden {
		t.Errorf("Standard user import: %d", rec.Code)
	}
}

func TestImportRefusesBadFiles(t *testing.T) {
	a, tok := exportApp(t)
	ok := "format: 1\nrouters: [{uid: r1, prefix: A}]\n"
	for name, tc := range map[string]struct {
		files map[string]string
		code  int
		want  string
	}{
		"newer format":      {map[string]string{"manifest.yaml": "format: 2\n"}, 422, "made by a newer Sode"},
		"newer entry":       {map[string]string{"manifest.yaml": "format: 2\n", "macros.yaml": "x"}, 422, "made by a newer Sode"},
		"no manifest":       {map[string]string{"users.yaml": "users: []\n"}, 400, "not a Sode export"},
		"unknown entry":     {map[string]string{"manifest.yaml": ok, "notes.txt": "hi"}, 400, "not part of a Sode export"},
		"path out":          {map[string]string{"manifest.yaml": ok, "../config.yaml": "x"}, 400, "not part of a Sode export"},
		"shared router UID": {map[string]string{"manifest.yaml": "format: 1\nrouters: [{uid: r1, prefix: A}, {uid: r1, prefix: B}]\n"}, 400, "share the UID"},
		"shared UID in routers.yaml": {map[string]string{"manifest.yaml": ok,
			"routers.yaml": "routers: [{uid: r1, prefix: A}, {uid: r1, prefix: A}]\n"}, 400, "share the UID"},
	} {
		t.Run(name, func(t *testing.T) {
			rec := postImport(t, a, tok, zipOf(t, tc.files))
			if rec.Code != tc.code || !strings.Contains(rec.Body.String(), tc.want) {
				t.Errorf("%d %s, want %d %q", rec.Code, rec.Body, tc.code, tc.want)
			}
		})
	}

	t.Run("too many entries", func(t *testing.T) {
		defer func(n int) { exportMaxEntries = n }(exportMaxEntries)
		exportMaxEntries = 2
		rec := postImport(t, a, tok, zipOf(t, map[string]string{"manifest.yaml": ok, "users.yaml": "", "presets.yaml": ""}))
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "at most 2") {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})
	t.Run("entry too large", func(t *testing.T) {
		defer func(n int64) { exportMaxEntry = n }(exportMaxEntry)
		exportMaxEntry = 100
		rec := postImport(t, a, tok, zipOf(t, map[string]string{"manifest.yaml": ok + "#" + strings.Repeat("x", 200) + "\n"}))
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "manifest.yaml is over") {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})
	t.Run("upload too large", func(t *testing.T) {
		defer func(n int64) { exportMaxUpload = n }(exportMaxUpload)
		exportMaxUpload = 100
		rec := postImport(t, a, tok, zipOf(t, map[string]string{"manifest.yaml": ok + "#" + strings.Repeat("x", 200) + "\n"}))
		if rec.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("%d %s", rec.Code, rec.Body)
		}
	})
	// Every refusal and success removes its staging directory.
	if ents, _ := filepathGlob(a.stagesDir()); len(ents) != 0 {
		t.Errorf("staging directories left: %v", ents)
	}
}

// Every top-level config key is in one Settings section, or in exportNotSettings, so a new setting cannot
// miss the export. The sections are the page's own.
func TestExportSectionsCoverTheConfig(t *testing.T) {
	where := map[string][]string{}
	for _, s := range exportSections {
		for _, k := range s.Keys {
			where[k] = append(where[k], s.Name)
		}
	}
	for _, k := range exportNotSettings {
		where[k] = append(where[k], "not settings")
	}
	for k := range yamlFields(reflect.TypeFor[config.Config]()) {
		if len(where[k]) != 1 {
			t.Errorf("config key %s is in %v; put it in one exportSections entry or exportNotSettings", k, where[k])
		}
		delete(where, k)
	}
	for k := range where {
		t.Errorf("exportSections names %s, which the config does not have", k)
	}
	order, _ := settingsSections(t)
	for _, s := range exportSections {
		if !slices.Contains(order, s.Name) {
			t.Errorf("section %s is not a Settings section in index.html", s.Name)
		}
	}
}

func filepathGlob(dir string) ([]string, error) {
	return filepath.Glob(filepath.Join(dir, "import-*"))
}

// An item on a router that is not live (one an import left Broken) still exports a file that imports.
func TestExportOfForeignRouterItemImports(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) {
		p := showPreset(*cfg, "Show", "MON 4")
		p.Items[0].Src.Router, p.Items[0].Dsts[0].Router = otherRouterUID, otherRouterUID
		cfg.Presets = []config.Preset{p}
	})
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = nil })
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	p, ok := presetNamed(t, a, "Show")
	if !ok || p.Items[0].Src.Router != otherRouterUID || len(p.Broken) == 0 {
		t.Fatalf("Show: %+v", p)
	}
}
