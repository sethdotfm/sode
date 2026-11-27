package app

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// presetTieApp: tieApp with Everyone's preset Show. It sends A.4 to B.2 through TIE 3 (t1) and A.4 to A.1.
func presetTieApp(t *testing.T) (*App, *fakeRouter, *fakeRouter, string) {
	t.Helper()
	a, fa, fb, key := tieApp(t, func(c *config.Config) {
		c.Presets = []config.Preset{{ID: "show", Name: "Show", Items: []config.PresetItem{
			{Src: config.Ref{Router: c.Routers[0].UID, N: 4}, Dsts: []config.Ref{{Router: "b2b2b2b2", N: 2}}, Levels: []int{1}, Ties: []string{"t1"}},
			{Src: config.Ref{Router: c.Routers[0].UID, N: 4}, Dsts: []config.Ref{{Router: c.Routers[0].UID, N: 1}}, Levels: []int{1}},
		}}}
	})
	a.mu.Lock()
	a.firstUnit().router.dstNames[1] = "MON 1"
	a.mu.Unlock()
	return a, fa, fb, key
}

// A preset fire with tie lines answers 409 tie_confirm_needed with the state of each stored tie line, and sends
// nothing. A request that names each tie line with its expected source goes through as one take.
func TestAPIPresetTieConfirm(t *testing.T) {
	a, fa, fb, key := presetTieApp(t)
	uidA := a.r().cfg.Routers[0].UID
	sentA, sentB := fa.sends(), fb.sends()
	rec := apiPost(a, "/api/v1/presets/everyone/Show", key, "", nil)
	wantErr(t, rec, http.StatusConflict, "tie_confirm_needed")
	b := tieBody(t, rec.Body)
	if len(b.Routes) != 1 || len(b.Routes[0].Paths) != 1 {
		t.Fatalf("routes %s", rec.Body)
	}
	h := b.Routes[0].Paths[0].Hops[0]
	if h.Tie == nil || h.Tie.ID != "t1" || h.Tie.Name != "TIE 3" || !h.Sends || h.Now == nil || h.Now.Origin == nil || h.Now.Origin.N != 3 {
		t.Fatalf("hop %+v", h)
	}
	if fa.sends() != sentA || fb.sends() != sentB {
		t.Fatal("the confirm sent something")
	}

	// A wrong expect is the stale check's 409 tie_line_changed.
	rec = apiPost(a, "/api/v1/presets/everyone/Show", key, fmt.Sprintf(`{"ties":[{"id":"t1","expect":{"router":%q,"n":9}}]}`, uidA), nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_changed")
	// A tie line that the preset does not store is refused.
	rec = apiPost(a, "/api/v1/presets/everyone/Show", key, `{"ties":[{"id":"t9","expect":null}]}`, nil)
	wantErr(t, rec, http.StatusBadRequest, "bad_request")

	log := connectLog(fa, fb)
	rec = apiPost(a, "/api/v1/presets/everyone/Show", key, fmt.Sprintf(`{"ties":[{"id":"t1","expect":{"router":%q,"n":3}}]}`, uidA), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if !slices.Contains(got, "B 5>2") || !slices.Contains(got, "A 4>10") || !slices.Contains(got, "A 4>1") {
		t.Fatalf("connects %v", got)
	}
	ans := tieTakeAnswer(t, rec.Body)
	if !ans.OK || len(ans.Results) != 2 {
		t.Fatalf("answer %s", rec.Body)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 3 || slices.ContainsFunc(es, func(e Entry) bool { return e.Preset != "Show" }) {
		t.Fatalf("entries %+v", es)
	}
}

// A deleted tie line makes the preset Broken with the tie kind. A fire leaves that route out and names it, and the
// other routes go with no confirm.
func TestPresetDeletedTieLineIsBroken(t *testing.T) {
	a, _, _, key := presetTieApp(t)
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error { c.TieLines = nil; return nil }, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	p := a.r().cfg.Presets[0]
	if len(p.Broken) != 1 || p.Broken[0].Kind != config.MissingTie || p.Broken[0].Tie != "t1" {
		t.Fatalf("broken %+v", p.Broken)
	}
	rec := apiPost(a, "/api/v1/presets/everyone/Show", key, "", nil)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "tie line that Sode no longer has") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
}
