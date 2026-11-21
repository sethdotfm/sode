package app

import (
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/web"
)

func workerGet(t *testing.T, w *shellWorker, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", "/sw.js", nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	w.serve(rec, req)
	return rec
}

var (
	workerBuild = regexp.MustCompile(`const BUILD = ("[^"\n]*");`)
	workerShell = regexp.MustCompile(`const SHELL = (\[[^\n]*\]);`)
)

// The served worker names this build and lists every file a page loads, never a page, an API route or itself.
func TestServiceWorkerIsFilledIn(t *testing.T) {
	h := userApp(t, false).Handler(web.FS)
	rec := staticGet(t, h, "/sw.js", nil)
	if rec.Code != 200 {
		t.Fatalf("GET /sw.js: %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
		t.Errorf("Content-Type %q, want JavaScript", ct)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "no-cache" {
		t.Errorf("Cache-Control %q, want no-cache so a new build is seen on the next load", cc)
	}
	body := rec.Body.String()
	if strings.Contains(body, "__SODE_") {
		t.Fatal("a placeholder is left in the served worker")
	}
	m := workerBuild.FindStringSubmatch(body)
	if m == nil || !strings.Contains(m[1], BuildLabel()) {
		t.Fatalf("BUILD is %v, want it to name %q", m, BuildLabel())
	}
	s := workerShell.FindStringSubmatch(body)
	if s == nil {
		t.Fatal("no SHELL list")
	}
	var shell []string
	if err := json.Unmarshal([]byte(s[1]), &shell); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"main.js", "panelpage.js", "core.js", "base.css", "bar.css", "lock.css", "vendor/patchbay/css/patchbay.css", "vendor/fonts/plex.css", "vendor/patchbay/icons/lucide.svg", "panels-logo.svg"} {
		if !slices.Contains(shell, want) {
			t.Errorf("SHELL has no %s", want)
		}
	}
	for _, f := range shell {
		if strings.HasSuffix(f, ".html") || f == "sw.js" || strings.HasPrefix(f, "test/") {
			t.Errorf("SHELL lists %s", f)
		}
	}
	if etag := rec.Header().Get("ETag"); etag == "" || staticGet(t, h, "/sw.js", map[string]string{"If-None-Match": etag}).Code != 304 {
		t.Error("an unchanged worker is not a 304")
	}
}

// Any change to a web file is a new build name, so the browser installs a new worker with a new cache.
func TestServiceWorkerBuildFollowsTheFiles(t *testing.T) {
	src := []byte("const BUILD = '__SODE_BUILD__';\nconst SHELL = ['__SODE_SHELL__'];\n")
	build := func(files fstest.MapFS) string {
		files["sw.js"] = &fstest.MapFile{Data: src}
		m := workerBuild.FindStringSubmatch(workerGet(t, &shellWorker{fsys: files}, nil).Body.String())
		if m == nil {
			t.Fatal("no BUILD")
		}
		return m[1]
	}
	one := build(fstest.MapFS{"a.js": {Data: []byte("1")}, "index.html": {Data: []byte("<p>")}})
	if again := build(fstest.MapFS{"a.js": {Data: []byte("1")}, "index.html": {Data: []byte("<p>")}}); again != one {
		t.Errorf("the same files give %s and %s", one, again)
	}
	for name, files := range map[string]fstest.MapFS{
		"a changed script": {"a.js": {Data: []byte("2")}, "index.html": {Data: []byte("<p>")}},
		"a changed page":   {"a.js": {Data: []byte("1")}, "index.html": {Data: []byte("<b>")}},
		"a new file":       {"a.js": {Data: []byte("1")}, "index.html": {Data: []byte("<p>")}, "b.css": {Data: []byte("")}},
	} {
		if got := build(files); got == one {
			t.Errorf("%s keeps the build name %s", name, got)
		}
	}
}
