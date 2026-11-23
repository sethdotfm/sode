package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// levelRouters gives a with the first router at 17 levels and a second router, B, with one level at wire offset 4.
func levelRouters(t *testing.T) (a *App, first, second *fakeRouter, u *routerUnit) {
	t.Helper()
	a = newTestApp(t, func(c *config.Config) {
		b := moreRouter("B", "One level", "10.0.0.9:8910", 1)
		b.LevelWireOffset, b.LevelNumbering = 4, "va"
		c.Levels, c.DefaultLevels = 17, "1-17"
		c.MoreRouters = []config.Router{b}
	})
	first = newFakeRouter(t, a)
	second, u = addFakeUnit(t, a, a.r().cfg.MoreRouters[0])
	return a, first, second, u
}

// takeReplies runs items for a client that is on the socket and gives the takeResult errors it got.
func takeReplies(t *testing.T, a *App, items []routeReq) []string {
	t.Helper()
	c := &client{ip: "10.0.0.1", send: make(chan []byte, 64)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	a.takeAllowed(c, items)
	a.mu.Lock()
	delete(a.clients, c)
	a.mu.Unlock()
	var errs []string
	for len(c.send) > 0 {
		var m struct {
			T   string `json:"t"`
			Err string `json:"err"`
		}
		if json.Unmarshal(<-c.send, &m) == nil && m.T == "takeResult" {
			errs = append(errs, m.Err)
		}
	}
	return errs
}

// Each router has its own levels. On a router with one level, typed levels 1-17 mean its one level. The take sends
// one connect on the wire level of that router. The History entry has level 1, which is all its levels.
func TestOneLevelRouterTakesAnyTypedLevels(t *testing.T) {
	a, first, second, u := levelRouters(t)
	if lv := a.r().levelsOf(u.uid); len(lv.levels) != 1 || lv.levels[0].Wire != 5 || lv.levels[0].Label != "V" {
		t.Fatalf("router B levels %+v", lv.levels)
	}
	if n := len(a.r().levelsOf("").levels); n != 17 {
		t.Fatalf("first router has %d levels", n)
	}
	typed := make([]int, 17)
	for i := range typed {
		typed[i] = i + 1
	}
	if errs := takeReplies(t, a, []routeReq{{ID: 1, Src: 4, Dst: 2, Levels: typed, Router: u.uid}}); len(errs) != 1 || errs[0] != "" {
		t.Fatalf("take results %q", errs)
	}
	if n := first.sends(); n != 0 {
		t.Fatalf("the first router got %d messages", n)
	}
	second.mu.Lock()
	got, tally := second.tally[[2]int{2, 5}], len(second.tally)
	second.mu.Unlock()
	if got != 4 || tally != 1 {
		t.Fatalf("router B tally %v", second.tally)
	}
	// The router's answer on wire 5 is level 1 of router B.
	a.mu.Lock()
	cached := u.router.tally[2][1]
	a.mu.Unlock()
	if cached != 4 {
		t.Errorf("router B route cache %v", u.router.tally)
	}
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || !es[0].OK || es[0].Router != u.uid || len(es[0].Levels) != 1 || es[0].Levels[0] != 1 {
		t.Fatalf("entry %+v", es)
	}
}

// A router with more than one level still refuses a level past its count, and sends nothing.
func TestManyLevelRouterRefusesLevelPastCount(t *testing.T) {
	a, first, second, _ := levelRouters(t)
	errs := takeReplies(t, a, []routeReq{{ID: 1, Src: 4, Dst: 2, Levels: []int{1, 18}}})
	if len(errs) != 1 || !strings.Contains(errs[0], "unknown level 18") {
		t.Fatalf("take results %q", errs)
	}
	if n := first.sends() + second.sends(); n != 0 {
		t.Errorf("sent %d messages", n)
	}
	if es, _ := a.hist.Page(-1, 1); len(es) != 0 {
		t.Errorf("entries %+v", es)
	}
}

// The hello's routers list carries each router's own levels, numbering and default levels.
func TestHelloRoutersCarryTheirLevels(t *testing.T) {
	a, _, _, u := levelRouters(t)
	var m helloMsg
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if len(m.Routers) != 2 {
		t.Fatalf("routers %+v", m.Routers)
	}
	one, many := m.Routers[1], m.Routers[0]
	if one.UID != u.uid || len(one.Levels) != 1 || one.Levels[0].Wire != 5 || one.LevelNumbering != "va" || one.DefaultLevels != "1-1" {
		t.Errorf("router B %+v", one)
	}
	if len(many.Levels) != 17 || many.LevelNumbering != "1" || many.DefaultLevels != "1-17" {
		t.Errorf("first router levels %d, numbering %q, default %q", len(many.Levels), many.LevelNumbering, many.DefaultLevels)
	}
}
