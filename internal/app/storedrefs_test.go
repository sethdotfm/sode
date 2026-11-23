package app

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// refKeys hold sources, destinations or levels. A number under one of them needs a router UID in its own mapping
// or in one around it. History rows and names tables name the router one time for the row or file.
var refKeys = map[string]bool{
	"n": true, "src": true, "dst": true, "dsts": true, "sources": true, "destinations": true,
	"members": true, "hide": true, "fixedDests": true, "levels": true,
}

// levelsNoRouter are the level lists with no router yet. Rights levels and the level subset of a panel or cell
// are UI levels 1..N of the one router. A second router needs them made references (docs/agents/server.md).
func levelsNoRouter(path string) bool {
	return strings.HasSuffix(path, "rights.levels") || strings.HasSuffix(path, "guestRights.levels") ||
		strings.HasSuffix(path, "settings.levels") || strings.Contains(path, ".cells[") && strings.HasSuffix(path, ".levels")
}

// -1 is the Communication Log's "none" for a row about no source or destination.
func hasNumber(v any) bool {
	switch x := v.(type) {
	case float64:
		return x >= 0
	case []any:
		for _, e := range x {
			if hasNumber(e) {
				return true
			}
		}
	}
	return false
}

func bareNumbers(v any, path string, routed bool) []string {
	var out []string
	switch x := v.(type) {
	case map[string]any:
		// A router's own entry (config routers, names tables) is routed by its uid.
		if r, _ := x["router"].(string); r != "" || strings.HasPrefix(path, ".routers[") {
			routed = true
		}
		// A preset item names its router one time, in src; its levels are that router's.
		src, _ := x["src"].(map[string]any)
		itemRouted := src != nil && src["router"] != nil && src["router"] != ""
		for k, e := range x {
			p := path + "." + k
			if refKeys[k] && hasNumber(e) && !routed && !(k == "levels" && (itemRouted || levelsNoRouter(p))) {
				out = append(out, p)
			}
			out = append(out, bareNumbers(e, p, routed)...)
		}
	case []any:
		for i, e := range x {
			out = append(out, bareNumbers(e, path+"["+strconv.Itoa(i)+"]", routed)...)
		}
	}
	return out
}

func normal(t *testing.T, v any) any {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	var out any
	json.Unmarshal(b, &out)
	return out
}

// Reads every file under dir, so a store added later is checked too.
func storedValues(t *testing.T, dir string) map[string]any {
	t.Helper()
	out := map[string]any{}
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		switch {
		case strings.HasSuffix(path, ".yaml"), strings.HasSuffix(path, ".checkpoints"):
			var v any
			if err := yaml.Unmarshal(b, &v); err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			out[rel] = normal(t, v)
		case strings.HasSuffix(path, ".json"):
			var v any
			if err := json.Unmarshal(b, &v); err != nil {
				t.Fatalf("%s: %v", rel, err)
			}
			out[rel] = v
		case strings.HasSuffix(path, ".jsonl"):
			sc := bufio.NewScanner(bytes.NewReader(b))
			sc.Buffer(nil, 1<<20)
			for i := 0; sc.Scan(); i++ {
				var v any
				if err := json.Unmarshal(sc.Bytes(), &v); err != nil {
					t.Fatalf("%s line %d: %v", rel, i+1, err)
				}
				out[rel+":"+strconv.Itoa(i+1)] = v
			}
		}
		return nil
	})
	return out
}

