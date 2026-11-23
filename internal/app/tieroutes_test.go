package app

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// tieApp: routers A and B, each with a fake router, and the tie line TIE 3 from A.10 to B.5. A.10 carries A.3 now,
// and B.20 and B.21 show what TIE 3 carries.
func tieApp(t *testing.T, mut func(*config.Config)) (a *App, fa, fb *fakeRouter, key string) {
	t.Helper()
	a, fa, key = takerApp(t, func(c *config.Config) {
		r := moreRouter("B", "Second", "10.0.0.9:8910", 1)
		r.UID = "b2b2b2b2"
		c.MoreRouters = []config.Router{r}
		c.TieLines = []config.TieLine{{ID: "t1", Name: "TIE 3", From: config.Ref{Router: c.UID, N: 10}, To: config.Ref{Router: r.UID, N: 5}}}
		if mut != nil {
			mut(c)
		}
	})
	var b *routerUnit
	fb, b = addFakeUnit(t, a, a.r().cfg.MoreRouters[0])
	a.onUnitState(a.first, swp08.StateUp, nil)
	a.onUnitState(b, swp08.StateUp, nil)
	wire := a.r().byUI[1].Wire
	// TIE 3 carries A.3 on every level of A, so it is not mixed.
	for l := 1; l <= a.r().routerOf(a.r().cfg.UID).Levels; l++ {
		fa.seed(10, 3, a.r().byUI[l].Wire)
	}
	fb.seed(20, 5, wire)
	fb.seed(21, 5, wire)
	fb.seed(2, 7, wire)
	a.mu.Lock()
	a.first.router.srcNames[4], a.first.router.dstNames[10] = "CAM 4", "TO B"
	b.router.srcNames[5], b.router.dstNames[2], b.router.dstNames[20], b.router.dstNames[21] = "FROM A", "B MON 2", "B MON 20", "B MON 21"
	a.mu.Unlock()
	return a, fa, fb, key
}

type tieBodyIn struct {
	Error  apiErr           `json:"error"`
	Routes []apiTieRouteOut `json:"routes"`
}

