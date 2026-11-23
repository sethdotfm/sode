package app

import (
	"net/http"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A destination on A and a source on B share a name, so Settings suggests a tie line. Until an Administrator accepts it,
// the suggestion is no tie line: the planner finds no path and nothing goes to either router.
func TestTakeNeverUsesUnacceptedSuggestion(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) { c.TieLines = nil })
	uidA := a.r().cfg.UID
	a.mu.Lock()
	a.first.router.dstNames[10], a.first.router.srcNames[4] = "Stage Mon", "CAM 4"
	a.mu.Unlock()
	log := connectLog(fa, fb)
	wantErr(t, apiPost(a, "/api/v1/takes", key, ends(uidA, "b2b2b2b2", 4, 2), nil), http.StatusConflict, "no_path")
	wantErr(t, apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil), http.StatusBadRequest, "bad_request")
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
}
