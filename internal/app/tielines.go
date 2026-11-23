package app

import (
	"net/http"
	"slices"
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
		cur.TieLines = a.nameTieLines(config.WithTieLineExtras(list, cur.TieLines), cur.TieLines)
		return nil
	}, func(_, nr *runtime) { a.broadcast(tieLinesMsg{T: "tieLines", TieLines: nr.cfg.TieLines}) })
}

// nameTieLines gives each end the name stored for the Name moved check (docs/tielines-design.md, Tie lines rule 8).
// An end takes the name at its number now, and loses its Name moved mark, in three cases. The tie line is new, the
// end has a new router or number, or the browser sent the end with no name (the edit form). Every other end keeps its
// saved name and mark. Thus a save of one tie line (a suggestion Accept too) clears no mark of another. A number with
// no router name keeps the name the browser sent. The browser never sets a mark.
func (a *App) nameTieLines(list, saved []config.TieLine) []config.TieLine {
	a.mu.Lock()
	defer a.mu.Unlock()
	nameNow := func(r *config.Ref, kind string) {
		u, ok := a.unit(r.Router)
		if !ok || r.Router == "" {
			return
		}
		if n := strings.TrimSpace(sideNames(kind, u.router.srcNames, u.router.dstNames)[r.N]); n != "" {
			r.Name = n
		}
	}
	for i := range list {
		t := &list[i]
		j := slices.IndexFunc(saved, func(s config.TieLine) bool { return t.ID != "" && s.ID == t.ID })
		var was *config.TieLine
		if j >= 0 {
			was = &saved[j]
		}
		var moved []config.Missing
		for _, end := range []struct {
			r, old *config.Ref
			kind   string
		}{{&t.From, tieEnd(was, true), config.MissingDestination}, {&t.To, tieEnd(was, false), config.MissingSource}} {
			if end.old == nil || end.r.Router != end.old.Router || end.r.N != end.old.N || strings.TrimSpace(end.r.Name) == "" {
				nameNow(end.r, end.kind)
				continue
			}
			end.r.Name = end.old.Name
			for _, m := range was.Moved {
				if m.Kind == end.kind {
					moved = append(moved, m)
				}
			}
		}
		t.Moved = moved
	}
	return list
}

// tieEnd is the upstream (from) or downstream end of t, or nil when t is nil.
func tieEnd(t *config.TieLine, from bool) *config.Ref {
	switch {
	case t == nil:
		return nil
	case from:
		return &t.From
	}
	return &t.To
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
	// Changed is the last change of the upstream destination, or null when Sode has seen none. Before: Sode was down
	// after that time, or found the change at a re-read, so it is possible that the tie line changed again.
	Changed *time.Time `json:"changed"`
	Before  bool       `json:"before"`
}

// The router names of the two ends stay out. Thus a caller with hidden visibility sees no name that it cannot see. The
// Tie line name is not a router name, so it stays.
func (a *App) apiTieLines(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	out := []apiTieLine{}
	rt := a.r()
	stamps := a.tieChanges.snapshot(rt.cfg.TieLines)
	for _, t := range rt.cfg.TieLines {
		l := apiTieLine{ID: t.ID, Name: t.Name, From: apiTieEnd{t.From.Router, t.From.N}, To: apiTieEnd{t.To.Router, t.To.N}}
		if st, ok := stamps[t.ID]; ok {
			at := st.At.UTC()
			l.Changed, l.Before = &at, st.Found || st.At.Before(a.started)
		}
		out = append(out, l)
	}
	apiJSONTagged(w, r, map[string][]apiTieLine{"tieLines": out})
}
