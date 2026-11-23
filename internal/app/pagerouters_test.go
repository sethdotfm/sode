package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/swp08"
)

// pageMsgs reads what the page got so far, by type.
func pageMsgs(t *testing.T, c *client) map[string][]map[string]any {
	t.Helper()
	out := map[string][]map[string]any{}
	for len(c.send) > 0 {
		var m map[string]any
		if err := json.Unmarshal(<-c.send, &m); err != nil {
			t.Fatal(err)
		}
		k, _ := m["t"].(string)
		out[k] = append(out[k], m)
	}
	return out
}

// The hello lists every router with its names, tally, levels and link state. It has no flat fields of one router.
func TestHelloListsEveryRouter(t *testing.T) {
	a, _, _, u := twoRouterFiles(t)
	readUnitNames(a, a.first, []string{"A CAM 1"}, []string{"A MON 1"})
	readUnitNames(a, u, []string{"B CAM 1"}, []string{"B MON 1"})
	a.mu.Lock()
	u.router.tally[0] = map[int]int{1: 0}
	u.status.Router = string(swp08.StateUp)
	var h struct {
		Routers []struct {
			UID, Prefix, Title string
			Levels             []map[string]any
			Srcs, Dsts         [][2]any
			Tally              [][3]int
			Status             map[string]any
		} `json:"routers"`
	}
	raw := a.hello(helloPage{})
	err := json.Unmarshal(raw, &h)
	a.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	var flat map[string]any
	if err := json.Unmarshal(raw, &flat); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"srcs", "dsts", "at", "savedAt", "tally", "status", "levels", "prefix", "routerUid"} {
		if _, ok := flat[k]; ok {
			t.Errorf("the hello has the flat field %q", k)
		}
	}
	if len(h.Routers) != 2 {
		t.Fatalf("routers: %+v", h.Routers)
	}
	f, b := h.Routers[0], h.Routers[1]
	if f.UID != a.r().cfg.UID || f.Prefix != "A" || len(f.Srcs) != 1 || f.Srcs[0][1] != "A CAM 1" {
		t.Errorf("first router: %+v", f)
	}
	if b.UID != u.uid || b.Prefix != "B" || b.Title != "Second" || len(b.Levels) != 3 || b.Dsts[0][1] != "B MON 1" {
		t.Errorf("second router: %+v", b)
	}
	if len(b.Tally) != 1 || b.Tally[0] != [3]int{0, 1, 0} || b.Status["router"] != "up" || b.Status["routerUid"] != u.uid {
		t.Errorf("second router tally %v, status %v", b.Tally, b.Status)
	}
}

// A tally, names or status message names its router, so the page keeps each router's state apart.
func TestPageMessagesNameTheirRouter(t *testing.T) {
	a, first, second, u := twoRouterFiles(t)
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 256)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()

	wire := a.r().byUI[1].Wire
	first.msg(swp08.ExtConnected(0, wire, 2, 4))
	second.msg(swp08.ExtConnected(0, wire, 3, 5))
	readUnitNames(a, u, []string{"B CAM 1"}, []string{"B MON 1"})
	a.onUnitState(u, swp08.StateUp, nil)
	got := pageMsgs(t, c)

	byRouter := map[string]string{}
	for _, m := range got["tally"] {
		byRouter[m["routerUid"].(string)] = strings.TrimSpace(string(mustJSON(t, m["u"])))
	}
	if byRouter[a.first.uid] != "[[2,1,4]]" || byRouter[u.uid] != "[[3,1,5]]" {
		t.Errorf("tally messages: %v", byRouter)
	}
	if n := got["names"]; len(n) != 1 || n[0]["routerUid"] != u.uid {
		t.Errorf("names messages: %v", n)
	}
	if s := got["status"]; len(s) == 0 || s[len(s)-1]["routerUid"] != u.uid || s[len(s)-1]["router"] != "up" {
		t.Errorf("status messages: %v", s)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// A take from the page names the router of each route; with no router it is the first router.
func TestPageTakeNamesItsRouter(t *testing.T) {
	a, first, second, u := twoRouterFiles(t)
	var m inMsg
	body := `{"t":"take","items":[{"id":1,"src":4,"dst":2,"levels":[1],"router":"` + u.uid + `"},{"id":2,"src":5,"dst":3,"levels":[1]}]}`
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	a.takeAllowed(&client{ip: "10.0.0.1"}, m.Items)
	wire := a.r().byUI[1].Wire
	first.mu.Lock()
	onFirst := first.tally[[2]int{3, wire}]
	_, wrong := first.tally[[2]int{2, wire}]
	first.mu.Unlock()
	second.mu.Lock()
	onSecond := second.tally[[2]int{2, wire}]
	second.mu.Unlock()
	if onFirst != 5 || onSecond != 4 || wrong {
		t.Errorf("first router dst 3 = %d (dst 2 set: %v), second router dst 2 = %d", onFirst, wrong, onSecond)
	}
}
