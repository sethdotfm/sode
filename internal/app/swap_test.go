package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// swapApp: Ada (Administrator), Ana and Bo (Standard), 50 named sources and destinations read twice. Destination
// 23 as shown is N 22 (numberOffset 1). Every item below uses it, except Bo's "Keep".
func swapApp(t *testing.T) (*App, string) {
	t.Helper()
	a := futureApp(t, "routers: [{address: 127.0.0.1:1}]\nusers:\n  - name: a\n  - name: b\n")
	var srcs, dsts []string
	for i := 1; i <= 50; i++ {
		srcs, dsts = append(srcs, fmt.Sprintf("CAM %d", i)), append(dsts, fmt.Sprintf("MON %d", i))
	}
	readNames(a, srcs, dsts)
	readNames(a, srcs, dsts)
	uid := a.r().cfg.UID
	ref := func(n int, name string) config.Ref { return config.Ref{Router: uid, N: n, Name: name} }
	preset := func(id, name string, dsts ...config.Ref) config.Preset {
		return config.Preset{ID: id, Name: name, Items: []config.PresetItem{{Src: ref(0, "CAM 1"), Dsts: dsts}}}
	}
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Presets = []config.Preset{
			preset("p1", "Studio", ref(22, "MON 23"), ref(39, "MON 40")),
			preset("p2", "Spare", ref(22, "MON 23")),
		}
		c.Presets[0].Broken = []config.Missing{{Kind: config.MissingDestination, Ref: ref(22, "MON 23")}}
		c.FilterSets = []config.FilterSet{{Name: "Desk", Dst: `23, !"MON 23", MON`, Sode: true}}
		for i := range c.Users {
			u := &c.Users[i]
			switch u.Name {
			case "Ana":
				u.Profile.Presets = []config.Preset{preset("a1", "Ana's", ref(22, "MON 23"))}
			case "Bo":
				u.Profile.Presets = []config.Preset{preset("b1", "Bo's", ref(22, "MON 23")), preset("b2", "Keep", ref(5, "MON 6"))}
				u.Profile.FilterSets = []config.FilterSet{{Name: "Mine", Dst: `"MON 23"`, Sode: true}}
			}
		}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	cells := []map[string]any{{"kind": "destination", "router": uid, "n": 22, "name": "MON 23"}}
	for _, who := range []string{"Ana", "Bo"} {
		if _, err := a.panels.create(Panel{Name: who + " desk", Owner: userID(t, a, who), Rows: 1, Cols: 1,
			Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: cells}}}); err != nil {
			t.Fatal(err)
		}
	}
	return a, uid
}

// A Request approval card shows the staged Swap with every count, panels too, and writes nothing.
func TestSwapShowsOnApprovalCard(t *testing.T) {
	a, _ := swapApp(t)
	req, _ := json.Marshal(swapReq{Side: config.MissingDestination, From: 22, To: 39,
		Items: []string{"preset/everyone/p1", "panel/" + panelNamed(t, a, "Ana desk").ID}})
	ch, _ := a.askChanges(map[string]fieldEdit{swapKey: {Value: req}}, config.RoleAdministrator)
	if len(ch) != 1 || ch[0].Key != swapKey || string(ch[0].New) != `{"Ana":1,"Everyone":1}` {
		t.Fatalf("card changes: %+v", ch)
	}
	if dstCellN(panelNamed(t, a, "Ana desk")) != 22 {
		t.Fatal("the card wrote the panel")
	}
}

func swapItems(t *testing.T, r map[string]any) map[string]map[string]any {
	t.Helper()
	if r["ok"] != true {
		t.Fatalf("swapList: %v", r)
	}
	out := map[string]map[string]any{}
	l, _ := r["items"].([]any)
	for _, x := range l {
		it := x.(map[string]any)
		out[it["name"].(string)] = it
	}
	return out
}

func panelNamed(t *testing.T, a *App, name string) Panel {
	t.Helper()
	for _, p := range a.panels.list() {
		if p.Name == name {
			return p
		}
	}
	t.Fatalf("no panel %s", name)
	return Panel{}
}

func dstCellN(p Panel) int {
	r, _ := cellRef(p.Pages[0].Cells[0])
	return r.N
}

func swapProfile(a *App, name string) config.Profile {
	u, _ := a.r().cfg.FindUserByName(name)
	return u.Profile
}

