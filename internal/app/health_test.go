package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

func getHealth(t *testing.T, a *App, method string) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	h := a.Handler(fstest.MapFS{})
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, "/api/health", nil))
	var body map[string]any
	if rec.Code != http.StatusMethodNotAllowed && rec.Body.Len() > 0 {
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("body %q: %v", rec.Body.String(), err)
		}
	}
	return rec, body
}

func TestHealthRouterUp(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	a.status.Router = "up"
	a.status.Addr = "10.0.0.1:8910"
	a.mu.Unlock()
	rec, b := getHealth(t, a, http.MethodGet)
	if rec.Code != 200 {
		t.Fatalf("code %d", rec.Code)
	}
	if rec.Header().Get("Cache-Control") != "no-store" {
		t.Errorf("cache-control %q", rec.Header().Get("Cache-Control"))
	}
	if b["server"] != "up" || b["router"] != "up" || b["addr"] != "10.0.0.1:8910" || b["version"] != VersionLabel() {
		t.Errorf("body %v", b)
	}
	if _, ok := b["err"]; ok {
		t.Errorf("err should be omitted: %v", b)
	}
	for _, k := range []string{"uptimeSeconds", "browsers"} {
		if _, ok := b[k]; !ok {
			t.Errorf("missing %s", k)
		}
	}
}

func TestHealthRouterNotUp(t *testing.T) {
	for _, st := range []string{"connecting", "down"} {
		a := newTestApp(t)
		a.mu.Lock()
		a.status.Router = st
		a.status.Err = "connection refused"
		a.mu.Unlock()
		rec, b := getHealth(t, a, http.MethodGet)
		if rec.Code != http.StatusServiceUnavailable {
			t.Errorf("%s: code %d", st, rec.Code)
		}
		if b["router"] != st || b["server"] != "up" || b["err"] != "connection refused" {
			t.Errorf("%s: body %v", st, b)
		}
	}
}

func TestHealthMethod(t *testing.T) {
	a := newTestApp(t)
	rec, _ := getHealth(t, a, http.MethodPost)
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("POST code %d", rec.Code)
	}
}
