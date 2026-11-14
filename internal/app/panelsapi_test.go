package app

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// REST seam for /api/v1/panels: real Handler, recorder, users Ana and Bo, and Cy (Trusted).

// problems lists the panel files the store skipped; production only logs them.
func (s *panelStore) problems() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.probs)
}

func panelsApp(t *testing.T) *App {
	t.Helper()
	a, _ := apiTestApp(t, func(c *config.Config) {
		cy := mkUser(t, "Cy", "4321")
		cy.Trusted = true
		c.Users = append(c.Users, cy)
	})
	return a
}

func userKey(t *testing.T, a *App, name string) string {
	t.Helper()
	return apiTestKey(t, a, keyOwner{Kind: actorUser, Name: name}, "Desk "+name, time.Time{})
}

func apiSend(a *App, method, path, key, body string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	r.RemoteAddr = apiPeer
	r.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func apiSendRaw(a *App, method, path, key string, body []byte) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, bytes.NewReader(body))
	r.RemoteAddr = apiPeer
	r.Header.Set("Authorization", "Bearer "+key)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func makePanel(t *testing.T, a *App, key, body string) string {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/panels", key, body)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	id, _ := apiBody(t, rec)["id"].(string)
	if id == "" {
		t.Fatalf("no id: %s", rec.Body.String())
	}
	return id
}

func panelIDs(t *testing.T, a *App, key string) string {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/panels", key, nil)
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var names []string
	for _, p := range apiBody(t, rec)["panels"].([]any) {
		names = append(names, p.(map[string]any)["name"].(string))
	}
	return strings.Join(names, ",")
}

func TestPanelRoundTrip(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Studio A","rows":2,"cols":3,"cells":[{"kind":"take","future":{"x":1}}],"settings":{"mode":"instant"},"nextVersionField":"keep me"}`)

	rec := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil)
	if rec.Code != 200 {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}
	p := apiBody(t, rec)
	if p["name"] != "Studio A" || p["owner"] != "Ana" || p["rows"] != 2.0 || p["cols"] != 3.0 || p["canEdit"] != true {
		t.Fatalf("panel %v", p)
	}
	if p["nextVersionField"] != "keep me" || page1Cells(p)[0].(map[string]any)["future"].(map[string]any)["x"] != 1.0 {
		t.Fatalf("unknown fields lost: %v", p)
	}
	if sh, _ := p["share"].(map[string]any); sh["mode"] != "me" {
		t.Errorf("share %v", p["share"])
	}

	// Update keeps what the body leaves out, and never moves the owner or id.
	rec = apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Studio B","owner":"Bo","id":"p-ffff","rows":4,"cols":8,"cells":[],"settings":{}}`)
	if rec.Code != 200 {
		t.Fatalf("update: %d %s", rec.Code, rec.Body.String())
	}
	p = apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))
	if p["name"] != "Studio B" || p["owner"] != "Ana" || p["id"] != id || p["nextVersionField"] != "keep me" {
		t.Fatalf("after update %v", p)
	}

	// The file is one YAML per panel beside the config, and a fresh store reads it back.
	dir := filepath.Join(filepath.Dir(a.cfgPath), "panels")
	raw, err := os.ReadFile(filepath.Join(dir, id+".yaml"))
	if err != nil || !strings.Contains(string(raw), "nextVersionField") {
		t.Fatalf("file: %v %s", err, raw)
	}
	again := loadPanels(a.cfgPath)
	if got, ok := again.get(id); !ok || got.Name != "Studio B" || got.Extra["nextVersionField"] != "keep me" {
		t.Fatalf("reload %+v %v", got, ok)
	}
}

func TestPanelCreateDefaultsAndValidation(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Plain"}`)
	p := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))
	if p["rows"] != 4.0 || p["cols"] != 8.0 || p["acts"] != "session" {
		t.Fatalf("defaults %v", p)
	}
	for _, body := range []string{`{}`, `{"name":"  "}`, `{"name":"x","rows":99}`, `{"name":"x","share":{"mode":"bogus"}}`, `not json`, `{"name":"x","acts":"robot"}`} {
		wantErr(t, apiSend(a, "POST", "/api/v1/panels", ana, body), 400, "bad_request")
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", ana, `{"name":"`+strings.Repeat("x", panelLayoutBodyMax+1)+`"}`), 413, "too_large")
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/p-nope", ana, nil), 404, "not_found")
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/..%2Fconfig", ana, nil), 404, "not_found")
}

func TestPanelOwnershipAndUseOnlySharing(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Ana's"}`)

	// Not shared: invisible to Bo, as if absent.
	if got := panelIDs(t, a, bo); got != "" {
		t.Fatalf("Bo sees %q", got)
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/"+id, bo, nil), 404, "not_found")
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, bo, `{"name":"mine now"}`), 404, "not_found")

	// Named: Bo may read, never change.
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Ana's","share":{"mode":"named","users":["bo"]}}`)
	p := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, bo, nil))
	if p["canEdit"] != false || p["name"] != "Ana's" {
		t.Fatalf("Bo's view %v", p)
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, bo, `{"name":"mine now"}`), 403, "not_owner")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panels/"+id, bo, ``), 403, "not_owner")

	// Everyone, then back to Only me: the share follows live.
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Ana's","share":{"mode":"everyone"}}`)
	if got := panelIDs(t, a, bo); got != "Ana's" {
		t.Fatalf("everyone: %q", got)
	}
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Ana's","share":{"mode":"me"}}`)
	if got := panelIDs(t, a, bo); got != "" {
		t.Fatalf("only me: %q", got)
	}
}