// Every store, filled through its own write path, holds router references only: no source, destination or level
// number without its router.
func TestStoresHoldNoBareNumbers(t *testing.T) {
	dir := settledTempDir(t)
	cfgPath := filepath.Join(dir, "config.yaml")
	cfg := config.Defaults()
	cfg.Normalize()
	cfg.Users = []config.User{mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890")}
	cfg.Users[0].Role = config.RoleTrusted
	withAdministrator(&cfg)
	hist, err := OpenHistory(filepath.Join(dir, "history.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	sl, err := OpenSentLog(filepath.Join(dir, "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, cfgPath, hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	f := newFakeRouter(t, a)
	shortTimers(a)
	stopLog := startLog(t, a)
	cfg = a.r().cfg
	cfg.APIClients = []config.APIClient{{Name: "PanelsBot", Scopes: []string{config.ScopeTake, config.ScopePresets, config.ScopeLayouts},
		Rights: config.Rights{Dsts: nums(2)}}}
	cfg.Presets = []config.Preset{{ID: "studio-a", Name: "Studio A", Items: []config.PresetItem{presetItem(1, []int{1}, 2)}}}
	cfg.GuestRights = config.Rights{Sources: nums(1), Levels: &[]int{1}}
	cfg.Users[1].Rights = config.Rights{Sources: nums(1), Dsts: nums(2), Levels: &[]int{1}}
	cfg.Users[1].Profile.Presets = []config.Preset{{Name: "Mine", Items: []config.PresetItem{presetItem(1, nil, 2)}}}
	cfg.FilterSets = []config.FilterSet{{Name: "Cams", Src: "1-4"}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.first.router.srcNames[1], a.first.router.srcNames[3] = "CAM 1", "CAM 3"
	a.first.router.dstNames[2] = "MON 2"
	a.mu.Unlock()
	a.names.save(a.r().cfg.UID, time.Now(), map[int]string{1: "CAM 1"}, map[int]string{2: "MON 2"})

	ana := userKey(t, a, "Ana")
	cells := `[{"r":1,"c":1,"kind":"source","n":1},{"r":1,"c":2,"kind":"destination","n":2,"levels":[1]},` +
		`{"r":1,"c":3,"kind":"preset","scope":"everyone","name":"Studio A"},{"r":1,"c":4,"kind":"take"}]`
	from := makePanel(t, a, ana, `{"name":"From","settings":{"fixedDests":[{"n":2}],"levels":[1]},"cells":`+cells+`}`)
	to := makePanel(t, a, ana, `{"name":"To","cells":`+cells+`}`)
	makePanel(t, a, ana, `{"name":"Grid","type":"grid","sources":{"members":[{"n":1},{"n":3}],"items":[{"n":1,"pinned":true}]},`+
		`"destinations":{"members":"all","hide":[{"n":4}]}}`)
	for _, c := range []struct{ method, path, body string }{
		{"POST", "/api/v1/panel-templates", `{"name":"T","scope":"me","sources":true,"dests":true,"cells":` + cells + `}`},
		{"POST", "/api/v1/panels/" + from + "/push", `{"targets":["` + to + `"]}`},
		{"POST", "/api/v1/panel-sets", `{"name":"Show","panels":["` + from + `"]}`},
		{"PUT", "/api/v1/default-artwork", `{"kind":"source","n":1,"art":"icon:camera"}`},
	} {
		if rec := apiSend(a, c.method, c.path, ana, c.body); rec.Code >= 300 {
			t.Fatalf("%s %s: %d %s", c.method, c.path, rec.Code, rec.Body.String())
		}
	}
	f.seed(2, 9, 0)
	if rec := apiPost(a, "/api/v1/takes", botKey(t, a), `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	stopLog()

	stores := storedValues(t, dir)
	for _, want := range []string{"config.yaml", "panels/" + from + ".yaml", "panels/" + to + ".checkpoints", "artdefaults.yaml",
		"history.jsonl:1", "sent.jsonl:1", "names/", "templates/me/", "panelsets/"} {
		if !slices.ContainsFunc(slices.Collect(maps.Keys(stores)), func(k string) bool { return strings.HasPrefix(k, want) }) {
			t.Errorf("no store %s among %v", want, slices.Collect(maps.Keys(stores)))
		}
	}
	for name, v := range stores {
		for _, p := range bareNumbers(v, "", false) {
			t.Errorf("%s: %s holds a number with no router", name, p)
		}
	}
}

func TestBareNumbersFindsNumbersWithNoRouter(t *testing.T) {
	var v any
	json.Unmarshal([]byte(`{"items":[{"src":4,"dsts":[{"n":1}]}],"rights":{"sources":[{"router":"u","n":1}],"levels":[1]},
		"history":{"router":"u","src":1,"dst":2,"levels":[1]},"cell":{"kind":"source","n":3}}`), &v)
	got := strings.Join(bareNumbers(v, "", false), " ")
	for _, want := range []string{".items[0].src", ".items[0].dsts[0].n", ".cell.n"} {
		if !strings.Contains(got, want) {
			t.Errorf("missed %s in %s", want, got)
		}
	}
	for _, not := range []string{"rights", "history"} {
		if strings.Contains(got, not) {
			t.Errorf("flagged %s: %s", not, got)
		}
	}
}
