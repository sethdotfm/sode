package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
)

// pageTiePlan sends the picker's tiePlan for routes and gives its answer.
func pageTiePlan(t *testing.T, a *App, c *client, id int, routes string) tieTakeMsg {
	t.Helper()
	a.serveTiePlan(c, &inMsg{T: "tiePlan", ID: id, Routes: json.RawMessage(routes)})
	deadline := time.After(10 * time.Second)
	for {
		select {
		case b := <-c.send:
			var got tieTakeMsg
			if json.Unmarshal(b, &got) == nil && got.T == "tiePlanResult" && got.ID == id {
				return got
			}
		case <-deadline:
			t.Fatal("no tiePlanResult")
		}
	}
}

// The open picker asks for the plan again and again. A plan sends nothing and writes no Communication Log row. It
// holds no tie line: a take through the same tie line goes while the picker is open.
func TestPageTiePlanHoldsNothing(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	defer startLog(t, a)()
	uidA := a.r().cfg.Routers[0].UID
	c := tiePageClient(a)
	log := connectLog(fa, fb)
	route := fmt.Sprintf(`[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1]}]`, uidA)
	for id := 1; id <= 3; id++ {
		got := pageTiePlan(t, a, c, id, route)
		var b tieBodyIn
		if err := json.Unmarshal(got.Body, &b); err != nil || got.Status != http.StatusConflict || b.Error.Code != "tie_line_needed" || len(b.Routes) != 1 {
			t.Fatalf("plan %d: %d %s", id, got.Status, got.Body)
		}
	}
	// A route with its ties is a take, not a plan: tiePlan never sends it.
	withTies := fmt.Sprintf(`[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":3}}]}]`, uidA, uidA)
	if got := pageTiePlan(t, a, c, 4, withTies); got.Status != http.StatusBadRequest {
		t.Fatalf("a plan with ties: %d %s", got.Status, got.Body)
	}
	if len(log()) != 0 {
		t.Fatalf("a plan sent %v", log())
	}
	// Another caller takes through TIE 3 while the picker is open.
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":`+withTies+`}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("take while a picker is open: %d %s", rec.Code, rec.Body)
	}
	for _, r := range waitLogged(t, a, 1) {
		if r.Result == "refused" {
			t.Errorf("a plan wrote %+v", r)
		}
	}
}

// An API caller (a panel) asks for the plan with POST /takes/plan and gets the 409 of the plan. Sode takes nothing,
// holds no tie line and writes no Communication Log row.
func TestAPITakePlanOnly(t *testing.T) {
	a, fa, fb, key := tieApp(t, nil)
	defer startLog(t, a)()
	uidA := a.r().cfg.Routers[0].UID
	log := connectLog(fa, fb)
	route := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1]}]}`, uidA)
	rec := apiPost(a, "/api/v1/takes/plan", key, route, nil)
	wantErr(t, rec, http.StatusConflict, "tie_line_needed")
	if b := tieBody(t, rec.Body); len(b.Routes) != 1 || len(b.Routes[0].Paths) != 1 {
		t.Fatalf("plan %s", rec.Body)
	}
	withTies := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":3}}]}]}`, uidA, uidA)
	wantErr(t, apiPost(a, "/api/v1/takes/plan", key, withTies, nil), http.StatusBadRequest, "bad_request")
	if len(log()) != 0 {
		t.Fatalf("a plan sent %v", log())
	}
	if rec := apiPost(a, "/api/v1/takes", key, withTies, nil); rec.Code != http.StatusOK {
		t.Fatalf("take: %d %s", rec.Code, rec.Body)
	}
	for _, r := range waitLogged(t, a, 1) {
		if r.Result == "refused" {
			t.Errorf("a plan wrote %+v", r)
		}
	}
}

// A Protect on a router after the first reaches the page in a protected message that names that router, so an open
// picker plans again. It has Sode's record of that router.
func TestProtectOfAnotherRouterReachesThePage(t *testing.T) {
	a, _, _, _ := tieApp(t, nil)
	c := tiePageClient(a)
	b, _ := a.unit("b2b2b2b2")
	a.mu.Lock()
	a.applyProtect(b, a.r(), swp08.Protect{Matrix: a.r().cfg.Routers[0].Matrix, Level: a.r().levelsOf(b.uid()).byUI[1].Wire, Dst: 20, Device: 9, Protected: true, Kind: swp08.KindProtect}, true)
	a.mu.Unlock()
	a.flush()
	deadline := time.After(5 * time.Second)
	for {
		select {
		case raw := <-c.send:
			var m struct {
				T         string
				U         [][]int
				By        []protectBy
				RouterUID string
			}
			if json.Unmarshal(raw, &m) != nil || m.T != "protected" {
				continue
			}
			if m.RouterUID != "b2b2b2b2" || len(m.U) != 1 || m.U[0][0] != 20 || m.By == nil {
				t.Fatalf("protected %s", raw)
			}
			return
		case <-deadline:
			t.Fatal("no protected message for router B")
		}
	}
}
