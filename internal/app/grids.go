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

// servePrefixed serves page at prefix+name (prefix is "/grid/" or
// "/settings/"; name "" or any slug when any is true), and under prefix the
// embedded files and the WebSocket the page asks for relative to itself.
func (a *App) servePrefixed(mux *http.ServeMux, static fs.FS, prefix, page, only string) {
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
		case only == "" && !strings.Contains(rest, "/"), rest == only:
			http.ServeFileFS(w, r, static, page)
		default:
			http.NotFound(w, r)
		}
	})
}
