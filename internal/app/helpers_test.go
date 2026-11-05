package app

import "github.com/sethdotfm/sode/internal/config"

// Wrappers the tests call; the server itself goes through serveConfig and
// saveSettings.

// SaveSettings applies cfg from c with no "keep device defaults".
func (a *App) SaveSettings(c *client, cfg config.Config, pin string, clearPin bool) error {
	return a.saveSettings(c, cfg, pin, clearPin, false)
}

// ApplyConfig validates, saves and applies new settings, skipping the
// connection's checks.
func (a *App) ApplyConfig(c config.Config) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	return a.applyConfig(func(cur *config.Config) error { *cur = c; return nil })
}

func (a *App) pageAllowed(c *client, g gate) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.pageAllowedLocked(c, g)
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
