package app

import (
	"encoding/json"
	"os"
	"regexp"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func TestBrowserCopyWritesOneSaveRowForTheUser(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "light"}, Presets: []config.Preset{testPreset("Old")}}); err != nil {
		t.Fatal(err)
	}
	onlySentinel(t, a)
	if err := admitMsg(a, ana, inMsg{T: "browserCopy"}); err != nil {
		t.Fatalf("browserCopy refused for a Standard user: %v", err)
	}
	err := a.BrowserCopy(ana, profileReq{
		Prefs:      map[string]string{"theme": "dark", "flip": "on"},
		FilterSets: []config.FilterSet{{Name: "Mine", Sode: true}},
		Presets:    []config.Preset{testPreset("Old"), testPreset("Open")},
	})
	if err != nil {
		t.Fatal(err)
	}
	p := anasProfile(t, a)
	if p.Prefs["theme"] != "dark" || p.Prefs["flip"] != "on" || len(p.FilterSets) != 1 || len(p.Presets) != 2 {
		t.Fatalf("profile %+v", p)
	}
	rows := waitSaves(t, a, 2)
	if len(rows) != 2 {
		t.Fatalf("want one Settings saves row after the sentinel, have %d", len(rows)-1)
	}
	e := rows[0]
	if e.Kind != kindSettings || e.Why != "browserCopy" || e.By != "Ana" || e.Save.Critical {
		t.Fatalf("row %+v save %+v", e, e.Save)
	}
	f := fieldsByKey(e)
	theme := f["profile/prefs/theme"]
	if theme.Section != "My user" || theme.Topic != "Preferences" || theme.Label != "Theme" || string(theme.Old) != `"light"` || string(theme.New) != `"dark"` {
		t.Fatalf("theme field %+v", theme)
	}
	if fl := f["profile/prefs/flip"]; fl.Label != "Destination before source" || len(fl.Old) != 0 || string(fl.New) != `"on"` {
		t.Fatalf("flip field %+v", fl)
	}
	names := func(vs []json.RawMessage) []string {
		var out []string
		for _, v := range vs {
			var s string
			_ = json.Unmarshal(v, &s)
			out = append(out, s)
		}
		return out
	}
	if pr := f["profile/presets"]; pr.Label != "Presets" || len(pr.Added) != 1 || names(pr.Added)[0] != "Open" || pr.Note != "" {
		t.Fatalf("presets field %+v", pr)
	}
	if fs := f["profile/filterSets"]; fs.Label != "Filter sets" || len(fs.Added) != 1 || names(fs.Added)[0] != "Mine" {
		t.Fatalf("filter sets field %+v", fs)
	}
}

// A moved preset keeps its id.
func TestBrowserCopyKeepsPresetID(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	p := testPreset("Moved")
	p.ID = "00112233aabbccdd"
	a.serveBrowserCopy(ana, &inMsg{T: "browserCopy", Presets: []config.Preset{p}})
	got := anasProfile(t, a)
	if len(got.Presets) != 1 || got.Presets[0].ID != p.ID {
		t.Fatalf("presets %+v", got.Presets)
	}
}

func TestBrowserCopyNotesReplacedItems(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Presets: []config.Preset{testPreset("Cam")}}); err != nil {
		t.Fatal(err)
	}
	other := testPreset("Cam")
	other.Items = []config.PresetItem{presetItem(3, []int{4}, 1)}
	if err := a.BrowserCopy(ana, profileReq{Presets: []config.Preset{other}}); err != nil {
		t.Fatal(err)
	}
	pr := fieldsByKey(waitSaves(t, a, 1)[0])["profile/presets"]
	if len(pr.Added) != 0 || pr.Note != "replaced: Cam" {
		t.Fatalf("presets field %+v", pr)
	}
}

func TestBrowserCopyNeedsASignedInUser(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	anon := watched(t, a, "10.0.0.4", "")
	if err := a.BrowserCopy(anon, profileReq{Prefs: map[string]string{"theme": "dark"}}); err == nil {
		t.Fatal("a guest copied this browser to a user")
	}
	// A name in the request does not pick the user: an Administrator copies only to herself.
	ada := watched(t, a, "10.0.0.5", "2468")
	if err := a.BrowserCopy(ada, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	if p := anasProfile(t, a); p.Prefs["theme"] != "" {
		t.Fatalf("Ana's profile changed: %+v", p)
	}
	if u, _ := a.r().cfg.FindUserByName("Ada"); u.Profile.Prefs["theme"] != "dark" {
		t.Fatalf("Ada's profile %+v", u.Profile)
	}
	if rows := waitSaves(t, a, 1); len(rows) != 1 || rows[0].By != "Ada" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestBrowserCopyWithNothingNewWritesNoRow(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	ana := watched(t, a, "10.0.0.1", "1234")
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	if err := a.BrowserCopy(ana, profileReq{Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	onlySentinel(t, a)
}

// onlySentinel writes a row and fails if another row came before it. The log writes in order.
func onlySentinel(t *testing.T, a *App) {
	t.Helper()
	a.recordSave(Actor{Kind: actorUser, Name: "test"}, "sentinel", []SavedField{newSavedField("brandText", "branding", "x")}, false)
	rows := waitSaves(t, a, 1)
	if len(rows) != 1 || rows[0].Why != "sentinel" {
		t.Fatalf("rows %+v", rows)
	}
}

func TestPrefLabelsMatchPage(t *testing.T) {
	b, err := os.ReadFile("../../web/index.html")
	if err != nil {
		t.Fatal(err)
	}
	re := regexp.MustCompile(`<label for="cfg-[\w-]+">([^<]+)</label>\s*<span class="pb-select-wrap"><select [^>]*data-pref="(\w+)"`)
	found := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(string(b), -1) {
		found[m[2]] = m[1]
	}
	if len(found) != len(config.PrefValues) || len(prefLabels) != len(found) {
		t.Fatalf("page %v, prefLabels %v", found, prefLabels)
	}
	for k, v := range found {
		if prefLabels[k] != v {
			t.Errorf("%s: page %q, prefLabels %q", k, v, prefLabels[k])
		}
	}
}
