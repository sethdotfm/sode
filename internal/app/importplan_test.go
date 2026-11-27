package app

import (
	"bytes"
	"encoding/json"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// foreign makes a file look like another install's.
func foreign(t *testing.T, file []byte) []byte {
	t.Helper()
	files := map[string]string{}
	for name, b := range unzipAll(t, file) {
		files[name] = string(b)
	}
	files["manifest.yaml"] = regexp.MustCompile(`(?m)^install: .*$`).ReplaceAllString(files["manifest.yaml"], "install: 0000000000000000")
	return zipOf(t, files)
}

func TestAddImportForeignClashRenames(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catPresets}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(5, "MON 5")} })

	// No recheck: Rename only adds.
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	it := res.Plan.Categories[catPresets].Items[0]
	if res.Plan.SameInstall || res.Plan.Critical || it.Choice != useRename || it.As != "Show 2" {
		t.Fatalf("plan: %+v %+v", res.Plan, it)
	}
	live, _ := presetNamed(t, a, "Show")
	two, ok := presetNamed(t, a, "Show 2")
	if !ok || live.Items[0].Dsts[0].N != 5 || two.Items[0].Dsts[0].N != 4 || two.ID == live.ID {
		t.Fatalf("presets: %+v", a.r().cfg.Presets)
	}
}

func TestAddImportUseCurrentAndSkipKeepLive(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	id := a.r().cfg.Presets[0].ID
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(5, "MON 5")} })
	for _, ch := range []string{useCurrent, useSkip} {
		res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd, Choices: map[string]map[string]string{catPresets: {id: ch}}}))
		if res.Plan.Critical || len(a.r().cfg.Presets) != 1 || a.r().cfg.Presets[0].Items[0].Dsts[0].N != 5 {
			t.Fatalf("%s: %+v %+v", ch, res.Plan, a.r().cfg.Presets)
		}
	}
}

func TestAddImportMissingReferenceIsBroken(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = nil })
	a.mu.Lock()
	a.first.router.dstNames[4] = "MON 9"
	a.mu.Unlock()

	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	p, ok := presetNamed(t, a, "Show")
	want := []config.Missing{{Kind: config.MissingDestination, Ref: config.Ref{Router: a.r().cfg.Routers[0].UID, N: 4, Name: "MON 4"}}}
	if !ok || !reflect.DeepEqual(p.Broken, want) {
		t.Fatalf("preset: %+v", p)
	}
	if b := res.Plan.Categories[catPresets].Items[0].Broken; !reflect.DeepEqual(b, want) {
		t.Errorf("plan Broken: %+v", b)
	}
}

func TestAddImportWritesOneSavesRow(t *testing.T) {
	a, _, tok := importApp(t)
	defer startLog(t, a)()
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")}
		cfg.FilterSets = []config.FilterSet{{Name: "Cams", Src: "CAM"}}
	})
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets, catFilterSets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets, cfg.FilterSets = nil, nil })
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	waitSaves(t, a, 1)
	time.Sleep(50 * time.Millisecond)
	rows := a.saves.Between(-1, 1<<62)
	if len(rows) != 1 {
		t.Fatalf("one import wrote %d rows", len(rows))
	}
	e := rows[0]
	if e.Kind != kindSettings || e.Why != "import" || e.By != "Ada" || e.Save.Import == nil || e.Save.Import.Mode != importAdd {
		t.Fatalf("row: %+v", e)
	}
	got := map[string]int{}
	for _, c := range e.Save.Import.Categories {
		got[c.Category] = c.Added
	}
	if got[catPresets] != 1 || got[catFilterSets] != 1 {
		t.Errorf("counts: %+v", e.Save.Import.Categories)
	}
	if b, _ := json.Marshal(e); bytes.Contains(b, []byte("CAM")) {
		t.Errorf("the row holds values: %s", b)
	}
}

