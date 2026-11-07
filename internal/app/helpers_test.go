package app

import (
	"encoding/json"

	"github.com/sethdotfm/sode/internal/config"
)

func decodeAndSave(a *App, c *client, b []byte) error {
	return a.saveSettings(c, b, "", false)
}

// Wrappers the tests call; the server itself goes through serveConfig and
// saveSettings.

// SaveSettings applies cfg from c as a message that carried every key.
func (a *App) SaveSettings(c *client, cfg config.Config, pin string, clearPin bool) error {
	b, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	return a.saveSettings(c, b, pin, clearPin)
}

// ApplyConfig validates, saves and applies new settings, skipping the
// connection's checks.
func (a *App) ApplyConfig(c config.Config) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.applyConfig(func(cur *config.Config) error { *cur = c; return nil })
}

// UserLock is what the userLock handler does.
func (a *App) UserLock(c *client) { a.setHeld(c, true) }

func (a *App) issueSession(c *client) string { return a.issueToken(c, false) }

func (a *App) issueSessionTab(c *client, tab bool) string { return a.issueToken(c, tab) }

func (a *App) pageAllowed(c *client, g gate) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pageAllowedMu(c, g)
}

// Len is how many entries the history holds.
func (h *History) Len() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.f == nil {
		return len(h.mem)
	}
	return len(h.offs)
}
