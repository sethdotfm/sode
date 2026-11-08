package app

import (
	"log"
	"net/http"
	"slices"
	"strconv"

	"github.com/sethdotfm/sode/internal/config"
)

// Paged reads: History, and the Communication Log (which ignores visibility: it shows raw
// router messages, and the log scope is the gate).

func init() {
	registerAPI(
		apiRoute{"GET", apiBase + "/history", config.ScopeHistory, (*App).apiHistory},
		apiRoute{"GET", apiBase + "/history/groups/last", config.ScopeHistory, (*App).apiLastGroup},
		apiRoute{"GET", apiBase + "/log", config.ScopeLog, (*App).apiLog},
	)
}

const apiHistoryDefault, apiLogDefault = 50, 100

// pageParams reads before (default -1, the newest page) and n (default def, at most limit).
func pageParams(r *http.Request, def, limit int) (before int64, n int, err error) {
	before, n = -1, def
	q := r.URL.Query()
	if s := q.Get("before"); s != "" {
		if before, err = strconv.ParseInt(s, 10, 64); err != nil {
			return 0, 0, err
		}
	}
	if s := q.Get("n"); s != "" {
		if n, err = strconv.Atoi(s); err != nil || n < 1 {
			return 0, 0, strconv.ErrSyntax
		}
	}
	return before, min(n, limit), nil
}

const badPage = "before and n must be whole numbers, n at least 1"

func (a *App) apiHistory(w http.ResponseWriter, r *http.Request, cl *apiCtx) {
	before, n, err := pageParams(r, apiHistoryDefault, historyMax)
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", badPage)
		return
	}
	page, err := a.hist.Page(int(before), n)
	if err != nil {
		log.Printf("api history: %v", err)
	}
	v := cl.view()
	out := make([]Entry, 0, len(page))
	for _, e := range page {
		if e, ok := v.Entry(e); ok {
			out = append(out, e)
		}
	}
	apiJSON(w, map[string]any{"before": before, "entries": out, "more": morePages(page)})
}

// apiLastGroup returns the newest take Group the caller can see, entries in take order
// (oldest id first). Externals are not takes, so their groups don't count. A hidden caller
// gets only the visible routes, and a Group with none is skipped for an older one.
func (a *App) apiLastGroup(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	v := cl.view()
	before := -1
	for {
		page, err := a.hist.Page(before, apiHistoryDefault)
		if err != nil {
			log.Printf("api last group: %v", err)
		}
		var id string
		var out []Entry
		for _, e := range page { // newest first
			if e.Ext || e.Group == "" || (id != "" && e.Group != id) {
				continue
			}
			if e, ok := v.Entry(e); ok {
				id = e.Group
				out = append(out, e)
			}
		}
		if id != "" {
			slices.Reverse(out)
			apiJSON(w, map[string]any{"group": id, "entries": out})
			return
		}
		if !morePages(page) {
			apiFail(w, http.StatusNotFound, "not_found", "History has no Group")
			return
		}
		before = page[len(page)-1].ID
	}
}

func (a *App) apiLog(w http.ResponseWriter, r *http.Request, _ *apiCtx) {
	before, n, err := pageParams(r, apiLogDefault, sentPage)
	if err != nil {
		apiFail(w, http.StatusBadRequest, "bad_request", badPage)
		return
	}
	page, more, err := a.sent.Page(before, n)
	if err != nil {
		log.Printf("api log: %v", err)
	}
	if page == nil {
		page = []Sent{}
	}
	apiJSON(w, map[string]any{"before": before, "entries": page, "more": more, "total": a.sent.Total()})
}
