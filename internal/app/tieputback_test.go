package app

import (
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// partialTie runs a tie take from A.4 to B.2 through TIE 3 whose last hop fails: B ignores the connect. The hop on
// A changed TIE 3 (it carried A.3), so the route is a partial failure. It returns the entry of the hop.
func partialTie(t *testing.T) (a *App, fa, fb *fakeRouter, key string, hop int) {
	t.Helper()
	a, fa, fb, key = tieApp(t, nil)
	fb.mu.Lock()
	fb.deaf[[2]int{2, a.r().byUI[1].Wire}] = true
	fb.mu.Unlock()
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(a.r().cfg.Routers[0].UID, [2]int{4, 2}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	r := b.Results[0]
	if r.Result != "failed" || r.Hops[0].Result != "connected" || r.Hops[0].Entry == nil {
		t.Fatalf("result %+v", r)
	}
	return a, fa, fb, key, *r.Hops[0].Entry
}

// A last hop that fails leaves TIE 3 changed. Put back sends the Prev of the hop on every level of A, as an Undo of
// the hop entry.
func TestAPITiePutBack(t *testing.T) {
	a, fa, fb, key, hop := partialTie(t)
	levelsA := a.r().routerOf(a.r().cfg.Routers[0].UID).Levels
	if es, _ := a.hist.Page(-1, 5); len(es) != 2 || es[1].ID != hop || !es[1].OK || es[0].OK || es[0].Hops[0].Result != "connected" {
		t.Fatalf("entries %+v", es)
	}
	log := connectLog(fa, fb)
	rec := apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", hop), key, "", nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK || len(b.Results) != 1 || b.Results[0].Src != 3 || b.Results[0].Dst != 10 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := log(); len(got) != levelsA || slices.ContainsFunc(got, func(s string) bool { return s != "A 3>10" }) {
		t.Errorf("connects %v, want %d of A 3>10", got, levelsA)
	}
	es, _ := a.hist.Page(-1, 1)
	if e := es[0]; e.Undo == nil || *e.Undo != hop || !e.OK || e.Dst != 10 || e.Tie != "" {
		t.Errorf("put-back entry %+v", e)
	}
	// TIE 3 carries A.3 again, not what the take set: a second put back sends nothing.
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", hop), key, "", nil)
	b = tieTakeAnswer(t, rec.Body)
	if b.OK || !strings.Contains(b.Results[0].Error, "not sent: TIE 3 now carries") {
		t.Errorf("second put back %d %s", rec.Code, rec.Body)
	}
	if got := log(); len(got) != levelsA {
		t.Errorf("connects %v", got)
	}
}

// Put back works only while the tie line carries what the take set. After someone else moves TIE 3 on one level,
// Sode sends nothing and says what TIE 3 carries now.
func TestAPITiePutBackRefusedAfterTieMoved(t *testing.T) {
	a, fa, fb, key, hop := partialTie(t)
	a.mu.Lock()
	a.firstUnit().router.srcNames[8] = "CAM 9"
	a.mu.Unlock()
	fa.seed(10, 8, a.r().byUI[1].Wire)
	log := connectLog(fa, fb)
	rec := apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", hop), key, "", nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if r := b.Results[0]; r.Result != "failed" || !strings.Contains(r.Error, "not sent: TIE 3 now carries CAM 9, not CAM 4, on levels") {
		t.Errorf("result %+v", r)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	es, _ := a.hist.Page(-1, 1)
	if e := es[0]; e.OK || e.Undo == nil || *e.Undo != hop || len(e.Unsent) == 0 {
		t.Errorf("entry %+v", e)
	}
}

// Put back is for the hop entry of a tie take only.
func TestAPITiePutBackNeedsHop(t *testing.T) {
	a, _, _, key, hop := partialTie(t)
	wantErr(t, apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", hop+1), key, "", nil), http.StatusConflict, "conflict")
	wantErr(t, apiPost(a, "/api/v1/history/99/putback", key, "", nil), http.StatusNotFound, "not_found")
	wantErr(t, apiPost(a, "/api/v1/history/x/putback", key, "", nil), http.StatusBadRequest, "bad_request")
}

// A put-back route from the page is checked the same way. The putBack tag on an entry that is not a hop sends
// nothing.
func TestPutBackTagOnPlainEntry(t *testing.T) {
	a, fa, fb, _, hop := partialTie(t)
	c := &client{ip: "10.0.0.1"}
	last := hop + 1 // the destination entry, on B
	log := connectLog(fa, fb)
	a.take(c, []routeReq{{ID: 1, Src: 7, Dst: 2, Levels: []int{1}, Undo: &last, PutBack: true}})
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
	es, _ := a.hist.Page(-1, 1)
	if e := es[0]; e.OK || !strings.Contains(e.Err, "is not a hop of a tie take") {
		t.Errorf("entry %+v", e)
	}
}

// A put-back that matches its hop entry needs no Rights: hops through tie lines need none. A plain Undo of the same
// entry still does.
func TestPutBackSkipsRights(t *testing.T) {
	a, _, _, _, hop := partialTie(t)
	rt := a.r()
	e, _ := a.hist.Get(hop)
	src := e.Prev[0][1]
	it := routeReq{Src: src, Dst: e.Dst, Levels: []int{1}, Undo: &hop, Router: e.Router}
	none := config.Rights{Sources: &[]config.Ref{}, Dsts: &[]config.Ref{}, Levels: config.RouterLevels{a.rightsUID(rt, e.Router): {}}}
	if msg := a.outsideRights(rt, none, []routeReq{it}); msg == "" {
		t.Fatal("a plain Undo of the hop passed empty Rights")
	}
	it.PutBack = true
	if msg := a.outsideRights(rt, none, []routeReq{it}); msg != "" {
		t.Errorf("put back refused: %s", msg)
	}
	it.Src = 4
	if msg := a.outsideRights(rt, none, []routeReq{it}); msg == "" {
		t.Error("a put-back with another source passed empty Rights")
	}
}

// The link of B drops during the take, after the hop on A went out. The take fails at the hop on B, and TIE 3 can be
// put back.
func TestAPITieTakeLinkDropMidTake(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	uidA := a.r().cfg.Routers[0].UID
	// The sender holds mu, so B drops on its own goroutine. A answers its connects only after B is down.
	var once sync.Once
	down := make(chan struct{})
	send := fa.u.sendMsg
	fa.u.sendMsg = func(m []byte, why swp08.Why) error {
		if m[0] != swp08.CmdExtConnect {
			return send(m, why)
		}
		once.Do(func() {
			go func() {
				fb.drop(errors.New("link lost"))
				close(down)
			}()
		})
		go func() {
			<-down
			_ = send(m, why)
		}()
		return nil
	}
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(uidA, [2]int{4, 2}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	r := b.Results[0]
	if r.Result != "failed" || r.Hops[0].Result != "connected" || r.Hops[0].Entry == nil {
		t.Fatalf("result %+v", r)
	}
	if n := fb.cmds(swp08.CmdExtConnect); n != 0 {
		t.Errorf("B got %d connects", n)
	}
	es, _ := a.hist.Page(-1, 2)
	if len(es) != 2 || es[0].OK || es[0].Router != "b2b2b2b2" || !es[1].OK {
		t.Fatalf("entries %+v", es)
	}
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", *r.Hops[0].Entry), key, "", nil)
	if b := tieTakeAnswer(t, rec.Body); rec.Code != http.StatusOK || !b.OK {
		t.Errorf("put back %d %s", rec.Code, rec.Body)
	}
}

// Three routers: A.4 to C.2 through TIE 3 (A to B) and TIE 5 (B to C). The hop on B fails, so Sode stops there: C
// gets nothing. TIE 3 changed and can be put back.
func TestAPITieTakeStopsAtMiddleHop(t *testing.T) {
	a, fa, fb, key := tieApp(t, func(c *config.Config) {
		r := moreRouter("C", "Third", "10.0.0.10:8910", 1)
		r.UID = "c3c3c3c3"
		c.Routers = append(c.Routers, r)
		c.TieLines = append(c.TieLines, config.TieLine{ID: "t2", Name: "TIE 5", From: config.Ref{Router: "b2b2b2b2", N: 11}, To: config.Ref{Router: r.UID, N: 6}})
	})
	fc, uc := addFakeUnit(t, a, a.r().cfg.Routers[2])
	a.onUnitState(uc, swp08.StateUp, nil)
	uidA, wire := a.r().cfg.Routers[0].UID, a.r().byUI[1].Wire
	fb.seed(11, 9, wire)
	fc.seed(2, 1, wire)
	fb.mu.Lock()
	fb.deaf[[2]int{11, wire}] = true
	fb.mu.Unlock()
	var mu sync.Mutex
	sentC := 0
	sendC := fc.u.sendMsg
	fc.u.sendMsg = func(m []byte, why swp08.Why) error {
		if m[0] == swp08.CmdExtConnect {
			mu.Lock()
			sentC++
			mu.Unlock()
		}
		return sendC(m, why)
	}
	log := connectLog(fa, fb)
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"c3c3c3c3","n":2},"levels":[1],"ties":[`+
		`{"id":"t1","expect":{"router":%q,"n":3}},{"id":"t2","expect":{"router":"b2b2b2b2","n":9}}]}]}`, uidA, uidA)
	rec := apiPost(a, "/api/v1/takes", key, body, nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := log(); !slices.Contains(got, "A 4>10") || !slices.Contains(got, "B 5>11") {
		t.Errorf("connects %v", got)
	}
	mu.Lock()
	if sentC != 0 {
		t.Errorf("C got %d connects", sentC)
	}
	mu.Unlock()
	r := b.Results[0]
	if len(r.Hops) != 2 || r.Hops[0].Result != "connected" || r.Hops[1].Result != "failed" || r.Result != "failed" ||
		!strings.Contains(r.Error, "the hop to TIE 5 failed") {
		t.Fatalf("result %+v", r)
	}
	if es, _ := a.hist.Page(-1, 5); len(es) != 3 || es[0].GroupN != 3 || es[0].Router != "c3c3c3c3" {
		t.Errorf("entries %+v", es)
	}
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", *r.Hops[0].Entry), key, "", nil)
	if b := tieTakeAnswer(t, rec.Body); rec.Code != http.StatusOK || !b.OK {
		t.Errorf("put back %d %s", rec.Code, rec.Body)
	}
	// The hop on B changed nothing, so it has nothing to put back.
	wantErr(t, apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", *r.Hops[1].Entry), key, "", nil), http.StatusUnprocessableEntity, "nothing_to_do")
}
