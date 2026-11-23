package app

import (
	"net/http"
	"strings"
	"time"

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
	ID   string `json:"id"`
	Name string `json:"name"`
	// From and To are null for a caller with hidden visibility that cannot see that end.
	From *apiTieEnd `json:"from"`
	To   *apiTieEnd `json:"to"`
	// Changed is the last change of the upstream destination, or null when Sode has seen none. Before: Sode was down
	// after that time, or found the change at a re-read, so it is possible that the tie line changed again.
	Changed *time.Time `json:"changed"`
	Before  bool       `json:"before"`
}

// The router names of the two ends stay out. A caller with hidden visibility gets null for an end that it cannot see.
// It gets no change time when it cannot see the upstream destination, as GET /tally and History hide them. It keeps
// the id and the Tie line name, which a take with `ties` names and a 409 body shows.
func (a *App) apiTieLines(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	out := []apiTieLine{}
	rt := a.r()
	v := a.view(cl)
	stamps := a.tieChanges.snapshot(rt.cfg.TieLines)
	for _, t := range rt.cfg.TieLines {
		l := apiTieLine{ID: t.ID, Name: t.Name}
		from := v.on(t.From.Router).Destination(t.From.N)
		if from {
			l.From = &apiTieEnd{t.From.Router, t.From.N}
		}
		if v.on(t.To.Router).Source(t.To.N) {
			l.To = &apiTieEnd{t.To.Router, t.To.N}
		}
		if st, ok := stamps[t.ID]; ok && from {
			at := st.At.UTC()
			l.Changed, l.Before = &at, st.Found || st.At.Before(a.started)
		}
		out = append(out, l)
	}
	apiJSONTagged(w, r, map[string][]apiTieLine{"tieLines": out})
}
