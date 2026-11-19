package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// signInToken signs name in on a new connection and answers its session token.
func signInToken(t *testing.T, a *App, name, pin string) string {
	t.Helper()
	c := &client{ip: "10.0.0.8", send: make(chan []byte, 4096)}
	register(a, c)
	if err := a.UserUnlockAs(c, name, pin); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" {
		t.Fatalf("no session token for %s", name)
	}
	return tok
}

// postApplyWith is postApply from a trusted proxy that says the request came over proto.
func postApplyWith(t *testing.T, a *App, tok string, zipBytes []byte, req importReq, proto string) *httptest.ResponseRecorder {
	t.Helper()
	return postMultipart(t, a, "/api/import/apply", req, zipBytes, map[string]string{sessionHeader: tok, "X-Forwarded-Proto": proto})
}

func lastSavesRow(t *testing.T, a *App) Sent {
	t.Helper()
	waitSaves(t, a, 1)
	time.Sleep(50 * time.Millisecond)
	rows := a.saves.Between(-1, 1<<62)
	if len(rows) == 0 || rows[len(rows)-1].Save == nil || rows[len(rows)-1].Save.Import == nil {
		t.Fatalf("no import row: %+v", rows)
	}
	return rows[len(rows)-1]
}

func replaceReq(cats ...string) importReq { return importReq{Mode: importReplace, Categories: cats} }

func noKeep(r importReq) importReq {
	off := false
	r.KeepAdmins = &off
	return r
}

// replaceApp is importApp with the PIN recheck already given.
func replaceApp(t *testing.T) (*App, string) {
	t.Helper()
	a, c, tok := importApp(t)
	grantRecheck(a, c)
	return a, tok
}

func presetNames(a *App) []string {
	var out []string
	for _, p := range a.r().cfg.Presets {
		out = append(out, p.Name)
	}
	slices.Sort(out)
	return out
}

func TestReplaceImportLeavesOnlyFilePresets(t *testing.T) {
	a, tok := replaceApp(t)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4"), showPreset(*cfg, "Open", "MON 4")}
	})
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(5, "MON 5")}
		cfg.Presets = append(cfg.Presets, showPreset(*cfg, "Late", "MON 5"))
	})
	res := applied(t, postApply(t, a, tok, file, replaceReq(catPresets)))
	if got := presetNames(a); !slices.Equal(got, []string{"Open", "Show"}) {
		t.Fatalf("presets: %v", got)
	}
	if p, _ := presetNamed(t, a, "Show"); p.Items[0].Dsts[0].N != 4 {
		t.Fatalf("Show: %+v", p)
	}
	if !res.Plan.Critical {
		t.Error("Replace import is always Critical")
	}
	if it := planItem(t, res.Plan, catPresets, "Late"); !it.Remove || it.writes() {
		t.Errorf("Late: %+v", it)
	}
}

// Replace import is Critical even when nothing in the file overwrites, since it removes.
func TestReplaceImportNeedsRecheck(t *testing.T) {
	a, _, tok := importApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Late", "MON 4")} })
	rec := postApply(t, a, tok, file, replaceReq(catPresets))
	if rec.Code != 403 || !bytes.Contains(rec.Body.Bytes(), []byte(`"recheck"`)) {
		t.Fatalf("without a recheck: %d %s", rec.Code, rec.Body)
	}
	if len(a.r().cfg.Presets) != 1 {
		t.Fatal("a refused import changed the presets")
	}
}

// A foreign file without PIN hashes: an imported Administrator keeps the Role but is PIN needed, so acts as a
// user without a PIN.
func TestImportedAdministratorWithoutHashIsPinNeeded(t *testing.T) {
	a, tok := replaceApp(t)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users = append(cfg.Users, config.User{Name: "Cy", Role: config.RoleAdministrator, PinNeeded: true})
	})
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catUsers}, Pick: map[string][]string{catUsers: {userID(t, a, "Cy")}}}))
	dropUser(t, a, "Cy")
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	cfg := a.r().cfg
	u, ok := cfg.FindUserByName("Cy")
	if !ok || u.Role != config.RoleAdministrator || !u.PinNeeded || u.HasPin() {
		t.Fatalf("Cy: %+v", u)
	}
	if role, _ := cfg.ActsAs(u); role == config.RoleAdministrator {
		t.Fatal("a PIN needed Administrator acts as one")
	}
}