// The check route answers the plan the apply would write with no choices made.
func TestImportCheckAnswersThePlan(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets[0].Name = "Show live" })
	rec := postImport(t, a, tok, file)
	var check importCheck
	if rec.Code != 200 || json.Unmarshal(rec.Body.Bytes(), &check) != nil {
		t.Fatalf("check: %d %s", rec.Code, rec.Body)
	}
	it := check.Plan.Categories[catPresets].Items[0]
	if !check.Plan.Critical || it.Clash != "Show live" || it.Choice != useNew {
		t.Fatalf("plan: %+v", check.Plan)
	}
}

// Settings from another install never bring its install UID: Item links made here must keep working.
func TestAddImportKeepsInstallUID(t *testing.T) {
	a, c, tok := importApp(t)
	uid := a.r().cfg.InstallUID
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catSettings}}))
	lifeCommit(t, a, func(cfg *config.Config) { cfg.BrandText = "Live" })
	grantRecheck(a, c)
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if cfg := a.r().cfg; cfg.InstallUID != uid || cfg.BrandText != "" {
		t.Fatalf("install UID %q (was %q), brand %q", cfg.InstallUID, uid, cfg.BrandText)
	}
}

// API settings an import changes write their API audit rows, as a save in Settings does.
func TestAddImportWritesAPIAuditRows(t *testing.T) {
	a, c, tok := importApp(t)
	defer startLog(t, a)()
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.API.GuestEnabled = true
		cfg.API.RateLimit = 77
		cfg.GuestAPIScopes = []string{config.ScopeNames}
	})
	file := exported(t, a, tok, exportReq{Categories: []string{catSettings}, Sections: []string{"api"}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.API.GuestEnabled, cfg.API.RateLimit, cfg.GuestAPIScopes = false, 5, nil })
	grantRecheck(a, c)
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	want := []string{"keylessScopes", "guestApi", "apiLimits"}
	var got map[string]bool
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		got = map[string]bool{}
		for _, e := range logged(t, a) {
			if e.Kind == kindAPI {
				got[e.Why] = true
			}
		}
		if len(got) >= len(want) {
			break
		}
	}
	for _, w := range want {
		if !got[w] {
			t.Errorf("no %s row; got %v", w, got)
		}
	}
}

// A file user with no id, such as one written by hand, is the live user of that name.
func TestAddImportMatchesUserWithoutIDByName(t *testing.T) {
	a, _, tok := importApp(t)
	ana := userID(t, a, "Ana")
	files := map[string]string{}
	for name, b := range unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catUsers}, Pick: map[string][]string{catUsers: {ana}}})) {
		files[name] = string(b)
	}
	files["users.yaml"] = regexp.MustCompile(`id: u-[0-9a-f]+\n\s*`).ReplaceAllString(files["users.yaml"], "")
	if strings.Contains(files["users.yaml"], ana) {
		t.Fatalf("the id is still in the file:\n%s", files["users.yaml"])
	}
	res := applied(t, postApply(t, a, tok, zipOf(t, files), importReq{Mode: importAdd}))
	if it := res.Plan.Categories[catUsers].Items[0]; !it.Same || it.Clash != "Ana" {
		t.Fatalf("plan: %+v", res.Plan.Categories[catUsers])
	}
	if n := len(a.r().cfg.Users); n != 3 || userID(t, a, "Ana") != ana {
		t.Fatalf("users: %+v", a.r().cfg.Users)
	}
}

func dropUser(t *testing.T, a *App, name string) {
	t.Helper()
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users = slices.DeleteFunc(cfg.Users, func(u config.User) bool { return u.Name == name })
	})
}

