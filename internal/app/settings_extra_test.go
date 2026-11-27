package app

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// futureApp starts from a file a newer release wrote, with keys this release does not know.
// Its users are Ana (1234), Bo (567890) and the Administrator Ada (2468).
func futureApp(t *testing.T, yamlText string) *App {
	t.Helper()
	cfg, err := config.Parse([]byte("format: 2\n" + yamlText))
	if err != nil {
		t.Fatal(err)
	}
	ana, bo := mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890")
	for i := range cfg.Users {
		src := bo
		if i == 0 {
			src = ana
		}
		cfg.Users[i].Name, cfg.Users[i].Pin, cfg.Users[i].Len = src.Name, src.Pin, src.Len
	}
	withAdministrator(&cfg)
	cfgPath := filepath.Join(settledTempDir(t), "config.yaml")
	if err := config.Save(cfgPath, cfg); err != nil {
		t.Fatal(err)
	}
	hist, _ := OpenHistory("")
	a, err := New(cfg, cfgPath, hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func savedUnknownKeys(t *testing.T, a *App) []string {
	t.Helper()
	c, _, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	return c.UnknownKeys()
}

func TestSettingsSaveKeepsUnknownKeys(t *testing.T) {
	a := futureApp(t, "futureKey: 1\nlocks:\n  futureLock: true\n")
	if err := decodeAndSave(a, adminClient(t, a), []byte(`{"routerName":"Studio X"}`)); err != nil {
		t.Fatal(err)
	}
	c, _, err := config.Load(a.cfgPath)
	if err != nil || c.Routers[0].Title != "Studio X" {
		t.Fatalf("save not written: %v %q", err, c.Routers[0].Title)
	}
	if k, want := c.UnknownKeys(), []string{"futureKey", "locks.futureLock"}; !slices.Equal(k, want) {
		t.Fatalf("UnknownKeys = %q, want %q", k, want)
	}
}

// Every users save rebuilds each user, not only the edited one.
func TestUsersSaveKeepsUnknownKeys(t *testing.T) {
	a := futureApp(t, "users:\n  - name: a\n    futureField: x\n  - name: b\n")
	mustSave(t, a, adminClient(t, a), map[string]any{ukey(a, "Bo", "role"): config.RoleTrusted})
	if k, want := savedUnknownKeys(t, a), []string{"users[0].futureField"}; !slices.Equal(k, want) {
		t.Fatalf("UnknownKeys = %q, want %q", k, want)
	}
}
