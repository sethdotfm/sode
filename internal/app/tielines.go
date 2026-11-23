package app

import (
	"net/http"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// Tie lines (GLOSSARY.md, docs/tielines-design.md): cfg.TieLines, saved whole by an Administrator in Settings >
// Routers > Tie lines, apart from the one Save. GET /tielines lists them.

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/tielines", "", (*App).apiTieLines})
}

type tieLinesMsg struct {
	T        string           `json:"t"` // "tieLines"
	TieLines []config.TieLine `json:"tieLines"`
}

func (a *App) SaveTieLines(c *client, list []config.TieLine) error {
	if list == nil {
		list = []config.TieLine{}
	}
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, config.RoleAdministrator); err != nil {
		return err
	}
	return a.commit(func(cur *config.Config) error {
		cur.TieLines = a.nameTieLines(config.WithTieLineExtras(list, cur.TieLines))
		return nil
	}, func(_, nr *runtime) { a.broadcast(tieLinesMsg{T: "tieLines", TieLines: nr.cfg.TieLines}) })
}

// The file keeps each end's router name at save time, for the Name moved check. A number with no router name keeps
// the name the browser sent.
func (a *App) nameTieLines(list []config.TieLine) []config.TieLine {
	a.mu.Lock()
	defer a.mu.Unlock()
	name := func(r *config.Ref, kind string) {
		u, ok := a.unit(r.Router)
		if !ok || r.Router == "" {
			return
		}
		if n := strings.TrimSpace(sideNames(kind, u.router.srcNames, u.router.dstNames)[r.N]); n != "" {
			r.Name = n
		}
	}
	for i := range list {
		name(&list[i].From, config.MissingDestination)
		name(&list[i].To, config.MissingSource)
	}
	return list
}

type apiTieEnd struct {
	Router string `json:"router"`
	N      int    `json:"n"`
}

type apiTieLine struct {
	ID   string    `json:"id"`
	Name string    `json:"name"`
	From apiTieEnd `json:"from"`
	To   apiTieEnd `json:"to"`
}

// The router names of the two ends stay out. Thus a caller with hidden visibility sees no name that it cannot see. The
// Tie line name is not a router name, so it stays.
func (a *App) apiTieLines(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	out := []apiTieLine{}
	for _, t := range a.r().cfg.TieLines {
		out = append(out, apiTieLine{t.ID, t.Name, apiTieEnd{t.From.Router, t.From.N}, apiTieEnd{t.To.Router, t.To.N}})
	}
	apiJSONTagged(w, r, map[string][]apiTieLine{"tieLines": out})
}