// A new user arrives PIN needed with their Role. Their API client, whose owner has no PIN, goes to the
// importing Administrator, as does a panel whose owner the import leaves out.
func TestAddImportGivesOwnerlessItemsToTheAdministrator(t *testing.T) {
	a, _, tok := importApp(t)
	ana := userID(t, a, "Ana")
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users[0].Role = config.RoleTrusted
		cfg.APIClients = []config.APIClient{{Name: "Bot", Scopes: []string{config.ScopeNames}, Owner: ana}}
	})
	if _, err := a.panels.create(Panel{Name: "Desk", Owner: ana, Rows: 1, Cols: 1}); err != nil {
		t.Fatal(err)
	}
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers, catAPIClients, catPanels}, Pick: map[string][]string{catUsers: {ana}}})

	b, _, tok2 := importApp(t)
	ada := userID(t, b, "Ada")
	dropUser(t, b, "Ana")
	res := applied(t, postApply(t, b, tok2, file, importReq{Mode: importAdd}))
	if res.Plan.Critical {
		t.Errorf("an import that only adds is Critical: %+v", res.Plan)
	}
	u, ok := b.r().cfg.FindUserByName("Ana")
	if !ok || u.HasPin() || !u.PinNeeded || u.Role != config.RoleTrusted {
		t.Fatalf("Ana: %+v", u)
	}
	if c, ok := b.r().cfg.FindAPIClient("Bot"); !ok || c.Owner != ada {
		t.Fatalf("client: %+v %v", c, ok)
	}
	if ps := b.panels.list(); len(ps) != 1 || ps[0].Name != "Desk" || ps[0].Owner != u.ID {
		t.Fatalf("panels: %+v", ps)
	}

	c, _, tok3 := importApp(t)
	dropUser(t, c, "Ana")
	applied(t, postApply(t, c, tok3, file, importReq{Mode: importAdd, Categories: []string{catPanels}}))
	if ps := c.panels.list(); len(ps) != 1 || ps[0].Owner != userID(t, c, "Ada") {
		t.Fatalf("panels: %+v", ps)
	}
}

// Use new on a user writes the file's Role and keeps the live PIN when the file carries no hash.
func TestAddImportUseNewKeepsLivePin(t *testing.T) {
	a, c, tok := importApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Users[0].Role = config.RoleTrusted })
	pin := a.r().cfg.Users[0].Pin
	if rec := postApply(t, a, tok, file, importReq{Mode: importAdd}); rec.Code != 403 {
		t.Fatalf("without a recheck: %d %s", rec.Code, rec.Body)
	}
	grantRecheck(a, c)
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if u := a.r().cfg.Users[0]; u.Role != config.RoleStandard || u.Pin != pin || !u.HasPin() {
		t.Fatalf("Ana: %+v", u)
	}
}

// A hashless foreign user renamed past a live user gets neither that user's PIN nor the importer's Role.
func TestAddImportRenamedHashlessUserIsPinNeeded(t *testing.T) {
	a, _, tok := importApp(t)
	files := map[string]string{}
	for name, b := range unzipAll(t, foreign(t, exported(t, a, tok, exportReq{Categories: []string{catUsers}}))) {
		files[name] = string(b)
	}
	y := strings.Replace(files["users.yaml"], "role: standard", "role: trusted", 1)
	files["users.yaml"] = regexp.MustCompile(`(?m)^( *)role: administrator$`).ReplaceAllString(y, "${1}role: trusted\n${1}noAutoLock: true")
	b, _, tok2 := importApp(t)
	applied(t, postApply(t, b, tok2, zipOf(t, files), importReq{Mode: importAdd}))
	for name, role := range map[string]string{"Ana 2": config.RoleTrusted, "Ada 2": config.RoleTrusted} {
		u, ok := b.r().cfg.FindUserByName(name)
		if !ok || u.HasPin() || !u.PinNeeded || u.Role != role {
			t.Errorf("%s: %+v", name, u)
		}
	}
}