func tieBody(t *testing.T, rec interface{ Bytes() []byte }) tieBodyIn {
	t.Helper()
	var b tieBodyIn
	if err := json.Unmarshal(rec.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// A route across routers with no ties gets 409 tie_line_needed with the ranked paths, and nothing goes to a router.
func TestAPITakeAcrossRoutersNeedsTieLine(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	sentA, sentB := fa.sends(), fb.sends()
	rec := apiPost(a, "/api/v1/takes", key, ends(a.r().cfg.UID, "b2b2b2b2", 4, 2), nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	b := tieBody(t, rec.Body)
	if len(b.Routes) != 1 || len(b.Routes[0].Paths) != 1 {
		t.Fatalf("routes %+v", b.Routes)
	}
	r := b.Routes[0]
	p := r.Paths[0]
	if r.Src.Name != "CAM 4" || r.Dst.Name != "B MON 2" || len(p.Hops) != 2 {
		t.Fatalf("route %+v", r)
	}
	up, last := p.Hops[0], p.Hops[1]
	if up.Tie == nil || up.Tie.Name != "TIE 3" || up.Src.N != 4 || up.Dst.N != 10 || up.Dst.Name != "TO B" || !up.Sends || up.Now == nil || up.Now.Origin.N != 3 {
		t.Errorf("first hop %+v", up)
	}
	if last.Tie != nil || last.Src.N != 5 || last.Dst.N != 2 || last.Src.Name != "FROM A" {
		t.Errorf("last hop %+v", last)
	}
	// Two Affected destinations plus one hop at the default weight.
	if p.Cost != 3 || len(p.Affected) != 2 || p.Affected[0].Name != "B MON 20" || p.Others != 0 {
		t.Errorf("cost %d, affected %+v", p.Cost, p.Affected)
	}
	if fa.sends() != sentA || fb.sends() != sentB {
		t.Errorf("a refused take sent %d and %d messages", fa.sends()-sentA, fb.sends()-sentB)
	}
	if es, _ := a.hist.Page(-1, 1); len(es) != 0 {
		t.Errorf("entries %+v", es)
	}
}

// The hop weight is a setting.
func TestAPITakeAcrossRoutersHopWeight(t *testing.T) {
	a, _, _, key := tieApp(t, func(c *config.Config) { c.TieHopWeight = 10 })
	rec := apiPost(a, "/api/v1/takes", key, ends(a.r().cfg.UID, "b2b2b2b2", 4, 2), nil)
	if b := tieBody(t, rec.Body); len(b.Routes) != 1 || b.Routes[0].Paths[0].Cost != 12 {
		t.Errorf("%d %s", rec.Code, rec.Body)
	}
}

// A router down on the only path, or no tie line at all, gives 409 no_path with each reason.
func TestAPITakeAcrossRoutersNoPath(t *testing.T) {
	a, _, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	fb.drop(errors.New("link lost"))
	rec := apiPost(a, "/api/v1/takes", key, ends(uidA, "b2b2b2b2", 4, 2), nil)
	wantErr(t, rec, http.StatusConflict, "no_path")
	if b := tieBody(t, rec.Body); len(b.Routes) != 1 || len(b.Routes[0].Paths) != 0 || len(b.Routes[0].Reasons) != 1 ||
		b.Routes[0].Reasons[0].Why != "router_down" || b.Routes[0].Reasons[0].Router != "b2b2b2b2" {
		t.Errorf("body %s", rec.Body)
	}

	// TIE 3 goes from A to B only.
	rec = apiPost(a, "/api/v1/takes", key, ends("b2b2b2b2", uidA, 5, 2), nil)
	wantErr(t, rec, http.StatusConflict, "no_path")
	if b := tieBody(t, rec.Body); len(b.Routes[0].Reasons) != 1 || b.Routes[0].Reasons[0].Why != "no_tie_line" {
		t.Errorf("body %s", rec.Body)
	}
}

// Hidden visibility blanks the names that the caller cannot see, and counts the Affected destinations it cannot see.
func TestAPITakeAcrossRoutersHidden(t *testing.T) {
	a, _, _, key := tieApp(t, func(c *config.Config) {
		// Rights name routers (a blank router becomes the first one), so the Dsts are on router B.
		c.APIClients[0].Rights = config.Rights{Visibility: config.VisHidden,
			Sources: &[]config.Ref{{Router: c.UID, N: 4}},
			Dsts:    &[]config.Ref{{Router: "b2b2b2b2", N: 2}, {Router: "b2b2b2b2", N: 20}}}
	})
	a.tieChanges.set("t1", time.Now().Add(-time.Hour), false)
	for _, code := range []string{"tie_line_needed", "no_path"} {
		if code == "no_path" {
			a.mu.Lock()
			a.first.router.protect[10] = map[int]protHold{1: {dev: 99, kind: swp08.KindProtect}}
			a.mu.Unlock()
		}
		rec := apiPost(a, "/api/v1/takes", key, ends(a.r().cfg.UID, "b2b2b2b2", 4, 2), nil)
		wantErr(t, rec, http.StatusConflict, code)
		b := tieBody(t, rec.Body)
		p := b.Routes[0].Paths[0]
		if b.Routes[0].Src.Name != "CAM 4" || b.Routes[0].Dst.Name != "B MON 2" {
			t.Errorf("%s: typed ends %+v", code, b.Routes[0])
		}
		if p.Hops[0].Tie.Name != "TIE 3" || p.Hops[0].Src.N != 4 || p.Hops[1].Dst.N != 2 {
			t.Errorf("%s: hops %+v", code, p.Hops)
		}
		// As GET /tielines: null for an end that the caller cannot see, and no change time of a hidden upstream end.
		path := apiBody(t, rec)["routes"].([]any)[0].(map[string]any)["paths"].([]any)[0].(map[string]any)
		if path["newest"] != nil {
			t.Errorf("%s: newest %v gives the change time of a hidden end", code, path["newest"])
		}
		hops := path["hops"].([]any)
		h0, h1 := hops[0].(map[string]any), hops[1].(map[string]any)
		if h0["dst"] != nil || h1["src"] != nil || h0["changed"] != nil || h0["before"] != nil || h0["now"] == nil {
			t.Errorf("%s: hidden hop ends %v %v", code, h0, h1)
		}
		if len(p.Affected) != 1 || p.Affected[0].N != 20 || p.Others != 1 {
			t.Errorf("%s: affected %+v, others %d", code, p.Affected, p.Others)
		}
	}
	a.mu.Lock()
	delete(a.first.router.protect, 10)
	a.mu.Unlock()
	// tie_line_changed hides the same ends. The origin that the 409 body gives still goes back as expect.
	take := func(src int) string {
		return fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":%d}}]}]}`,
			a.r().cfg.UID, a.r().cfg.UID, src)
	}
	rec := apiPost(a, "/api/v1/takes", key, take(9), nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_changed")
	hops := apiBody(t, rec)["routes"].([]any)[0].(map[string]any)["paths"].([]any)[0].(map[string]any)["hops"].([]any)
	h0 := hops[0].(map[string]any)
	if h0["dst"] != nil || hops[1].(map[string]any)["src"] != nil || h0["changed"] != nil {
		t.Errorf("tie_line_changed: hidden hop ends %v", hops)
	}
	o := h0["now"].(map[string]any)["origin"].(map[string]any)
	if rec := apiPost(a, "/api/v1/takes", key, take(int(o["n"].(float64))), nil); rec.Code != http.StatusOK {
		t.Errorf("take with the expect of the 409 body: %d %s", rec.Code, rec.Body)
	}
}

// Rights cover the typed ends before Sode lists any path.
// The typed levels of a route across routers are levels of the destination's router, as for a route on one router.
// On B, with one level, levels 1-17 are level 1, which the Rights allow.
func TestAPITakeAcrossRoutersTypedLevels(t *testing.T) {
	a, _, _, key := tieApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Levels: config.RouterLevels{"b2b2b2b2": {1}}}
	})
	all := `[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17]`
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":%s}]}`, a.r().cfg.UID, all)
	if rec := apiPost(a, "/api/v1/takes", key, body, nil); rec.Code != http.StatusConflict || apiErrCode(t, rec) != "tie_line_needed" {
		t.Fatalf("levels 1-17 to B: %d %s", rec.Code, rec.Body.String())
	}
	// A Rights limit on B names B's level, in B's numbering.
	a, _, _, key = tieApp(t, func(c *config.Config) {
		c.MoreRouters[0].Levels, c.MoreRouters[0].LevelNumbering = 3, "0"
		c.APIClients[0].Rights = config.Rights{Levels: config.RouterLevels{"b2b2b2b2": {1}}}
	})
	body = fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1,2]}]}`, a.r().cfg.UID)
	rec := apiPost(a, "/api/v1/takes", key, body, nil)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "level 1 is outside") {
		t.Fatalf("level 2 of B: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAPITakeAcrossRoutersOutsideRights(t *testing.T) {
	a, _, _, key := tieApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = config.Rights{Dsts: nums(3)}
	})
	wantErr(t, apiPost(a, "/api/v1/takes", key, ends(a.r().cfg.UID, "b2b2b2b2", 4, 2), nil), http.StatusForbidden, "outside_rights")
}
