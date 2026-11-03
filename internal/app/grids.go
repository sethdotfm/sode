package app

import (
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Grid panels (#150): /grid/<link> shows a panel and /settings/grids edits
// them. Each panel is a file, grids/<link>.yml beside the config file
// (config.GridsDir); update writes and removes them as the list changes. The
// pages are served at a deeper path than the rest of the UI, so they say
// <base href="../"> and address everything from the root.

// gridsMsg is every browser's copy of the panels after a save. Only they
// changed, so it is not a full snapshot that would reset Recent.
type gridsMsg struct {
	T           string        `json:"t"` // "grids"
	Grids       []config.Grid `json:"grids"`
	GridConfirm bool          `json:"gridConfirm"`
}

func gridsBroadcast(cfg config.Config) any {
	return gridsMsg{T: "grids", Grids: cfg.Grids, GridConfirm: cfg.GridConfirm}
}

// SaveGrid saves one panel for everyone, from c (the editor), creating it or
// replacing the one with its link. was is the link it had before a rename
// ("" = not renamed): that panel goes, and the new link must be free.
// They're settings, so a settings PIN guards them too.
func (a *App) SaveGrid(c *client, g config.Grid, was string) error {
	return a.saveShared(c, func(cur *config.Config) error {
		g = cloneGrid(g)
		// The link is compared as it will be stored.
		g.Link = strings.ToLower(strings.TrimSpace(g.Link))
		was = strings.TrimSpace(was)
		i := slices.IndexFunc(cur.Grids, func(x config.Grid) bool { return x.Link == g.Link })
		if was != "" && was != g.Link {
			j := slices.IndexFunc(cur.Grids, func(x config.Grid) bool { return x.Link == was })
			switch {
			case j < 0:
				return fmt.Errorf("there is no grid %q to rename", was)
			case i >= 0:
				return fmt.Errorf("there is already a grid %q", g.Link)
			}
			cur.Grids = slices.Delete(cur.Grids, j, j+1)
			i = -1
		}
		if i >= 0 {
			cur.Grids[i] = g
		} else {
			cur.Grids = append(cur.Grids, g)
		}
		slices.SortFunc(cur.Grids, func(x, y config.Grid) int { return strings.Compare(x.Link, y.Link) })
		return nil
	}, gridsBroadcast)
}

// DeleteGrid removes the panel with that link, for everyone, from c.
func (a *App) DeleteGrid(c *client, link string) error {
	return a.saveShared(c, func(cur *config.Config) error {
		i := slices.IndexFunc(cur.Grids, func(x config.Grid) bool { return x.Link == link })
		if i < 0 {
			return fmt.Errorf("there is no grid %q", link)
		}
		cur.Grids = slices.Delete(cur.Grids, i, i+1)
		return nil
	}, gridsBroadcast)
}

// SetGridConfirm saves whether panels set to "default" confirm their takes
// (config.yaml; not a panel's file).
func (a *App) SetGridConfirm(c *client, on bool) error {
	return a.saveShared(c, func(cur *config.Config) error {
		cur.GridConfirm = on
		return nil
	}, gridsBroadcast)
}

// cloneGrid copies g so the copy shares no slices with the caller's.
func cloneGrid(g config.Grid) config.Grid {
	g.Levels = slices.Clone(g.Levels)
	g.Cells = slices.Clone(g.Cells)
	return g
}

var errNoGrid = errors.New("no grid in the request")

// serveGridPages adds the two grid pages to mux: /grid/<link> (exactly one
// segment; unknown links are the page's to explain) and /settings/grids.
func serveGridPages(mux *http.ServeMux, static fs.FS) {
	mux.HandleFunc("/grid/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/grid/")
		if rest == "" || strings.Contains(rest, "/") {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, static, "grid.html")
	})
	mux.HandleFunc("/settings/grids", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-cache")
		http.ServeFileFS(w, r, static, "settings-grids.html")
	})
}
