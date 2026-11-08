package app

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
)

func httpsApp(t *testing.T, on bool) *App {
	t.Helper()
	a := newTestApp(t)
	cfg := a.r().cfg
	cfg.TrustedProxies = []string{"10.0.0.2"}
	cfg.RequireHTTPS = on
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return a
}

func httpsGet(a *App, path, peer string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("GET", path, nil)
	r.RemoteAddr = peer + ":4000"
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{"index.html": {Data: []byte("hi")}}).ServeHTTP(rec, r)
	return rec
}

var proto = map[string]string{"X-Forwarded-Proto": "https"}

func TestRequireHTTPSRefusesInsecureEverywhere(t *testing.T) {
	a := httpsApp(t, true)
	for _, p := range []string{"/", "/ws", "/api/yaml", "/api/v1/names", "/log"} {
		rec := httpsGet(a, p, "10.0.0.2", nil)
		if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "HTTPS") {
			t.Errorf("%s: %d %q", p, rec.Code, rec.Body.String())
		}
	}
}

func TestRequireHTTPSIgnoresHeaderFromUntrustedPeer(t *testing.T) {
	a := httpsApp(t, true)
	rec := httpsGet(a, "/", "192.0.2.9", proto)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("spoofed header accepted: %d", rec.Code)
	}
	if rec.Header().Get("Strict-Transport-Security") != "" {
		t.Error("HSTS on a refusal")
	}
}

func TestRequireHTTPSAllowsTrustedProxyWithHeader(t *testing.T) {
	a := httpsApp(t, true)
	rec := httpsGet(a, "/", "10.0.0.2", proto)
	if rec.Code != http.StatusOK {
		t.Fatalf("code %d", rec.Code)
	}
	if h := rec.Header().Get("Strict-Transport-Security"); !strings.HasPrefix(h, "max-age=") {
		t.Errorf("HSTS %q", h)
	}
	if rec := httpsGet(a, "/", "10.0.0.2", map[string]string{"X-Forwarded-Proto": "http"}); rec.Code != http.StatusForbidden {
		t.Errorf("http proto accepted: %d", rec.Code)
	}
}

func TestRequireHTTPSExemptsHealth(t *testing.T) {
	a := httpsApp(t, true)
	for _, p := range []string{"/healthz", "/api/health"} {
		if rec := httpsGet(a, p, "192.0.2.9", nil); rec.Code == http.StatusForbidden {
			t.Errorf("%s refused", p)
		}
	}
}

func TestRequireHTTPSOffChangesNothing(t *testing.T) {
	a := httpsApp(t, false)
	rec := httpsGet(a, "/", "192.0.2.9", nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Strict-Transport-Security") != "" {
		t.Fatalf("code %d, hsts %q", rec.Code, rec.Header().Get("Strict-Transport-Security"))
	}
}

func TestRequireHTTPSSaveNeedsSecureRequest(t *testing.T) {
	a := httpsApp(t, false)
	cfg := a.r().cfg
	cfg.RequireHTTPS = true
	if err := a.SaveSettings(&client{}, cfg, "", false); err != errInsecureSave {
		t.Fatalf("insecure save: %v", err)
	}
	if a.r().cfg.RequireHTTPS {
		t.Fatal("turned on")
	}
	if err := a.SaveSettings(&client{secure: true}, cfg, "", false); err != nil {
		t.Fatal(err)
	}
	if !a.r().cfg.RequireHTTPS {
		t.Fatal("not turned on")
	}
	// Staying on from an insecure connection is not a toggle.
	if err := a.SaveSettings(&client{}, cfg, "", false); err != nil {
		t.Fatalf("resave while on: %v", err)
	}
}

func TestRequireHTTPSSaveRefusesEmptyProxies(t *testing.T) {
	a := newTestApp(t)
	cfg := a.r().cfg
	cfg.RequireHTTPS = true
	if err := a.SaveSettings(&client{secure: true}, cfg, "", false); err == nil {
		t.Fatal("saved with no trusted proxies")
	}
}

func TestRequireHTTPSToggleIsLogged(t *testing.T) {
	a := httpsApp(t, false)
	defer startLog(t, a)()
	cfg := a.r().cfg
	cfg.RequireHTTPS = true
	if err := a.SaveSettings(&client{ip: "10.1.1.1", secure: true}, cfg, "", false); err != nil {
		t.Fatal(err)
	}
	cfg.RequireHTTPS = false
	if err := a.SaveSettings(&client{ip: "10.1.1.1", secure: true}, cfg, "", false); err != nil {
		t.Fatal(err)
	}
	var rows []string
	for _, e := range waitLogged(t, a, 2) {
		if e.Kind == kindAPI {
			rows = append(rows, e.Why+" "+e.Result+" "+e.By)
		}
	}
	if len(rows) != 2 || rows[0] != "requireHttps on 10.1.1.1" || rows[1] != "requireHttps off 10.1.1.1" {
		t.Fatalf("rows %q", rows)
	}
}

func TestRequireHTTPSGuardedByPin(t *testing.T) {
	live := config.Defaults()
	next := live
	next.RequireHTTPS = true
	if !config.Changed(live, next, "pin") {
		t.Fatal("requireHttps is not PIN-guarded")
	}
}
