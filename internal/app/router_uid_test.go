package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestAddressChangeKeepsRouterUID(t *testing.T) {
	a := futureApp(t, "routers: [{address: 127.0.0.1:1}]\n")
	uid := a.r().cfg.Routers[0].UID
	if len(uid) < 16 || a.r().cfg.Routers[0].Prefix != "A" {
		t.Fatalf("router uid %q prefix %q", uid, a.r().cfg.Routers[0].Prefix)
	}
	mustSave(t, a, adminClient(t, a), map[string]any{aKey(a, "address"): "127.0.0.1:2"})
	if got := a.r().cfg; got.Routers[0].Address != "127.0.0.1:2" || got.Routers[0].UID != uid || got.Routers[0].Prefix != "A" {
		t.Fatalf("after the save: address %q uid %q prefix %q, want uid %q", got.Routers[0].Address, got.Routers[0].UID, got.Routers[0].Prefix, uid)
	}
	file, _, err := config.Load(a.cfgPath)
	if err != nil || file.Routers[0].UID != uid || file.Routers[0].Address != "127.0.0.1:2" {
		t.Fatalf("file: uid %q address %q %v", file.Routers[0].UID, file.Routers[0].Address, err)
	}
	b, _ := os.ReadFile(a.cfgPath)
	if !strings.Contains(string(b), "routers:") {
		t.Errorf("file has no routers list:\n%s", b)
	}
}

// The Settings message has no UID or prefix key: with one router the prefix is hidden. The UID
// reaches browsers only as the hello's routerUid and inside stored references (presets).
func TestConfigMessageHidesRouterUIDAndPrefix(t *testing.T) {
	a := newTestApp(t)
	b, err := json.Marshal(a.r().cfg)
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	for _, k := range []string{"uid", "UID", "prefix", "Prefix", "routers", "Router"} {
		if _, ok := m[k]; ok {
			t.Errorf("config JSON has %q", k)
		}
	}
	// The settings of every router are in the hello's routers list, not flat keys.
	for _, k := range []string{"router", "routerName", "matrix", "levels", "numberOffset", "protectDevice"} {
		if _, ok := m[k]; ok {
			t.Errorf("config JSON has the flat router key %q", k)
		}
	}
	if strings.Contains(string(b), a.r().cfg.Routers[0].UID) {
		t.Errorf("config JSON with no references holds the UID")
	}
}

// The hello's routers list carries the prefix for typing (A.23) and the UID, for the references This browser presets
// store; no page shows the UID. Outside the list it is only in the version keys of its one Save fields.
func TestHelloCarriesPrefixNotUID(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	var m map[string]any
	json.Unmarshal(b, &m)
	if first := firstHelloRouter(t, m); first["prefix"] != "A" || first["uid"] != a.r().cfg.Routers[0].UID {
		t.Errorf("hello first router prefix %v, uid %v", first["prefix"], first["uid"])
	}
	delete(m, "routers")
	delete(m, "versions")
	if b, _ := json.Marshal(m); strings.Contains(string(b), a.r().cfg.Routers[0].UID) {
		t.Error("hello holds the router UID outside the routers list")
	}
}

// A prefix change sends every open page the reload message; a save that keeps the prefix
// sends none.
func TestPrefixChangeReloadsEveryPage(t *testing.T) {
	a := userApp(t, false)
	pages := []*client{{ip: "10.0.0.1", send: make(chan []byte, 64)}, {ip: "10.0.0.2", send: make(chan []byte, 64)}}
	a.mu.Lock()
	for _, c := range pages {
		a.clients[c] = struct{}{}
	}
	a.mu.Unlock()
	reloads := func(c *client) int {
		n := 0
		for {
			select {
			case b := <-c.send:
				if strings.Contains(string(b), `"t":"reload"`) {
					n++
				}
			default:
				return n
			}
		}
	}
	if err := a.ApplyConfig(a.r().cfg.Clone()); err != nil {
		t.Fatal(err)
	}
	for _, c := range pages {
		if n := reloads(c); n != 0 {
			t.Fatalf("%s got %d reloads from a save that kept the prefix", c.ip, n)
		}
	}
	next := a.r().cfg.Clone()
	next.Routers[0].Prefix = "C"
	if err := a.ApplyConfig(next); err != nil {
		t.Fatal(err)
	}
	if a.r().cfg.Routers[0].Prefix != "C" {
		t.Fatalf("prefix %q after the import", a.r().cfg.Routers[0].Prefix)
	}
	for _, c := range pages {
		if n := reloads(c); n != 1 {
			t.Errorf("%s got %d reload messages, want 1", c.ip, n)
		}
	}
}
