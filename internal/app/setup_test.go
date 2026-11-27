package app

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func setupApp(t *testing.T, edit ...func(*config.Config)) *App {
	t.Helper()
	cfg := config.Defaults()
	cfg.Routers[0].Address = "127.0.0.1:1"
	cfg.Routers[0].Levels = 3
	for _, fn := range edit {
		fn(&cfg)
	}
	cfg.Normalize()
	hist, _ := OpenHistory("")
	a, err := New(cfg, filepath.Join(settledTempDir(t), "config.yaml"), hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	shortTimers(a)
	return a
}

func wizardAnswers() map[string]any {
	return map[string]any{
		"brandText": "Studio B", "brandBorder": "teal", "name": "Ada", "pin": "2468", "title": "Studio Ultrix",
		"router": "127.0.0.1:2", "levels": 4, "levelNumbering": "0", "numberOffset": 0,
		"fireMode": "native", "group": 120, "preset": "secured",
	}
}

func setupInHello(t *testing.T, a *App) bool {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(a.hello(helloPage{}), &m); err != nil {
		t.Fatal(err)
	}
	return m["setup"] != nil
}

func dirFiles(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			out = append(out, p)
		}
		return nil
	})
	return out
}

// With SODE_CONFIG and no users, only the setup messages pass until Finish. A take, a save and an
// Administrator action are refused. A keyless API take is also refused.
func TestSetupWizardRefusesTakesAndSavesBeforeFinish(t *testing.T) {
	a := setupApp(t, func(c *config.Config) {
		c.API.GuestEnabled = true
		c.GuestAPIScopes = []string{config.ScopeNames, config.ScopeTake}
	})
	p := dialPeer(t, a)
	if p.hello["setup"] == nil {
		t.Fatalf("the hello does not open the wizard: %v", p.hello["setup"])
	}

	p.send(map[string]any{"t": "take", "items": []any{map[string]any{"id": 7, "dst": 1, "src": 1, "levels": []int{1}}}})
	if r := p.until("takeResult"); !strings.Contains(r["err"].(string), "not set up yet") {
		t.Fatalf("take before Finish: %v", r)
	}
	r := p.saveFields(3, map[string]any{"brandText": "X"})
	if r["ok"] == true || !strings.Contains(r["err"].(string), "not set up yet") || r["locked"] == true {
		t.Fatalf("settings save before Finish: %v", r)
	}
	if r := p.ask("usersList", 4, nil); r["ok"] == true {
		t.Fatalf("usersList before Finish: %v", r)
	}
	if rec := apiPost(a, "/api/v1/takes", "", `{"takes":[{"dst":0,"src":0}]}`, nil); rec.Code < 400 {
		t.Fatalf("keyless API take before Finish: %d %s", rec.Code, rec.Body)
	}
	for _, path := range []string{"/api/export", "/api/import"} {
		if rec := apiPost(a, path, "", `{}`, nil); rec.Code != 503 {
			t.Errorf("%s before Finish: %d %s", path, rec.Code, rec.Body)
		}
	}
	if rec := apiDo(a, "GET", "/api/health", "", nil); rec.Code == 403 || rec.Code == 409 {
		t.Fatalf("health before Finish: %d", rec.Code)
	}
}

// Without SODE_CONFIG there is no wizard; with no users every browser is an Administrator.
func TestSetupWizardNotInEnvOnlyMode(t *testing.T) {
	a := newTestApp(t)
	p := dialPeer(t, a)
	if p.hello["setup"] != nil {
		t.Fatalf("env-only mode opens the wizard: %v", p.hello["setup"])
	}
	if r := p.saveFields(3, map[string]any{"brandText": "X"}); r["ok"] != true {
		t.Fatalf("env-only save: %v", r)
	}
}

// Finish writes the wizard's values and the preset, makes the first Administrator and signs this
// browser in as them. Nothing is on disk before it, even after the router step read names.
func TestSetupWizardFinishMakesASignedInAdministrator(t *testing.T) {
	a := setupApp(t)
	dir := filepath.Dir(a.cfgPath)
	p := dialPeer(t, a)

	if r := p.ask("setupRouter", 2, map[string]any{"router": "127.0.0.1:2"}); r["ok"] != true {
		t.Fatalf("router step: %v", r)
	}
	if a.r().cfg.Routers[0].Address != "127.0.0.1:2" {
		t.Fatalf("the router step did not move the link: %q", a.r().cfg.Routers[0].Address)
	}
	readNames(a, []string{"CAM 1"}, []string{"MON 1"})
	if f := dirFiles(t, dir); len(f) > 0 {
		t.Fatalf("written before Finish: %v", f)
	}

	r := p.ask("setupFinish", 3, map[string]any{"setup": wizardAnswers()})
	if r["ok"] != true {
		t.Fatalf("Finish: %v", r)
	}
	c := loadFile(t, a.cfgPath)
	ada, ok := c.FindUserByName("Ada")
	if !ok || ada.Role != config.RoleAdministrator || !config.CheckPin(ada.Pin, "2468") || ada.Len != 4 {
		t.Fatalf("first user: %+v", ada)
	}
	if c.BrandText != "Studio B" || c.BrandBorder != "teal" || c.Routers[0].Title != "Studio Ultrix" || c.Routers[0].Address != "127.0.0.1:2" || c.Routers[0].Levels != 4 ||
		c.Routers[0].LevelNumbering != "0" || c.Routers[0].NumberOffset != 0 || c.Routers[0].FireMode != "native" || c.Routers[0].SalvoGroup != 120 {
		t.Fatalf("wizard values not saved: %+v", c.DefaultRouter())
	}
	// Secured access: both page locks on, no keyless API, PIN waits on.
	if !c.Locks.Main || !c.Locks.Panel || c.API.GuestEnabled || !c.PinWaits || c.NewPanelActs != "session" {
		t.Fatalf("Secured access not applied: locks %+v api %v waits %v acts %q", c.Locks, c.API.GuestEnabled, c.PinWaits, c.NewPanelActs)
	}
	if r := p.ask("usersList", 4, nil); r["ok"] != true {
		t.Fatalf("not signed in as the Administrator after Finish: %v", r)
	}
	if setupInHello(t, a) {
		t.Fatal("the hello still opens the wizard")
	}

	q := dialPeer(t, a)
	if r := q.ask("setupFinish", 5, map[string]any{"setup": wizardAnswers()}); r["ok"] == true {
		t.Fatalf("a second Finish passed: %v", r)
	}
}