// With PINs in the file, the hash travels. Use new with another hash replaces the PIN, which ends that
// user's sessions.
func TestImportCarriesPinHashes(t *testing.T) {
	a, tok := replaceApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers}, IncludePins: true})
	newPin, _ := config.HashPin("9999")
	lifeCommit(t, a, func(cfg *config.Config) {
		i := slices.IndexFunc(cfg.Users, func(u config.User) bool { return u.Name == "Ana" })
		cfg.Users[i].Pin = newPin
	})
	anaTok := signInToken(t, a, "Ana", "9999")
	dropUser(t, a, "Bo")

	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	cfg := a.r().cfg
	bo, ok := cfg.FindUserByName("Bo")
	if !ok || !bo.HasPin() || bo.PinNeeded || bo.Len != 6 {
		t.Fatalf("Bo: %+v", bo)
	}
	if ana, _ := cfg.FindUserByName("Ana"); ana.Pin == newPin {
		t.Fatal("Use new kept Ana's live PIN although the file holds hers")
	}
	if it := planItem(t, res.Plan, catUsers, "Ana"); !strings.Contains(it.Note, "Ends Ana's keys and sessions") {
		t.Errorf("Ana's item: %+v", it)
	}
	if _, ok := a.tokens.user(anaTok, cfg); ok {
		t.Fatal("Ana's session outlived her PIN change")
	}
	if b, _ := json.Marshal(res); bytes.Contains(b, []byte(bo.Pin)) {
		t.Fatal("the plan carries a PIN hash")
	}
}

// Keep current Administrators ticked keeps every live Administrator the file does not hold. A file without
// hashes always keeps the importing Administrator.
func TestReplaceImportKeepsAdministrators(t *testing.T) {
	a, tok := replaceApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers}, Pick: map[string][]string{catUsers: {userID(t, a, "Bo")}}})
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users = append(cfg.Users, mkUser(t, "Eve", "1357"))
		cfg.Users[len(cfg.Users)-1].Role = config.RoleAdministrator
	})
	applied(t, postApply(t, a, tok, file, replaceReq(catUsers)))
	if got := userNames(a); !slices.Equal(got, []string{"Ada", "Bo", "Eve"}) {
		t.Fatalf("kept: %v", got)
	}
	applied(t, postApply(t, a, tok, file, noKeep(replaceReq(catUsers))))
	if got := userNames(a); !slices.Equal(got, []string{"Ada", "Bo"}) {
		t.Fatalf("without Keep current Administrators: %v", got)
	}
	if u, _ := a.r().cfg.FindUserByName("Ada"); !u.HasPin() || u.Role != config.RoleAdministrator {
		t.Fatalf("the importing Administrator: %+v", u)
	}
}

func userNames(a *App) []string {
	var out []string
	for _, u := range a.r().cfg.Users {
		out = append(out, u.Name)
	}
	slices.Sort(out)
	return out
}

func TestImportRefusedWhenNoAdministratorWithPinRemains(t *testing.T) {
	a, tok := replaceApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers, catPresets}, IncludePins: true,
		Pick: map[string][]string{catUsers: {userID(t, a, "Bo")}}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Late", "MON 4")} })
	if p := postCheck(t, a, tok, file, noKeep(replaceReq(catUsers, catPresets))).Plan; !strings.Contains(p.Refused, "Administrator") {
		t.Fatalf("the review does not say the import is refused: %+v", p)
	}
	rec := postApply(t, a, tok, file, noKeep(replaceReq(catUsers, catPresets)))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "Administrator") {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body)
	}
	if got := userNames(a); !slices.Equal(got, []string{"Ada", "Ana", "Bo"}) {
		t.Fatalf("a refused import changed the users: %v", got)
	}
	if len(a.r().cfg.Presets) != 1 {
		t.Fatal("a refused import changed the presets")
	}
}

