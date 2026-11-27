package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// Redo goes to the router of its entry. A plain entry on B redoes on B. Redo of a tie take opens the picker
// (docs/tielines-design.md, "Redo and Retry"): 409 tie_line_needed with the paths as they are now, and nothing is
// sent. A hop entry has no Redo of its own.
func TestAPIRedoUsesRouterOfEntry(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	r := tieTakeAnswer(t, rec.Body).Results[0]
	if rec.Code != http.StatusOK || !r.OK || r.Entry == nil || r.Hops[0].Entry == nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	log := connectLog(fa, fb)
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/redo", *r.Entry), key, "", nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	if b := tieBody(t, rec.Body); len(b.Routes) != 1 || len(b.Routes[0].Paths) != 1 || b.Routes[0].Src != (apiTieEndOut{uidA, 4, "CAM 4"}) || b.Routes[0].Dst.N != 2 {
		t.Errorf("routes %+v", b.Routes)
	}
	wantErr(t, apiPost(a, fmt.Sprintf("/api/v1/history/%d/redo", *r.Hops[0].Entry), key, "", nil), http.StatusConflict, "conflict")
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}

	rec = apiPost(a, "/api/v1/takes", key, ends("b2b2b2b2", "b2b2b2b2", 5, 21), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	plain := *tieTakeAnswer(t, rec.Body).Results[0].Entry
	fb.seed(21, 7, a.r().byUI[1].Wire)
	before := len(log())
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/redo", plain), key, "", nil)
	if b := tieTakeAnswer(t, rec.Body); rec.Code != http.StatusOK || !b.OK {
		t.Fatalf("redo %d %s", rec.Code, rec.Body)
	}
	if got := log()[before:]; !slices.Equal(got, []string{"B 5>21"}) {
		t.Errorf("connects %v, want B 5>21", got)
	}
}

// TIE 3 carries A.4 when a tie take from A.4 is planned, so its hop has nothing to send. A plain take that puts A.3
// back holds the lock, and the tie take waits. After the wait TIE 3 carries A.3, as the caller expects. The tie take
// plans again: it sends the hop on A, so B.2 gets A.4 and not A.3.
func TestAPITieTakePlansAgainAfterLock(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	fa.u.routeTimeout = 5 * time.Second
	uidA := a.r().cfg.Routers[0].UID
	levelsA := a.r().routerOf(uidA).Levels
	seedAll(a, fa, 10, 4)
	log := connectLog(fa, fb)
	arrived, open := gateConnects(fa)
	t.Cleanup(open)
	all := make([]int, levelsA)
	for i := range all {
		all[i] = i + 1
	}
	plain := runTake(a, routeReq{ID: 1, Src: 3, Dst: 10, Levels: all})
	wait(t, "the plain take's connects", arrived)
	tie := post(a, key, tieRoutes(uidA, [2]int{4, 2}))
	waitFor(t, "the tie take to wait for TIE 3", func() bool { return a.tieLocks.waiting.Load() == 1 })
	open()
	wait(t, "the plain take", plain)
	rec := wait(t, "the tie take", tie)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK || b.Results[0].Hops[0].Result != "connected" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	got := log()
	if i := slices.Index(got, "A 4>10"); i < 0 || i > slices.Index(got, "B 5>2") {
		t.Errorf("connects %v: want A 4>10 before B 5>2", got)
	}
	fa.mu.Lock()
	on := fa.tally[[2]int{10, a.r().byUI[1].Wire}]
	fa.mu.Unlock()
	if on != 4 {
		t.Errorf("TIE 3 carries A.%d, want A.4", on)
	}
}

