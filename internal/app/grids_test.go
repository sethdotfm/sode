package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
)

func testGrid(link string) config.Grid {
	return config.Grid{Link: link, Name: link, Cols: 2, Rows: 2, Confirm: "default",
		Cells: []config.GridCell{{X: 0, Y: 0, W: 1, H: 1, Kind: "dst", Ref: "1"}}}
}

// fileApp is an App whose config is saved to a file in a temp directory.
func fileApp(t *testing.T) (*App, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Defaults()
	cfg.Normalize()
	hist, _ := OpenHistory("")
	a, err := New(cfg, path, hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return a, path
}

// listen registers c as a connected browser and returns what it is sent.
func listen(a *App, c *client) chan []byte {
	c.send = make(chan []byte, 32)
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	return c.send
}

func gridFile(path, link string) string {
	return filepath.Join(filepath.Dir(path), "grids", link+".yml")
}

func TestGridRoutes(t *testing.T) {
	a := newTestApp(t)
	h := a.Handler(fstest.MapFS{
		"grid.html":           {Data: []byte("GRID PAGE")},
		"settings-grids.html": {Data: []byte("EDITOR PAGE")},
		"core.js":             {Data: []byte("// core")},
		"index.html":          {Data: []byte("INDEX")},
	})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}
	for path, want := range map[string]string{
		"/grid/studio-a":        "GRID PAGE",
		"/grid/anything-at-all": "GRID PAGE", // unknown links are the page's to explain
		"/grid/ws":              "GRID PAGE", // links can't clash with files: the server serves none here
		"/settings/grids":       "EDITOR PAGE",
		"/core.js":              "// core",
	} {
		rec := get(path)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d %q, want %q", path, rec.Code, rec.Body.String(), want)
		}
	}
	for _, path := range []string{"/grid/studio-a", "/settings/grids"} {
		if rec := get(path); rec.Header().Get("Cache-Control") != "no-cache" {
			t.Errorf("%s: cache-control %q", path, rec.Header().Get("Cache-Control"))
		}
	}
	// Exactly one non-empty segment; the embedded files are not served under
	// the prefixes (the pages use <base href="../">).
	for _, path := range []string{"/grid/", "/grid", "/grid/a/b", "/grid/core.js/", "/grid/core.js/x", "/settings/", "/settings/other", "/settings/grids/", "/settings/core.js", "/settings/ws"} {
		if rec := get(path); rec.Code != http.StatusNotFound && rec.Code != http.StatusTemporaryRedirect { // /grid redirects to /grid/, which is 404
			t.Errorf("%s: %d %q, want 404", path, rec.Code, rec.Body.String())
		}
	}
	if rec := get("/grid/a/b"); rec.Code != http.StatusNotFound {
		t.Errorf("/grid/a/b: %d", rec.Code)
	}
	// The socket and icon stay at the root.
	for _, path := range []string{"/ws", "/icon.png"} {
		if rec := get(path); rec.Code == http.StatusNotFound || strings.Contains(rec.Body.String(), "PAGE") {
			t.Errorf("%s: %d", path, rec.Code)
		}
	}
}

// The pages, from the real embedded UI, and the files they name at the root.
func TestGridRoutesEmbedded(t *testing.T) {
	a := newTestApp(t)
	for path, want := range map[string]string{
		"/grid/x":                           "grid.js",
		"/settings/grids":                   "gridedit.js",
		"/grid.js":                          "gridgeom.js",
		"/gridgeom.js":                      "rectBetween",
		"/grid.css":                         "sode-grid",
		"/gridedit.js":                      "gridsResult",
		"/vendor/patchbay/css/patchbay.css": ".pb-btn",
	} {
		rec := get(t, a, path)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d, want a body with %q", path, rec.Code, want)
		}
	}
}

