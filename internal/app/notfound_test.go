package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// A mistyped page path gets a Sode page with a link back to the main page. The link is relative, so it works
// behind a proxy that serves Sode under a path. API paths and callers that do not ask for HTML keep the plain reply.
func TestUnknownPageGetsSodePage(t *testing.T) {
	a := newTestApp(t)
	srv := httptest.NewServer(a.Handler(web.FS))
	defer srv.Close()

	get := func(path, accept string) (int, string, string) {
		t.Helper()
		req, _ := http.NewRequest(http.MethodGet, srv.URL+path, nil)
		if accept != "" {
			req.Header.Set("Accept", accept)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		b, _ := io.ReadAll(res.Body)
		return res.StatusCode, res.Header.Get("Content-Type"), string(b)
	}
	const html = "text/html,application/xhtml+xml,*/*;q=0.8"
	for path, home := range map[string]string{
		"/logs":                 "./",
		"/no-such-page.html":    "./",
		"/panels/editor/x":      "../../",
		"/panels/not%20an%20id": "../",
		"/vendor/nothing.css":   "../",
		"/log/":                 "../",
		"/panels/editor/":       "../../",
		"/panels/a%2Fb":         "../",
	} {
		code, ctype, body := get(path, html)
		if code != http.StatusNotFound || !strings.HasPrefix(ctype, "text/html") {
			t.Errorf("%s: %d %s", path, code, ctype)
			continue
		}
		if !strings.Contains(body, "This page does not exist.") || !strings.Contains(body, `<base href="`+home+`">`) || !strings.Contains(body, `href="./">Open Sode</a>`) {
			t.Errorf("%s: want the Sode page with base %q, got %q", path, home, body)
		}
	}
	for _, c := range []struct{ path, accept string }{
		{"/api/nothing", html},
		{"/api/log/nothing", html},
		{"/api/", html},
		{"/logs", ""},
		{"/logs", "application/json"},
	} {
		if code, _, body := get(c.path, c.accept); code != http.StatusNotFound || body != "404 page not found\n" {
			t.Errorf("%s (Accept %q): %d %q, want the plain reply", c.path, c.accept, code, body)
		}
	}
	if code, ctype, _ := get("/api/v1/nothing", html); code != http.StatusNotFound || !strings.Contains(ctype, "json") {
		t.Errorf("/api/v1/nothing: %d %s, want the API's JSON reply", code, ctype)
	}
	if code, _, body := get("/log", html); code != http.StatusOK || strings.Contains(body, "This page does not exist.") {
		t.Errorf("/log: %d", code)
	}
}
