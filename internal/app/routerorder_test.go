package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// helloOrder: the UIDs of the hello's routers list, and of the links of health and status.
func helloOrder(a *App) (hello, links []string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, r := range a.helloRouters(a.r()) {
		hello = append(hello, r.UID)
	}
	for _, l := range a.routerLinks() {
		links = append(links, l.UID)
	}
	return hello, links
}

// routers/order puts the routers in a new order. The first router of the list is the default router: a blank UID
// names it. No router reconnects, and each unit keeps its link, names and tally. The order is not Critical.
func TestRoutersOrderKeepsUnits(t *testing.T) {
	a, fs := threeRouters(t)
	uids := a.r().cfg.RouterUIDs()
	units := []*routerUnit{fs[0].u, fs[1].u, fs[2].u}
	for i, f := range fs {
		f.restore()
		f.seed(5, i+1, 0)
	}
	a.mu.Lock()
	units[2].router.srcNames = map[int]string{0: "C CAM"}
	a.mu.Unlock()
	sends := []int{fs[0].sends(), fs[1].sends(), fs[2].sends()}
	live := a.r().cfg

	want := []string{uids[2], uids[0], uids[1]}
	res, err := a.saveKeys(adminClient(t, a), map[string]any{routersOrderKey: want})
	if k, r := firstRefused(res); err != nil || k != "" {
		t.Fatalf("save: %v %s %+v", err, k, r)
	}
	if got := a.r().cfg.RouterUIDs(); !slices.Equal(got, want) {
		t.Fatalf("order %v, want %v", got, want)
	}
	if e := criticalEdits(live, a.r().cfg, config.User{Role: config.RoleAdministrator}); len(e) != 0 {
		t.Errorf("a reorder is Critical: %v", e)
	}
	if a.firstUnit() != units[2] {
		t.Errorf("first unit %s, want router C", a.firstUnit().uid())
	}
	if u, _ := a.unit(""); u != units[2] || a.unitOrFirst("") != units[2] {
		t.Error("a blank UID is not the new first router")
	}
	for i, u := range units {
		if got, _ := a.unit(uids[i]); got != u {
			t.Errorf("router %d has another unit", i)
		}
	}
	a.mu.Lock()
	for i, u := range units {
		if got := u.router.tally[5][1]; got != i+1 {
			t.Errorf("router %d tally %d, want %d", i, got, i+1)
		}
		if u.status.Router != string(swp08.StateUp) {
			t.Errorf("router %d status %q", i, u.status.Router)
		}
	}
	name := units[2].router.srcNames[0]
	a.mu.Unlock()
	if name != "C CAM" {
		t.Errorf("router C's names went: %q", name)
	}
	for i, f := range fs {
		if f.sends() != sends[i] {
			t.Errorf("router %d sent %d messages after the reorder", i, f.sends()-sends[i])
		}
	}
	if hello, links := helloOrder(a); !slices.Equal(hello, want) || !slices.Equal(links, want) {
		t.Errorf("hello order %v, links %v", hello, links)
	}

	// A save from the old order is a conflict.
	stale, _ := json.Marshal(uids)
	res, _, err = a.saveSettingsFields(adminClient(t, a), map[string]fieldEdit{routersOrderKey: {Value: stale, Version: fieldVersion(live, routersOrderKey)}}, nil, "", "")
	if err != nil || res[routersOrderKey].Reason != reasonConflict {
		t.Errorf("stale order: %v %+v", err, res[routersOrderKey])
	}
}

// A router that the order does not name (one that the same save adds) comes after the named ones.
func TestReorderRoutersKeepsUnnamed(t *testing.T) {
	a, _ := threeRouters(t)
	rs := a.r().cfg.Routers
	got := reorderRouters(rs, []string{rs[1].UID, "nope", rs[0].UID})
	if ids := []string{got[0].UID, got[1].UID, got[2].UID}; !slices.Equal(ids, []string{rs[1].UID, rs[0].UID, rs[2].UID}) {
		t.Fatalf("order %v", ids)
	}
}

// Remove… works on the first router while another router stays. The next router is then the default router, and a
// blank UID names it. The last router stays.
func TestRemoveFirstRouter(t *testing.T) {
	a, fs := threeRouters(t)
	uids := a.r().cfg.RouterUIDs()
	b := fs[1].u
	for _, f := range fs {
		f.restore()
	}
	res, err := a.saveKeys(adminClient(t, a), map[string]any{"router/" + uids[0] + "/remove": true})
	if k, r := firstRefused(res); err != nil || k != "" {
		t.Fatalf("save: %v %s %+v", err, k, r)
	}
	if got := a.r().cfg.RouterUIDs(); !slices.Equal(got, uids[1:]) {
		t.Fatalf("routers %v", got)
	}
	if a.firstUnit() != b || a.unitOrFirst("") != b {
		t.Errorf("first unit %s, want router B", a.firstUnit().uid())
	}
	if _, ok := a.units()[uids[0]]; ok || len(a.units()) != 2 {
		t.Errorf("units %v", a.units())
	}
	a.mu.Lock()
	up := b.status.Router
	a.mu.Unlock()
	if up != string(swp08.StateUp) {
		t.Errorf("router B reconnected: %q", up)
	}
	if hello, _ := helloOrder(a); !slices.Equal(hello, uids[1:]) {
		t.Errorf("hello order %v", hello)
	}

	// Both routers in one save: the last one stays, so nothing goes.
	res, _ = a.saveKeys(adminClient(t, a), map[string]any{"router/" + uids[1] + "/remove": true, "router/" + uids[2] + "/remove": true})
	if k, r := firstRefused(res); k == "" || !strings.Contains(r.Err, "the last router stays") {
		t.Errorf("remove every router: %s %+v", k, r)
	}
	if len(a.r().cfg.Routers) != 2 {
		t.Errorf("routers %v", a.r().cfg.RouterUIDs())
	}
}

// The uptime file goes to the router it was written for, also after a reorder. A file of a router that Sode no
// longer has starts empty, with the first router.
func TestUptimeFileFollowsItsRouter(t *testing.T) {
	a, _ := threeRouters(t)
	uids := a.r().cfg.RouterUIDs()
	dir := t.TempDir()
	path := filepath.Join(dir, "uptime.jsonl")
	if err := os.WriteFile(uptimeRouterPath(path), []byte(uids[1]+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	up, err := OpenUptime(path)
	if err != nil {
		t.Fatal(err)
	}
	a.SetUptime(up)
	if b, _ := a.unit(uids[1]); b.up != up {
		t.Error("the file did not go to router B")
	}

	a2, _ := threeRouters(t)
	path2 := filepath.Join(dir, "other.jsonl")
	if err := os.WriteFile(path2, []byte(`{"t":1,"s":"up"}`+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(uptimeRouterPath(path2), []byte("gone0000gone0000\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	up2, err := OpenUptime(path2)
	if err != nil {
		t.Fatal(err)
	}
	a2.SetUptime(up2)
	if a2.firstUnit().up != up2 {
		t.Error("the file of a gone router did not go to the first router")
	}
	up2.mu.Lock()
	n := len(up2.trans)
	up2.mu.Unlock()
	if n != 1 {
		t.Errorf("the history of a gone router stays: %d transitions", n)
	}
	if b, _ := os.ReadFile(uptimeRouterPath(path2)); strings.TrimSpace(string(b)) != a2.firstUnit().uid() {
		t.Errorf("router file %q", b)
	}
}