func TestGridSaveFiles(t *testing.T) {
	a, path := fileApp(t)
	c := &client{ip: "10.0.0.9"}
	rx := listen(a, c)
	if err := a.SaveGrid(c, testGrid("one"), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gridFile(path, "one")); err != nil {
		t.Fatalf("no file: %v", err)
	}
	var m gridsMsg
	if err := json.Unmarshal(<-rx, &m); err != nil || m.T != "grids" || len(m.Grids) != 1 || !m.GridConfirm {
		t.Fatalf("broadcast %v %+v", err, m)
	}
	// Replace by link.
	g := testGrid("one")
	g.Name = "Renamed"
	if err := a.SaveGrid(c, g, ""); err != nil || len(a.r().cfg.Grids) != 1 || a.r().cfg.Grids[0].Name != "Renamed" {
		t.Fatalf("%v %+v", err, a.r().cfg.Grids)
	}
	// A second panel; the list is kept sorted by link.
	if err := a.SaveGrid(c, testGrid("alpha"), ""); err != nil || a.r().cfg.Grids[0].Link != "alpha" {
		t.Fatalf("%v %+v", err, a.r().cfg.Grids)
	}
	// Rename: the old file goes, the new one appears.
	if err := a.SaveGrid(c, testGrid("beta"), "alpha"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(gridFile(path, "alpha")); err == nil {
		t.Fatal("old file kept after a rename")
	}
	if _, err := os.Stat(gridFile(path, "beta")); err != nil {
		t.Fatal(err)
	}
	// A rename onto another panel, or from one that isn't there, is refused.
	if err := a.SaveGrid(c, testGrid("one"), "beta"); err == nil {
		t.Fatal("rename onto an existing panel")
	}
	if err := a.SaveGrid(c, testGrid("gamma"), "nothing"); err == nil {
		t.Fatal("rename of a missing panel")
	}
	if len(a.r().cfg.Grids) != 2 {
		t.Fatalf("%+v", a.r().cfg.Grids)
	}
	// Delete.
	if err := a.DeleteGrid(c, "beta"); err != nil || len(a.r().cfg.Grids) != 1 {
		t.Fatal(err)
	}
	if _, err := os.Stat(gridFile(path, "beta")); err == nil {
		t.Fatal("deleted panel's file kept")
	}
	if err := a.DeleteGrid(c, "beta"); err == nil {
		t.Fatal("deleting a missing panel")
	}
	// The default is in config.yaml, panels are not.
	if err := a.SetGridConfirm(c, false); err != nil || a.r().cfg.GridConfirm {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "gridConfirm: false") || strings.Contains(string(b), "Renamed") {
		t.Fatalf("config.yaml:\n%s", b)
	}
	// A restart reads the files back.
	cfg, _, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.LoadGrids(path); err != nil || len(cfg.Grids) != 1 || cfg.Grids[0].Name != "Renamed" {
		t.Fatalf("%v %+v", err, cfg.Grids)
	}
}

func TestGridSaveRefusals(t *testing.T) {
	a, path := fileApp(t)
	c := &client{ip: "10.0.0.9"}
	for name, g := range map[string]config.Grid{
		"traversal":  testGrid("../evil"),
		"slash":      testGrid("a/b"),
		"dots":       testGrid(".."),
		"empty link": testGrid(""),
	} {
		if err := a.SaveGrid(c, g, ""); err == nil {
			t.Errorf("%s accepted", name)
		}
	}
	bad := testGrid("two")
	bad.Cells = append(bad.Cells, config.GridCell{X: 0, Y: 0, W: 1, H: 1, Kind: "src", Ref: "2"})
	if err := a.SaveGrid(c, bad, ""); err == nil {
		t.Fatal("overlap accepted")
	}
	two := testGrid("two")
	two.Cells = append(two.Cells, config.GridCell{X: 1, Y: 0, W: 1, H: 1, Kind: "confirm", Ref: "1"})
	if err := a.SaveGrid(c, two, ""); err == nil {
		t.Fatal("confirm cell with a ref accepted")
	}
	if len(a.r().cfg.Grids) != 0 {
		t.Fatalf("%+v", a.r().cfg.Grids)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(path), "evil.yml")); err == nil {
		t.Fatal("file outside the directory")
	}
	if ents, _ := os.ReadDir(filepath.Join(filepath.Dir(path), "grids")); len(ents) != 0 {
		t.Fatalf("files written for refused panels: %v", ents)
	}
	// A cell kind of each sort saves: a spacer, a confirm cell.
	ok := testGrid("kinds")
	ok.Cells = append(ok.Cells, config.GridCell{X: 1, Y: 0, W: 1, H: 1, Kind: "confirm"}, config.GridCell{X: 0, Y: 1, W: 1, H: 1})
	if err := a.SaveGrid(c, ok, ""); err != nil {
		t.Fatal(err)
	}
}

func TestGridPin(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	if err := a.SaveGrid(c, testGrid("one"), ""); err != nil {
		t.Fatal(err)
	}
	// A settings save from a page with an old copy keeps the grids.
	stale := a.r().cfg
	stale.Grids, stale.GridConfirm = nil, false
	if err := a.SaveSettings(c, stale, "2468", false); err != nil || len(a.r().cfg.Grids) != 1 || !a.r().cfg.GridConfirm {
		t.Fatalf("settings save changed the grids: %v %+v", err, a.r().cfg.Grids)
	}
	if err := a.SaveGrid(c, testGrid("two"), ""); !errors.Is(err, errLocked) {
		t.Fatalf("locked save: %v", err)
	}
	if err := a.DeleteGrid(c, "one"); !errors.Is(err, errLocked) {
		t.Fatalf("locked delete: %v", err)
	}
	if err := a.SetGridConfirm(c, false); !errors.Is(err, errLocked) {
		t.Fatalf("locked confirm: %v", err)
	}
	if a.Unlock(c, "2468") != nil {
		t.Fatal("unlock")
	}
	if err := a.DeleteGrid(c, "one"); err != nil || len(a.r().cfg.Grids) != 0 {
		t.Fatalf("unlocked delete: %v", err)
	}
	if err := a.SetGridConfirm(c, false); err != nil || a.r().cfg.GridConfirm {
		t.Fatalf("unlocked confirm: %v", err)
	}
}

func TestReplyOp(t *testing.T) {
	a := newTestApp(t)
	c := &client{ip: "10.0.0.9"}
	rx := listen(a, c)
	get := func() map[string]any {
		var m map[string]any
		json.Unmarshal(<-rx, &m)
		return m
	}
	a.replyOp(c, "grids", 7, 1, "grid", nil)
	if m := get(); m["t"] != "gridsResult" || m["ok"] != true || m["id"] != float64(7) || m["locked"] != nil {
		t.Fatalf("%v", m)
	}
	a.replyOp(c, "salvos", 8, 2, "salvo", errLocked)
	if m := get(); m["t"] != "salvosResult" || m["ok"] != false || m["locked"] != true || m["err"] == "" {
		t.Fatalf("%v", m)
	}
	a.replyOp(c, "config", -1, 0, "", errors.New("nope"))
	if m := get(); m["t"] != "configResult" || m["id"] != nil || m["locked"] != false {
		t.Fatalf("%v", m)
	}
}
