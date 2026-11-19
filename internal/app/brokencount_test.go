package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// brokenCountApp: swapApp with Studio Broken, Spare both Broken and Name moved (it counts as Broken), Bo's "Keep"
// Name moved, Bo's filter set Broken and Bo's panel Broken.
func brokenCountApp(t *testing.T) *App {
	t.Helper()
	a, uid := swapApp(t)
	mark := []config.Missing{{Kind: config.MissingDestination, Ref: config.Ref{Router: uid, N: 22, Name: "MON 23"}}}
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Presets[1].Broken, c.Presets[1].Moved = mark, mark
		for i := range c.Users {
			if u := &c.Users[i]; u.Name == "Bo" {
				u.Profile.Presets[1].Moved = mark
				u.Profile.FilterSets[0].Broken = []config.Missing{{Kind: config.MissingDestination, Ref: config.Ref{Router: uid, N: -1, Name: "MON 23"}}}
			}
		}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	if err := a.panels.rewrite(func(p *Panel) bool {
		if p.Name != "Bo desk" {
			return false
		}
		p.Broken = mark
		return true
	}); err != nil {
		t.Fatal(err)
	}
	return a
}

type countRow struct {
	broken, moved int
	names         map[string]string // item name to status
}

func brokenCounts(t *testing.T, r map[string]any) countRow {
	t.Helper()
	if r["ok"] != true {
		t.Fatalf("brokenCount: %v", r)
	}
	routers, _ := r["routers"].([]any)
	if len(routers) != 1 {
		t.Fatalf("routers = %v", r["routers"])
	}
	rt := routers[0].(map[string]any)
	out := countRow{broken: int(rt["broken"].(float64)), moved: int(rt["moved"].(float64)), names: map[string]string{}}
	items, _ := rt["items"].([]any)
	for _, x := range items {
		it := x.(map[string]any)
		out.names[it["name"].(string)] = it["status"].(string)
	}
	return out
}

// The Settings > Routers count covers everyone's items; My user's covers only the user's own.
func TestBrokenCountPerRouterAndMine(t *testing.T) {
	a := brokenCountApp(t)
	admin := dialPeer(t, a)
	admin.signIn("Ada", "2468")
	all := brokenCounts(t, admin.ask("brokenCount", 1, map[string]any{}))
	if all.broken != 4 || all.moved != 1 {
		t.Errorf("everyone: %d Broken, %d Name moved; want 4 and 1 (%v)", all.broken, all.moved, all.names)
	}
	want := map[string]string{"Studio": "broken", "Spare": "broken", "Keep": "moved", "Mine": "broken", "Bo desk": "broken"}
	for name, st := range want {
		if all.names[name] != st {
			t.Errorf("%s: status %q, want %q", name, all.names[name], st)
		}
	}
	if len(all.names) != len(want) {
		t.Errorf("items = %v, want only %v", all.names, want)
	}

	bo := dialPeer(t, a)
	bo.signIn("Bo", "567890")
	mine := brokenCounts(t, bo.ask("brokenCount", 1, map[string]any{"mine": true}))
	if mine.broken != 2 || mine.moved != 1 {
		t.Errorf("Bo: %d Broken, %d Name moved; want 2 and 1 (%v)", mine.broken, mine.moved, mine.names)
	}
	if r := bo.ask("brokenCount", 2, map[string]any{}); r["ok"] == true {
		t.Errorf("a Standard user counted everyone's items: %v", r)
	}
}

// Each listed item says how to reach the part to fix: Swap's side and number for its first Broken reference.
func TestBrokenCountNamesTheSwap(t *testing.T) {
	a := brokenCountApp(t)
	admin := dialPeer(t, a)
	admin.signIn("Ada", "2468")
	r := admin.ask("brokenCount", 1, map[string]any{})
	rt := r["routers"].([]any)[0].(map[string]any)
	for _, x := range rt["items"].([]any) {
		it := x.(map[string]any)
		if it["name"] != "Studio" {
			continue
		}
		fix, _ := it["swap"].(map[string]any)
		if fix["side"] != "destination" || fix["from"] != float64(22) {
			t.Fatalf("Studio swap = %v", it["swap"])
		}
		if it["id"] != "preset/everyone/p1" || it["kind"] != "preset" {
			t.Fatalf("Studio = %v", it)
		}
		return
	}
	t.Fatal("Studio not listed")
}
