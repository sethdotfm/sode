package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"
)

// pageTieTake sends the main page's tieTake for routes and gives its answer.
func pageTieTake(t *testing.T, a *App, c *client, id int, routes string) tieTakeMsg {
	t.Helper()
	m := &inMsg{T: "tieTake", ID: id, Routes: json.RawMessage(routes)}
	a.serveTieTake(c, m)
	deadline := time.After(10 * time.Second)
	for {
		select {
		case b := <-c.send:
			var got tieTakeMsg
			if json.Unmarshal(b, &got) == nil && got.T == "tieTakeResult" {
				if got.ID != id {
					t.Fatalf("answer id %d, want %d", got.ID, id)
				}
				return got
			}
		case <-deadline:
			t.Fatal("no tieTakeResult")
		}
	}
}

func pageClient(a *App) *client {
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 4096)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	return c
}

// The page asks with no ties and gets the plan of POST /takes. Then it sends the chosen tie line, and the take runs
// upstream first. The entries name the page's caller, as a plain take from the page does.
func TestPageTieTakeAsksThenSends(t *testing.T) {
	a, fa, fb, _ := tieApp(t, nil)
	uidA := a.r().cfg.UID
	c := pageClient(a)
	log := connectLog(fa, fb)

	route := fmt.Sprintf(`[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1]}]`, uidA)
	got := pageTieTake(t, a, c, 7, route)
	if got.Status != http.StatusConflict {
		t.Fatalf("plan: %d %s", got.Status, got.Body)
	}
	var plan tieBodyIn
	if err := json.Unmarshal(got.Body, &plan); err != nil {
		t.Fatal(err)
	}
	if plan.Error.Code != "tie_line_needed" || len(plan.Routes) != 1 || len(plan.Routes[0].Paths) != 1 {
		t.Fatalf("plan %s", got.Body)
	}
	p := plan.Routes[0].Paths[0]
	if h := p.Hops[0]; h.Tie == nil || h.Tie.ID != "t1" || h.Tie.Name != "TIE 3" || h.Now == nil || h.Now.Origin == nil || h.Now.Origin.N != 3 {
		t.Fatalf("first hop %+v", p.Hops[0])
	}
	if len(log()) != 0 {
		t.Fatalf("a plan sent %v", log())
	}

	route = fmt.Sprintf(`[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":3}}],"affected":[{"router":"b2b2b2b2","n":20},{"router":"b2b2b2b2","n":21}]}]`, uidA, uidA)
	got = pageTieTake(t, a, c, 8, route)
	if got.Status != http.StatusOK {
		t.Fatalf("take: %d %s", got.Status, got.Body)
	}
	var out tieTakeBody
	if err := json.Unmarshal(got.Body, &out); err != nil || !out.OK || out.Results[0].Result != "connected" {
		t.Fatalf("take %v %s", err, got.Body)
	}
	if sent := log(); len(sent) == 0 || sent[len(sent)-1] != "B 5>2" || sent[0] != "A 4>10" {
		t.Fatalf("connects %v", sent)
	}
	es, _ := a.hist.Page(-1, 5)
	for _, e := range es {
		if e.Client != "10.0.0.9" {
			t.Errorf("entry %d names %q", e.ID, e.Client)
		}
	}
}

// A take that saw another source on the tie line is refused before anything goes out.
func TestPageTieTakeStale(t *testing.T) {
	a, fa, fb, _ := tieApp(t, nil)
	uidA := a.r().cfg.UID
	c := pageClient(a)
	log := connectLog(fa, fb)
	route := fmt.Sprintf(`[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":9}}]}]`, uidA, uidA)
	got := pageTieTake(t, a, c, 3, route)
	var b apiErrBody
	if err := json.Unmarshal(got.Body, &b); err != nil || got.Status != http.StatusConflict || b.Error.Code != "tie_line_changed" {
		t.Fatalf("%d %s", got.Status, got.Body)
	}
	if len(log()) != 0 {
		t.Fatalf("sent %v", log())
	}
}
