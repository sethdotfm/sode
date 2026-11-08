package app

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
)

// REST write seam: a POST through the real Handler and the fake router, with shortTimers.

func apiPost(a *App, path, key, body string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", path, strings.NewReader(body))
	r.RemoteAddr = apiPeer
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

// takerApp is apiTestApp with the take scope on PanelsBot and shortened router timers.
func takerApp(t *testing.T, mut func(*config.Config)) (*App, *fakeRouter, string) {
	t.Helper()
	a, f := apiTestApp(t, func(c *config.Config) {
		c.APIClients[0].Scopes = append(c.APIClients[0].Scopes, config.ScopeTake)
		if mut != nil {
			mut(c)
		}
	})
	shortTimers(a)
	return a, f, botKey(t, a)
}

func TestAPIWTakeByRoutes(t *testing.T) {
	a, f, key := takerApp(t, nil)
	f.seed(2, 1, 0)
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	body := apiBody(t, rec)
	rs := body["results"].([]any)
	r0 := rs[0].(map[string]any)
	if len(rs) != 1 || r0["result"] != "connected" || r0["ok"] != true || r0["dst"] != float64(2) || r0["entry"] == nil {
		t.Fatalf("body %s", rec.Body.String())
	}
	if body["ok"] != true || body["checkSkipped"] != false {
		t.Fatalf("body %s", rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 1 || es[0].Actor == nil || es[0].Actor.Name != "PanelsBot" || es[0].Client != "PanelsBot (Panel 43)" {
		t.Fatalf("entries %+v", es)
	}
}
