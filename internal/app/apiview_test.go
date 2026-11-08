package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// view* helpers: a client PanelsBot whose rights allow source 1 and destination 2 only,
// hidden or read-only, over a router that also has source 5 and destination 7.

func viewRights(vis string) config.Rights {
	return config.Rights{Sources: &[]int{1}, Dsts: &[]int{2}, Presets: &[]string{"Studio A"}, Visibility: vis}
}

func viewApp(t *testing.T, vis string) (*App, string) {
	t.Helper()
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.APIClients[0].Rights = viewRights(vis)
		c.Presets = append(c.Presets, config.Preset{Name: "Secret mix", Items: []config.PresetItem{{Src: 5, Dsts: []int{7}}}})
	})
	a.mu.Lock()
	a.router.srcNames[5] = "SECRET SRC"
	a.router.dstNames[7] = "SECRET DST"
	a.router.tally[7] = map[int]int{1: 5}
	a.router.tally[2][2] = 5 // visible destination, hidden source
	a.router.protect[7] = map[int]int{1: 0}
	a.mu.Unlock()
	return a, botKey(t, a)
}

func viewGet(t *testing.T, a *App, key, path string) map[string]any {
	t.Helper()
	rec := apiDo(a, "GET", path, key, nil)
	if rec.Code != 200 {
		t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
	}
	return apiBody(t, rec)
}

func TestHiddenRightsDropNamesTallyAndProtected(t *testing.T) {
	a, key := viewApp(t, config.VisHidden)

	names := viewGet(t, a, key, "/api/v1/names")
	if got := len(names["sources"].([]any)); got != 1 {
		t.Errorf("sources: %d", got)
	}
	if got := len(names["destinations"].([]any)); got != 1 {
		t.Errorf("destinations: %d", got)
	}

	rows := viewGet(t, a, key, "/api/v1/tally")["routes"].([]any)
	if len(rows) != 1 {
		t.Fatalf("tally rows %v", rows)
	}
	if r := rows[0].(map[string]any); r["dst"] != float64(2) || r["src"] != float64(1) {
		t.Errorf("row %v", r)
	}
	if got := viewGet(t, a, key, "/api/v1/protected")["protected"].([]any); len(got) != 0 {
		t.Errorf("protected %v", got) // dst 3 and 7 are both outside the rights
	}

	ps := viewGet(t, a, key, "/api/v1/presets")["presets"].([]any)
	if len(ps) != 1 || ps[0].(map[string]any)["name"] != "Studio A" {
		t.Errorf("presets %v", ps)
	}
}

func TestReadOnlyRightsStillSeeEverything(t *testing.T) {
	a, key := viewApp(t, "")
	if got := len(viewGet(t, a, key, "/api/v1/names")["sources"].([]any)); got != 2 {
		t.Errorf("sources: %d", got)
	}
	if got := len(viewGet(t, a, key, "/api/v1/tally")["routes"].([]any)); got != 3 {
		t.Errorf("tally rows: %d", got)
	}
	if got := len(viewGet(t, a, key, "/api/v1/presets")["presets"].([]any)); got != 2 {
		t.Errorf("presets: %d", got)
	}
}

func TestHiddenRightsDropHistoryEntries(t *testing.T) {
	a, key := viewApp(t, config.VisHidden)
	ok := Entry{Src: 1, SrcName: "CAM 1", Dst: 2, DstName: "MON 2", Levels: []int{1}, Prev: [][2]int{{1, 1}}, OK: true}
	hiddenDst := Entry{Src: 1, Dst: 7, Levels: []int{1}, Prev: [][2]int{{1, -1}}, OK: true}
	hiddenSrc := Entry{Src: 5, SrcName: "SECRET SRC", Dst: 2, Levels: []int{1}, OK: true}
	// Visible entry whose level 2 came from a hidden source: that level goes, level 1 stays.
	partial := Entry{Src: 1, Dst: 2, Levels: []int{1, 2}, Prev: [][2]int{{1, 1}, {2, 5}}, Failed: []int{2}, Now: [][2]int{{2, 5}}, OK: false,
		Preset: "Secret mix", Group: "g", GroupN: 3}
	for _, e := range []Entry{ok, hiddenDst, hiddenSrc, partial} {
		e := e
		a.record(&e)
	}
	h := viewGet(t, a, key, "/api/v1/history")["entries"].([]any)
	if len(h) != 2 {
		b, _ := json.Marshal(h)
		t.Fatalf("entries %s", b)
	}
	p := h[0].(map[string]any) // newest first
	if lv := p["levels"].([]any); len(lv) != 1 || lv[0] != float64(1) {
		t.Errorf("levels %v", lv)
	}
	b, _ := json.Marshal(p)
	for _, leak := range []string{"SECRET", "Secret mix", "groupN", `"failed"`, `[2,5]`} {
		if strings.Contains(string(b), leak) {
			t.Errorf("entry leaks %s: %s", leak, b)
		}
	}
}

// The view check that Undo, Redo and Retry planning uses: a hidden caller gets the entry as
// it sees it, or false when it is not theirs to see.
func TestViewEntryIsTheCheckForUndoPlans(t *testing.T) {
	hid := apiView{viewRights(config.VisHidden)}
	if _, ok := hid.Entry(Entry{Src: 1, Dst: 7, Levels: []int{1}}); ok {
		t.Error("hidden destination entry is visible")
	}
	open := apiView{viewRights("")}
	e, ok := open.Entry(Entry{Src: 5, Dst: 7, Levels: []int{1}})
	if !ok || e.Dst != 7 {
		t.Error("read-only rights hide an entry")
	}
}
