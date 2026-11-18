package app

import (
	"bytes"
	"compress/gzip"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

func staticGet(t *testing.T, h http.Handler, path string, hdr map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestStaticFilesETagAndGzip(t *testing.T) {
	h := userApp(t, false).Handler(web.FS)
	for _, path := range []string{"/", "/main.js", "/base.css", "/panels", "/log", "/vendor/patchbay/icons/lucide.svg"} {
		plain := staticGet(t, h, path, nil)
		if plain.Code != 200 || plain.Header().Get("Content-Encoding") != "" {
			t.Fatalf("%s: %d %v", path, plain.Code, plain.Header())
		}
		tag := plain.Header().Get("ETag")
		if tag == "" || plain.Header().Get("Cache-Control") != "no-cache" || plain.Header().Get("Content-Security-Policy") == "" {
			t.Fatalf("%s: headers %v", path, plain.Header())
		}
		if r := staticGet(t, h, path, map[string]string{"If-None-Match": tag}); r.Code != 304 || r.Body.Len() != 0 {
			t.Errorf("%s: If-None-Match gave %d with %d bytes", path, r.Code, r.Body.Len())
		}
		gz := staticGet(t, h, path, map[string]string{"Accept-Encoding": "br, gzip"})
		if gz.Header().Get("Content-Encoding") != "gzip" || gz.Header().Get("ETag") == tag || !strings.Contains(gz.Header().Get("Vary"), "Accept-Encoding") {
			t.Fatalf("%s: gzip headers %v", path, gz.Header())
		}
		zr, err := gzip.NewReader(gz.Body)
		if err != nil {
			t.Fatal(err)
		}
		got, _ := io.ReadAll(zr)
		if !bytes.Equal(got, plain.Body.Bytes()) || gz.Body.Len() >= plain.Body.Len() {
			t.Errorf("%s: gzip body %d bytes, plain %d", path, gz.Body.Len(), plain.Body.Len())
		}
		if r := staticGet(t, h, path, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": gz.Header().Get("ETag")}); r.Code != 304 {
			t.Errorf("%s: gzip If-None-Match gave %d", path, r.Code)
		}
		if r := staticGet(t, h, path, map[string]string{"Accept-Encoding": "gzip;q=0"}); r.Header().Get("Content-Encoding") != "" {
			t.Errorf("%s: gzip sent although q=0", path)
		}
	}
	// woff2 is compressed already.
	font := staticGet(t, h, "/vendor/fonts/IBMPlexSans-Regular.woff2", map[string]string{"Accept-Encoding": "gzip"})
	if font.Code != 200 || font.Header().Get("Content-Encoding") != "" || font.Header().Get("ETag") == "" || font.Header().Get("Content-Type") != "font/woff2" {
		t.Errorf("woff2: %d %v", font.Code, font.Header())
	}
	if r := staticGet(t, h, "/index.html", nil); r.Code != http.StatusMovedPermanently {
		t.Errorf("/index.html: %d", r.Code)
	}
	if r := staticGet(t, h, "/nope.js", nil); r.Code != 404 {
		t.Errorf("/nope.js: %d", r.Code)
	}
}

// Pages must draw on a network with no internet, so nothing they load comes from another host.
func TestPagesLoadNothingFromOutside(t *testing.T) {
	outside := regexp.MustCompile(`(?i)(href|src)="(https?:)?//|url\(\s*["']?(https?:)?//|@import`)
	err := fs.WalkDir(web.FS, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !(strings.HasSuffix(name, ".html") || strings.HasSuffix(name, ".css")) {
			return err
		}
		b, _ := fs.ReadFile(web.FS, name)
		if m := outside.Find(b); m != nil {
			t.Errorf("%s loads from outside Sode: %s", name, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{"plex.css", "OFL.txt", "VERSION", "IBMPlexSans-Regular.woff2", "IBMPlexSans-Medium.woff2", "IBMPlexSans-SemiBold.woff2", "IBMPlexMono-Regular.woff2", "IBMPlexMono-Medium.woff2"} {
		if _, err := fs.Stat(web.FS, "vendor/fonts/"+f); err != nil {
			t.Error(err)
		}
	}
	if strings.Contains(csp, "https:") {
		t.Errorf("csp names an outside host: %s", csp)
	}
}
