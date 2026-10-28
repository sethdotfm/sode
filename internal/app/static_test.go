package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

func TestFaviconServed(t *testing.T) {
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	a.Handler(web.FS).ServeHTTP(rec, httptest.NewRequest("GET", "/favicon.svg", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
		t.Fatalf("content type %q", ct)
	}
}

func TestAppManifestServed(t *testing.T) {
	a := newTestApp(t)
	for path, want := range map[string]string{
		"/manifest.webmanifest": "application/manifest+json",
		"/icon-192.png":         "image/png",
		"/icon-512.png":         "image/png",
		"/apple-touch-icon.png": "image/png",
	} {
		rec := httptest.NewRecorder()
		a.Handler(web.FS).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("%s: status %d", path, rec.Code)
		}
		if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, want) {
			t.Fatalf("%s: content type %q, want %q", path, ct, want)
		}
	}
}
