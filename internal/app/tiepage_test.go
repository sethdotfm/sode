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

func tiePageClient(a *App) *client {
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
	uidA := a.r().cfg.Routers[0].UID
	c := tiePageClient(a)
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
	uidA := a.r().cfg.Routers[0].UID
	c := tiePageClient(a)
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

// A page's tie take is recorded as a plain take from the page. It has the same History name and no API actor. Its
// Communication Log rows are command rows. This holds for a signed-in user and for a Guest.
func TestPageTieTakeActsAsThePage(t *testing.T) {
	for _, user := range []bool{true, false} {
		t.Run(fmt.Sprintf("user=%v", user), func(t *testing.T) {
			a, _, _, _ := tieApp(t, nil)
			defer startLog(t, a)()
			uidA := a.r().cfg.Routers[0].UID
			c := tiePageClient(a)
			want := c.ip
			if user {
				if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
					t.Fatal(err)
				}
				want = "Ada"
			}
			a.serveTake(c, &inMsg{T: "take", Items: []routeReq{{ID: 1, Src: 4, Dst: 21, Levels: []int{1}, Router: "b2b2b2b2"}}})
			for deadline := time.After(10 * time.Second); ; {
				var m routeResult
				select {
				case b := <-c.send:
					json.Unmarshal(b, &m)
				case <-deadline:
					t.Fatal("no takeResult")
				}
				if m.T == "takeResult" {
					break
				}
			}
			route := fmt.Sprintf(`[{"src":{"router":%q,"n":4},"dst":{"router":"b2b2b2b2","n":2},"levels":[1],"ties":[{"id":"t1","expect":{"router":%q,"n":3}}]}]`, uidA, uidA)
			if got := pageTieTake(t, a, c, 9, route); got.Status != http.StatusOK {
				t.Fatalf("%d %s", got.Status, got.Body)
			}
			es, _ := a.hist.Page(-1, 10)
			if len(es) != 3 {
				t.Fatalf("entries %+v", es)
			}
			for _, e := range es {
				if e.Client != want || e.Actor != nil || e.BrowserOf != "" {
					t.Errorf("entry %d: client %q actor %+v browserOf %q, want %q as a plain take", e.ID, e.Client, e.Actor, e.BrowserOf, want)
				}
			}
			sent := 0
			for _, r := range waitLogged(t, a, 2) {
				if r.Kind == kindAPI {
					t.Errorf("api row %+v", r)
				}
				if r.Kind == kindCommand && r.Why == "take" && r.Result == "sent" {
					sent++
					if r.By != want {
						t.Errorf("row by %q, want %q", r.By, want)
					}
				}
			}
			if sent != 2 {
				t.Errorf("%d sent take rows, want one for each take", sent)
			}
		})
	}
}
