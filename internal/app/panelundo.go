package app

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// The Undo cell: each press undoes the newest take of the calling panel instance that nothing has undone yet.
// Its own route with the take scope, so a panel key undoes without the history scope (PanelClientScopes).
// The instance is the caller's own (panelHeaders), never a request field.

func init() {
	registerAPI(
		apiRoute{"GET", apiBase + "/panels/{id}/undo", config.ScopeTake, (*App).apiPanelUndoNext},
		apiRoute{"POST", apiBase + "/panels/{id}/undo", config.ScopeTake, (*App).apiPanelUndo},
	)
}

// undoScan bounds the History walk in entries: each is a read from the file.
const undoScan = 1000

// undoInterleave is how many entries of other takes a Group's own may be spread across.
const undoInterleave = 16

type undoRoute struct {
	Src    int   `json:"src"`
	Dst    int   `json:"dst"`
	Levels []int `json:"levels"`
}

// undoTarget is the take to undo, as the caller sees its entries, and the plan that restores it. ID is the
// take's newest History entry, which GET hands out and POST may send back to undo only that take.
type undoTarget struct {
	ID      int
	Entries []Entry
	Plan    *takePlan
}

// panelUndoTarget is nil when this instance has nothing left to undo within undoScan.
func (a *App) panelUndoTarget(cl *apiCtx, panelID string) *undoTarget {
	instance := cl.Actor.instance
	if cl.Actor.panel != panelID || instance == "" {
		return nil
	}
	v := cl.view()
	rt := a.r()
	undone := map[int]bool{}
	skip := "" // a Group already judged, so its other entries are passed over
	before, seen := -1, 0
	for seen < undoScan {
		page, err := a.hist.Page(before, min(historyMax, undoScan-seen))
		if err != nil {
			log.Printf("panel undo: %v", err)
		}
		for i := 0; i < len(page) && seen < undoScan; i++ { // newest first
			e := page[i]
			seen++
			if e.Undo != nil {
				// A failed or refused undo restored nothing, so its take stays the next to undo.
				if e.OK {
					undone[*e.Undo] = true
				}
				continue
			}
			if e.Ext || undone[e.ID] || e.PanelID != panelID || !strings.EqualFold(e.Instance, instance) || (e.Group != "" && e.Group == skip) {
				continue
			}
			take := []Entry{e}
			if e.Group != "" {
				skip = e.Group
				rest := page[i+1:]
				if n := 1 + groupCount(rest, e.Group); n < e.GroupN {
					// A take running at the same time interleaved its entries, and Page keeps only an unbroken
					// Group whole, so the rest is on a later page: read it from here, with room for the other take's.
					if rest, err = a.hist.Page(e.ID, 2*e.GroupN+undoInterleave); err != nil {
						log.Printf("panel undo: %v", err)
					}
				}
				for _, g := range rest {
					if g.Group == e.Group && !undone[g.ID] && g.Undo == nil {
						take = append(take, g)
					}
				}
			}
			if es, p := undoPlanOf(rt, v, take); p != nil {
				// The newest entry the caller can see, so Hidden visibility never hands out a hidden id.
				return &undoTarget{es[len(es)-1].ID, es, p}
			}
		}
		if !morePages(page) {
			break
		}
		before = page[len(page)-1].ID
	}
	return nil
}

func groupCount(es []Entry, group string) int {
	n := 0
	for _, e := range es {
		if e.Group == group {
			n++
		}
	}
	return n
}

// undoPlanOf joins the undo of each entry of one take, oldest first; nil when none is visible or none restores anything.
func undoPlanOf(rt *runtime, v apiView, take []Entry) ([]Entry, *takePlan) {
	var es []Entry
	p := &takePlan{}
	for i := len(take) - 1; i >= 0; i-- {
		e, ok := v.Entry(take[i])
		if !ok {
			continue
		}
		u := planUndo(rt, v, e)
		if len(u.Items) == 0 {
			continue
		}
		es = append(es, e)
		p.Items = append(p.Items, u.Items...)
		p.Skipped = append(p.Skipped, u.Skipped...)
		p.Untouched = append(p.Untouched, u.Untouched...)
	}
	if len(p.Items) == 0 {
		return nil, nil
	}
	return es, p
}

func (a *App) apiPanelUndoNext(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	u := a.panelUndoTarget(cl, p.ID)
	if u == nil {
		apiJSON(w, map[string]any{"undo": nil})
		return
	}
	routes := make([]undoRoute, len(u.Plan.Items))
	for i, it := range u.Plan.Items {
		routes[i] = undoRoute{it.Src, it.Dst, it.Levels}
	}
	_, rights := a.callerOf(cl.client())
	reason := a.outsideRights(a.r(), rights, u.Plan.Items)
	apiJSON(w, map[string]any{"undo": map[string]any{"id": u.ID, "entries": u.Entries, "routes": routes, "allowed": reason == "", "reason": reason}})
}

// apiPanelUndo takes an optional {"id": n}: the id GET answered, so a press undoes the take the cell showed.
func (a *App) apiPanelUndo(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	a.apiTakeRun(w, r, cl, func(body []byte) (*takePlan, *apiProblem) {
		var in struct {
			ID *int `json:"id"`
		}
		if len(bytes.TrimSpace(body)) > 0 {
			if err := json.Unmarshal(body, &in); err != nil {
				return nil, problem(http.StatusBadRequest, "bad_request", "the body is {\"id\": n} or empty")
			}
		}
		u := a.panelUndoTarget(cl, p.ID)
		switch {
		case u == nil:
			return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "this panel instance has no take left to undo")
		case in.ID != nil && *in.ID != u.ID:
			return nil, problem(http.StatusConflict, "conflict", "the take to undo is now entry %d, not %d; ask again", u.ID, *in.ID)
		}
		return u.Plan, nil
	})
}
