package app

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A Settings save that changes the address keeps the router UID in the runtime and the file.
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