// A new panel brings its artwork under the same ids, so its cells still find it.
func TestAddImportBringsPanelArt(t *testing.T) {
	a, _, tok := importApp(t)
	p, err := a.panels.create(Panel{Name: "Desk", Owner: userID(t, a, "Ana"), Rows: 1, Cols: 1})
	if err != nil {
		t.Fatal(err)
	}
	art, _, err := a.panels.addArt(p.ID, imgBytes("png", 1, 1), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	file := exported(t, a, tok, exportReq{Categories: []string{catPanels}})
	b, _, tok2 := importApp(t)
	applied(t, postApply(t, b, tok2, file, importReq{Mode: importAdd}))
	ps := b.panels.list()
	if len(ps) != 1 || len(ps[0].Art) != 1 || ps[0].Art[0].ID != art.ID {
		t.Fatalf("panels: %+v", ps)
	}
	if data, _, ok := b.panels.artData(ps[0].ID, art.ID); !ok || !bytes.Equal(data, imgBytes("png", 1, 1)) {
		t.Fatalf("art: %q %v", data, ok)
	}
}

// A panel image that is missing or not its stated type refuses the panel. The live artwork stays.
func TestAddImportRefusesPanelWithBadArt(t *testing.T) {
	a, c, tok := importApp(t)
	p, err := a.panels.create(Panel{Name: "Desk", Owner: userID(t, a, "Ana"), Rows: 1, Cols: 1})
	if err != nil {
		t.Fatal(err)
	}
	art, _, err := a.panels.addArt(p.ID, imgBytes("png", 1, 1), "image/png")
	if err != nil {
		t.Fatal(err)
	}
	file := unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catPanels}}))
	if _, err := a.panels.replace(Panel{ID: p.ID, Name: "Desk", Owner: p.Owner, Rows: 1, Cols: 2, Art: []PanelArt{art}}); err != nil {
		t.Fatal(err)
	}
	grantRecheck(a, c)
	entry := "art/panels/" + p.ID + "/" + art.ID + ".png"
	for name, data := range map[string]*string{"missing": nil, "html": ptr("<html><script>x</script></html>")} {
		files := map[string]string{}
		for k, v := range file {
			if k != entry || data != nil {
				files[k] = string(v)
			}
		}
		if data != nil {
			files[entry] = *data
		}
		res := applied(t, postApply(t, a, tok, zipOf(t, files), importReq{Mode: importAdd}))
		if it := res.Plan.Categories[catPanels].Items[0]; it.Refused == "" {
			t.Errorf("%s: not refused: %+v", name, it)
		}
		if lp, _ := a.panels.get(p.ID); len(lp.Art) != 1 || lp.Cols != 2 {
			t.Errorf("%s: live panel: %+v", name, lp)
		}
	}
}

// Use new on a panel whose file short name another live panel now holds keeps the live short name.
func TestAddImportUseNewPanelSlugTaken(t *testing.T) {
	a, c, tok := importApp(t)
	mk := func(name, slug string) Panel {
		p, err := a.panels.create(Panel{Name: name, Slug: slug, Owner: userID(t, a, "Ana"), Rows: 1, Cols: 1})
		if err != nil {
			t.Fatal(err)
		}
		return p
	}
	desk, wall := mk("Desk", "desk"), mk("Wall", "")
	file := exported(t, a, tok, exportReq{Categories: []string{catPanels}, Pick: map[string][]string{catPanels: {desk.ID}}})
	desk.Slug, desk.Cols = "front", 2
	wall.Slug = "desk"
	for _, p := range []Panel{desk, wall} {
		if _, err := a.panels.replace(p); err != nil {
			t.Fatal(err)
		}
	}
	grantRecheck(a, c)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if got, _ := a.panels.get(desk.ID); len(res.Failed) != 0 || got.Cols != 1 || got.Slug != "front" {
		t.Fatalf("failed %v, desk %+v", res.Failed, got)
	}
}

