package app

import (
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// New makes a unit and a Router link for each router in the settings, each with its own address.
func TestNewLinksEveryRouter(t *testing.T) {
	a := newTestApp(t, func(c *config.Config) {
		c.MoreRouters = []config.Router{moreRouter("B", "Second", "10.0.0.9:8910", 3)}
	})
	b := a.r().cfg.MoreRouters[0]
	u, ok := a.unit(b.UID)
	if len(a.routers) != 2 || !ok || u == a.first {
		t.Fatalf("units %v", a.routers)
	}
	if u.sw == nil || a.first.sw == nil || u.status.Addr != "10.0.0.9:8910" || a.first.status.Addr == u.status.Addr {
		t.Errorf("second link %v at %q, first at %q", u.sw, u.status.Addr, a.first.status.Addr)
	}
	if got := a.unitsInOrder(a.r()); len(got) != 2 || got[0] != a.first || got[1] != u {
		t.Errorf("units in order %v", got)
	}
}

// apiTwoRouters: an API client with the take scope, and two routers, A and B, each with a fake router.
func apiTwoRouters(t *testing.T) (a *App, fa, fb *fakeRouter, b *routerUnit, key string) {
	t.Helper()
	a, fa, key = takerApp(t, func(c *config.Config) {
		c.MoreRouters = []config.Router{moreRouter("B", "Second", "10.0.0.9:8910", 3)}
	})
	fb, b = addFakeUnit(t, a, a.r().cfg.MoreRouters[0])
	fb.seed(2, 7, a.r().byUI[1].Wire)
	a.mu.Lock()
	b.router.srcNames[1], b.router.dstNames[2] = "B CAM 1", "B MON 2"
	a.mu.Unlock()
	return a, fa, fb, b, key
}

func ends(src, dst string, s, d int) string {
	return fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":%q,"n":%d},"levels":[1]}]}`, src, s, dst, d)
}

// POST /takes with router references at both ends patches that router only.
func TestAPITakeOnEachRouter(t *testing.T) {
	a, fa, fb, b, key := apiTwoRouters(t)
	uidA := a.r().cfg.UID
	wire := a.r().byUI[1].Wire

	rec := apiPost(a, "/api/v1/takes", key, ends(b.uid, b.uid, 4, 2), nil)
	if rec.Code != http.StatusOK || apiBody(t, rec)["ok"] != true {
		t.Fatalf("take on B: %d %s", rec.Code, rec.Body)
	}
	if n := fa.sends(); n != 0 {
		t.Errorf("router A got %d messages for a take on B", n)
	}
	fb.mu.Lock()
	got := fb.tally[[2]int{2, wire}]
	fb.mu.Unlock()
	if got != 4 {
		t.Errorf("router B dst 2 = %d, want 4", got)
	}
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || es[0].Router != b.uid {
		t.Fatalf("entry %+v", es)
	}

	sentB := fb.sends()
	rec = apiPost(a, "/api/v1/takes", key, ends(uidA, uidA, 5, 4), nil)
	if rec.Code != http.StatusOK || apiBody(t, rec)["ok"] != true {
		t.Fatalf("take on A: %d %s", rec.Code, rec.Body)
	}
	if fa.sends() == 0 || fb.sends() != sentB {
		t.Errorf("take on A: A got %d, B got %d more", fa.sends(), fb.sends()-sentB)
	}

	// Undo of the take on B goes to B, though the request names no router.
	es, _ := a.hist.Page(-1, 2)
	var onB Entry
	for _, e := range es {
		if e.Router == b.uid {
			onB = e
		}
	}
	sentA := fa.sends()
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/undo", onB.ID), key, "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("undo on B: %d %s", rec.Code, rec.Body)
	}
	if fa.sends() != sentA {
		t.Errorf("undo of a take on B sent %d messages to A", fa.sends()-sentA)
	}
	fb.mu.Lock()
	got = fb.tally[[2]int{2, wire}]
	fb.mu.Unlock()
	if es, _ := a.hist.Page(-1, 1); len(es) != 1 || es[0].Router != b.uid || es[0].Undo == nil || got != 7 {
		t.Errorf("undo: B dst 2 = %d, entry %+v", got, es)
	}
}

