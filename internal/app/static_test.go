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
