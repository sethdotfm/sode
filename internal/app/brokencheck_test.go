package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

var (
	checkSrcs = []string{"CAM 1", "CAM 2", "CAM 3", "CAM 4"}
	checkDsts = []string{"MON 1", "MON 2", "MON 3", "MON 4"}
)

// brokenApp has one Everyone preset, Studio: CAM 3 (source 2) to MON 1 (destination 0), after two reads of the
// check names.
func brokenApp(t *testing.T) *App {
	t.Helper()
	a := futureApp(t, "routers: [{address: 127.0.0.1:1}]\n")
	readNames(a, checkSrcs, checkDsts)
	readNames(a, checkSrcs, checkDsts)
	uid := a.r().cfg.Routers[0].UID
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Presets = []config.Preset{{Name: "Studio", Items: []config.PresetItem{{
			Src:  config.Ref{Router: uid, N: 2, Name: "CAM 3"},
			Dsts: []config.Ref{{Router: uid, N: 0, Name: "MON 1"}},
		}}}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func studio(t *testing.T, a *App) config.Preset {
	t.Helper()
	ps := a.r().cfg.Presets
	if len(ps) != 1 {
		t.Fatalf("presets: %+v", ps)
	}
	return ps[0]
}

func without(l []string, i int) []string {
	out := append([]string{}, l...)
	return append(out[:i], out[i+1:]...)
}

func with(l []string, i int, name string) []string {
	out := append([]string{}, l...)
	out[i] = name
	return out
}

func TestBrokenNeedsTwoPolls(t *testing.T) {
	a := brokenApp(t)
	gone := with(checkSrcs, 2, "") // the router no longer names source 2
	readNames(a, gone, checkDsts)
	if p := studio(t, a); len(p.Broken) != 0 {
		t.Fatalf("one poll made the preset Broken: %+v", p.Broken)
	}
	readNames(a, gone, checkDsts)
	p := studio(t, a)
	if len(p.Broken) != 1 || p.Broken[0].Kind != config.MissingSource || p.Broken[0].Ref.N != 2 || p.Broken[0].Ref.Name != "CAM 3" {
		t.Fatalf("two polls: broken = %+v, want source 2 CAM 3", p.Broken)
	}
}

// The counted read is the base of the next check, so a later write to the live names must not reach it.
func TestCountedPollIsACopy(t *testing.T) {
	a := brokenApp(t)
	cur := namePoll{map[int]string{}, map[int]string{}}
	for i := range checkSrcs {
		cur.src[i], cur.dst[i] = checkSrcs[i], checkDsts[i]
	}
	a.mu.Lock()
	if _, fault := a.countPollMu(a.firstUnit(), cur); fault {
		t.Fatal("read taken for a fault")
	}
	cur.src[0] = "changed"
	got := a.first.router.lastPoll.src[0]
	a.mu.Unlock()
	if got != "CAM 1" {
		t.Fatalf("counted read changed to %q", got)
	}
}

func TestBrokenIgnoresPollWithMostNamesBlank(t *testing.T) {
	a := brokenApp(t)
	blank := []string{"", "", "", ""}
	for range 3 {
		readNames(a, blank, with(checkDsts, 1, ""))
	}
	if p := studio(t, a); len(p.Broken) != 0 || p.Items[0].Src.Name != "CAM 3" {
		t.Fatalf("a router fault changed the preset: %+v", p)
	}
}

// A fault read is not shown or saved, so after a restart the base still has the names.
func TestFaultReadKeepsNamesAndTable(t *testing.T) {
	a := brokenApp(t)
	readNames(a, []string{"", "", "", ""}, with(checkDsts, 1, ""))
	a.mu.Lock()
	src := a.first.router.srcNames[2]
	a.mu.Unlock()
	b, _ := os.ReadFile(namesTablePath(a))
	if src != "CAM 3" || !strings.Contains(string(b), "CAM 3") {
		t.Fatalf("fault read shown (%q) or saved:\n%s", src, b)
	}
}

func TestRenameUpdatesStoredNameAndLogs(t *testing.T) {
	a := brokenApp(t)
	renamed := with(checkSrcs, 2, "CAM C")
	readNames(a, renamed, checkDsts)
	if p := studio(t, a); p.Items[0].Src.Name != "CAM 3" {
		t.Fatalf("one poll renamed: %+v", p.Items[0].Src)
	}
	drainEvents(a)
	readNames(a, renamed, checkDsts)
	p := studio(t, a)
	if p.Items[0].Src.Name != "CAM C" || len(p.Broken) != 0 || len(p.Moved) != 0 {
		t.Fatalf("after two polls: %+v", p)
	}
	var row Sent
	for _, e := range events(a) {
		if e.Kind == kindSettings {
			t.Errorf("a rename wrote a Settings saves row: %+v", e)
		}
		if e.Why == "routerNames" {
			row = e
		}
	}
	if row.Kind != kindLink || !strings.Contains(row.Note, "CAM 3") || !strings.Contains(row.Note, "CAM C") {
		t.Fatalf("no Router names row naming the rename: %+v", row)
	}
}

func TestNameMovedKeepsNumber(t *testing.T) {
	a := brokenApp(t)
	repatched := []string{"CAM 3", "CAM 2", "CAM X", "CAM 4"}
	readNames(a, repatched, checkDsts)
	readNames(a, repatched, checkDsts)
	p := studio(t, a)
	if len(p.Moved) != 1 || p.Moved[0].Ref.N != 2 || p.Items[0].Src.N != 2 || p.Items[0].Src.Name != "CAM 3" || len(p.Broken) != 0 {
		t.Fatalf("name moved: %+v", p)
	}
	plan, perr := planPreset(a, a.r(), p)
	if perr != nil || len(plan.Items) != 1 || plan.Items[0].Src != 2 || len(plan.LeftOut) != 0 {
		t.Fatalf("a Name moved preset did not fire on its stored number: %+v %v", plan, perr)
	}
}

func TestBrokenPresetFiresWhatExists(t *testing.T) {
	a := brokenApp(t)
	uid := a.r().cfg.Routers[0].UID
	a.cfgMu.Lock()
	a.commit(func(c *config.Config) error {
		c.Presets[0].Items = append(c.Presets[0].Items, config.PresetItem{
			Src: config.Ref{Router: uid, N: 0, Name: "CAM 1"}, Dsts: []config.Ref{{Router: uid, N: 1, Name: "MON 2"}}})
		return nil
	}, nil)
	a.cfgMu.Unlock()
	gone := without(checkSrcs, 3) // source 3 goes; 2 is still CAM 3
	gone = with(gone, 2, "")
	readNames(a, gone, checkDsts)
	readNames(a, gone, checkDsts)
	p := studio(t, a)
	if len(p.Broken) != 1 {
		t.Fatalf("broken = %+v", p.Broken)
	}
	plan, perr := planPreset(a, a.r(), p)
	if perr != nil || len(plan.Items) != 1 || plan.Items[0].Src != 0 || plan.Items[0].Dst != 1 {
		t.Fatalf("plan = %+v %v, want only CAM 1 to MON 2", plan, perr)
	}
	if len(plan.LeftOut) != 1 || !strings.Contains(plan.LeftOut[0], "CAM 3") || !strings.Contains(plan.LeftOut[0], "source 3") {
		t.Fatalf("left out = %q, want CAM 3 (source 3)", plan.LeftOut)
	}
}

func drainEvents(a *App) {
	for {
		select {
		case <-a.evq:
		default:
			return
		}
	}
}

func events(a *App) []Sent {
	var out []Sent
	for {
		select {
		case e := <-a.evq:
			out = append(out, e)
		default:
			return out
		}
	}
}

func TestBrokenPanelCellsAndQuietFixedDests(t *testing.T) {
	a := brokenApp(t)
	uid := a.r().cfg.Routers[0].UID
	p, err := a.panels.create(Panel{Name: "Desk", Rows: 1, Cols: 2,
		Settings: map[string]any{panelFixedKey: []config.Ref{{Router: uid, N: 1, Name: "MON 2"}}},
		Pages: []PanelPage{{ID: "pg-1", Enabled: true, Cells: []map[string]any{
			{"kind": "source", "router": uid, "n": 2, "name": "CAM 3"},
			{"kind": "destination", "router": uid, "n": 3, "name": "MON 4"},
		}}}})
	if err != nil {
		t.Fatal(err)
	}
	srcs, dsts := with(checkSrcs, 2, ""), with(checkDsts, 1, "MON B")
	readNames(a, srcs, dsts)
	readNames(a, srcs, dsts)
	got, _ := a.panels.get(p.ID)
	if len(got.Broken) != 1 || got.Broken[0].Ref.N != 2 || got.Broken[0].Kind != config.MissingSource {
		t.Fatalf("panel broken = %+v", got.Broken)
	}
	fixed, _ := fixedList(got.Settings[panelFixedKey])
	if len(fixed) != 1 || fixed[0].Name != "MON B" {
		t.Fatalf("fixed destinations = %+v, want MON B", fixed)
	}
	if got.Rev <= p.Rev {
		t.Fatalf("rev %d, was %d", got.Rev, p.Rev)
	}
}

func TestBrokenFilterSetTerms(t *testing.T) {
	a := brokenApp(t)
	a.cfgMu.Lock()
	a.commit(func(c *config.Config) error {
		c.FilterSets = []config.FilterSet{
			{Name: "Exact", Src: `"CAM 3"`, Sode: true},
			{Name: "Number", Dst: `4`, Sode: true},
			{Name: "Search", Src: `CAM 3, /CAM 3/, 'CAM 3'`, Dst: `!"MON 4"`, Sode: true},
		}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	srcs, dsts := with(checkSrcs, 2, "CAM C"), with(checkDsts, 3, "")
	readNames(a, srcs, dsts)
	if fs := a.r().cfg.FilterSets; len(fs[0].Broken)+len(fs[1].Broken) != 0 {
		t.Fatalf("one poll broke a set: %+v", fs)
	}
	readNames(a, srcs, dsts)
	fs := a.r().cfg.FilterSets
	if len(fs[0].Broken) != 1 || fs[0].Broken[0].Ref.Name != "CAM 3" {
		t.Errorf("exact term: %+v", fs[0].Broken)
	}
	if len(fs[1].Broken) != 1 || fs[1].Broken[0].Kind != config.MissingDestination || fs[1].Broken[0].Ref.N != 3 {
		t.Errorf("number term: %+v", fs[1].Broken)
	}
	if len(fs[2].Broken) != 0 {
		t.Errorf("contains, regex or not terms broke a set: %+v", fs[2].Broken)
	}
}

func TestQuietRenameInRightsAndMePresets(t *testing.T) {
	a := brokenApp(t)
	uid := a.r().cfg.Routers[0].UID
	a.cfgMu.Lock()
	a.commit(func(c *config.Config) error {
		c.Users[0].Rights.Sources = &[]config.Ref{{Router: uid, N: 2, Name: "CAM 3"}}
		c.Users[0].Profile.Presets = []config.Preset{{Name: "Mine", Items: []config.PresetItem{{
			Src: config.Ref{Router: uid, N: 2, Name: "CAM 3"}, Dsts: []config.Ref{{Router: uid, N: 0, Name: "MON 1"}}}}}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	renamed := with(checkSrcs, 2, "CAM C")
	readNames(a, renamed, checkDsts)
	readNames(a, renamed, checkDsts)
	u := a.r().cfg.Users[0]
	if l := *u.Rights.Sources; l[0].Name != "CAM C" {
		t.Errorf("rights: %+v", l)
	}
	if p := u.Profile.Presets[0]; p.Items[0].Src.Name != "CAM C" {
		t.Errorf("Me preset: %+v", p)
	}
}

func TestImportBrokenStaysOnRename(t *testing.T) {
	a := brokenApp(t)
	uid := a.r().cfg.Routers[0].UID
	a.cfgMu.Lock()
	a.commit(func(c *config.Config) error {
		c.Presets[0].Broken = []config.Missing{{Kind: config.MissingSource, Ref: config.Ref{Router: uid, N: 2, Name: "CAM 3"}}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	renamed := with(checkSrcs, 2, "CAM C")
	readNames(a, renamed, checkDsts)
	readNames(a, renamed, checkDsts)
	if p := studio(t, a); len(p.Broken) != 1 || p.Items[0].Src.Name != "CAM 3" {
		t.Fatalf("a Broken reference was renamed or cleared: %+v", p)
	}
	readNames(a, checkSrcs, checkDsts)
	readNames(a, checkSrcs, checkDsts)
	if p := studio(t, a); len(p.Broken) != 0 {
		t.Fatalf("still Broken with its name back at its number: %+v", p.Broken)
	}
}

// A rename that lengthens names must not push a list past its byte cap, or the next Load refuses the file.
func TestRenameKeepsListUnderByteCap(t *testing.T) {
	a := brokenApp(t)
	uid := a.r().cfg.Routers[0].UID
	var ps []config.Preset
	for p := 0; p < config.MaxPresets; p++ {
		it := config.PresetItem{Src: config.Ref{Router: uid, N: 0, Name: "CAM 1"}}
		for d := 0; d < config.MaxPresetDsts; d++ {
			it.Dsts = append(it.Dsts, config.Ref{Router: uid, N: d % 4, Name: checkDsts[d%4]})
		}
		ps = append(ps, config.Preset{Name: fmt.Sprintf("P%d", p), Items: []config.PresetItem{it}})
		if b, _ := json.Marshal(ps); len(b) > config.MaxPresetsBytes-2000 {
			ps = ps[:len(ps)-1]
			break
		}
	}
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Presets = ps
		c.Users[0].Profile.Presets = []config.Preset{{Name: "Mine", Items: []config.PresetItem{{
			Src: config.Ref{Router: uid, N: 0, Name: "CAM 1"}, Dsts: []config.Ref{{Router: uid, N: 0, Name: "MON 1"}}}}}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	long := []string{"MONITOR WALL 1", "MONITOR WALL 2", "MONITOR WALL 3", "MONITOR WALL 4"}
	readNames(a, checkSrcs, long)
	readNames(a, checkSrcs, long)
	got := a.r().cfg.Presets
	if b, _ := json.Marshal(got); len(b) > config.MaxPresetsBytes {
		t.Fatalf("presets grew past the cap: %d bytes", len(b))
	}
	if got[0].Items[0].Dsts[0].Name != "MON 1" {
		t.Fatalf("a list over the cap took the new names: %+v", got[0].Items[0].Dsts[0])
	}
	if mine := a.r().cfg.Users[0].Profile.Presets[0]; mine.Items[0].Dsts[0].Name != "MONITOR WALL 1" {
		t.Fatalf("a list under its cap kept the old names: %+v", mine.Items[0].Dsts[0])
	}
	if _, _, err := config.Load(a.cfgPath); err != nil {
		t.Fatalf("the file no longer loads: %v", err)
	}
}

func TestBootComparesSavedNamesTable(t *testing.T) {
	a := brokenApp(t)
	hist, _ := OpenHistory("")
	// The router renamed source 2 while Sode was off. The saved names table counts as the poll before the first read.
	b, err := New(a.r().cfg, a.cfgPath, hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	renamed := with(checkSrcs, 2, "CAM C")
	readNames(b, renamed, checkDsts)
	if p := studio(t, b); p.Items[0].Src.Name != "CAM 3" {
		t.Fatalf("the first read alone renamed: %+v", p.Items[0].Src)
	}
	readNames(b, renamed, checkDsts)
	if p := studio(t, b); p.Items[0].Src.Name != "CAM C" {
		t.Fatalf("after start: %+v", p.Items[0].Src)
	}
}

// The two-poll rule over the real SW-P-08 client and the simulator, renaming through SIM_NAMES_FILE.
func TestBrokenWithSimulator(t *testing.T) {
	slowTest(t)
	names := filepath.Join(t.TempDir(), "names")
	if err := os.WriteFile(names, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	sim, _ := simServer(t, "SIM_NAMES_FILE="+names)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	lastPoll := func() *namePoll {
		a.mu.Lock()
		defer a.mu.Unlock()
		return a.first.router.lastPoll
	}
	// poll reads the names one time and waits until the Broken check has counted the read and run.
	poll := func() {
		t.Helper()
		was := lastPoll()
		a.RefreshNames()
		for end := time.Now().Add(10 * time.Second); lastPoll() == was; time.Sleep(20 * time.Millisecond) {
			if time.Now().After(end) {
				t.Fatal("no counted name read")
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	for end := time.Now().Add(10 * time.Second); lastPoll() == nil; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(end) {
			t.Fatal("the app never read names from swp08sim")
		}
	}
	uid := a.r().cfg.Routers[0].UID
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error {
		c.Presets = []config.Preset{{Name: "Studio", Items: []config.PresetItem{{
			Src: config.Ref{Router: uid, N: 12, Name: "CAM 1"}, Dsts: []config.Ref{{Router: uid, N: 0, Name: "PGM"}}}}}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	poll()
	if err := os.WriteFile(names, []byte("src 12\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	poll()
	if p := studio(t, a); len(p.Broken) != 0 {
		t.Fatalf("one poll made the preset Broken: %+v", p.Broken)
	}
	poll()
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		p := studio(t, a)
		if len(p.Broken) == 1 && p.Broken[0].Ref.N == 12 {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("two polls: broken = %+v", p.Broken)
		}
	}
}

// echoPresets saves the Everyone presets as a browser sends them: references only, no marks.
func echoPresets(t *testing.T, a *App) {
	t.Helper()
	var sent []config.Preset
	for _, p := range a.r().cfg.Presets {
		sent = append(sent, config.Preset{ID: p.ID, Name: p.Name, Items: p.Items})
	}
	if err := a.SavePresets(adminClient(t, a), sent); err != nil {
		t.Fatal(err)
	}
}

// A save names every reference with the name at its number now. The marks keep the stored name.
func TestMarksSurviveASave(t *testing.T) {
	a := brokenApp(t)
	repatched := []string{"CAM 3", "CAM 2", "CAM X", "CAM 4"}
	readNames(a, repatched, checkDsts)
	readNames(a, repatched, checkDsts)
	echoPresets(t, a)
	readNames(a, repatched, checkDsts)
	readNames(a, repatched, checkDsts)
	if p := studio(t, a); len(p.Moved) != 1 || p.Items[0].Src.Name != "CAM 3" {
		t.Fatalf("Name moved after a save: %+v", p)
	}
}

func TestImportBrokenSurvivesASave(t *testing.T) {
	a := brokenApp(t)
	uid := a.r().cfg.Routers[0].UID
	a.cfgMu.Lock()
	a.commit(func(c *config.Config) error {
		c.Presets[0].Items[0].Src.Name = "VTR 9" // the file's name; this router has CAM 3 at 2
		c.Presets[0].Broken = []config.Missing{{Kind: config.MissingSource, Ref: config.Ref{Router: uid, N: 2, Name: "VTR 9"}}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	echoPresets(t, a)
	readNames(a, checkSrcs, checkDsts)
	readNames(a, checkSrcs, checkDsts)
	p := studio(t, a)
	if len(p.Broken) != 1 || p.Items[0].Src.Name != "VTR 9" {
		t.Fatalf("a save cleared an import's Broken mark: %+v", p)
	}
	if plan, _ := planPreset(a, a.r(), p); plan != nil && len(plan.Items) != 0 {
		t.Fatalf("the Broken source fired: %+v", plan.Items)
	}
}

// A Broken exact term carries the name that replaced it, so Replace term in the filter editor can offer it.
func TestBrokenFilterTermNamesTheNewName(t *testing.T) {
	a := brokenApp(t)
	a.cfgMu.Lock()
	a.commit(func(c *config.Config) error {
		c.FilterSets = []config.FilterSet{{Name: "Exact", Src: `"CAM 3"`, Sode: true}}
		return nil
	}, nil)
	a.cfgMu.Unlock()
	srcs := with(checkSrcs, 2, "CAM C")
	readNames(a, srcs, checkDsts)
	readNames(a, srcs, checkDsts)
	readNames(a, srcs, checkDsts)
	b := a.r().cfg.FilterSets[0].Broken
	if len(b) != 1 || b[0].Now != "CAM C" {
		t.Fatalf("Broken = %+v, want one mark naming CAM C", b)
	}
}
