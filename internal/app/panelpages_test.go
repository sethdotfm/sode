package app

import (
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func TestPanelPagesAreServed(t *testing.T) {
	a := userApp(t, false)
	srv := httptest.NewServer(a.Handler(web.FS))
	defer srv.Close()
	for path, title := range map[string]string{
		"/panels":        "Sode Panels",
		"/panels/":       "Sode Panels",
		"/panels/editor": "Sode Panel Editor",
		"/panels/p-1a2b": "Sode Panel",
	} {
		res, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("%s: %d", path, res.StatusCode)
			continue
		}
		if !strings.Contains(string(body), "<title>"+title+"</title>") {
			t.Errorf("%s: wrong page, want title %q", path, title)
		}
		if res.Header.Get("Cache-Control") != "no-cache" || !strings.Contains(res.Header.Get("Content-Security-Policy"), "script-src 'self'") {
			t.Errorf("%s: headers %v", path, res.Header)
		}
	}
}

// The panel gate is the Locks.Panel switch, apart from main and log.
func TestPanelGate(t *testing.T) {
	a := userApp(t, false)
	cfg := a.r().cfg
	cfg.Locks = config.Locks{Panel: true}
	if !cfg.Locks.Any() {
		t.Fatal("Locks.Any ignores the panel lock")
	}
	if !gatePanel.on(cfg.Locks) || gateMain.on(cfg.Locks) || gateLog.on(cfg.Locks) {
		t.Fatal("gates do not follow their own switch")
	}
	if g, ok := pageGates["panel"]; !ok || g != gatePanel {
		t.Fatal("page type panel has no gate")
	}
}

func TestPanelLockEnforcedOnPanelPages(t *testing.T) {
	cfg := config.Defaults()
	cfg.Normalize()
	cfg.Users = []config.User{mkUser(t, "Ana", "1234")}
	cfg.Locks = config.Locks{Panel: true}
	cfg.SettingsPin = mkUser(t, "", "2468").Pin
	hist, _ := OpenHistory("")
	sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, filepath.Join(settledTempDir(t), "config.yaml"), hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.1"}
	c.u.page = "panel"
	a.mu.Lock()
	m := a.lockStateMu(c)
	a.mu.Unlock()
	if !m.Enforced || m.Page != "panel" || !m.Locked {
		t.Fatalf("lockState %+v", m)
	}
	if a.pageAllowed(c, gatePanel) {
		t.Fatal("panel page open with nobody signed in")
	}
	if !a.pageAllowed(c, gateMain) || !a.pageAllowed(c, gateLog) {
		t.Fatal("the panel lock closed another page")
	}
}