// Finish writes one Settings saves row, with every value new and the PIN only as changed. Under
// Standard access the new user is persistent, so the page gets a token to keep the sign-in.
func TestSetupWizardFinishRecordsOneSavesRow(t *testing.T) {
	a := setupApp(t)
	defer startLog(t, a)()
	p := dialPeer(t, a)
	ans := wizardAnswers()
	ans["preset"] = "standard"
	r := p.ask("setupFinish", 3, map[string]any{"setup": ans})
	if tok, _ := r["token"].(string); r["ok"] != true || tok == "" || r["tab"] == true {
		t.Fatalf("Finish: %v", r)
	}
	rows := waitSaves(t, a, 1)
	if len(rows) != 1 || rows[0].Why != "setup" || rows[0].By != "Ada" {
		t.Fatalf("saves rows: %+v", rows)
	}
	by := map[string]SavedField{}
	for _, f := range rows[0].Save.Fields {
		if f.Old != nil || f.Removed != nil {
			t.Errorf("%s has an old value: %s", f.Key, f.Old)
		}
		by[f.Key] = f
	}
	for key, want := range map[string]string{"brandText": `"Studio B"`, aKey(a, "address"): `"127.0.0.1:2"`, aKey(a, "title"): `"Studio Ultrix"`, aKey(a, "levels"): "4", aKey(a, "fireMode"): `"native"`, "pinWaits": "true"} {
		if got := string(by[key].New); got != want {
			t.Errorf("%s: new %s, want %s", key, got, want)
		}
	}
	var pin bool
	for k, f := range by {
		if strings.HasSuffix(k, "/pin") {
			pin = f.Changed && f.New == nil
		}
	}
	if !pin {
		t.Errorf("no PIN field marked changed: %v", by)
	}
	b, _ := json.Marshal(rows[0])
	if strings.Contains(string(b), "2468") {
		t.Fatal("the PIN is in the row")
	}
}

// A refused Finish writes nothing and keeps the wizard open.
func TestSetupWizardFinishRefusesBadAnswers(t *testing.T) {
	a := setupApp(t)
	p := dialPeer(t, a)
	for i, bad := range []map[string]any{
		{"name": ""}, {"pin": "12"}, {"pin": "12ab"}, {"levels": 0}, {"preset": "wide open"}, {"router": "nohost"},
		{"title": strings.Repeat("x", 33)},
	} {
		ans := wizardAnswers()
		for k, v := range bad {
			ans[k] = v
		}
		if r := p.ask("setupFinish", 10+i, map[string]any{"setup": ans}); r["ok"] == true {
			t.Errorf("%v passed", bad)
		}
	}
	if f := dirFiles(t, filepath.Dir(a.cfgPath)); len(f) > 0 {
		t.Fatalf("a refused Finish wrote %v", f)
	}
	if !setupInHello(t, a) {
		t.Fatal("a refused Finish closed the wizard")
	}
}

// The Access step's sheet comes from config.AccessPresets, so the page holds no copy.
func TestSetupWizardHelloCarriesThePresetSheet(t *testing.T) {
	t.Setenv("SODE_ACCESS_PRESET", "Open access")
	var m struct {
		Setup struct {
			Preset  string `json:"preset"`
			Presets []struct {
				Key  string          `json:"key"`
				Name string          `json:"name"`
				Rows map[string]bool `json:"rows"`
			} `json:"presets"`
		} `json:"setup"`
	}
	if err := json.Unmarshal(setupApp(t).hello(helloPage{}), &m); err != nil {
		t.Fatal(err)
	}
	if m.Setup.Preset != "open" {
		t.Errorf("SODE_ACCESS_PRESET not preselected: %q", m.Setup.Preset)
	}
	want := map[string][]bool{
		"takeNoSignIn":   {true, true, false},
		"sharedNoSignIn": {true, false, false},
		"panelLinks":     {true, true, false},
		"appsRead":       {true, true, false},
		"appsTake":       {true, false, false},
		"pinWaits":       {false, true, true},
		"tieNoSignIn":    {true, false, false},
	}
	if len(m.Setup.Presets) != 3 || m.Setup.Presets[1].Name != "Standard access" {
		t.Fatalf("presets: %+v", m.Setup.Presets)
	}
	for row, cols := range want {
		for i, p := range m.Setup.Presets {
			if v, ok := p.Rows[row]; !ok || v != cols[i] {
				t.Errorf("%s / %s: %v, want %v", p.Name, row, v, cols[i])
			}
		}
	}
}

// Before Finish, admit lets through the hello's hi, activity, History and the setup messages only.
func TestSetupWizardAdmitsOnlySetupMessages(t *testing.T) {
	a := setupApp(t)
	c := &client{ip: "10.0.0.1"}
	for m, r := range policy {
		err := a.admit(c, r.forMsg(&inMsg{T: m}))
		if r.setup != (err == nil) || (err != nil && !errors.Is(err, errSetup)) {
			t.Errorf("%s while setup is pending: %v", m, err)
		}
	}
}
