package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// connectLog notes each connect that the units of fa and fb send, in the order sent across both routers.
func connectLog(fa, fb *fakeRouter) func() []string {
	var mu sync.Mutex
	var out []string
	for _, f := range []*fakeRouter{fa, fb} {
		name, send := "A", f.u.sendMsg
		if f == fb {
			name = "B"
		}
		f.u.sendMsg = func(m []byte, why swp08.Why) error {
			if m[0] == swp08.CmdExtConnect {
				mu.Lock()
				out = append(out, fmt.Sprintf("%s %d>%d", name, int(m[5])<<8|int(m[6]), int(m[3])<<8|int(m[4])))
				mu.Unlock()
			}
			return send(m, why)
		}
	}
	return func() []string {
		mu.Lock()
		defer mu.Unlock()
		return slices.Clone(out)
	}
}

// tieRoutes is a POST /takes body: each route goes from A.src to B.dst on level 1 through TIE 3 (t1).
func tieRoutes(uidA string, srcDst ...[2]int) string {
	var rs []string
	for _, sd := range srcDst {
		rs = append(rs, fmt.Sprintf(`{"src":{"router":%q,"n":%d},"dst":{"router":"b2b2b2b2","n":%d},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":3}}]}`,
			uidA, sd[0], sd[1], uidA))
	}
	return `{"routes":[` + strings.Join(rs, ",") + `]}`
}

type tieTakeBody struct {
	OK      bool          `json:"ok"`
	Results []apiRouteOut `json:"results"`
}

func tieTakeAnswer(t *testing.T, rec interface{ Bytes() []byte }) tieTakeBody {
	t.Helper()
	var b tieTakeBody
	if err := json.Unmarshal(rec.Bytes(), &b); err != nil {
		t.Fatal(err)
	}
	return b
}

// A route with ties sends the hop on A first, on every level of A, then the typed levels on B. The take is one
// Group: the entry of the hop, and the entry of the destination asked for. That entry keeps both routers, the hop
// and the Via chain from before.
func TestAPITieTakeSendsUpstreamFirst(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	levelsA := a.r().routerOf(uidA).Levels
	log := connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if len(got) != levelsA+1 || got[len(got)-1] != "B 5>2" || slices.ContainsFunc(got[:levelsA], func(s string) bool { return s != "A 4>10" }) {
		t.Fatalf("connects %v, want %d of A 4>10, then B 5>2", got, levelsA)
	}
	b := tieTakeAnswer(t, rec.Body)
	r := b.Results[0]
	if !b.OK || r.Result != "connected" || r.Src != 4 || r.Dst != 2 || r.Entry == nil || len(r.Hops) != 1 {
		t.Fatalf("answer %s", rec.Body)
	}
	h := r.Hops[0]
	if h.Tie != "t1" || h.TieName != "TIE 3" || h.Router != uidA || h.Src != 4 || h.Dst != 10 || h.Result != "connected" || h.Entry == nil ||
		len(h.Levels) != levelsA || !slices.Contains(h.Prev, [2]int{1, 3}) {
		t.Errorf("hop %+v", h)
	}

	es, _ := a.hist.Page(-1, 5)
	if len(es) != 2 {
		t.Fatalf("entries %+v", es)
	}
	dst, hop := es[0], es[1] // newest first
	if hop.ID != *h.Entry || hop.Tie != "t1" || hop.Router != uidA || hop.Src != 4 || hop.Dst != 10 || !hop.OK {
		t.Errorf("hop entry %+v", hop)
	}
	if dst.ID != *r.Entry || dst.Router != "b2b2b2b2" || dst.Src != 5 || dst.Dst != 2 || dst.SrcName != "FROM A" || !dst.OK || dst.Tie != "" {
		t.Errorf("destination entry %+v", dst)
	}
	if dst.Origin == nil || *dst.Origin != (TieEnd{Router: uidA, N: 4, Name: "CAM 4"}) {
		t.Errorf("origin %+v", dst.Origin)
	}
	if len(dst.Hops) != 1 || dst.Hops[0].Entry == nil || *dst.Hops[0].Entry != hop.ID || !slices.Equal(dst.Hops[0].Prev, hop.Prev) {
		t.Errorf("hops %+v, hop prev %v", dst.Hops, hop.Prev)
	}
	// B.2 had B.7 on level 1, with no tie line into B.7.
	if len(dst.ViaPrev) != 1 || dst.ViaPrev[0].Level != 1 || dst.ViaPrev[0].Origin == nil || dst.ViaPrev[0].Origin.N != 7 || len(dst.ViaPrev[0].Via) != 0 {
		t.Errorf("via before %+v", dst.ViaPrev)
	}
	if hop.Group == "" || hop.Group != dst.Group || hop.GroupN != 2 || dst.GroupN != 2 {
		t.Errorf("groups %q/%d and %q/%d", hop.Group, hop.GroupN, dst.Group, dst.GroupN)
	}
}

// Two routes that share the hop on A send it one time, and both keep the same hop record.
func TestAPITieTakeSharesHop(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	levelsA := a.r().routerOf(uidA).Levels
	log := connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}, [2]int{4, 21}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK || len(b.Results) != 2 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if len(got) != levelsA+2 || slices.ContainsFunc(got[:levelsA], func(s string) bool { return s != "A 4>10" }) {
		t.Fatalf("connects %v", got)
	}
	if tail := got[levelsA:]; !slices.Contains(tail, "B 5>2") || !slices.Contains(tail, "B 5>21") {
		t.Errorf("connects on B %v", tail)
	}
	h0, h1 := b.Results[0].Hops[0], b.Results[1].Hops[0]
	if h0.Entry == nil || h1.Entry == nil || *h0.Entry != *h1.Entry {
		t.Errorf("hops %+v and %+v", h0, h1)
	}
	if es, _ := a.hist.Page(-1, 5); len(es) != 3 || es[0].GroupN != 3 || es[2].Group != es[0].Group {
		t.Errorf("entries %+v", es)
	}
}

