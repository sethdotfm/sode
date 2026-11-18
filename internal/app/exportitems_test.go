package app

import (
	"bytes"
	"encoding/json"
	"maps"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// Two presets and one panel picked: the zip holds those, the router they name and nothing else.
func TestExportTwoPresetsAndOnePanel(t *testing.T) {
	a, tok := exportApp(t)
	seedEverything(t, a)
	ana := userID(t, a, "Ana")
	lifeCommit(t, a, func(c *config.Config) {
		c.Presets = append(c.Presets,
			config.Preset{ID: "pr-2", Name: "Late", Items: []config.PresetItem{{Src: c.SourceRef(1, ""), Dsts: []config.Ref{c.DestinationRef(1, "")}}}},
			config.Preset{ID: "pr-3", Name: "Spare", Items: []config.PresetItem{{Src: c.SourceRef(3, ""), Dsts: []config.Ref{c.DestinationRef(3, "")}}}})
	})
	other, err := a.panels.create(Panel{Name: "Other", Owner: ana, Rows: 1, Cols: 1, Pages: []PanelPage{{ID: "pg-1", Enabled: true}}})
	if err != nil {
		t.Fatal(err)
	}
	desk := ""
	for _, p := range a.panels.list() {
		if p.Name == "Desk" {
			desk = p.ID
		}
	}
	rec := postExport(t, a, tok, exportReq{Categories: []string{catPresets, catPanels},
		Pick: map[string][]string{catPresets: {"pr-1", "pr-3"}, catPanels: {desk}}})
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	files := unzipAll(t, rec.Body.Bytes())
	var names []string
	for n := range files {
		if n != "names/"+a.r().cfg.UID+".yaml" && !bytes.HasPrefix([]byte(n), []byte("art/panels/"+desk+"/")) {
			names = append(names, n)
		}
	}
	slices.Sort(names)
	if want := []string{"manifest.yaml", "panels.yaml", "presets.yaml", "routers.yaml"}; !slices.Equal(names, want) {
		t.Fatalf("entries %v, want %v and the names table", slices.Sorted(maps.Keys(files)), want)
	}
	var pr exportPresets
	yaml.Unmarshal(files["presets.yaml"], &pr)
	if len(pr.Presets) != 2 || pr.Presets[0].Name != "Show" || pr.Presets[1].Name != "Spare" {
		t.Errorf("presets: %+v", pr.Presets)
	}
	var pn exportPanels
	yaml.Unmarshal(files["panels.yaml"], &pn)
	if len(pn.Panels) != 1 || pn.Panels[0].ID != desk || pn.Panels[0].ID == other.ID || len(pn.Templates) != 0 || len(pn.CheckpointSets) != 0 {
		t.Errorf("panels: %+v", pn)
	}
}

func getItems(t *testing.T, a *App, tok string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest("GET", "/api/export/items", nil)
	if tok != "" {
		r.Header.Set(sessionHeader, tok)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func postSize(t *testing.T, a *App, tok string, req exportReq) int {
	t.Helper()
	b, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/export/size", bytes.NewReader(b))
	r.Header.Set(sessionHeader, tok)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	if rec.Code != 200 {
		t.Fatalf("size: %d %s", rec.Code, rec.Body)
	}
	var out struct{ Size int }
	json.Unmarshal(rec.Body.Bytes(), &out)
	return out.Size
}

// Every item the list offers, sent back alone in pick, exports exactly that item, and its size is counted.
func TestExportItemsRoundTripThroughPick(t *testing.T) {
	a, tok := exportApp(t)
	seedEverything(t, a)
	rec := getItems(t, a, tok)
	if rec.Code != 200 {
		t.Fatalf("items: %d %s", rec.Code, rec.Body)
	}
	var list exportList
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, c := range list.Categories {
		got = append(got, c.Name)
		sum := 0
		for _, s := range c.Sections {
			sum += s.Size
		}
		for _, it := range c.Items {
			sum += it.Size
			if it.Size <= 0 || it.Name == "" {
				t.Errorf("%s item %+v", c.Name, it)
			}
			if c.Name == catRouters {
				continue
			}
			files := unzipAll(t, postExport(t, a, tok, exportReq{Categories: []string{c.Name}, Pick: map[string][]string{c.Name: {it.ID}}}).Body.Bytes())
			if n := countItems(t, c.Name, files); n != 1 {
				t.Errorf("%s %q alone exported %d items", c.Name, it.ID, n)
			}
			if _, named := files["routers.yaml"]; named != it.Router {
				t.Errorf("%s %q: names the router %v, routers.yaml went %v", c.Name, it.Name, it.Router, named)
			}
		}
		if c.Name != catBrowser && (c.Size != sum || sum == 0) {
			t.Errorf("%s: size %d, items add up to %d", c.Name, c.Size, sum)
		}
	}
	if !slices.Equal(got, everyCategory) {
		t.Errorf("categories %v", got)
	}
	var secs []string
	for _, s := range list.Categories[0].Sections {
		secs = append(secs, s.ID)
	}
	if len(secs) != len(exportSections) || secs[0] != exportSections[0].Name {
		t.Errorf("sections %v", secs)
	}
	all := exportReq{Categories: everyCategory, IncludePins: true}
	if want := len(postExport(t, a, tok, all).Body.Bytes()); list.All != want || postSize(t, a, tok, all) != want {
		t.Errorf("Back up everything is %d bytes, list says %d", want, list.All)
	}
}

func countItems(t *testing.T, cat string, files map[string][]byte) int {
	t.Helper()
	var v struct {
		Users          []any `yaml:"users"`
		Presets        []any `yaml:"presets"`
		FilterSets     []any `yaml:"filterSets"`
		APIClients     []any `yaml:"apiClients"`
		Panels         []any `yaml:"panels"`
		Templates      []any `yaml:"templates"`
		CheckpointSets []any `yaml:"checkpointSets"`
		Defaults       []any `yaml:"defaults"`
		Icons          []any `yaml:"icons"`
	}
	if err := yaml.Unmarshal(files[categoryFile(cat)], &v); err != nil {
		t.Fatal(err)
	}
	return len(v.Users) + len(v.Presets) + len(v.FilterSets) + len(v.APIClients) + len(v.Panels) + len(v.Templates) +
		len(v.CheckpointSets) + len(v.Defaults) + len(v.Icons)
}

// Back up everything: the body backupBody in web/exportrules.js sends exports every category with PINs.
func TestExportBackUpEverything(t *testing.T) {
	a, tok := exportApp(t)
	seedEverything(t, a)
	body := json.RawMessage(`{"categories":["settings","routers","users","presets","filterSets","panels","artwork","apiClients","browser"],` +
		`"includePins":true,"browser":{"theme":"dark","filterSets":[],"presets":[]}}`)
	rec := postExport(t, a, tok, body)
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	files := unzipAll(t, rec.Body.Bytes())
	for _, c := range exportCategories {
		if len(files[c.File]) == 0 {
			t.Errorf("no %s", c.File)
		}
	}
	var users exportUsers
	yaml.Unmarshal(files["users.yaml"], &users)
	for _, u := range users.Users {
		if u.Pin == "" || u.Len == 0 {
			t.Errorf("user %s went without a PIN", u.Name)
		}
	}
	if len(users.Users) != len(a.r().cfg.Users) {
		t.Errorf("%d users of %d", len(users.Users), len(a.r().cfg.Users))
	}
}

func TestExportItemsNeedAnAdministrator(t *testing.T) {
	a, _ := exportApp(t)
	if rec := getItems(t, a, ""); rec.Code != http.StatusUnauthorized {
		t.Errorf("no session: %d", rec.Code)
	}
}

// A non-persistent Administrator signed in at the PIN lock has no token; exportSession gives one that the
// export routes accept. A Standard user is refused, and an install with no users needs none.
func TestExportSessionGivesTheRoutesAToken(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 64)}
	register(a, c)
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	if err := admitMsg(a, c, inMsg{T: "exportSession"}); err != nil {
		t.Fatal(err)
	}
	tok, err := a.ExportSession(c)
	if err != nil || tok == "" {
		t.Fatalf("token %q, %v", tok, err)
	}
	if rec := getItems(t, a, tok); rec.Code != 200 {
		t.Fatalf("items with the token: %d %s", rec.Code, rec.Body)
	}
	again, _ := a.ExportSession(c)
	if again != tok {
		t.Error("a second ask made a new token")
	}

	bo := &client{ip: "10.0.0.8", send: make(chan []byte, 64)}
	register(a, bo)
	if err := a.UserUnlockAs(bo, "Bo", "567890"); err != nil {
		t.Fatal(err)
	}
	if err := admitMsg(a, bo, inMsg{T: "exportSession"}); err == nil {
		t.Error("a Standard user got an export session")
	}

	none := newTestApp(t)
	g := &client{ip: "10.0.0.7", send: make(chan []byte, 64)}
	register(none, g)
	if tok, err := none.ExportSession(g); err != nil || tok != "" {
		t.Errorf("no users: %q, %v", tok, err)
	}
	if rec := getItems(t, none, ""); rec.Code != 200 {
		t.Errorf("no users, no token: %d", rec.Code)
	}
}
