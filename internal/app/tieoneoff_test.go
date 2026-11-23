package app

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/tieplan"
)

// oneOffRoute is a POST /takes body: A.src to B.dst on level 1 through the One-off tie line A.11 to B.6. expect is
// the JSON of the expect field, or "" for none.
func oneOffRoute(uidA string, src, dst int, expect string) string {
	ex := ""
	if expect != "" {
		ex = `,"expect":` + expect
	}
	return fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":"b2b2b2b2","n":%d},"levels":[1],"ties":[{"from":{"router":%q,"n":11},"to":{"router":"b2b2b2b2","n":6}%s}]}]}`,
		uidA, src, dst, uidA, ex)
}

// A take through a One-off tie line sends the hop on A, then B. The caller needs no Rights on A.11, the stated
// upstream destination. The entry keeps both stated ends and who stated them. Sode keeps the One-off tie line in
// memory, not in the settings, while B.6 is routed: Via names it. When B.6 is routed nowhere, Sode lets it go.
func TestAPIOneOffTieLineTake(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Dsts: &[]config.Ref{{Router: "b2b2b2b2", N: 2}}}
	})
	uidA := a.r().cfg.UID
	levelsA := a.r().routerOf(uidA).Levels
	log := connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, oneOffRoute(uidA, 4, 2, ""), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if len(got) != levelsA+1 || got[0] != "A 4>11" || got[len(got)-1] != "B 6>2" {
		t.Fatalf("connects %v, want A 4>11 on each level, then B 6>2", got)
	}
	r := tieTakeAnswer(t, rec.Body).Results[0]
	if r.Result != "connected" || len(r.Hops) != 1 {
		t.Fatalf("answer %s", rec.Body)
	}
	h := r.Hops[0]
	if !isOneOff(h.Tie) || h.TieName != "One-off A.12" || h.OneOff == nil {
		t.Fatalf("hop %+v", h)
	}
	if h.OneOff.From.Router != uidA || h.OneOff.From.N != 11 || h.OneOff.To != (TieEnd{Router: "b2b2b2b2", N: 6}) || h.OneOff.By == "" {
		t.Errorf("one-off %+v", *h.OneOff)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es[0].Hops) != 1 || es[0].Hops[0].OneOff == nil {
		t.Errorf("entry hops %+v", es[0].Hops)
	}
	if n := len(a.r().cfg.TieLines); n != 1 {
		t.Errorf("%d saved tie lines, want 1", n)
	}
	if strings.Contains(apiDo(a, "GET", "/api/v1/tielines", key, nil).Body.String(), "oneoff-") {
		t.Error("GET /tielines lists the One-off tie line")
	}

	a.mu.Lock()
	x := a.newViaTrace(a.r())
	row := apiRouteRow{Dst: 2, Level: 1, Src: 6}
	x.fill(a.viewOf(config.Rights{}).on("b2b2b2b2"), &row)
	live := len(a.liveOneOffs(a.r()))
	a.mu.Unlock()
	if len(row.Via) != 1 || !row.Via[0].OneOff || row.Via[0].Name != "One-off A.12" || row.Origin == nil || row.Origin.N != 4 {
		t.Errorf("via %+v origin %+v", row.Via, row.Origin)
	}
	if live != 1 {
		t.Fatalf("%d One-off tie lines kept, want 1", live)
	}
	if l := a.oneOffList(1); len(l.OneOffs) != 1 || l.OneOffs[0].To.N != 6 || l.OneOffs[0].By == "" {
		t.Errorf("list %+v", l)
	}

	fb.seed(2, 7, a.r().byUI[1].Wire)
	a.mu.Lock()
	live = len(a.liveOneOffs(a.r()))
	a.mu.Unlock()
	if live != 0 {
		t.Errorf("%d One-off tie lines kept after B.6 is routed nowhere, want 0", live)
	}
}