func TestPanelTrustedReachesAnyPanel(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Ana's"}`)
	if got := panelIDs(t, a, cy); got != "Ana's" {
		t.Fatalf("Cy list %q", got)
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, `{"name":"Fixed by Cy"}`); rec.Code != 200 {
		t.Fatalf("Trusted update: %d %s", rec.Code, rec.Body.String())
	}
	p := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))
	if p["name"] != "Fixed by Cy" || p["owner"] != "Ana" {
		t.Fatalf("after Trusted edit %v", p)
	}
	// Trusted is read live: unticking ends the reach at once.
	cfg := a.r().cfg
	for i := range cfg.Users {
		cfg.Users[i].Trusted = false
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, cy, `{"name":"again"}`), 404, "not_found")
}

func TestPanelSaveACopyAndDelete(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"},"cells":[{"kind":"take"}],"future":7}`)

	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", bo, ``)
	if rec.Code != 201 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body.String())
	}
	c := apiBody(t, rec)
	cid := c["id"].(string)
	if cid == id || c["owner"] != "Bo" || c["name"] != "Wall copy" || c["canEdit"] != true || c["future"] != 7.0 {
		t.Fatalf("copy %v", c)
	}
	if sh := c["share"].(map[string]any); sh["mode"] != "me" {
		t.Errorf("a copy starts private: %v", sh)
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+cid, bo, `{"name":"Bo's wall"}`); rec.Code != 200 {
		t.Fatalf("edit copy: %d", rec.Code)
	}
	// A named copy name, and the original is untouched.
	rec = apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", bo, `{"name":"Mine"}`)
	if rec.Code != 201 || apiBody(t, rec)["name"] != "Mine" {
		t.Fatalf("named copy: %d %s", rec.Code, rec.Body.String())
	}
	if p := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil)); p["name"] != "Wall" {
		t.Fatalf("original %v", p)
	}
	// A panel that Bo cannot see cannot be copied.
	hidden := makePanel(t, a, ana, `{"name":"Private"}`)
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+hidden+"/copy", bo, ``), 404, "not_found")

	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+cid, bo, ``); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/"+cid, bo, nil), 404, "not_found")
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.cfgPath), "panels", cid+".yaml")); !os.IsNotExist(err) {
		t.Errorf("file left behind: %v", err)
	}
}

func TestPanelScopesAndWhoOwns(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"}}`)

	// PanelsBot has every read scope but neither panel scope.
	bot := botKey(t, a)
	wantErr(t, apiDo(a, "GET", "/api/v1/panels", bot, nil), 403, "scope")
	cfg := a.r().cfg
	cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, config.ScopePanels)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := panelIDs(t, a, bot); got != "Wall" {
		t.Fatalf("reader sees %q", got)
	}
	// Reading is not managing, and a client cannot own a panel.
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", bot, `{"name":"x"}`), 403, "scope")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panels/"+id, bot, ``), 403, "scope")
	cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, config.ScopePanelsManage)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", bot, `{"name":"x"}`), 403, "user_required")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panels/"+id, bot, ``), 403, "not_owner")
}

func TestPanelAPISwitchAndRateLimitApply(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	cfg := a.r().cfg
	cfg.API.RateLimit = 2
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	apiDo(a, "GET", "/api/v1/panels", ana, nil)
	apiDo(a, "GET", "/api/v1/panels", ana, nil)
	wantErr(t, apiDo(a, "GET", "/api/v1/panels", ana, nil), 429, "rate_limited")
	cfg.API.Enabled = false
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/panels", ana, nil), 503, "api_off")
}

func TestPanelStoreLoadsMissingFolderAndSkipsBadFiles(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	s := loadPanels(cfgPath)
	if len(s.list()) != 0 || len(s.problems()) != 0 {
		t.Fatal("a missing folder must load empty")
	}
	if _, err := s.create(Panel{Name: "Good", Owner: "Ana"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(filepath.Dir(cfgPath), "panels")
	os.WriteFile(filepath.Join(dir, "p-bad1.yaml"), []byte("name: [unclosed"), 0o600)
	os.WriteFile(filepath.Join(dir, "p-bad2.yaml"), []byte("id: p-other\nname: Wrong id\nowner: Ana\n"), 0o600)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("ignored"), 0o600)
	again := loadPanels(cfgPath)
	if l := again.list(); len(l) != 1 || l[0].Name != "Good" {
		t.Fatalf("loaded %+v", l)
	}
	if len(again.problems()) != 2 {
		t.Fatalf("problems %v", again.problems())
	}
}

func TestPanelStoreMemoryOnlyWithoutConfigPath(t *testing.T) {
	s := loadPanels("")
	p, err := s.create(Panel{Name: "Temp", Owner: "Ana"})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := s.get(p.ID); !ok || got.Name != "Temp" {
		t.Fatal("memory store lost the panel")
	}
}

func TestPanelWritesLeaveNoTempFiles(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"x"}`)
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"y"}`)
	ents, _ := os.ReadDir(filepath.Join(filepath.Dir(a.cfgPath), "panels"))
	if len(ents) != 1 || ents[0].Name() != id+".yaml" {
		t.Fatalf("folder holds %v", ents)
	}
}