// Users with no id, as in a file written by hand, map one by one: an ownerless panel does not go to one of them.
func TestAddImportUsersWithoutIDsKeepApart(t *testing.T) {
	a, _, tok := importApp(t)
	if _, err := a.panels.create(Panel{Name: "Desk", Rows: 1, Cols: 1}); err != nil {
		t.Fatal(err)
	}
	pick := map[string][]string{catUsers: {userID(t, a, "Ana"), userID(t, a, "Bo")}}
	files := map[string]string{}
	for name, b := range unzipAll(t, foreign(t, exported(t, a, tok, exportReq{Categories: []string{catUsers, catPanels}, Pick: pick}))) {
		files[name] = string(b)
	}
	files["users.yaml"] = regexp.MustCompile(`id: u-[0-9a-f]+\n\s*`).ReplaceAllString(files["users.yaml"], "")
	b, _, tok2 := importApp(t)
	dropUser(t, b, "Ana")
	dropUser(t, b, "Bo")
	applied(t, postApply(t, b, tok2, zipOf(t, files), importReq{Mode: importAdd}))
	if ps := b.panels.list(); len(ps) != 1 || ps[0].Owner != userID(t, b, "Ada") {
		t.Fatalf("panels: %+v", ps)
	}
}

// A key an older file lacks keeps its live value. An empty key the export leaves out still clears the live one.
func TestAddImportSettingsKeepKeysTheFileLacks(t *testing.T) {
	a, c, tok := importApp(t)
	files := map[string]string{}
	for name, b := range unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catSettings}, Sections: []string{"branding", "confirm"}})) {
		files[name] = string(b)
	}
	files["settings.yaml"] = regexp.MustCompile(`(?m)^\s*confirmLockoutMs: .*\n`).ReplaceAllString(files["settings.yaml"], "")
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.ConfirmLockoutMs, cfg.ConfirmEnter, cfg.BrandText = 900, !cfg.ConfirmEnter, "X"
	})
	enter := a.r().cfg.ConfirmEnter
	grantRecheck(a, c)
	applied(t, postApply(t, a, tok, zipOf(t, files), importReq{Mode: importAdd}))
	if cfg := a.r().cfg; cfg.ConfirmLockoutMs != 900 || cfg.ConfirmEnter == enter || cfg.BrandText != "" {
		t.Fatalf("lockout %d, enter %v, brand %q", cfg.ConfirmLockoutMs, cfg.ConfirmEnter, cfg.BrandText)
	}
}

// A nested key an older file lacks keeps its live value too.
func TestAddImportSettingsKeepNestedKeysTheFileLacks(t *testing.T) {
	a, c, tok := importApp(t)
	files := map[string]string{}
	secs := []string{"locks", "api", "everyone-defaults", "users"}
	for name, b := range unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catSettings}, Sections: secs})) {
		files[name] = string(b)
	}
	files["settings.yaml"] = regexp.MustCompile(`(?m)^\s+(panel|logReads|compact|noAutoLock): .*\n`).ReplaceAllString(files["settings.yaml"], "")
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Locks.Panel, cfg.API.LogReads, cfg.DeviceDefaults.Compact, cfg.NewUserDefaults.NoAutoLock = true, true, true, true
	})
	grantRecheck(a, c)
	applied(t, postApply(t, a, tok, zipOf(t, files), importReq{Mode: importAdd}))
	if cfg := a.r().cfg; !cfg.Locks.Panel || !cfg.API.LogReads || !cfg.DeviceDefaults.Compact || !cfg.NewUserDefaults.NoAutoLock {
		t.Fatalf("locks %+v, api %+v, compact %v, new users %+v", cfg.Locks, cfg.API, cfg.DeviceDefaults.Compact, cfg.NewUserDefaults)
	}
}