func TestSwapDestinationUpdatesEveryTickedItem(t *testing.T) {
	a, uid := swapApp(t)
	defer startLog(t, a)()
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	list := swapItems(t, p.ask("swapList", 1, map[string]any{"swap": map[string]any{"side": "destination", "from": 22}}))
	for _, name := range []string{"Studio", "Spare", "Desk", "Ana's", "Bo's", "Mine", "Ana desk", "Bo desk"} {
		if list[name] == nil {
			t.Fatalf("swapList left out %s: %v", name, list)
		}
	}
	if list["Keep"] != nil {
		t.Fatalf("swapList has an item that does not use destination 23: %v", list["Keep"])
	}
	if list["Studio"]["broken"] != true || list["Spare"]["broken"] == true {
		t.Fatalf("broken flags: Studio %v, Spare %v", list["Studio"], list["Spare"])
	}
	if list["Ana's"]["owner"] != "Ana" || list["Studio"]["owner"] != "" {
		t.Fatalf("owners: %v %v", list["Ana's"], list["Studio"])
	}
	var ticked []string
	for name, it := range list {
		if name != "Spare" {
			ticked = append(ticked, it["id"].(string))
		}
	}
	r := p.ask("settingsSave", 2, map[string]any{"fields": map[string]any{
		"swap": map[string]any{"value": map[string]any{"router": uid, "side": "destination", "from": 22, "to": 39, "items": ticked}},
	}})
	wantSaved(t, r, "swap")

	cfg := a.r().cfg
	studio, spare := cfg.Presets[0], cfg.Presets[1]
	if d := studio.Items[0].Dsts; len(d) != 1 || d[0].N != 39 || d[0].Name != "MON 40" {
		t.Errorf("Studio holds 23 and 40; want one destination 40, got %+v", d)
	}
	if len(studio.Broken) != 0 {
		t.Errorf("Studio keeps the Broken mark of destination 23: %+v", studio.Broken)
	}
	if spare.Items[0].Dsts[0].N != 22 {
		t.Errorf("Spare was not ticked but changed: %+v", spare.Items[0].Dsts)
	}
	if got := cfg.FilterSets[0].Dst; got != `40, !"MON 40", MON` {
		t.Errorf("Desk filter = %q", got)
	}
	if d := swapProfile(a, "Ana").Presets[0].Items[0].Dsts[0]; d.N != 39 || d.Name != "MON 40" {
		t.Errorf("Ana's preset: %+v", d)
	}
	bo := swapProfile(a, "Bo")
	if bo.Presets[0].Items[0].Dsts[0].N != 39 || bo.Presets[1].Items[0].Dsts[0].N != 5 {
		t.Errorf("Bo's presets: %+v", bo.Presets)
	}
	if bo.FilterSets[0].Dst != `"MON 40"` {
		t.Errorf("Bo's filter set: %q", bo.FilterSets[0].Dst)
	}
	for _, name := range []string{"Ana desk", "Bo desk"} {
		if n := dstCellN(panelNamed(t, a, name)); n != 39 {
			t.Errorf("%s: cell n = %d", name, n)
		}
	}

	rows := waitSaves(t, a, 1)
	f, ok := fieldsByKey(rows[0])["swap"]
	if !ok {
		t.Fatalf("no swap field in the Settings saves row: %+v", rows[0].Save)
	}
	var counts map[string]int
	if err := json.Unmarshal(f.New, &counts); err != nil {
		t.Fatal(err)
	}
	if want := map[string]int{"Everyone": 2, "Ana": 2, "Bo": 3}; !mapsEqual(counts, want) {
		t.Errorf("counts per owner = %v, want %v (label %q)", counts, want, f.Label)
	}
	if f.Label != "Swap destination 23 for 40" {
		t.Errorf("label = %q", f.Label)
	}
}

func mapsEqual(a, b map[string]int) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func TestSwapMineTouchesOnlyOwnItems(t *testing.T) {
	a, uid := swapApp(t)
	defer startLog(t, a)()
	admin := dialPeer(t, a)
	admin.signIn("Ada", "2468")
	all := swapItems(t, admin.ask("swapList", 1, map[string]any{"swap": map[string]any{"side": "destination", "from": 22}}))

	p := dialPeer(t, a)
	p.signIn("Bo", "567890")
	mine := swapItems(t, p.ask("swapList", 1, map[string]any{"swap": map[string]any{"side": "destination", "from": 22, "mine": true}}))
	got := slices.Sorted(func(yield func(string) bool) {
		for k := range mine {
			if !yield(k) {
				return
			}
		}
	})
	if want := []string{"Bo desk", "Bo's", "Mine"}; !slices.Equal(got, want) {
		t.Fatalf("Bo's list = %v, want %v", got, want)
	}
	if r := p.ask("swapList", 2, map[string]any{"swap": map[string]any{"side": "destination", "from": 22}}); r["ok"] == true {
		t.Fatalf("a Standard user listed everyone's items: %v", r)
	}
	var ids []string
	for _, it := range all {
		ids = append(ids, it["id"].(string))
	}
	r := p.ask("swapMine", 3, map[string]any{"swap": map[string]any{"router": uid, "side": "destination", "from": 22, "to": 39, "items": ids}})
	if r["ok"] != true {
		t.Fatalf("swapMine: %v", r)
	}
	cfg := a.r().cfg
	if cfg.Presets[0].Items[0].Dsts[0].N != 22 || cfg.FilterSets[0].Dst != `23, !"MON 23", MON` {
		t.Errorf("Bo's Swap changed Everyone items: %+v %q", cfg.Presets[0].Items[0].Dsts, cfg.FilterSets[0].Dst)
	}
	if swapProfile(a, "Ana").Presets[0].Items[0].Dsts[0].N != 22 || dstCellN(panelNamed(t, a, "Ana desk")) != 22 {
		t.Errorf("Bo's Swap changed Ana's items")
	}
	bo := swapProfile(a, "Bo")
	if bo.Presets[0].Items[0].Dsts[0].N != 39 || bo.FilterSets[0].Dst != `"MON 40"` || dstCellN(panelNamed(t, a, "Bo desk")) != 39 {
		t.Errorf("Bo's own items not swapped: %+v %q", bo.Presets[0].Items[0].Dsts, bo.FilterSets[0].Dst)
	}
	sv := p.ask("settingsSave", 4, map[string]any{"fields": map[string]any{
		"swap": map[string]any{"value": map[string]any{"router": uid, "side": "destination", "from": 22, "to": 39, "items": ids}},
	}})
	wantRefused(t, sv, "swap", "needs Approval")
}