// The panels and API clients of the users a Replace import removes go to the importing Administrator.
func TestReplaceImportMovesRemovedUsersItems(t *testing.T) {
	a, tok := replaceApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers}, Pick: map[string][]string{catUsers: {userID(t, a, "Ada")}}})
	ana := userID(t, a, "Ana")
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.APIClients = []config.APIClient{{Name: "Bot", Scopes: []string{config.ScopeNames}, Owner: ana}}
	})
	if _, err := a.panels.create(Panel{Name: "Desk", Owner: ana, Rows: 1, Cols: 1}); err != nil {
		t.Fatal(err)
	}
	applied(t, postApply(t, a, tok, file, replaceReq(catUsers)))
	ada := userID(t, a, "Ada")
	if c, ok := a.r().cfg.FindAPIClient("Bot"); !ok || c.Owner != ada {
		t.Fatalf("Bot: %+v %v", c, ok)
	}
	if ps := a.panels.list(); len(ps) != 1 || ps[0].Owner != ada {
		t.Fatalf("panels: %+v", ps)
	}
}

// Replace import of Panels and Artwork removes what the file does not hold.
func TestReplaceImportEmptiesPanelsAndArtwork(t *testing.T) {
	a, tok := replaceApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catPanels, catArtwork}})
	seedPanelExtras(t, a)
	res := applied(t, postApply(t, a, tok, file, replaceReq(catPanels, catArtwork)))
	if len(res.Failed) != 0 {
		t.Fatalf("failed: %v", res.Failed)
	}
	if n := len(a.panels.list()) + len(a.templates.list()) + len(a.panels.sets.sorted(nil)) + len(a.icons.list()) + len(a.artDefaults.list()); n != 0 {
		t.Fatalf("%d items left after replacing with an empty file", n)
	}
}

// Routers ticked: the file's router settings go over the one router in place, which keeps its UID. The install
// UID stays too.
func TestReplaceImportRouterKeepsUIDs(t *testing.T) {
	a, tok := replaceApp(t)
	before := a.r().cfg
	file := foreign(t, exported(t, a, tok, exportReq{Categories: []string{catRouters}}))
	files := map[string]string{}
	for name, b := range unzipAll(t, file) {
		files[name] = string(b)
	}
	other := "0123456789abcdef"
	for _, n := range []string{"manifest.yaml", "routers.yaml"} {
		files[n] = strings.ReplaceAll(files[n], before.UID, other)
	}
	files["routers.yaml"] = regexp.MustCompile(`(?m)^(\s*)title: .*$`).ReplaceAllString(files["routers.yaml"], "${1}title: Truck B")
	delete(files, "names/"+before.UID+".yaml")
	applied(t, postApply(t, a, tok, zipOf(t, files), replaceReq(catRouters)))
	cfg := a.r().cfg
	if cfg.UID != before.UID || cfg.InstallUID != before.InstallUID || cfg.Title != "Truck B" {
		t.Fatalf("router %s %q, install %s", cfg.UID, cfg.Title, cfg.InstallUID)
	}
}

// Turning Require HTTPS on from a request that is not secure would lock the page out (ADR 0005).
func TestImportRequireHTTPSNeedsSecureRequest(t *testing.T) {
	for _, mode := range []string{importAdd, importReplace} {
		a, tok := replaceApp(t)
		lifeCommit(t, a, func(cfg *config.Config) { cfg.TrustedProxies = []string{"192.0.2.1"} })
		files := map[string]string{}
		for name, b := range unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catSettings}})) {
			files[name] = string(b)
		}
		files["settings.yaml"] = strings.Replace(files["settings.yaml"], "requireHttps: false", "requireHttps: true", 1)
		file := zipOf(t, files)
		req := importReq{Mode: mode, Categories: []string{catSettings}}
		res := applied(t, postApply(t, a, tok, file, req))
		if a.r().cfg.RequireHTTPS || res.Plan.Categories[catSettings].Refused == "" {
			t.Fatalf("%s: an insecure import turned Require HTTPS on: %+v", mode, res.Plan.Categories[catSettings])
		}
		applied(t, postApplyWith(t, a, tok, file, req, "https"))
		if !a.r().cfg.RequireHTTPS {
			t.Fatalf("%s: a secure import did not turn it on", mode)
		}
	}
}