// pageClient is a page on the socket: takeResults reads what Sode sends it.
func pageClient(a *App) *client {
	c := &client{ip: "10.0.0.1", send: make(chan []byte, 256)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	return c
}

// Retry from the page of the entry of a tie take runs as the API Retry does (tieretry.go). It uses the same tie
// lines and the stale check, and sends only the hops that did not hold. The page sends the last hop as its route; Sode does not send it
// as a plain route.
func TestPageRetryOfTieTake(t *testing.T) {
	a, fa, fb, _, hop := partialTie(t)
	dest := hop + 1
	fb.mu.Lock()
	fb.deaf = map[[2]int]bool{}
	fb.mu.Unlock()
	c := pageClient(a)
	log := connectLog(fa, fb)
	// The page retries each failed entry of the Group: here only the entry of the destination.
	rs := takeResults(t, a, c, []routeReq{{ID: 7, Src: 5, Dst: 2, Levels: []int{1}, Router: "b2b2b2b2", Retry: &dest}})
	if len(rs) != 1 || rs[0].ID != 7 || !rs[0].OK || rs[0].Entry == nil {
		t.Fatalf("results %+v", rs)
	}
	if got := log(); !slices.Equal(got, []string{"B 5>2"}) {
		t.Errorf("connects %v, want only B 5>2", got)
	}
	e, _ := a.hist.Get(*rs[0].Entry)
	if e.Retry == nil || *e.Retry != dest || e.Origin == nil || e.Origin.N != 4 || len(e.Hops) != 1 || e.Hops[0].Result != "already" {
		t.Errorf("retry entry %+v", e)
	}
	// A hop entry has no Retry of its own.
	rs = takeResults(t, a, c, []routeReq{{ID: 8, Src: 4, Dst: 10, Levels: []int{1}, Retry: &hop}})
	if len(rs) != 1 || rs[0].ID != 8 || rs[0].OK || rs[0].Entry != nil || !strings.Contains(rs[0].Err, "hop of a tie take") {
		t.Errorf("hop retry %+v", rs)
	}
	if got := log(); len(got) != 1 {
		t.Errorf("connects %v", got)
	}
}

// If TIE 3 moved after the failed take, Retry from the page sends nothing, as the API Retry does.
func TestPageRetryOfTieTakeAfterTieMoved(t *testing.T) {
	a, fa, fb, _, hop := partialTie(t)
	dest := hop + 1
	fa.seed(10, 8, a.r().byUI[1].Wire)
	c := pageClient(a)
	log := connectLog(fa, fb)
	rs := takeResults(t, a, c, []routeReq{{ID: 3, Src: 5, Dst: 2, Levels: []int{1}, Router: "b2b2b2b2", Retry: &dest}})
	if len(rs) != 1 || rs[0].ID != 3 || rs[0].OK || !strings.Contains(rs[0].Err, "tie line changed") {
		t.Errorf("results %+v", rs)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
}

// A put-back from the page sends only the levels that the hop changed, as the API put-back does. Here the hop failed
// on level 2 of A, and then someone routed A.9 there. A put-back item with every level leaves level 2 alone.
func TestPagePutBackOnlyChangedLevels(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	levelsA := a.r().routerOf(uidA).Levels
	wire2 := a.r().byUI[2].Wire
	fa.mu.Lock()
	fa.deaf[[2]int{10, wire2}] = true
	fa.mu.Unlock()
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	r := tieTakeAnswer(t, rec.Body).Results[0]
	if rec.Code != http.StatusOK || r.OK || r.Hops[0].Result != "failed" || r.Hops[0].Entry == nil {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	hop := *r.Hops[0].Entry
	fa.seed(10, 9, wire2)
	c := pageClient(a)
	log := connectLog(fa, fb)
	all := make([]int, levelsA)
	for i := range all {
		all[i] = i + 1
	}
	rs := takeResults(t, a, c, []routeReq{{ID: 1, Src: 3, Dst: 10, Levels: all, Undo: &hop, PutBack: true}})
	if len(rs) != 1 || !rs[0].OK {
		t.Fatalf("results %+v", rs)
	}
	if got := log(); len(got) != levelsA-1 {
		t.Errorf("%d connects %v, want %d: none on level 2", len(got), got, levelsA-1)
	}
	e, _ := a.hist.Get(*rs[0].Entry)
	if slices.Contains(e.Levels, 2) || len(e.Levels) != levelsA-1 {
		t.Errorf("put-back entry levels %v", e.Levels)
	}
	// An item with only levels that the hop did not change sends nothing.
	rs = takeResults(t, a, c, []routeReq{{ID: 2, Src: 3, Dst: 10, Levels: []int{2}, Undo: &hop, PutBack: true}})
	if len(rs) != 1 || rs[0].OK || rs[0].Entry != nil {
		t.Errorf("results %+v", rs)
	}
	if got := log(); len(got) != levelsA-1 {
		t.Errorf("connects %v", got)
	}
}

// A plain route to the upstream destination of a tie line cannot go in the same take as a hop through that tie
// line. The two would fight for it. Sode refuses the take before it sends anything, in either order.
func TestAPITieTakeRefusesPlainRouteOnItsTieLine(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	log := connectLog(fa, fb)
	tie := strings.TrimSuffix(strings.TrimPrefix(tieRoutes(uidA, [2]int{4, 2}), `{"routes":[`), `]}`)
	plain := fmt.Sprintf(`{"src":{"router":%q,"n":6},"dst":{"router":%q,"n":10},"levels":[1]}`, uidA, uidA)
	for _, body := range []string{`{"routes":[` + tie + `,` + plain + `]}`, `{"routes":[` + plain + `,` + tie + `]}`} {
		rec := apiPost(a, "/api/v1/takes", key, body, nil)
		wantErr(t, rec, http.StatusConflict, "conflict")
		if !strings.Contains(rec.Body.String(), "TIE 3") {
			t.Errorf("body %s does not name the tie line", rec.Body)
		}
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
}

// Three routers: TIE 3 (A to B) is mixed, and B.11 sends it on to C through TIE 5. The chain of TIE 5 is then mixed
// too. The 409 body gives one source for each level of B in levels. A caller that sends that list back as expect
// gets its take.
func TestAPITieTakeDownstreamOfMixedTieLine(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) {
		r := moreRouter("C", "Third", "10.0.0.10:8910", 1)
		r.UID = "c3c3c3c3"
		c.Routers = append(c.Routers, r)
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t2", Name: "TIE 5", From: config.Ref{Router: "b2b2b2b2", N: 11}, To: config.Ref{Router: r.UID, N: 6}})
	})
	fc, uc := addFakeUnit(t, a, a.r().cfg.Routers[2])
	a.onUnitState(uc, swp08.StateUp, nil)
	wire := a.r().byUI[1].Wire
	fa.seed(10, 6, a.r().byUI[2].Wire) // TIE 3: A.3 on level 1, A.6 on level 2
	fb.seed(11, 5, wire)               // TIE 5 carries what TIE 3 carries
	fc.seed(2, 1, wire)
	log := connectLog(fa, fb)
	route := func(expect string) string {
		return `{"routes":[{"src":{"router":"b2b2b2b2","n":9},"dst":{"router":"c3c3c3c3","n":2},"levels":[1],"ties":[{"id":"t2","expect":` + expect + `}]}]}`
	}
	b := tieChangedBody(t, apiPost(a, "/api/v1/takes", key, route("null"), nil))
	now := b.Routes[0].Paths[0].Hops[0].Now
	if now == nil || !now.Mixed || len(now.Levels) != 1 || now.Levels[0] == nil || now.Levels[0].N != 3 {
		t.Fatalf("now %+v", now)
	}
	seen, err := json.Marshal(now.Levels)
	if err != nil {
		t.Fatal(err)
	}
	if rec := apiPost(a, "/api/v1/takes", key, route(string(seen)), nil); rec.Code != http.StatusOK {
		t.Fatalf("expect %s: %d %s", seen, rec.Code, rec.Body)
	}
	if got := log(); !slices.Equal(got, []string{"B 9>11"}) {
		t.Errorf("connects %v", got)
	}
}
