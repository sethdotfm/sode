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
	uid := a.r().cfg.UID
	if len(uid) < 16 || a.r().cfg.Prefix != "A" {
		t.Fatalf("router uid %q prefix %q", uid, a.r().cfg.Prefix)
	}
	if err := decodeAndSave(a, adminClient(t, a), []byte(`{"router":"127.0.0.1:2"}`)); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg; got.Address != "127.0.0.1:2" || got.UID != uid || got.Prefix != "A" {
		t.Fatalf("after the save: address %q uid %q prefix %q, want uid %q", got.Address, got.UID, got.Prefix, uid)
	}
	file, _, err := config.Load(a.cfgPath)
	if err != nil || file.UID != uid || file.Address != "127.0.0.1:2" {
		t.Fatalf("file: uid %q address %q %v", file.UID, file.Address, err)
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
	if m["router"] != a.r().cfg.Address || m["routerName"] == nil || m["matrix"] == nil {
		t.Errorf("flat router keys missing: %v", m)
	}
	if strings.Contains(string(b), a.r().cfg.UID) {
		t.Errorf("config JSON with no references holds the UID")
	}
}

// The hello carries the prefix for typing (A.23). The UID goes only as routerUid, for the
// references This browser presets store, and in the routers list; no page shows it.
func TestHelloCarriesPrefixNotUID(t *testing.T) {
	a := newTestApp(t)
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	var m map[string]any
	json.Unmarshal(b, &m)
	if m["prefix"] != "A" {
		t.Errorf("hello prefix %v", m["prefix"])
	}
	if m["routerUid"] != a.r().cfg.UID {
		t.Errorf("hello routerUid %v", m["routerUid"])
	}
	// The routers list names each router by UID too (TestHelloListsEveryRouter).
	delete(m, "routers")
	if b, _ := json.Marshal(m); strings.Count(string(b), a.r().cfg.UID) != 1 {
		t.Error("hello holds the router UID outside routerUid")
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
	next.Prefix = "C"
	if err := a.ApplyConfig(next); err != nil {
		t.Fatal(err)
	}
	if a.r().cfg.Prefix != "C" {
		t.Fatalf("prefix %q after the import", a.r().cfg.Prefix)
	}
	for _, c := range pages {
		if n := reloads(c); n != 1 {
			t.Errorf("%s got %d reload messages, want 1", c.ip, n)
		}
	}
}
