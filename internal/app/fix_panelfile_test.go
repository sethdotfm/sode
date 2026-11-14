package app

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func fixPanelFile(a *App, id string) string {
	return filepath.Join(filepath.Dir(a.cfgPath), panelsDir, id+".yaml")
}

func fixKeyWorks(a *App, secret string) bool {
	return apiSend(a, "GET", "/api/v1/tally", secret, "").Code == 200
}

func fixHasClient(a *App, name string) bool {
	_, ok := a.r().cfg.FindAPIClient(name)
	return ok
}

func TestStartupKeepsClientOfDamagedPanelFile(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	a.keys.flush()
	path := fixPanelFile(a, id)
	good, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("name: [not yaml\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	b := fixRestart(t, a, func(*config.Config) {})
	b.keys.flush()
	if _, ok := b.panels.get(id); ok {
		t.Fatal("the damaged panel file loaded")
	}
	if !fixHasClient(b, panelClientName(id)) {
		t.Fatal("the client of a damaged panel file was dropped")
	}
	if !fixKeyWorks(b, secret) {
		t.Fatal("the key of a damaged panel file's client stopped working")
	}

	if err := os.WriteFile(path, good, 0o600); err != nil {
		t.Fatal(err)
	}
	c := fixRestart(t, b, func(*config.Config) {})
	if _, ok := c.panels.get(id); !ok {
		t.Fatal("the repaired panel did not load")
	}
	if !fixHasClient(c, panelClientName(id)) || !fixKeyWorks(c, secret) {
		t.Fatal("the repaired panel came back without its client or key")
	}
}

func TestStartupDropsClientOfDeletedPanelFile(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	a.keys.flush()
	if err := os.Remove(fixPanelFile(a, id)); err != nil {
		t.Fatal(err)
	}
	b := fixRestart(t, a, func(*config.Config) {})
	if fixHasClient(b, panelClientName(id)) {
		t.Fatal("the client of a deleted panel file was kept")
	}
	if fixKeyWorks(b, secret) {
		t.Fatal("the key of a deleted panel file's client still works")
	}
}

func TestStartupDropsClientTheLoadedPanelDoesNotName(t *testing.T) {
	a, _, _, id := keyedPanel(t)
	b := fixRestart(t, a, func(c *config.Config) {
		c.APIClients = append(c.APIClients, config.APIClient{Name: "Old wall", Panel: id, Scopes: []string{}})
	})
	if fixHasClient(b, "Old wall") {
		t.Fatal("a client its panel does not name was kept")
	}
	if !fixHasClient(b, panelClientName(id)) {
		t.Fatal("the panel's own client was dropped")
	}
}

func TestStartupKeepsPanelClientsWhenPanelsFolderUnreadable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads any folder")
	}
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	a.keys.flush()
	dir := filepath.Dir(fixPanelFile(a, id))
	if err := os.Chmod(dir, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	b := fixRestart(t, a, func(*config.Config) {})
	if !fixHasClient(b, panelClientName(id)) {
		t.Fatal("a panel client was dropped when the panels folder could not be read")
	}
	if !fixKeyWorks(b, secret) {
		t.Fatal("a panel client's key stopped working when the panels folder could not be read")
	}
}
