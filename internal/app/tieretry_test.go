package app

import (
	"fmt"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// lateHopTie runs a tie take from A.4 to B.2 through TIE 3. A says nothing about A.10 (it holds the hop's connects),
// so the hop fails with no answer and B gets nothing. It returns the destination entry and the hop entry.
func lateHopTie(t *testing.T) (a *App, fa, fb *fakeRouter, key string, log func() []string, dest, hop int) {
	t.Helper()
	a, fa, fb, key = tieApp(t, nil)
	fa.mu.Lock()
	fa.ignore[10] = true
	fa.mu.Unlock()
	log = connectLog(fa, fb)
	rec := apiPost(a, "/api/v1/takes", key, tieRoutes(a.r().cfg.Routers[0].UID, [2]int{4, 2}), nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	r := b.Results[0]
	if r.Result != "failed" || r.Hops[0].Result != "failed" || r.Hops[0].Entry == nil || r.Entry == nil {
		t.Fatalf("result %+v", r)
	}
	return a, fa, fb, key, log, *r.Entry, *r.Hops[0].Entry
}

// answerLate is router A that answers the held connects of the hop after the take gave up.
func answerLate(a *App, fa *fakeRouter) {
	fa.mu.Lock()
	fa.ignore = map[int]bool{}
	fa.mu.Unlock()
	for l := 1; l <= a.r().routerOf(a.r().cfg.Routers[0].UID).Levels; l++ {
		fa.mu.Lock()
		fa.tally[[2]int{10, a.r().byUI[l].Wire}] = 4
		fa.mu.Unlock()
		fa.msg(swp08.ExtConnected(0, a.r().byUI[l].Wire, 10, 4))
	}
	endBurst(a)
}

// A Late answer on a hop that is not the last never starts the later hops. It amends the hop entry, and the entry
// of the destination marks the hop as changed late, so the row offers the put-back.
func TestAPITieLateMiddleHop(t *testing.T) {
	a, fa, _, key, log, dest, hop := lateHopTie(t)
	answerLate(a, fa)
	if got := log(); slices.ContainsFunc(got, func(s string) bool { return strings.HasPrefix(s, "B ") }) {
		t.Errorf("connects %v: a late hop started the later hop", got)
	}
	h, _ := a.hist.Get(hop)
	if !h.OK || len(h.Late) == 0 || len(h.Failed) != 0 {
		t.Errorf("hop entry %+v", h)
	}
	d, _ := a.hist.Get(dest)
	if d.OK || len(d.Hops) != 1 || !d.Hops[0].Late || d.Hops[0].Result != "connected" || d.Hops[0].Entry == nil || *d.Hops[0].Entry != hop {
		t.Errorf("destination entry %+v", d)
	}
	if a.hist.Len() != 2 {
		t.Errorf("%d entries, want 2: a late hop records no new entry", a.hist.Len())
	}
	// The put-back is the same as for a hop that changed in time.
	rec := apiPost(a, fmt.Sprintf("/api/v1/history/%d/putback", hop), key, "", nil)
	if b := tieTakeAnswer(t, rec.Body); rec.Code != http.StatusOK || !b.OK || b.Results[0].Src != 3 {
		t.Errorf("put back %d %s", rec.Code, rec.Body)
	}
}

// After a late hop, TIE 3 holds the source. Retry sends only the hop that did not go: the last hop on B.
func TestAPITieRetryAfterLateHop(t *testing.T) {
	a, fa, _, key, log, dest, _ := lateHopTie(t)
	answerLate(a, fa)
	before := len(log())
	rec := apiPost(a, fmt.Sprintf("/api/v1/history/%d/retry", dest), key, "", nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK || len(b.Results) != 1 {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	if got := log()[before:]; !slices.Equal(got, []string{"B 5>2"}) {
		t.Errorf("connects %v, want only B 5>2", got)
	}
	r := b.Results[0]
	if r.Result != "connected" || r.Src != 4 || r.Dst != 2 || len(r.Hops) != 1 || r.Hops[0].Result != "already" {
		t.Errorf("result %+v", r)
	}
	e, _ := a.hist.Get(*r.Entry)
	if e.Retry == nil || *e.Retry != dest || e.Origin == nil || e.Origin.N != 4 {
		t.Errorf("retry entry %+v", e)
	}
}

// Three routers: A.4 to C.2 through TIE 3 (A to B) and TIE 5 (B to C). The hop on B fails. TIE 3 still holds A.4,
// so Retry sends the hop on B and the last hop on C, and nothing on A.
func TestAPITieRetrySendsOnlyFailedHops(t *testing.T) {
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
	var sentC []string
	sendC := fc.u.sendMsg
	fc.u.sendMsg = func(m []byte, why swp08.Why) error {
		if m[0] == swp08.CmdExtConnect {
			mu.Lock()
			sentC = append(sentC, fmt.Sprintf("C %d>%d", int(m[5])<<8|int(m[6]), int(m[3])<<8|int(m[4])))
			mu.Unlock()
		}
		return sendC(m, why)
	}
	log := connectLog(fa, fb)
	body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"c3c3c3c3","n":2},"levels":[1],"ties":[`+
		`{"id":"t1","expect":{"router":%q,"n":3}},{"id":"t2","expect":{"router":"b2b2b2b2","n":9}}]}]}`, uidA, uidA)
	rec := apiPost(a, "/api/v1/takes", key, body, nil)
	b := tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || b.OK || b.Results[0].Hops[1].Result != "failed" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	dest := *b.Results[0].Entry
	failedHop := *b.Results[0].Hops[1].Entry
	fb.mu.Lock()
	fb.deaf = map[[2]int]bool{}
	fb.mu.Unlock()
	before := len(log())
	rec = apiPost(a, fmt.Sprintf("/api/v1/history/%d/retry", dest), key, "", nil)
	b = tieTakeAnswer(t, rec.Body)
	if rec.Code != http.StatusOK || !b.OK {
		t.Fatalf("retry %d %s", rec.Code, rec.Body)
	}
	if got := log()[before:]; !slices.Equal(got, []string{"B 5>11"}) {
		t.Errorf("connects on A and B %v, want only B 5>11", got)
	}
	mu.Lock()
	if !slices.Equal(sentC, []string{"C 6>2"}) {
		t.Errorf("connects on C %v", sentC)
	}
	mu.Unlock()
	r := b.Results[0]
	if r.Result != "connected" || len(r.Hops) != 2 || r.Hops[0].Result != "already" || r.Hops[1].Result != "connected" {
		t.Errorf("result %+v", r)
	}
	h, _ := a.hist.Get(*r.Hops[1].Entry)
	if h.Retry == nil || *h.Retry != failedHop || h.Tie != "t2" {
		t.Errorf("hop entry %+v", h)
	}
}

// If an earlier hop no longer holds, Retry sends nothing. It answers 409 tie_line_changed, so the caller opens the
// picker with the tie lines as they are now.
func TestAPITieRetryAfterTieMoved(t *testing.T) {
	a, fa, fb, key, _ := partialTie(t)
	dest := a.hist.Len() - 1
	fa.seed(10, 8, a.r().byUI[1].Wire)
	log := connectLog(fa, fb)
	rec := apiPost(a, fmt.Sprintf("/api/v1/history/%d/retry", dest), key, "", nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_changed")
	if b := tieBody(t, rec.Body); len(b.Routes) != 1 || len(b.Routes[0].Paths) != 1 {
		t.Errorf("routes %+v", b.Routes)
	}
	if got := log(); len(got) != 0 {
		t.Errorf("connects %v", got)
	}
}

// Retry is for the entry of the destination asked for. A hop entry has its own put-back; a tie take that succeeded
// has nothing to retry.
func TestAPITieRetryRefusals(t *testing.T) {
	a, _, _, key, hop := partialTie(t)
	wantErr(t, apiPost(a, fmt.Sprintf("/api/v1/history/%d/retry", hop), key, "", nil), http.StatusConflict, "conflict")
	a2, _, _, key2 := tieApp(t, nil)
	rec := apiPost(a2, "/api/v1/takes", key2, tieRoutes(a2.r().cfg.Routers[0].UID, [2]int{4, 2}), nil)
	r := tieTakeAnswer(t, rec.Body).Results[0]
	wantErr(t, apiPost(a2, fmt.Sprintf("/api/v1/history/%d/retry", *r.Entry), key2, "", nil), http.StatusConflict, "conflict")
}