// A foreign panel shared with Bo follows the file's Bo, here renamed "Bo 2", never the live Bo. A public
// panel waits while Public panels allowed is off, and says so.
func TestAddImportPanelShareFollowsUsers(t *testing.T) {
	a, _, tok := importApp(t)
	mk := func(p Panel) {
		if _, err := a.panels.create(p); err != nil {
			t.Fatal(err)
		}
	}
	mk(Panel{Name: "Desk", Owner: userID(t, a, "Ada"), Rows: 1, Cols: 1, Share: PanelShare{Mode: panelShareNamed, Users: []string{"Bo"}}})
	mk(Panel{Name: "Lobby", Owner: userID(t, a, "Ada"), Rows: 1, Cols: 1, Acts: panelActsPublic})
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catUsers, catPanels}, Pick: map[string][]string{catUsers: {userID(t, a, "Bo")}}}))

	b, _, tok2 := importApp(t)
	lifeCommit(t, b, func(cfg *config.Config) {
		cfg.PublicPanels = false
		cfg.Users[1].Role = config.RoleTrusted // b's Bo is someone else
	})
	res := applied(t, postApply(t, b, tok2, file, importReq{Mode: importAdd}))
	var desk Panel
	for _, p := range b.panels.list() {
		if p.Name == "Desk" {
			desk = p
		}
		if p.Name == "Lobby" {
			t.Errorf("a public panel arrived with Public panels allowed off: %+v", p)
		}
	}
	if !slices.Equal(desk.Share.Users, []string{"Bo 2"}) {
		t.Fatalf("Desk shared with %v", desk.Share.Users)
	}
	refused := 0
	for _, it := range res.Plan.Categories[catPanels].Items {
		if it.Refused != "" {
			refused++
		}
	}
	if refused != 1 {
		t.Errorf("plan: %+v", res.Plan.Categories[catPanels])
	}
}

// Rights name presets by id, so they follow a preset that imports under another id.
func TestAddImportMapsRightsPresets(t *testing.T) {
	a, _, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")}
	})
	lifeCommit(t, a, func(cfg *config.Config) {
		ids := []string{cfg.Presets[0].ID}
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Bo" {
				cfg.Users[i].Rights.Presets = &ids
			}
		}
		cfg.APIClients = []config.APIClient{{Name: "Cue", Scopes: []string{config.ScopeNames}, Rights: config.Rights{Presets: &ids}}}
	})
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catUsers, catPresets, catAPIClients}}))
	dropUser(t, a, "Bo")
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(5, "MON 5")}
		cfg.APIClients = nil
	})
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	two, ok := presetNamed(t, a, "Show 2")
	if !ok {
		t.Fatalf("presets: %+v", a.r().cfg.Presets)
	}
	bo, _ := a.r().cfg.FindUserByName("Bo")
	cue, _ := a.r().cfg.FindAPIClient("Cue")
	for who, r := range map[string]config.Rights{"Bo": bo.Rights, "Cue": cue.Rights} {
		if r.Presets == nil || !slices.Equal(*r.Presets, []string{two.ID}) {
			t.Errorf("%s's preset Rights: %v, want [%s]", who, r.Presets, two.ID)
		}
	}
}

// The status label is in the Routers group of settings.yaml under its file name statusLabel, and an import sets it.
func TestStatusLabelExportsAndImports(t *testing.T) {
	a, c, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.RouterShow = "name" })
	files := map[string]string{}
	for name, b := range unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catSettings}, Sections: []string{"routers"}})) {
		files[name] = string(b)
	}
	if !regexp.MustCompile(`(?m)^routers:\n\s+statusLabel: name\n`).MatchString(files["settings.yaml"]) {
		t.Fatalf("settings.yaml:\n%s", files["settings.yaml"])
	}
	lifeCommit(t, a, func(cfg *config.Config) { cfg.RouterShow = "none" })
	grantRecheck(a, c)
	applied(t, postApply(t, a, tok, zipOf(t, files), importReq{Mode: importAdd}))
	if got := a.r().cfg.RouterShow; got != "name" {
		t.Fatalf("status label %q after the import", got)
	}
}
