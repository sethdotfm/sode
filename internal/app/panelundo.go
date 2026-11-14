package app

import (
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

// undoScan bounds the History walk: each entry is a read from the file, and the page asks after every take.
const undoScan = 1000

type undoRoute struct {
	Src    int   `json:"src"`
	Dst    int   `json:"dst"`
	Levels []int `json:"levels"`
}

// panelUndoTarget is the take to undo, as the caller sees its entries, and the plan that restores it;
// nil when this instance has nothing left to undo within undoScan.
func (a *App) panelUndoTarget(cl *apiCtx, panelID string) ([]Entry, *takePlan) {
	instance := cl.Actor.instance
	if cl.Actor.panel != panelID || instance == "" {
		return nil, nil
	}
	v := cl.view()
	rt := a.r()
	undone := map[int]bool{}
	skip := "" // a Group already judged, so its other entries are passed over
	before, seen := -1, 0
	for seen < undoScan {
		page, err := a.hist.Page(before, historyMax)
		if err != nil {
			log.Printf("panel undo: %v", err)
		}
		for i := 0; i < len(page); i++ { // newest first
			e := page[i]
			seen++
			if e.Undo != nil {
				undone[*e.Undo] = true
				continue
			}
			if e.Ext || undone[e.ID] || e.PanelID != panelID || !strings.EqualFold(e.Instance, instance) || (e.Group != "" && e.Group == skip) {
				continue
			}
			take := []Entry{e}
			if e.Group != "" {
				skip = e.Group
				// A page never ends inside a Group, so its older entries follow here.
				for j := i + 1; j < len(page) && page[j].Group == e.Group; j++ {
					if !undone[page[j].ID] && page[j].Undo == nil {
						take = append(take, page[j])
					}
				}
			}
			if es, p := undoPlanOf(rt, v, take); p != nil {
				return es, p
			}
		}
		if !morePages(page) {
			break
		}
		before = page[len(page)-1].ID
	}
	return nil, nil
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
	es, plan := a.panelUndoTarget(cl, p.ID)
	if plan == nil {
		apiJSON(w, map[string]any{"undo": nil})
		return
	}
	routes := make([]undoRoute, len(plan.Items))
	for i, it := range plan.Items {
		routes[i] = undoRoute{it.Src, it.Dst, it.Levels}
	}
	_, rights := a.callerOf(cl.client())
	reason := a.outsideRights(a.r(), rights, plan.Items)
	apiJSON(w, map[string]any{"undo": map[string]any{"entries": es, "routes": routes, "allowed": reason == "", "reason": reason}})
}

func (a *App) apiPanelUndo(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	p, ok := a.panelFor(w, r, cl, false)
	if !ok {
		return
	}
	a.apiTakeRun(w, r, cl, func([]byte) (*takePlan, *apiProblem) {
		if _, plan := a.panelUndoTarget(cl, p.ID); plan != nil {
			return plan, nil
		}
		return nil, problem(http.StatusUnprocessableEntity, "nothing_to_do", "this panel instance has no take left to undo")
	})
}
