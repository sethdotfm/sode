package app

import (
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Add router saves through the one Save. The new router gets a UID, the free prefix and a title. It gets a unit and
// its Router link. The hello lists it with the versions of its keys. A new router changes the router connection, so it
// needs the PIN recheck.
func TestAddRouterInSettings(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	key := "router/new:1/add"
	add := map[string]any{"address": "10.0.0.7:8910", "matrix": 2}
	wantRefused(t, p.saveFields(1, map[string]any{key: add}), key, "needs PIN recheck")
	passed(t, "recheck", p.recheck("2468"))
	r := p.saveFields(2, map[string]any{key: add})
	wantSaved(t, r, key)

	cfg := a.r().cfg
	if len(cfg.Routers) != 2 {
		t.Fatalf("routers %+v", cfg.Routers)
	}
	b := cfg.Routers[1]
	if b.UID == "" || b.UID == cfg.Routers[0].UID || b.Prefix != "B" || b.Title != "Router B" || b.Address != "10.0.0.7:8910" ||
		b.Matrix != 2 || b.Levels != config.Defaults().Routers[0].Levels {
		t.Fatalf("new router %+v", b)
	}
	res, _ := r["results"].(map[string]any)
	val, _ := res[key].(map[string]any)["value"].(map[string]any)
	vers, _ := val["versions"].(map[string]any)
	if val["id"] != b.UID || vers["router/"+b.UID+"/remove"] == nil || vers["router/"+b.UID+"/levels"] == nil {
		t.Fatalf("add result %v", val)
	}
	u, ok := a.unit(b.UID)
	if !ok || u.sw == nil || u.status.Addr != "10.0.0.7:8910" {
		t.Fatalf("unit of the new router: %v %+v", ok, u)
	}
	p = dialPeer(t, a)
	routers, _ := p.hello["routers"].([]any)
	hv, _ := p.hello["versions"].(map[string]any)
	if len(routers) != 2 || routers[1].(map[string]any)["prefix"] != "B" || hv["router/"+cfg.Routers[0].UID+"/prefix"] == nil {
		t.Fatalf("hello routers %v", routers)
	}

	// A prefix that a router has is refused; a third router gets C.
	p.signIn("Ada", "2468")
	passed(t, "recheck", p.recheck("2468"))
	wantRefused(t, p.saveFields(3, map[string]any{"router/new:2/add": map[string]any{"prefix": "a"}}), "router/new:2/add", "invalid")
	wantSaved(t, p.saveFields(4, map[string]any{"router/new:3/add": map[string]any{"title": "Stage 2"}}), "router/new:3/add")
	if c := a.r().cfg.Routers[2]; c.Prefix != "C" || c.Title != "Stage 2" {
		t.Errorf("third router %+v", c)
	}
}

// The prefix, title, address and matrix of a router after the first save as router/<uid>/<attribute>. A new address
// reconnects only that router.
func TestRouterListFields(t *testing.T) {
	a, uid := routerLevelsApp(t)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	passed(t, "recheck", p.recheck("2468"))
	k := func(attr string) string { return "router/" + uid + "/" + attr }
	r := p.saveFields(1, map[string]any{k("prefix"): "BA", k("title"): "Stage 2", k("address"): "10.0.0.8:8910", k("matrix"): 1})
	for _, at := range []string{"prefix", "title", "address", "matrix"} {
		wantSaved(t, r, k(at))
	}
	b := a.r().cfg.Routers[1]
	if b.Prefix != "BA" || b.Title != "Stage 2" || b.Address != "10.0.0.8:8910" || b.Matrix != 1 {
		t.Fatalf("router %+v", b)
	}
	if u, _ := a.unit(uid); u.status.Addr != "10.0.0.8:8910" || a.firstUnit().status.Addr == "10.0.0.8:8910" {
		t.Errorf("router B's unit at %q, the first router's at %q", u.status.Addr, a.firstUnit().status.Addr)
	}
	// The first router's fields have the same keys.
	first := a.r().cfg.Routers[0].UID
	wantSaved(t, p.saveFields(2, map[string]any{"router/" + first + "/prefix": "S"}), "router/"+first+"/prefix")
	if a.r().cfg.Routers[0].Prefix != "S" {
		t.Errorf("first prefix %q", a.r().cfg.Routers[0].Prefix)
	}
	if spec, _ := lookupField("router/" + first + "/address"); spec.get(a.r().cfg) == nil {
		t.Error("the first router's address has no key")
	}
}

// Remove router lists the items it breaks and needs the PIN recheck. Presets, filter sets, panel cells and Grid
// members that name the router then become Broken (kind router). Rights and panel level subsets lose that router.
// Its Default artwork goes and its unit stops. The first router cannot go.
func TestRemoveRouterInSettings(t *testing.T) {
	a, uid := routerLevelsApp(t)
	first := a.r().cfg.Routers[0].UID
	ana := idOf(a, "Ana")
	onB := config.Ref{Router: uid, N: 4, Name: "B CAM"}
	onA := config.Ref{Router: first, N: 1, Name: "CAM 2"}
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error {
		c.Presets = []config.Preset{
			{ID: "p1", Name: "Stage", Items: []config.PresetItem{{Src: onB, Dsts: []config.Ref{onB}}}},
			{ID: "p2", Name: "Home", Items: []config.PresetItem{{Src: onA, Dsts: []config.Ref{onA}}}},
		}
		c.FilterSets = []config.FilterSet{{ID: "f1", Name: "B cams", Src: "4", Router: uid, Sode: true}}
		c.TieLines = []config.TieLine{{ID: "t1", Name: "TIE 3", From: onA, To: onB}}
		i := userIndex(c, ana)
		c.Users[i].Rights = config.Rights{Sources: &[]config.Ref{onA, onB}, Levels: config.RouterLevels{first: {1}, uid: {2}}}
		c.Users[i].Profile.Presets = []config.Preset{{ID: "p3", Name: "Mine", Items: []config.PresetItem{{Src: onA, Dsts: []config.Ref{onA}}, {Src: onB, Dsts: []config.Ref{onB}}}}}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	desk, err := a.panels.create(Panel{Name: "Desk", Owner: ana, Rows: 1, Cols: 1,
		Settings: map[string]any{panelLevelsKey: map[string]any{first: []any{1}, uid: []any{2}}},
		Pages:    []PanelPage{{ID: "pg-1", Enabled: true, Cells: []map[string]any{{"kind": "source", "router": uid, "n": 4, "name": "B CAM"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	home, err := a.panels.create(Panel{Name: "Home", Owner: ana, Rows: 1, Cols: 1,
		Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: []map[string]any{{"kind": "source", "router": first, "n": 1, "name": "CAM 2"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	n := 4
	if _, err := a.artDefaults.set(ArtDefault{Kind: artKindSource, Router: uid, N: &n, Name: "B CAM", Art: "icon:camera"}, nil); err != nil {
		t.Fatal(err)
	}

	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	uses := p.ask("routerUses", 1, map[string]any{"router": uid})
	items, _ := uses["items"].([]any)
	var names []string
	for _, it := range items {
		m := it.(map[string]any)
		names = append(names, m["kind"].(string)+" "+m["name"].(string))
	}
	if want := []string{"filterSet B cams", "panel Desk", "preset Stage", "preset Mine", "tieLine TIE 3"}; !slices.Equal(names, want) {
		t.Fatalf("routerUses %v, want %v", names, want)
	}

	key := "router/" + uid + "/remove"
	wantRefused(t, p.saveFields(2, map[string]any{key: true}), key, "needs PIN recheck")
	passed(t, "recheck", p.recheck("2468"))
	wantSaved(t, p.saveFields(3, map[string]any{key: true}), key)

	cfg := a.r().cfg
	if len(cfg.Routers) != 1 {
		t.Fatalf("routers left %+v", cfg.Routers)
	}
	if len(cfg.TieLines) != 0 {
		t.Errorf("tie lines left %+v", cfg.TieLines)
	}
	if _, ok := a.unit(uid); ok || len(a.units()) != 1 {
		t.Errorf("the unit of the removed router stays")
	}
	routerMark := func(l []config.Missing) bool {
		return slices.ContainsFunc(l, func(m config.Missing) bool { return m.Kind == config.MissingRouter && m.Ref.Router == uid })
	}
	if !routerMark(cfg.Presets[0].Broken) || len(cfg.Presets[1].Broken) != 0 || !routerMark(cfg.FilterSets[0].Broken) {
		t.Errorf("Everyone presets %+v, filter sets %+v", cfg.Presets, cfg.FilterSets)
	}
	ua, _ := cfg.FindUserByID(ana)
	if !routerMark(ua.Profile.Presets[0].Broken) {
		t.Errorf("Ana's preset %+v", ua.Profile.Presets[0])
	}
	if l := *ua.Rights.Sources; len(l) != 1 || l[0].Router != first || len(ua.Rights.Levels) != 1 || ua.Rights.Levels[uid] != nil {
		t.Errorf("Ana's Rights %+v", ua.Rights)
	}
	got, _ := a.panels.get(desk.ID)
	if !routerMark(got.Broken) {
		t.Errorf("panel Desk broken %+v", got.Broken)
	}
	if by, _ := got.Settings[panelLevelsKey].(map[string]any); len(by) != 1 || by[uid] != nil {
		t.Errorf("panel Desk levels %v", got.Settings[panelLevelsKey])
	}
	if h, _ := a.panels.get(home.ID); len(h.Broken) != 0 || h.Rev != home.Rev {
		t.Errorf("panel Home changed: %+v", h)
	}
	if l := a.artDefaults.list(); len(l) != 0 {
		t.Errorf("Default artwork %+v", l)
	}

	// The last router stays.
	k := "router/" + first + "/remove"
	wantRefused(t, p.saveFields(4, map[string]any{k: true}), k, "invalid")
}

// A router delete removes only the tie lines with an end on that router.
func TestDropRouterRemovesItsTieLines(t *testing.T) {
	c := config.Defaults()
	c.Routers = append(c.Routers[:1:1], []config.Router{moreRouter("B", "B", "10.0.0.8:8910", 1), moreRouter("C", "C", "10.0.0.9:8910", 1)}...)
	c.Normalize()
	a, b, cc := c.Routers[0].UID, c.Routers[1].UID, c.Routers[2].UID
	c.TieLines = []config.TieLine{
		{ID: "ab", Name: "AB", From: config.Ref{Router: a, N: 1}, To: config.Ref{Router: b, N: 1}},
		{ID: "ca", Name: "CA", From: config.Ref{Router: cc, N: 1}, To: config.Ref{Router: a, N: 2}},
		{ID: "bc", Name: "BC", From: config.Ref{Router: b, N: 2}, To: config.Ref{Router: cc, N: 2}},
	}
	saved := config.CloneTieLines(c.TieLines)
	dropRouter(&c, b)
	if len(c.TieLines) != 1 || c.TieLines[0].ID != "ca" {
		t.Fatalf("tie lines %+v", c.TieLines)
	}
	if len(saved) != 3 || saved[0].ID != "ab" {
		t.Errorf("the drop changed the list it was given")
	}
	if err := c.Validate(); err != nil {
		t.Errorf("Validate: %v", err)
	}
}

// Remove router drops the number and range terms of the hidden destinations when they are numbers of that router.
// Name terms stay. If no term that hides remains, the text goes, so the hidden destinations never get wider.
func TestDropRouterNarrowsHiddenDestinations(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"9", ""},
		{"4, 9-12", ""},
		{"9, !CAM", ""},
		{"9, MON", "MON"},
		{`9, "MON 2", !3`, `"MON 2"`},
		{"/^MON/i, 9", "/^MON/i"},
		{"MON, !3", "MON"},
		{"'unclosed", ""},
	} {
		cfg := config.Defaults()
		cfg.Routers = append(cfg.Routers[:1:1], []config.Router{moreRouter("B", "B", "10.0.0.8:8910", 1)}...)
		cfg.Normalize()
		b := cfg.Routers[1].UID
		cfg.HiddenDsts, cfg.HiddenDstsRouter = c.in, b
		dropRouter(&cfg, b)
		if cfg.HiddenDsts != c.want || cfg.HiddenDstsRouter != "" {
			t.Errorf("%q: hidden %q on %q, want %q", c.in, cfg.HiddenDsts, cfg.HiddenDstsRouter, c.want)
		}
	}
	// The hidden destinations of another router stay as they are.
	cfg := config.Defaults()
	cfg.Routers = append(cfg.Routers[:1:1], []config.Router{moreRouter("B", "B", "10.0.0.8:8910", 1)}...)
	cfg.Normalize()
	cfg.HiddenDsts, cfg.HiddenDstsRouter = "9, !3", cfg.Routers[0].UID
	dropRouter(&cfg, cfg.Routers[1].UID)
	if cfg.HiddenDsts != "9, !3" || cfg.HiddenDstsRouter != cfg.Routers[0].UID {
		t.Errorf("first router's: %q on %q", cfg.HiddenDsts, cfg.HiddenDstsRouter)
	}
}

// With two routers, a name read of one router keeps the marks of references on the other router. A reference on a
// router that Sode does not have is Broken.
func TestBrokenCheckKeepsOtherRouterMarks(t *testing.T) {
	k := &pollCheck{uid: "aa", routers: []string{"aa", "bb"}, prev: namePoll{map[int]string{}, map[int]string{}}, cur: namePoll{map[int]string{}, map[int]string{}}}
	onB := config.Ref{Router: "bb", N: 3, Name: "B CAM"}
	mark := config.Missing{Kind: config.MissingSource, Ref: onB}
	p := config.Preset{Name: "P", Broken: []config.Missing{mark}, Items: []config.PresetItem{{Src: onB, Dsts: []config.Ref{{Router: "cc", N: 1}}, Levels: []int{2}}}}
	k.preset(&p, "Everyone")
	if len(p.Broken) != 2 || !missingEqual(p.Broken[:1], []config.Missing{mark}) || p.Broken[1].Kind != config.MissingRouter || p.Broken[1].Ref.Router != "cc" {
		t.Fatalf("broken %+v", p.Broken)
	}
}

// The detail view of a new router sends every setting with Add router. Each one is in the new router; a value that
// does not fit refuses the add.
func TestAddRouterTakesEverySetting(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	passed(t, "recheck", p.recheck("2468"))
	key := "router/new:1/add"
	add := map[string]any{"prefix": "B", "title": "Stage 2", "address": "10.0.0.7:8910", "matrix": 20, "nameRequests": "ext", "levels": 4,
		"levelWireOffset": 0, "levelNumbering": "va", "defaultLevels": "1-2", "numberOffset": 0, "nameRereadSeconds": 120, "nameLength": 16,
		"tallyResyncSeconds": 60, "fireMode": "sequential", "salvoGroup": 9, "protectDevice": 201}
	wantSaved(t, p.saveFields(1, map[string]any{key: add}), key)
	b := a.r().cfg.Routers[1]
	if b.Matrix != 20 || b.NameRequests != "ext" || b.Levels != 4 || b.LevelWireOffset != 0 || b.LevelNumbering != "va" || b.DefaultLevels != "1-2" ||
		b.NumberOffset != 0 || b.NameRefreshSeconds != 120 || b.NameLength != 16 || b.TallyResyncSeconds != 60 || b.SalvoGroup != 9 || b.ProtectDevice != 201 {
		t.Fatalf("new router %+v", b)
	}
	wantRefused(t, p.saveFields(2, map[string]any{"router/new:2/add": map[string]any{"protectDevice": 9000}}), "router/new:2/add", "invalid")
}
