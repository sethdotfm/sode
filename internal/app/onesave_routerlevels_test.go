package app

import (
	"path/filepath"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// routerLevelsApp is userApp with a second router, B, that has 17 levels.
func routerLevelsApp(t *testing.T) (*App, string) {
	t.Helper()
	cfg := config.Defaults()
	cfg.Routers = append(cfg.Routers[:1:1], []config.Router{moreRouter("B", "Second", "10.0.0.9:8910", 17)}...)
	cfg.Normalize()
	cfg.Users = []config.User{mkUser(t, "Ana", "1234")}
	withAdministrator(&cfg)
	hist, _ := OpenHistory("")
	sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, filepath.Join(settledTempDir(t), "config.yaml"), hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	return a, a.r().cfg.Routers[1].UID
}

// The level settings of a router after the first save with the one Save, as keys router/<uid>/<attribute>. The hello
// carries their versions. A new level count is a router connection field, so it needs the PIN recheck. It saves
// together with the new default levels of its router.
func TestOneSaveRouterLevels(t *testing.T) {
	a, uid := routerLevelsApp(t)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	key := func(attr string) string { return "router/" + uid + "/" + attr }
	vers, _ := p.hello["versions"].(map[string]any)
	for _, at := range routerAttrNames() {
		if vers[key(at)] == nil {
			t.Fatalf("the hello has no version for %s: %v", key(at), vers)
		}
	}

	r := p.saveFields(1, map[string]any{key("levelNumbering"): "va", key("defaultLevels"): "1-4"})
	wantSaved(t, r, key("levelNumbering"))
	wantSaved(t, r, key("defaultLevels"))
	b := a.r().cfg.Routers[1]
	if b.LevelNumbering != "va" || b.DefaultLevels != "1-4" || a.r().cfg.Routers[0].LevelNumbering != "1" {
		t.Fatalf("router B %q %q, first router numbering %q", b.LevelNumbering, b.DefaultLevels, a.r().cfg.Routers[0].LevelNumbering)
	}
	if l := a.r().levelsOf(uid).levels[0].Label; l != "V" {
		t.Errorf("router B level 1 label %q", l)
	}

	p = dialPeer(t, a)
	p.signIn("Ada", "2468")
	wantRefused(t, p.saveFields(2, map[string]any{key("levels"): 5}), key("levels"), "needs PIN recheck")
	passed(t, "recheck", p.recheck("2468"))
	// Alone, 1 level does not fit the default levels 1-4.
	r = p.saveFields(3, map[string]any{key("levels"): 1, key("defaultLevels"): "1"})
	wantSaved(t, r, key("levels"))
	wantSaved(t, r, key("defaultLevels"))
	if n := len(a.r().levelsOf(uid).levels); n != 1 || a.r().cfg.Routers[0].Levels != 17 {
		t.Errorf("router B has %d levels, the first router %d", n, a.r().cfg.Routers[0].Levels)
	}

	// A level count that the default levels do not fit is refused; an unknown router or attribute is no key.
	r = p.saveFields(4, map[string]any{key("defaultLevels"): "1-5"})
	wantRefused(t, r, key("defaultLevels"), "invalid")
	if _, ok := lookupField("router/" + uid + "/routerShow"); ok {
		t.Error("router/<uid>/routerShow is a key")
	}
	if spec, _ := lookupField("router/b0b0/levels"); spec.get(a.r().cfg) != nil {
		t.Error("a router that is not there has a value")
	}
}

// Blank Default levels are every level. They save blank, and stay blank in the file, the hello and GET /tally after
// the level count changes.
func TestOneSaveBlankDefaultLevels(t *testing.T) {
	a, uid := routerLevelsApp(t)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	key := func(attr string) string { return "router/" + uid + "/" + attr }
	wantSaved(t, p.saveFields(1, map[string]any{key("defaultLevels"): "1-4"}), key("defaultLevels"))
	wantSaved(t, p.saveFields(2, map[string]any{key("defaultLevels"): ""}), key("defaultLevels"))
	passed(t, "recheck", p.recheck("2468"))
	wantSaved(t, p.saveFields(3, map[string]any{key("levels"): 5}), key("levels"))

	b, _ := a.r().cfg.RouterByUID(uid)
	if b.DefaultLevels != "" || b.Levels != 5 {
		t.Fatalf("router B default levels %q, %d levels", b.DefaultLevels, b.Levels)
	}
	if got := b.DefaultLevelList(); len(got) != 5 || got[0] != 1 || got[4] != 5 {
		t.Errorf("router B default level list %v", got)
	}
	disk, _, err := config.Load(a.cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if r, _ := disk.RouterByUID(uid); r.DefaultLevels != "" {
		t.Errorf("the file has default levels %q", r.DefaultLevels)
	}

	p = dialPeer(t, a)
	p.signIn("Ada", "2468")
	routers, _ := p.hello["routers"].([]any)
	found := false
	for _, x := range routers {
		if r, _ := x.(map[string]any); r["uid"] == uid {
			found = true
			if v, ok := r["defaultLevels"]; !ok || v != "" {
				t.Errorf("the hello's router B default levels %v (%t)", v, ok)
			}
		}
	}
	if !found {
		t.Fatalf("the hello has no router B: %v", routers)
	}
	a.mu.Lock()
	tally := a.tallySnapshot(a.unitOrFirst(uid), apiView{})
	a.mu.Unlock()
	if tally["defaultLevels"] != "" {
		t.Errorf("GET /tally default levels %v", tally["defaultLevels"])
	}
}

// With two or more routers, log lines and the Settings saves record name a router as "<prefix> <title>". The record
// shows Default levels in the router's Level numbering.
func TestRouterNameAndRecordLevels(t *testing.T) {
	a, uid := routerLevelsApp(t)
	u, _ := a.unit(uid)
	if got := a.routerName(a.firstUnit()); got != "Router A" {
		t.Errorf("first router %q", got)
	}
	if got, sw := a.routerName(u), u.sw.Name(); got != "B Second" || sw != got {
		t.Errorf("router B %q, link %q", got, sw)
	}
	cfg := a.r().cfg
	b := &cfg.Routers[1]
	b.LevelNumbering, b.DefaultLevels = "va", "1-3"
	key := "router/" + uid + "/defaultLevels"
	if got := string(recordJSON(cfg, key)); got != `"V, 1-2"` {
		t.Errorf("record value %s", got)
	}
	if got := itemLabel(key, cfg, cfg); got != "B Second: Default levels" {
		t.Errorf("record label %q", got)
	}

	one := config.Defaults()
	one.Routers[0].Title = "Main"
	one.Normalize()
	a1, err := New(one, "", nil, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := a1.routerName(a1.firstUnit()); got != "Main" {
		t.Errorf("one router %q", got)
	}
}
