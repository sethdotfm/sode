package app

import (
	"encoding/json"
	"reflect"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// asMap decodes JSON to a generic map, so shapes compare regardless of
// struct field order.
func asMap(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("%v in %s", err, b)
	}
	return m
}

// legacyHello is the hello as it was built from a map[string]any.
func legacyHello(a *App, p helloPage) []byte {
	r := a.r()
	n := a.namesMsg()
	m := map[string]any{"t": "hello", "srcs": n.Srcs, "dsts": n.Dsts, "at": n.At}
	m["tally"] = legacyTriples(a.router.tally)
	m["protect"] = legacyTriples(a.router.protect)
	m["status"] = a.status
	m["recent"] = p.recent
	m["more"] = p.more
	m["levels"] = r.levels
	m["levelDefaults"] = config.DefaultLevelColors
	m["cfg"] = r.cfg
	m["pinSet"] = r.cfg.SettingsPin != ""
	m["configPath"] = a.cfgPath
	m["version"] = VersionLabel()
	b, _ := json.Marshal(m)
	return b
}

// legacyTriples is the bare [dst, level, value] arrays routeChanges replaced.
func legacyTriples(m map[int]map[int]int) [][3]int {
	out := [][3]int{}
	for _, c := range routeChanges(m) {
		out = append(out, [3]int{c.Dst, c.Level, c.Src})
	}
	return out
}

func TestHelloMatchesLegacy(t *testing.T) {
	a := newTestApp(t)
	fillRouterDerived(a)
	for _, p := range []helloPage{{}, {recent: []Entry{{ID: 3, Prev: [][2]int{{1, 5}}, Now: [][2]int{{1, -1}}}}, more: true}} {
		a.mu.Lock()
		got, want := a.hello(p), legacyHello(a, p)
		a.mu.Unlock()
		if !reflect.DeepEqual(asMap(t, got), asMap(t, want)) {
			t.Fatalf("hello differs:\n got %s\nwant %s", got, want)
		}
	}
}

// TestRouteChangeWire pins routeChange to the bare [dst, level, src] arrays
// it replaced, in a delta, a hello's tally and protect, and back.
func TestRouteChangeWire(t *testing.T) {
	old := map[string]any{"t": "tally", "u": [][3]int{{5, 1, 7}, {6, 2, -1}}}
	want, _ := json.Marshal(old)
	got, err := json.Marshal(deltaMsg{T: "tally", U: []routeChange{{5, 1, 7}, {6, 2, -1}}})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(asMap(t, got), asMap(t, want)) {
		t.Fatalf("delta: got %s want %s", got, want)
	}
	var back deltaMsg
	if err := json.Unmarshal(want, &back); err != nil || !reflect.DeepEqual(back.U, []routeChange{{5, 1, 7}, {6, 2, -1}}) {
		t.Fatalf("decode: %v %+v", err, back)
	}

	a := newTestApp(t)
	fillRouterDerived(a)
	a.mu.Lock()
	h := asMap(t, a.hello(helloPage{}))
	a.mu.Unlock()
	if !reflect.DeepEqual(h["tally"], []any{[]any{5.0, 1.0, 7.0}}) || !reflect.DeepEqual(h["protect"], []any{[]any{5.0, 1.0, 9.0}}) {
		t.Fatalf("hello tally/protect: %v %v", h["tally"], h["protect"])
	}
	a.mu.Lock()
	a.router.tally, a.router.protect = map[int]map[int]int{}, map[int]map[int]int{}
	h = asMap(t, a.hello(helloPage{}))
	a.mu.Unlock()
	if !reflect.DeepEqual(h["tally"], []any{}) || !reflect.DeepEqual(h["protect"], []any{}) {
		t.Fatalf("empty tally/protect must be [], got %v %v", h["tally"], h["protect"])
	}
}

// TestRecentWire: a recent message keeps its old shape, and omitempty keys
// stay absent.
func TestRecentWire(t *testing.T) {
	e := &Entry{ID: 4, Src: 7, Dst: 2, Levels: []int{1}, Prev: [][2]int{{1, 5}}, OK: true}
	got, err := json.Marshal(recentMsg{T: "recent", E: e})
	if err != nil {
		t.Fatal(err)
	}
	m := asMap(t, got)
	em := m["e"].(map[string]any)
	if !reflect.DeepEqual(em["prev"], []any{[]any{1.0, 5.0}}) {
		t.Fatalf("prev: %v", em["prev"])
	}
	for _, k := range []string{"now", "failed", "err", "undo", "retry", "already", "group", "ext", "follow", "found", "late", "unsent", "sendErr", "cut"} {
		if _, ok := em[k]; ok {
			t.Errorf("key %q should be absent", k)
		}
	}
	if _, ok := m["update"]; ok {
		t.Error("update should be absent")
	}
}

// TestHistoryLineRoundTrip reads a history line as written before this
// change and writes it back unchanged (as JSON values).
func TestHistoryLineRoundTrip(t *testing.T) {
	line := `{"id":12,"time":"2026-10-23T18:01:40.123456789Z","client":"10.0.0.7","src":50,"srcName":"CAM 51","dst":3,"dstName":"KiPro 4","levels":[1,2,3],"prev":[[1,4],[2,4],[3,-1]],"ok":false,"err":"level 3 no answer","failed":[3],"now":[[3,9]],"already":[1],"late":[2]}`
	var e Entry
	if err := json.Unmarshal([]byte(line), &e); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(e.Prev, [][2]int{{1, 4}, {2, 4}, {3, -1}}) || !reflect.DeepEqual(e.Now, [][2]int{{3, 9}}) {
		t.Fatalf("prev/now: %v %v", e.Prev, e.Now)
	}
	b, err := json.Marshal(&e)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(asMap(t, b), asMap(t, []byte(line))) {
		t.Fatalf("round trip:\n got %s\nwant %s", b, line)
	}
}

func TestEntryLogLine(t *testing.T) {
	undo, follow := 3, 4
	cases := []struct {
		e    Entry
		want string
	}{
		{Entry{Client: "10.0.0.7", Src: 1, SrcName: "CAM 2", Dst: 4, DstName: "MON 5", Levels: []int{1, 2}, OK: true},
			`TAKE client=10.0.0.7 src=2 "CAM 2" dst=5 "MON 5" levels=[1 2] undo=- retry=- group=- result="ok"`},
		{Entry{Client: "10.0.0.7", Src: 1, SrcName: "CAM 2", Dst: 4, DstName: "MON 5", Levels: []int{1}, Undo: &undo, Group: "g1", GroupN: 2, Err: "no answer"},
			`TAKE client=10.0.0.7 src=2 "CAM 2" dst=5 "MON 5" levels=[1] undo=3 retry=- group=g1/2 result="no answer"`},
		{Entry{Ext: true, Found: true, Src: 1, SrcName: "CAM 2", Dst: 4, DstName: "MON 5", Levels: []int{1}, Follow: &follow, FollowGroup: "g9"},
			`EXT src=2 "CAM 2" dst=5 "MON 5" levels=[1] group=- follow=4 followGroup=g9 found=true`},
	}
	for _, c := range cases {
		if got := c.e.logLine(1); got != c.want {
			t.Errorf("got  %s\nwant %s", got, c.want)
		}
	}
}