func TestSwapSourceFilterNumberTermsUseShownNumbers(t *testing.T) {
	sw := swapper{uid: "r", from: 2, to: 11, off: 1, toName: `CAM "12"`, fromNames: []string{"CAM 3"}}
	s := config.FilterSet{Src: `3,  "cam 3" , 'CAM 3', /CAM 3/, 2-4, !3`}
	if !sw.filterSet(&s) {
		t.Fatal("not changed")
	}
	if want := `12,  "CAM ""12""" , 'CAM 3', /CAM 3/, 2-4, !12`; s.Src != want {
		t.Fatalf("src = %q, want %q", s.Src, want)
	}
}

// The stored panel keeps its list; only the edge and fixed destinations change.
func TestSwapGridEdgeAndFixedDestinations(t *testing.T) {
	a, uid := swapApp(t)
	ref := func(n int, name string) config.Ref { return config.Ref{Router: uid, N: n, Name: name} }
	grid, err := a.panels.create(Panel{Name: "Grid", Type: panelTypeGrid, Owner: userID(t, a, "Bo"),
		Settings: map[string]any{panelFixedKey: []config.Ref{ref(22, "MON 23"), ref(39, "MON 40")}},
		Sources:  &GridEdge{Members: GridMembers{All: true}},
		Destinations: &GridEdge{
			Members: GridMembers{List: []config.Ref{ref(22, "MON 23"), ref(39, "MON 40"), ref(5, "MON 6")}},
			Items:   []GridItem{{Router: uid, N: 22, Name: "MON 23", Art: "old"}, {Router: uid, N: 39, Name: "MON 40", Art: "kept"}},
		}})
	if err != nil {
		t.Fatal(err)
	}
	p := dialPeer(t, a)
	p.signIn("Bo", "567890")
	list := swapItems(t, p.ask("swapList", 1, map[string]any{"swap": map[string]any{"side": "destination", "from": 22, "mine": true}}))
	if list["Grid"] == nil {
		t.Fatalf("Grid not listed: %v", list)
	}
	if got, _ := a.panels.get(grid.ID); got.Rev != grid.Rev || got.Destinations.Members.List[0].N != 22 || dstCellN(panelNamed(t, a, "Bo desk")) != 22 {
		t.Fatalf("the list changed a stored panel: %+v", got.Destinations)
	}
	r := p.ask("swapMine", 2, map[string]any{"swap": map[string]any{"router": uid, "side": "destination", "from": 22, "to": 39,
		"items": []string{list["Grid"]["id"].(string)}}})
	if r["ok"] != true {
		t.Fatalf("swapMine: %v", r)
	}
	got, _ := a.panels.get(grid.ID)
	var ns []int
	for _, m := range got.Destinations.Members.List {
		ns = append(ns, m.N)
	}
	if !slices.Equal(ns, []int{39, 5}) {
		t.Errorf("members = %v, want [39 5]", ns)
	}
	if it := got.Destinations.Items; len(it) != 1 || it[0].N != 39 || it[0].Art != "kept" {
		t.Errorf("items = %+v, want To's settings kept", it)
	}
	if fixed, _ := fixedList(got.Settings[panelFixedKey]); len(fixed) != 1 || fixed[0].N != 39 {
		t.Errorf("fixed = %+v", fixed)
	}
	if dstCellN(panelNamed(t, a, "Bo desk")) != 22 {
		t.Error("an unticked panel changed")
	}
}

func TestSwapSourceChangesPresetSource(t *testing.T) {
	sw := swapper{uid: "r", from: 0, to: 4, toName: "CAM 5", fromNames: []string{"CAM 1"}}
	p := config.Preset{Items: []config.PresetItem{{Src: config.Ref{Router: "r", N: 0, Name: "CAM 1"}, Dsts: []config.Ref{{Router: "r", N: 0}}}},
		Moved: []config.Missing{{Kind: config.MissingSource, Ref: config.Ref{Router: "r", N: 0}}}}
	if !sw.preset(&p) || p.Items[0].Src.N != 4 || p.Items[0].Src.Name != "CAM 5" || p.Items[0].Dsts[0].N != 0 || len(p.Moved) != 0 {
		t.Fatalf("preset = %+v", p)
	}
	sw.fromNames = append(sw.fromNames, "CAM 5")
	s := config.FilterSet{Src: `"cam 5"`}
	if sw.filterSet(&s) {
		t.Fatalf("a term that already names To changed: %q", s.Src)
	}
}
