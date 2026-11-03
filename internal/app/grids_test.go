package app

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
)

func testGrid(link string) config.Grid {
	return config.Grid{Link: link, Name: link, Cols: 2, Rows: 2, Confirm: "default",
		Cells: []config.GridCell{{X: 0, Y: 0, W: 1, H: 1, Kind: "dst", Ref: "1"}}}
}

func TestGridRoutes(t *testing.T) {
	a := newTestApp(t)
	h := a.Handler(fstest.MapFS{
		"grid.html":           {Data: []byte("GRID PAGE")},
		"settings-grids.html": {Data: []byte("EDITOR PAGE")},
		"core.js":             {Data: []byte("// core")},
		"index.html":          {Data: []byte("INDEX")},
		"vendor/x.css":        {Data: []byte("x{}")},
	})
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec
	}
	for path, want := range map[string]string{
		"/grid/studio-a":        "GRID PAGE",
		"/grid/anything-at-all": "GRID PAGE", // unknown links are the page's to explain
		"/grid/":                "GRID PAGE",
		"/grid/core.js":         "// core", // the page's relative files
		"/grid/vendor/x.css":    "x{}",
		"/settings/grids":       "EDITOR PAGE",
		"/settings/core.js":     "// core",
	} {
		rec := get(path)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d %q, want %q", path, rec.Code, rec.Body.String(), want)
		}
	}
	if rec := get("/grid/studio-a"); rec.Header().Get("Cache-Control") != "no-cache" {
		t.Errorf("cache-control %q", rec.Header().Get("Cache-Control"))
	}
	for _, path := range []string{"/settings/other", "/settings/", "/grid/a/b"} {
		if rec := get(path); rec.Code != http.StatusNotFound {
			t.Errorf("%s: %d, want 404", path, rec.Code)
		}
	}
	// The socket is reachable under the prefixes (a plain GET is refused
	// as not an upgrade, but it is routed to the socket, not the page).
	for _, path := range []string{"/grid/ws", "/settings/ws"} {
		if rec := get(path); rec.Code == http.StatusNotFound || strings.Contains(rec.Body.String(), "PAGE") {
			t.Errorf("%s: %d %q", path, rec.Code, rec.Body.String())
		}
	}
}

// The pages and their relative files, from the real embedded UI.
func TestGridRoutesEmbedded(t *testing.T) {
	a := newTestApp(t)
	for path, want := range map[string]string{
		"/grid/x":                                    "grid.js",
		"/settings/grids":                            "gridedit.js",
		"/grid/grid.js":                              "gridgeom.js",
		"/grid/gridgeom.js":                          "rectBetween",
		"/grid/grid.css":                             "sode-grid",
		"/grid/vendor/patchbay/css/patchbay.css":     ".pb-btn",
		"/settings/gridedit.js":                      "gridsResult",
		"/settings/vendor/patchbay/css/patchbay.css": ".pb-btn",
	} {
		rec := get(t, a, path)
		if rec.Code != 200 || !strings.Contains(rec.Body.String(), want) {
			t.Errorf("%s: %d, want a body with %q", path, rec.Code, want)
		}
	}
}

func ptr(b bool) *bool { return &b }

// A grids message without gridConfirm keeps the current default.
func TestSaveGridsKeepsConfirm(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	if err := a.SaveGrids(c, nil, ptr(false)); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveGrids(c, []config.Grid{testGrid("one")}, nil); err != nil || a.r().cfg.GridConfirm {
		t.Fatalf("confirm changed by an absent value: %v", err)
	}
}

func TestSaveGrids(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	if err := a.SaveGrids(c, []config.Grid{testGrid("one")}, ptr(false)); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	if len(cfg.Grids) != 1 || cfg.GridConfirm {
		t.Fatalf("%+v", cfg.Grids)
	}
	// A settings save from a page with an old copy keeps the grids.
	stale := cfg
	stale.Grids, stale.GridConfirm = nil, true
	if err := a.SaveSettings(c, stale, "", false); err != nil || len(a.r().cfg.Grids) != 1 || a.r().cfg.GridConfirm {
		t.Fatalf("settings save changed the grids: %v %+v", err, a.r().cfg.Grids)
	}
	bad := testGrid("two")
	bad.Cells = append(bad.Cells, config.GridCell{X: 0, Y: 0, W: 1, H: 1, Kind: "src", Ref: "2"})
	if err := a.SaveGrids(c, []config.Grid{bad}, ptr(true)); err == nil || len(a.r().cfg.Grids) != 1 {
		t.Fatalf("overlap accepted: %v", err)
	}
	// With a PIN, saving needs the unlock.
	if err := a.SaveSettings(c, a.r().cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveGrids(c, nil, ptr(true)); !errors.Is(err, errLocked) {
		t.Fatalf("locked save: %v", err)
	}
	if a.Unlock(c, "2468") != nil {
		t.Fatal("unlock")
	}
	if err := a.SaveGrids(c, nil, ptr(true)); err != nil || len(a.r().cfg.Grids) != 0 || !a.r().cfg.GridConfirm {
		t.Fatalf("unlocked save: %v", err)
	}
}