func TestReplaceImportWritesSavesRow(t *testing.T) {
	a, tok := replaceApp(t)
	defer startLog(t, a)()
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Late", "MON 4")} })
	applied(t, postApply(t, a, tok, file, replaceReq(catPresets)))
	e := lastSavesRow(t, a)
	if e.Save.Import.Mode != importReplace || len(e.Save.Import.Categories) != 1 || e.Save.Import.Categories[0].Removed != 1 {
		t.Fatalf("row: %+v", e.Save.Import)
	}
}

// Replace import ends only the Inbox cards of the users it removes.
func TestReplaceImportEndsRemovedUsersCards(t *testing.T) {
	a, ana, bo := inboxApp(t)
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 4096)}
	register(a, c)
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	grantRecheck(a, c)
	p := mePreset("Show", 1, 2)
	setProfile(t, a, "Ana", config.Profile{Presets: []config.Preset{p}})
	setProfile(t, a, "Bo", config.Profile{Presets: []config.Preset{p}})
	toBo := sendOne(t, a, ana, inboxReq{To: []string{"Bo"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}})
	toAna := sendOne(t, a, bo, inboxReq{To: []string{"Ana"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: p.ID}}})
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers}, Pick: map[string][]string{catUsers: {userID(t, a, "Ada"), userID(t, a, "Ana")}}})
	applied(t, postApply(t, a, tok, file, replaceReq(catUsers)))
	if got, _ := a.inbox.get(toBo.ID); got.State != cardExpired {
		t.Fatalf("card to Bo: %s", got.State)
	}
	if got, _ := a.inbox.get(toAna.ID); got.State != cardWaiting {
		t.Fatalf("card to Ana: %s", got.State)
	}
}

// A same-install backup restored through Replace keeps what it holds, Default artwork included.
func TestReplaceImportRoundTripKeepsEverything(t *testing.T) {
	a, tok := replaceApp(t)
	seedPanelExtras(t, a)
	file := exported(t, a, tok, exportReq{Categories: []string{catPanels, catArtwork}})
	res := applied(t, postApply(t, a, tok, file, replaceReq(catPanels, catArtwork)))
	if len(res.Failed) != 0 {
		t.Fatalf("failed: %v", res.Failed)
	}
	p, tp, s, i, d := len(a.panels.list()), len(a.templates.list()), len(a.panels.sets.sorted(nil)), len(a.icons.list()), len(a.artDefaults.list())
	if p+tp+s+i+d != 5 {
		t.Fatalf("panels %d, templates %d, sets %d, icons %d, defaults %d", p, tp, s, i, d)
	}
}

// A file item that is refused or waits for names keeps its live twin: nothing would replace it.
func TestReplaceImportKeepsTwinsOfRefusedItems(t *testing.T) {
	a, tok := replaceApp(t)
	_, _, _, ic := seedPanelExtras(t, a)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	files := map[string]string{}
	for name, b := range unzipAll(t, exported(t, a, tok, exportReq{Categories: []string{catPanels, catArtwork, catPresets}})) {
		files[name] = string(b)
	}
	files["panels.yaml"] = strings.Replace(files["panels.yaml"], "rows: 1", "rows: 99", 1)
	files["art/icons/"+ic.fileName()] = "not an image"
	a.mu.Lock()
	clear(a.router.srcNames)
	clear(a.router.dstNames)
	a.mu.Unlock()
	res := applied(t, postApply(t, a, tok, zipOf(t, files), replaceReq(catPanels, catArtwork, catPresets)))
	if len(res.Failed) != 0 {
		t.Fatalf("failed: %v", res.Failed)
	}
	if n := len(a.panels.list()); n != 1 {
		t.Errorf("%d panels: the refused panel's live twin is gone", n)
	}
	if n := len(a.icons.list()); n != 1 {
		t.Errorf("%d icons: the refused icon's live twin is gone", n)
	}
	if _, ok := presetNamed(t, a, "Show"); !ok {
		t.Error("the waiting preset's live twin is gone")
	}
}
