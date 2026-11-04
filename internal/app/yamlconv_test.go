package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func postYAML(t *testing.T, method, query, body string) *httptest.ResponseRecorder {
	t.Helper()
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, httptest.NewRequest(method, "/api/yaml"+query, strings.NewReader(body)))
	return rec
}

func TestYAMLKeepsOrderAndTypes(t *testing.T) {
	in := `{"sode":1,"site":"Studio A","prefs":{"clock":"24","flip":"on","x":"true"},` +
		`"presets":[{"name":"Show","items":[{"src":12,"dsts":[{"n":40,"name":"MV 1"}],"levels":[1,2]}]}],"ok":true,"none":null}`
	rec := postYAML(t, "POST", "", in)
	if rec.Code != 200 {
		t.Fatalf("to yaml: %d %s", rec.Code, rec.Body)
	}
	y := rec.Body.String()
	want := "sode: 1\nsite: Studio A\nprefs:\n    clock: \"24\"\n    flip: \"on\"\n    x: \"true\"\npresets:\n"
	if !strings.HasPrefix(y, want) {
		t.Fatalf("yaml:\n%s", y)
	}
	for _, s := range []string{"- n: 40\n", "name: MV 1", "ok: true", "none: null"} {
		if !strings.Contains(y, s) {
			t.Errorf("missing %q in:\n%s", s, y)
		}
	}
	if strings.Contains(y, `"n"`) || strings.Contains(y, "{") {
		t.Errorf("keys quoted or flow style:\n%s", y)
	}
	back := postYAML(t, "POST", "?from=yaml", y)
	if back.Code != 200 {
		t.Fatalf("to json: %d %s", back.Code, back.Body)
	}
	for _, s := range []string{`"clock":"24"`, `"flip":"on"`, `"x":"true"`, `"sode":1`, `"ok":true`, `"n":40`, `"levels":[1,2]`} {
		if !strings.Contains(back.Body.String(), s) {
			t.Errorf("missing %s in %s", s, back.Body)
		}
	}
}

func TestYAMLSizeCap(t *testing.T) {
	big := `{"site":"` + strings.Repeat("x", 1<<20) + `"}`
	if rec := postYAML(t, "POST", "", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestYAMLRejects(t *testing.T) {
	cases := []struct{ name, query, body string }{
		{"bad yaml", "?from=yaml", "a: [1\n"},
		{"bad json", "", `{`},
		{"yaml text as json", "", "a: 1\n"},
		{"empty yaml", "?from=yaml", ""},
		{"empty json", "", ""},
		{"json list", "", `[1,2]`},
		{"json scalar", "", `3`},
		{"yaml list", "?from=yaml", "- a\n- b\n"},
		{"yaml scalar", "?from=yaml", "hello\n"},
		{"yaml number key", "?from=yaml", "1: a\n"},
	}
	for _, c := range cases {
		rec := postYAML(t, "POST", c.query, c.body)
		if rec.Code != 400 || strings.TrimSpace(rec.Body.String()) == "" {
			t.Errorf("%s: got %d %q, want 400", c.name, rec.Code, rec.Body)
		}
	}
}

func TestYAMLMethodAndHeaders(t *testing.T) {
	rec := postYAML(t, "GET", "", "")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("GET: %d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
	for _, r := range []*httptest.ResponseRecorder{rec, postYAML(t, "POST", "", `{"a":1}`)} {
		h := r.Header()
		if h.Get("Content-Security-Policy") == "" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "same-origin" {
			t.Fatalf("security headers missing: %v", h)
		}
	}
}
