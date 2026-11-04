package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
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

const sampleDoc = `{"sode":1,"site":"Studio A","user":"Ana","exported":"2026-11-03T12:00:00Z",
"prefs":{"theme":"dark","clock":"24"},
"filterSets":[{"name":"Cams","src":"1-8","dst":"","sode":true,"ext":false}],
"presets":[{"name":"Show open","items":[{"src":12,"srcName":"CAM 1","dsts":[{"n":40,"name":"MV 1"}],"levels":[1,2]}]}]}`

func TestYAMLRoundTrip(t *testing.T) {
	rec := postYAML(t, "POST", "", sampleDoc)
	if rec.Code != 200 {
		t.Fatalf("to yaml: %d %s", rec.Code, rec.Body)
	}
	y := rec.Body.String()
	if !strings.Contains(y, "sode: 1") || !strings.Contains(y, "srcName: CAM 1") {
		t.Fatalf("yaml: %s", y)
	}
	back := postYAML(t, "POST", "?from=yaml", y)
	if back.Code != 200 {
		t.Fatalf("to json: %d %s", back.Code, back.Body)
	}
	var want, got any
	json.Unmarshal([]byte(sampleDoc), &want)
	if err := json.Unmarshal(back.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	// fields with false/empty values may be added by the typed struct; compare typed.
	var w, g exportDoc
	json.Unmarshal([]byte(sampleDoc), &w)
	json.Unmarshal(back.Body.Bytes(), &g)
	if !reflect.DeepEqual(w, g) {
		t.Fatalf("round trip differs:\n%+v\n%+v", w, g)
	}
}

func TestYAMLSizeCap(t *testing.T) {
	big := `{"sode":1,"site":"` + strings.Repeat("x", 1<<20) + `"}`
	if rec := postYAML(t, "POST", "", big); rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("got %d", rec.Code)
	}
}

func TestYAMLRejects(t *testing.T) {
	cases := []struct {
		name, query, body string
		code              int
	}{
		{"unknown json field", "", `{"sode":1,"bogus":2}`, 400},
		{"unknown yaml field", "?from=yaml", "sode: 1\nbogus: 2\n", 400},
		{"bad yaml", "?from=yaml", "sode: [1\n", 400},
		{"bad json", "", `{`, 400},
		{"empty", "?from=yaml", "", 400},
		{"wrong version", "", `{"sode":2}`, 422},
		{"missing version", "", `{"site":"x"}`, 422},
		{"duplicate preset", "", `{"sode":1,"presets":[{"name":"A","items":[]},{"name":"a","items":[]}]}`, 422},
		{"bad level", "", `{"sode":1,"presets":[{"name":"A","items":[{"src":1,"dsts":[{"n":2}],"levels":[0]}]}]}`, 422},
		{"bad time", "", `{"sode":1,"exported":"yesterday"}`, 422},
	}
	for _, c := range cases {
		rec := postYAML(t, "POST", c.query, c.body)
		if rec.Code != c.code || strings.TrimSpace(rec.Body.String()) == "" {
			t.Errorf("%s: got %d %q, want %d", c.name, rec.Code, rec.Body, c.code)
		}
	}
}

func TestYAMLMethodAndHeaders(t *testing.T) {
	rec := postYAML(t, "GET", "", "")
	if rec.Code != http.StatusMethodNotAllowed || rec.Header().Get("Allow") != "POST" {
		t.Fatalf("GET: %d allow=%q", rec.Code, rec.Header().Get("Allow"))
	}
	for _, r := range []*httptest.ResponseRecorder{rec, postYAML(t, "POST", "", sampleDoc)} {
		h := r.Header()
		if h.Get("Content-Security-Policy") == "" || h.Get("X-Content-Type-Options") != "nosniff" || h.Get("Referrer-Policy") != "same-origin" {
			t.Fatalf("security headers missing: %v", h)
		}
	}
}

// A filter set that leaves out sode/ext means "show them" (true), not false:
// false would hide every take and Recent would look empty.
func TestYAMLFilterDefaults(t *testing.T) {
	a := &App{}
	req := httptest.NewRequest("POST", "/api/yaml?from=yaml", strings.NewReader("sode: 1\nfilterSets:\n  - name: Cams\n    src: 1-8\n  - {name: Off, sode: false, ext: false}\n"))
	w := httptest.NewRecorder()
	a.serveYAML(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d: %s", w.Code, w.Body)
	}
	var got struct {
		FilterSets []struct {
			Name      string `json:"name"`
			Sode, Ext *bool
		} `json:"filterSets"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if f := got.FilterSets[0]; f.Sode == nil || !*f.Sode || f.Ext == nil || !*f.Ext {
		t.Errorf("omitted sode/ext should be true: %+v", f)
	}
	if f := got.FilterSets[1]; f.Sode == nil || *f.Sode || f.Ext == nil || *f.Ext {
		t.Errorf("explicit false must stay false: %+v", f)
	}
}
