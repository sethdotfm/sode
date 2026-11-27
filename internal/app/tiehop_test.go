package app

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// plainB is a route on router B only, from B.src to B.dst on level 1.
func plainB(src, dst int) string {
	return fmt.Sprintf(`{"src":{"router":"b2b2b2b2","n":%d},"dst":{"router":"b2b2b2b2","n":%d},"levels":[1]}`, src, dst)
}

// withRoute adds a route to a POST /takes body that tieRoutes made.
func withRoute(body, route string) string {
	return strings.TrimSuffix(body, "]}") + "," + route + "]}"
}

// A Protect on the upstream destination of a hop refuses the route before Sode sends anything. No hop of that route
// goes, and each of its steps has a failed entry. The other routes of the take go.
func TestAPITieTakeRefusesProtectedHop(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	a.mu.Lock()
	a.first.router.protect[10] = map[int]protHold{2: {dev: 99, kind: swp08.KindProtect}}
	a.mu.Unlock()
	log := connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, withRoute(tieRoutes(uidA, [2]int{4, 2}), plainB(6, 30)), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK || len(b.Results) != 2 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := log(); !slices.Equal(got, []string{"B 6>30"}) {
		t.Errorf("connects %v, want only the route on B", got)
	}
	r := b.Results[0]
	if r.OK || r.Result != "failed" || !strings.Contains(r.Error, "TIE 3 is protected on levels 2") || r.Hops[0].Result != "failed" {
		t.Errorf("tie route %+v", r)
	}
	if p := b.Results[1]; !p.OK || p.Result != "connected" {
		t.Errorf("plain route %+v", p)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 3 || es[0].GroupN != 3 {
		t.Fatalf("entries %+v", es)
	}
	for _, e := range es {
		if e.Dst == 30 {
			continue
		}
		if e.OK || len(e.Unsent) == 0 || !strings.Contains(e.Err, "not sent: TIE 3 is protected") {
			t.Errorf("entry %+v", e)
		}
	}
}

// A Router lock on the destination asked for refuses its route, so its hop does not go either. A route that shares
// that hop still sends it.
func TestAPITieTakeRefusesRouterLockOnLastHop(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	levelsA := a.r().routerOf(uidA).Levels
	ub, _ := a.unit("b2b2b2b2")
	a.mu.Lock()
	ub.router.protect[2] = map[int]protHold{1: {dev: 9, kind: swp08.KindRouterLock}}
	a.mu.Unlock()
	log := connectLog(fa, fb)

	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	if r := b.Results[0]; r.Result != "failed" || !strings.Contains(r.Error, "the destination is protected on levels 1") {
		t.Errorf("result %+v", r)
	}

	rec = apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}, [2]int{4, 21}), nil)
	b = tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK || len(b.Results) != 2 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if len(got) != levelsA+1 || got[levelsA] != "B 5>21" || slices.Contains(got, "B 5>2") {
		t.Errorf("connects %v, want the shared hop and B 5>21", got)
	}
	if r0, r1 := b.Results[0], b.Results[1]; r0.Result != "failed" || r1.Result != "connected" || r0.Hops[0].Result != "connected" {
		t.Errorf("results %+v and %+v", r0, r1)
	}
}

// In fireMode native, the steps of a tie take are ordinary connects: two routes on B in one wave stage nothing.
func TestAPITieTakeNativeSendsConnects(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) { c.Routers[0].FireMode = "native" })
	uidA := a.r().cfg.Routers[0].UID
	levelsA := a.r().routerOf(uidA).Levels
	log := connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}, [2]int{4, 21}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if len(got) != levelsA+2 || slices.ContainsFunc(got[:levelsA], func(s string) bool { return s != "A 4>10" }) {
		t.Fatalf("connects %v", got)
	}
	if tail := got[levelsA:]; !slices.Contains(tail, "B 5>2") || !slices.Contains(tail, "B 5>21") {
		t.Errorf("connects on B %v", tail)
	}
	if s := fa.salvoSeq() + fb.salvoSeq(); s != "" {
		t.Errorf("salvo messages %q", s)
	}
}

// The Result of a tie route comes from all its hops. A last hop that is Already after a hop that changed is a normal
// success. When no hop changed, the route is Already.
func TestAPITieTakeResultAcrossHops(t *testing.T) {
	a, _, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	fb.seed(2, 5, a.r().byUI[1].Wire) // B.2 already takes FROM A
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	r := b.Results[0]
	if r.Result != "connected" || !r.OK || len(r.Already) != 0 || r.Hops[0].Result != "connected" {
		t.Errorf("result %+v", r)
	}
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || !slices.Equal(es[0].Already, []int{1}) {
		t.Errorf("destination entry %+v", es)
	}

	// Now TIE 3 carries CAM 4 and B.2 has it: nothing changes on any hop.
	body := strings.Replace(tieRoutes(uidA, [2]int{4, 2}), `"n":3}`, `"n":4}`, 1)
	rec = apiPost(a, "/api/v1/takes", key, body, nil)
	b = tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if r := b.Results[0]; r.Result != "already" || !slices.Equal(r.Already, []int{1}) || r.Hops[0].Result != "already" {
		t.Errorf("second result %+v", r)
	}
}
