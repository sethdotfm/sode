package app

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/sethdotfm/sode/internal/config"
)

// Set and free Sode's Protect on one destination (protect.go). An API call is its own Confirm (ADR 0002).

func init() {
	registerAPI(
		apiRoute{"POST", apiBase + "/protected/{dst}", config.ScopeProtect, (*App).apiProtectSet},
		apiRoute{"DELETE", apiBase + "/protected/{dst}", config.ScopeProtect, (*App).apiProtectClear},
	)
}

type apiProtectOut struct {
	OK     bool   `json:"ok"`
	Dst    int    `json:"dst"`
	Levels []int  `json:"levels"`
	Failed []int  `json:"failed,omitempty"`
	Still  []int  `json:"still,omitempty"`
	Error  string `json:"error,omitempty"`
	Entry  *int   `json:"entry,omitempty"`
}

func (a *App) apiProtectSet(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	a.apiProtect(w, r, cl, true)
}

func (a *App) apiProtectClear(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	a.apiProtect(w, r, cl, false)
}

func (a *App) apiProtect(w http.ResponseWriter, r *http.Request, cl *apiCtx, set bool) {
	dst, err := strconv.Atoi(r.PathValue("dst"))
	if err != nil || dst < 0 {
		apiFail(w, http.StatusBadRequest, "bad_request", "the destination must be the router's number, 0-based")
		return
	}
	// The hidden view gets the same answer for a destination it cannot see as for one that is not there.
	if !a.view(cl).on("").Destination(dst) {
		apiFail(w, http.StatusNotFound, "not_found", "no such destination")
		return
	}
	done, ok := a.writes.start("protect")
	if !ok {
		apiFail(w, http.StatusServiceUnavailable, "router_down", errShuttingDown.Error())
		return
	}
	defer done()
	c := cl.client()
	why := "protect"
	var o protectOutcome
	if set {
		o = a.protectSet(c, dst, nil)
	} else {
		why = "protectClear"
		o = a.protectClear(c, dst, nil, nil)
	}
	note := fmt.Sprintf("destination %d", dst+a.r().cfg.NumberOffset)
	if o.Err != nil {
		a.apiEvent(cl.Actor, why, outcome(o.Err), note, o.Err)
		var pe protectErr
		if errors.As(o.Err, &pe) {
			apiFail(w, pe.status, pe.code, pe.msg)
			return
		}
		apiFail(w, http.StatusInternalServerError, "internal", o.Err.Error())
		return
	}
	e := o.Entry
	id := e.ID
	out := apiProtectOut{OK: e.OK, Dst: dst, Levels: e.Levels, Failed: e.Failed, Still: e.Still, Error: e.Err, Entry: &id}
	result := "ok"
	if !e.OK {
		result = "failed"
	}
	a.apiEvent(cl.Actor, why, result, note, nil)
	apiJSON(w, out)
}