// A plan with oneOffs gives a path through the One-off tie line. Its Affected destinations downstream are unknown,
// and the body gives its id and ends. Nothing is sent, and Sode keeps nothing.
func TestAPIOneOffTieLinePlan(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"oneOffs":[{"from":{"router":%q,"n":11},"to":{"router":"b2b2b2b2","n":6}}]}]}`, uidA, uidA)
	rec := apiPost(a, "/api/v1/takes", key, body, nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	b := tieBody(t, rec.Body)
	ro := b.Routes[0]
	if len(ro.OneOffs) != 1 || ro.OneOffs[0].From != (apiTieEnd{uidA, 11}) || ro.OneOffs[0].To != (apiTieEnd{"b2b2b2b2", 6}) {
		t.Fatalf("oneOffs %+v", ro.OneOffs)
	}
	id := ro.OneOffs[0].ID
	i := slices.IndexFunc(ro.Paths, func(p apiTiePath) bool { return p.Hops[0].Tie.ID == id })
	if i < 0 || len(ro.Paths) != 2 {
		t.Fatalf("paths %+v", ro.Paths)
	}
	p := ro.Paths[i]
	if !p.Hops[0].Tie.OneOff || len(p.Unknown) != 1 || p.Unknown[0].Why != "one_off" {
		t.Errorf("path %+v", p)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if n := len(a.liveOneOffs(a.r())); n != 0 {
		t.Errorf("%d One-off tie lines kept after a plan, want 0", n)
	}
}

// The Protect check covers the stated upstream destination: the route is refused and nothing goes to B.
func TestAPIOneOffTieLineProtected(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	a.mu.Lock()
	a.first.router.protect[11] = map[int]protHold{1: {5, 1}}
	a.mu.Unlock()
	log := connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, oneOffRoute(uidA, 4, 2, ""), nil)
	r := tieTakeAnswer(t, rec.Body).Results[0]
	if rec.Code != http.StatusOK || r.OK || !strings.Contains(r.Error, "protected") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v, want none", got)
	}
}

// Stated ends of a saved tie line are that tie line. An expect on a One-off tie line is compared as for a saved one.
func TestAPIOneOffTieLineEnds(t *testing.T) {
	a, _, _, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"from":{"router":%q,"n":10},"to":{"router":"b2b2b2b2","n":5},"expect":{"router":%q,"n":3}}]}]}`, uidA, uidA, uidA)
	rec := apiPost(a, "/api/v1/takes", key, body, nil)
	if r := tieTakeAnswer(t, rec.Body).Results[0]; rec.Code != http.StatusOK || r.Hops[0].Tie != "t1" || r.Hops[0].OneOff != nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	wantErr(t, apiPost(a, "/api/v1/takes", key, oneOffRoute(uidA, 4, 21, `{"router":"`+uidA+`","n":3}`), nil), http.StatusConflict, "tie_line_changed")
	for _, bad := range []string{
		fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"ties":[{"id":"t1","from":{"router":%q,"n":11},"to":{"router":"b2b2b2b2","n":6}}]}]}`, uidA, uidA),
		fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"ties":[{"from":{"router":%q,"n":11}}]}]}`, uidA, uidA),
		fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"ties":[{"from":{"router":%q,"n":11},"to":{"router":%q,"n":6}}]}]}`, uidA, uidA, uidA),
	} {
		wantErr(t, apiPost(a, "/api/v1/takes", key, bad, nil), http.StatusBadRequest, "bad_request")
	}
}

// Redo of a take through a One-off tie line offers it again, from the ends in History.
func TestAPIOneOffTieLineRedo(t *testing.T) {
	a, _, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	rec := apiPost(a, "/api/v1/takes", key, oneOffRoute(uidA, 4, 2, ""), nil)
	r := tieTakeAnswer(t, rec.Body).Results[0]
	if rec.Code != http.StatusOK || r.Entry == nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	// B.6 is routed nowhere now, so Sode lets the One-off tie line go. Redo still offers it.
	fb.seed(2, 7, a.r().byUI[1].Wire)
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/redo", *r.Entry), key, "", nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	b := tieBody(t, rec.Body)
	if len(b.Routes[0].Paths) != 2 || len(b.Routes[0].OneOffs) != 1 || b.Routes[0].OneOffs[0].ID != r.Hops[0].Tie {
		t.Errorf("redo routes %+v", b.Routes)
	}
	a.mu.Lock()
	in := a.tieInput(a.r())
	a.mu.Unlock()
	if slices.ContainsFunc(in.Ties, func(x tieplan.Tie) bool { return x.OneOff }) {
		t.Error("the plan of a Redo keeps the One-off tie line")
	}
}
