package app

import (
	"errors"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

func fixImportText(t *testing.T, a *App, mut func(*config.Config)) string {
	t.Helper()
	_, cfg := exportOf(t, a, false)
	mut(&cfg)
	b, err := yaml.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestImportRequireHTTPSNeedsSecureRequest(t *testing.T) {
	a := userApp(t, false)
	text := fixImportText(t, a, func(c *config.Config) {
		c.TrustedProxies = []string{"10.0.0.2"}
		c.RequireHTTPS = true
	})
	insecure := adminClient(t, a)
	if err := a.ImportConfig(insecure, text); !errors.Is(err, errInsecureSave) {
		t.Fatalf("insecure import: %v", err)
	}
	if a.r().cfg.RequireHTTPS || len(a.r().cfg.TrustedProxies) != 0 {
		t.Fatal("a refused import changed the config")
	}
	secure := adminClient(t, a)
	secure.secure = true
	if err := a.ImportConfig(secure, text); err != nil {
		t.Fatal(err)
	}
	if !a.r().cfg.RequireHTTPS {
		t.Fatal("not turned on")
	}
	// Staying on from an insecure connection is not a toggle.
	if err := a.ImportConfig(insecure, text); err != nil {
		t.Fatalf("re-import while on: %v", err)
	}
}

func TestImportRequireHTTPSRefusesEmptyProxies(t *testing.T) {
	a := userApp(t, false)
	text := fixImportText(t, a, func(c *config.Config) { c.RequireHTTPS = true })
	c := adminClient(t, a)
	c.secure = true
	if err := a.ImportConfig(c, text); err == nil {
		t.Fatal("imported Require HTTPS with no trusted proxies")
	}
}

func TestImportWritesAPIAuditRows(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	text := fixImportText(t, a, func(c *config.Config) {
		c.TrustedProxies = []string{"10.0.0.2"}
		c.RequireHTTPS = true
		c.API.Enabled = true
		c.API.GuestEnabled = true
		c.API.RateLimit = 77
		c.APIClients = []config.APIClient{{Name: "ImpBot", Scopes: []string{config.ScopeNames}}}
		c.GuestAPIScopes = []string{config.ScopeNames}
		c.Users[0].NoAutoLock = true
		c.Users[1].Rights = config.Rights{Sources: &[]int{1}}
	})
	c := adminClient(t, a)
	c.secure = true
	if err := a.ImportConfig(c, text); err != nil {
		t.Fatal(err)
	}
	want := []string{"requireHttps", "apiEnabled", "guestApi", "apiLimits", "clientEdit", "rightsEdit", "noAutoLockEdit"}
	var got map[string]bool
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		got = map[string]bool{}
		for _, e := range logged(t, a) {
			if e.Kind == kindAPI {
				got[e.Why] = true
			}
		}
		if len(got) >= len(want) {
			break
		}
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("no %s row; got %v", w, got)
		}
	}
}