// A tie line that already carries the source on every level sends nothing on A. Its hop is already, with no entry.
func TestAPITieTakeCarriedTieLine(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	for l := 1; l <= a.r().routerOf(uidA).Levels; l++ {
		fa.seed(10, 4, a.r().byUI[l].Wire)
	}
	log := connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := log(); !slices.Equal(got, []string{"B 5>2"}) {
		t.Errorf("connects %v", got)
	}
	if h := b.Results[0].Hops[0]; h.Result != "already" || h.Entry != nil {
		t.Errorf("hop %+v", h)
	}
	if es, _ := a.hist.Page(-1, 5); len(es) != 1 || es[0].Group != "" || len(es[0].Hops) != 1 {
		t.Errorf("entries %+v", es)
	}
}

// A hop that fails stops the route: B gets nothing, and the destination's entry fails as not sent.
func TestAPITieTakeStopsAtFailedHop(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	a.mu.Lock()
	a.first.router.protect[10] = map[int]protHold{1: {dev: 99, kind: swp08.KindProtect}}
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
	r := b.Results[0]
	if r.Result != "failed" || !strings.Contains(r.Error, "the hop to TIE 3 failed") || r.Hops[0].Result != "failed" {
		t.Errorf("result %+v", r)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 2 || es[0].OK || es[1].OK || !slices.Equal(es[0].Unsent, []int{1}) || es[0].GroupN != 2 {
		t.Errorf("entries %+v", es)
	}
}

// Ties that make no path are refused before anything goes to a router.
func TestAPITieTakeRefusesBadTies(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	route := func(src, dst string, ties string) string {
		return fmt.Sprintf(`{"src":{"router":%q,"n":4},"dst":{"router":%q,"n":2},"levels":[1],"ties":%s}`, src, dst, ties)
	}
	for _, c := range []struct {
		body   string
		status int
		code   string
	}{
		{`{"routes":[` + route(uidA, "b2b2b2b2", `[{"id":"nope"}]`) + `]}`, http.StatusBadRequest, "bad_request"},
		{`{"routes":[` + route(uidA, uidA, `[{"id":"t1"}]`) + `]}`, http.StatusBadRequest, "bad_request"},
		{`{"routes":[` + route(uidA, "b2b2b2b2", `[{"id":"t1","other":1}]`) + `]}`, http.StatusBadRequest, "bad_request"},
		{tieRoutes(uidA, [2]int{4, 2}, [2]int{6, 21}), http.StatusConflict, "conflict"},
	} {
		wantErr(t, apiPost(a, "/api/v1/takes", key, c.body, nil), c.status, c.code)
	}
	fb.drop(fmt.Errorf("link lost"))
	wantErr(t, apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil), http.StatusConflict, "no_path")
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	if es, _ := a.hist.Page(-1, 5); len(es) != 0 {
		t.Errorf("entries %+v", es)
	}
}

// Each hop follows its own router's levels. A has 2 levels and B has 4.
// The hop on A switches both levels of A.
// The last hop switches the typed levels of B. No map goes between the routers.
func TestAPITieTakeLevelsFollowEachRouter(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) {
		c.Levels, c.DefaultLevels = 2, "1-2"
		c.MoreRouters[0].Levels, c.MoreRouters[0].DefaultLevels = 4, "1-4"
	})
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	body := `{"routes":[{"src":{"router":"` + uidA + `","n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[3,4],"ties":[{"id":"t1","expect":{"router":"` + uidA + `","n":3}}]}]}`
	rec := apiPost(a, "/api/v1/takes", key, body, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if len(got) != 4 || got[0] != "A 4>10" || got[1] != "A 4>10" {
		t.Fatalf("connects %v, want 2 of A 4>10, then 2 on B", got)
	}
	if tail := got[2:]; !slices.Equal(slices.Sorted(slices.Values(tail)), []string{"B 5>2", "B 5>2"}) {
		t.Errorf("connects on B %v", tail)
	}
	r := tieTakeAnswer(t, rec.Body).Results[0]
	if len(r.Hops) != 1 || !slices.Equal(r.Hops[0].Levels, []int{1, 2}) {
		t.Errorf("hop %+v", r.Hops)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 2 || !slices.Equal(es[0].Levels, []int{3, 4}) {
		t.Errorf("entries %+v", es)
	}
}

// A typed level past the count of the destination's router is refused.
// No hop goes out, even if the router upstream has that level.
func TestAPITieTakeRefusesLevelPastDestination(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) {
		c.Levels, c.DefaultLevels = 4, "1-4"
		c.MoreRouters[0].Levels, c.MoreRouters[0].DefaultLevels = 2, "1-2"
	})
	uidA := a.r().cfg.UID
	log := connectLog(fa, fb)
	body := `{"routes":[{"src":{"router":"` + uidA + `","n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[3],"ties":[{"id":"t1"}]}]}`
	rec := apiPost(a, "/api/v1/takes", key, body, nil)
	wantErr(t, rec, http.StatusBadRequest, "bad_request")
	if !strings.Contains(rec.Body.String(), "unknown level") {
		t.Errorf("answer %s", rec.Body)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
}
