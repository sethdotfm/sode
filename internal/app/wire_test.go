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
	m["tally"] = triples(a.router.tally)
	m["protect"] = triples(a.router.protect)
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