// With no tie line, a route with its ends on two routers has no path. With two routers, a bare
// number does not say which router it means.
func TestAPITakeEndsRefused(t *testing.T) {
	a, fa, fb, b, key := apiTwoRouters(t)
	uidA := a.r().cfg.UID
	for _, c := range []struct {
		name, body string
		status     int
		code       string
	}{
		{"ends on two routers", ends(uidA, b.uid, 4, 2), http.StatusConflict, "no_path"},
		{"bare numbers", `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, http.StatusBadRequest, "router_needed"},
		{"one bare end", fmt.Sprintf(`{"routes":[{"src":4,"dst":{"router":%q,"n":2}}]}`, b.uid), http.StatusBadRequest, "router_needed"},
		{"unknown router", ends("b0b0b0b0", "b0b0b0b0", 4, 2), http.StatusNotFound, "unknown_router"},
		{"no n", fmt.Sprintf(`{"routes":[{"src":{"router":%q},"dst":{"router":%q,"n":2}}]}`, b.uid, b.uid), http.StatusBadRequest, "bad_request"},
		{"a prefix", `{"routes":[{"src":{"router":"B","n":4},"dst":{"router":"B","n":2}}]}`, http.StatusNotFound, "unknown_router"},
	} {
		t.Run(c.name, func(t *testing.T) {
			wantErr(t, apiPost(a, "/api/v1/takes", key, c.body, nil), c.status, c.code)
		})
	}
	if fa.sends()+fb.sends() != 0 {
		t.Errorf("refused takes sent %d and %d messages", fa.sends(), fb.sends())
	}
	if es, _ := a.hist.Page(-1, 1); len(es) != 0 {
		t.Errorf("entries %+v", es)
	}
}

// With one router, a router reference and a bare number both still work.
func TestAPITakeOneRouterTakesEither(t *testing.T) {
	a, f, key := takerApp(t, nil)
	uid := a.r().cfg.UID
	for _, body := range []string{ends(uid, uid, 4, 2), `{"routes":[{"src":5,"dst":2,"levels":[1]}]}`} {
		if rec := apiPost(a, "/api/v1/takes", key, body, nil); rec.Code != http.StatusOK || apiBody(t, rec)["ok"] != true {
			t.Errorf("%s: %d %s", body, rec.Code, rec.Body)
		}
	}
	if f.sends() == 0 {
		t.Error("nothing sent")
	}
}

// With two routers, a read about one router needs the router parameter; GET /routers lists every router.
func TestAPIReadsNeedARouter(t *testing.T) {
	a, _, _, b, key := apiTwoRouters(t)
	for _, path := range []string{"/api/v1/names", "/api/v1/tally", "/api/v1/status", "/api/v1/stream?topics=tally"} {
		wantErr(t, apiDo(a, "GET", path, key, nil), http.StatusBadRequest, "router_needed")
	}
	rec := apiDo(a, "GET", "/api/v1/names?router="+b.uid, key, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("names of B: %d %s", rec.Code, rec.Body)
	}
	srcs, _ := apiBody(t, rec)["sources"].([]any)
	if len(srcs) != 1 || srcs[0].(map[string]any)["name"] != "B CAM 1" {
		t.Errorf("names of B %s", rec.Body)
	}
	if rec := apiDo(a, "GET", "/api/v1/tally?router="+a.r().cfg.UID, key, nil); rec.Code != http.StatusOK {
		t.Errorf("tally of A: %d %s", rec.Code, rec.Body)
	}
	rec = apiDo(a, "GET", "/api/v1/routers", key, nil)
	list, _ := apiBody(t, rec)["routers"].([]any)
	if len(list) != 2 || list[1].(map[string]any)["uid"] != b.uid || list[1].(map[string]any)["prefix"] != "B" {
		t.Errorf("routers %s", rec.Body)
	}
}

// A settings change to router B cuts the takes in flight on B only; a take held on A goes on.
func TestRouterChangeCutsOnlyItsTakes(t *testing.T) {
	a, fa, fb, b, key := apiTwoRouters(t)
	uidA := a.r().cfg.UID
	fa.hold, fb.hold = 300*time.Millisecond, 300*time.Millisecond
	var wg sync.WaitGroup
	recs := make([]map[string]any, 2)
	for i, body := range []string{ends(uidA, uidA, 4, 2), ends(b.uid, b.uid, 4, 2)} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			rec := apiPost(a, "/api/v1/takes", key, body, nil)
			if rec.Code == http.StatusOK {
				recs[i] = apiBody(t, rec)
			}
		}()
	}
	time.Sleep(150 * time.Millisecond) // both connects sent, both held
	cfg := a.r().cfg.Clone()
	cfg.MoreRouters[0].Address = "10.0.0.10:8910"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wg.Wait()
	result := func(m map[string]any) (bool, string) {
		rs, _ := m["results"].([]any)
		if len(rs) != 1 {
			return false, fmt.Sprint(m)
		}
		r := rs[0].(map[string]any)
		e, _ := r["error"].(string)
		return r["ok"] == true, e
	}
	if ok, e := result(recs[0]); !ok {
		t.Errorf("the take on A failed: %s", e)
	}
	if ok, e := result(recs[1]); ok || !strings.Contains(e, errRouterChanged.Error()) {
		t.Errorf("the take on B: ok %v, error %q", ok, e)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if b.status.Addr != "10.0.0.10:8910" || a.first.status.Addr == b.status.Addr {
		t.Errorf("status of B %q, of A %q", b.status.Addr, a.first.status.Addr)
	}
}
