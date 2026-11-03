package app

import (
	"io/fs"
	"net/http"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Grid panels (#150): /grid/<link> shows a panel and /settings/grids edits
// them. Both are pages served at a deeper path than the rest of the UI, whose
// files, icon and WebSocket are all addressed relative to the page, so each
// prefix also serves those: /grid/core.js, /grid/ws and so on.

// gridsMsg is every browser's copy of the panels after a save. Only they
// changed, so it is not a full snapshot that would reset Recent.
type gridsMsg struct {
	T           string        `json:"t"` // "grids"
	Grids       []config.Grid `json:"grids"`
	GridConfirm bool          `json:"gridConfirm"`
}

// SaveGrids replaces the panels and the confirm default for everyone, from
// c (the editor). They're settings, so a settings PIN guards them too.
func (a *App) SaveGrids(c *client, grids []config.Grid, confirm bool) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.locked(c) {
		return errLocked
	}
	_, nr, err := a.update(func(cur *config.Config) error {
		cur.Grids, cur.GridConfirm = grids, confirm
		return nil
	})
	if err != nil {
		return err
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.broadcast(gridsMsg{T: "grids", Grids: nr.cfg.Grids, GridConfirm: nr.cfg.GridConfirm})
	a.mu.Unlock()
	return nil
}

// settingsPages are the pages under /settings/ by their name there. A mux
// takes a prefix once (a second handler for "/settings/" panics at startup),
// so another settings page is one more entry here, not another
// servePrefixed call.
var settingsPages = map[string]string{"grids": "settings-grids.html"}

// servePrefixed serves the pages under prefix ("/grid/" or "/settings/"):
// pages[name] for each name in pages and, if any is set, that file for every
// other single-segment name (and for none). It also serves the embedded
// files, the icon and the WebSocket the pages ask for relative to themselves.
func (a *App) servePrefixed(mux *http.ServeMux, static fs.FS, prefix string, pages map[string]string, any string) {
	files := http.FileServerFS(static)
	mux.HandleFunc(prefix, func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, prefix)
		w.Header().Set("Cache-Control", "no-cache")
		switch {
		case rest == "ws":
			a.serveWS(w, r)
		case rest == "icon.png":
			serveIcon(w, r)
		case rest == "manifest.webmanifest":
			a.serveManifest(w, r)
		case rest != "" && strings.Contains(rest, "."), strings.HasPrefix(rest, "vendor/"):
			// A file of the UI.
			r2 := r.Clone(r.Context())
			r2.URL.Path = "/" + rest
			files.ServeHTTP(w, r2)
		case pages[rest] != "":
			http.ServeFileFS(w, r, static, pages[rest])
		case any != "" && !strings.Contains(rest, "/"):
			http.ServeFileFS(w, r, static, any)
		default:
			http.NotFound(w, r)
		}
	})
}
