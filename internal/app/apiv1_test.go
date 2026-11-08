package app

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// apiT* helpers are the REST seam: a real Handler, a recorder, the fake router.

const apiPeer = "192.0.2.7:4000"

// apiTestApp has the API on, a client "PanelsBot" with every read scope but log, and names
// and tally seeded through the fake router.
func apiTestApp(t *testing.T, mut func(*config.Config)) (*App, *fakeRouter) {
	t.Helper()
	a := userApp(t, false)
	f := newFakeRouter(t, a)
	cfg := a.r().cfg
	cfg.API.Enabled = true
	cfg.APIClients = []config.APIClient{
		{Name: "PanelsBot", Scopes: []string{config.ScopeNames, config.ScopeTally, config.ScopeProtected, config.ScopeStatus, config.ScopePresets, config.ScopeHistory, config.ScopeLayouts}},
		{Name: "LogBot", Scopes: []string{config.ScopeLog}},
		{Name: "NamesBot", Scopes: []string{config.ScopeNames}},
	}
	cfg.Presets = []config.Preset{{Name: "Studio A", Items: []config.PresetItem{{Src: 1, Dsts: []int{2}}}}}
	if mut != nil {
		mut(&cfg)
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.router.srcNames[1] = "CAM 1"
	a.router.dstNames[2] = "MON 2"
	a.router.tally[2] = map[int]int{1: 1}
	a.router.protect[3] = map[int]int{1: 0}
	a.mu.Unlock()
	return a, f
}

func apiTestKey(t *testing.T, a *App, o keyOwner, name string, exp time.Time) string {
	t.Helper()
	secret, _, err := a.keys.createCapped(o, name, exp, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

func botKey(t *testing.T, a *App) string {
	return apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Panel 43", time.Time{})
}

func apiDo(a *App, method, path, key string, hdr map[string]string) *httptest.ResponseRecorder {
	return apiDoFrom(a, apiPeer, method, path, key, hdr)
}

func apiDoFrom(a *App, peer, method, path, key string, hdr map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, path, nil)
	r.RemoteAddr = peer
	if key != "" {
		r.Header.Set("Authorization", "Bearer "+key)
	}
	for k, v := range hdr {
		r.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func apiBody(t *testing.T, rec *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &m); err != nil {
		t.Fatalf("body %q: %v", rec.Body.String(), err)
	}
	return m
}

// apiErrCode is the machine-readable reason of an error response.
func apiErrCode(t *testing.T, rec *httptest.ResponseRecorder) string {
	t.Helper()
	e, _ := apiBody(t, rec)["error"].(map[string]any)
	if e == nil || e["message"] == "" {
		t.Fatalf("not an error body: %s", rec.Body.String())
	}
	code, _ := e["code"].(string)
	return code
}

func wantErr(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status || apiErrCode(t, rec) != code {
		t.Fatalf("got %d %s, want %d %s", rec.Code, rec.Body.String(), status, code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("content type %q", ct)
	}
}

func TestAPIOffRefusesEverything(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) { c.API.Enabled = false })
	key := botKey(t, a)
	wantErr(t, apiDo(a, "GET", "/api/v1/names", key, nil), 503, "api_off")
	wantErr(t, apiDo(a, "GET", "/api/v1/names", "", nil), 503, "api_off")
	if rec := apiDo(a, "GET", "/api/health", "", nil); rec.Code == 0 || rec.Code == 403 {
		t.Errorf("health %d", rec.Code)
	}
}

func TestAPIAuthRefusals(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	wantErr(t, apiDo(a, "GET", "/api/v1/names", "sode_notakey", nil), 401, "invalid_key")
	exp := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Old", time.Now().Add(30*time.Millisecond))
	time.Sleep(60 * time.Millisecond)
	wantErr(t, apiDo(a, "GET", "/api/v1/names", exp, nil), 401, "key_expired")
	// keys travel only in the Authorization header
	rec := apiDo(a, "GET", "/api/v1/names?key="+botKey(t, a), "", nil)
	wantErr(t, rec, 401, "key_required")
	r := httptest.NewRequest("GET", "/api/v1/names", nil)
	r.Header.Set("Authorization", "Basic abc")
	r.RemoteAddr = apiPeer
	rec = httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	wantErr(t, rec, 401, "invalid_key")
}

func TestAPIReads(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := botKey(t, a)
	list := func(path, field string) []any {
		t.Helper()
		rec := apiDo(a, "GET", path, key, nil)
		if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		if rec.Header().Get("Cache-Control") != "no-store" {
			t.Errorf("%s: cache-control %q", path, rec.Header().Get("Cache-Control"))
		}
		l, ok := apiBody(t, rec)[field].([]any)
		if !ok {
			t.Fatalf("%s: no %s list in %s", path, field, rec.Body.String())
		}
		return l
	}
	names := apiBody(t, apiDo(a, "GET", "/api/v1/names", key, nil))
	if names["numberOffset"] != float64(1) {
		t.Errorf("names %v", names)
	}
	src := names["sources"].([]any)
	if len(src) != 1 || src[0].(map[string]any)["n"] != float64(1) || src[0].(map[string]any)["name"] != "CAM 1" {
		t.Errorf("sources %v", src)
	}
	if d := names["destinations"].([]any); len(d) != 1 || d[0].(map[string]any)["name"] != "MON 2" {
		t.Errorf("destinations %v", d)
	}
	routes := list("/api/v1/tally", "routes")
	if len(routes) != 1 || routes[0].(map[string]any)["dst"] != float64(2) || routes[0].(map[string]any)["src"] != float64(1) {
		t.Errorf("tally %v", routes)
	}
	if lv := list("/api/v1/tally", "levels"); len(lv) != 17 && len(lv) == 0 {
		t.Errorf("levels %v", lv)
	}
	if p := list("/api/v1/protected", "protected"); len(p) != 1 || p[0].(map[string]any)["dst"] != float64(3) {
		t.Errorf("protected %v", p)
	}
	if p := list("/api/v1/presets", "presets"); len(p) != 1 || p[0].(map[string]any)["scope"] != "everyone" || p[0].(map[string]any)["name"] != "Studio A" {
		t.Errorf("presets %v", p)
	}
	list("/api/v1/layouts", "layouts")
	st := apiBody(t, apiDo(a, "GET", "/api/v1/status", key, nil))
	if st["server"] != "up" || st["router"] == "" || st["version"] != VersionLabel() {
		t.Errorf("status %v", st)
	}
}

func TestAPIHistoryPages(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	for i := range 5 {
		a.hist.Append(&Entry{Client: "x", Src: i, OK: true})
	}
	key := botKey(t, a)
	page := apiBody(t, apiDo(a, "GET", "/api/v1/history?n=2", key, nil))
	e := page["entries"].([]any)
	if len(e) != 2 || e[0].(map[string]any)["id"] != float64(4) || page["more"] != true {
		t.Fatalf("page %v", page)
	}
	next := apiBody(t, apiDo(a, "GET", "/api/v1/history?n=2&before=3", key, nil))
	if e := next["entries"].([]any); len(e) != 2 || e[0].(map[string]any)["id"] != float64(2) {
		t.Fatalf("next %v", next)
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/history?n=zero", key, nil), 400, "bad_request")
}

func TestAPIScopes(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	names := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "NamesBot"}, "N", time.Time{})
	if rec := apiDo(a, "GET", "/api/v1/names", names, nil); rec.Code != 200 {
		t.Fatalf("names: %d", rec.Code)
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/tally", names, nil), 403, "scope")
	wantErr(t, apiDo(a, "GET", "/api/v1/log", botKey(t, a), nil), 403, "scope")
	logKey := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "LogBot"}, "L", time.Time{})
	a.logEvent(event(kindBrowser, "connect", "10.1.1.1", "connected"))
	rec := apiDo(a, "GET", "/api/v1/log?n=5", logKey, nil)
	if rec.Code != 200 {
		t.Fatalf("log: %d %s", rec.Code, rec.Body.String())
	}
	if _, ok := apiBody(t, rec)["entries"].([]any); !ok {
		t.Errorf("log body %s", rec.Body.String())
	}
}

func TestAPIUserKeyActsAsUser(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) {
		for i := range c.Users {
			if c.Users[i].Name == "Ana" {
				c.Users[i].Profile.Presets = []config.Preset{{Name: "Mine", Items: []config.PresetItem{{Src: 1, Dsts: []int{2}}}}}
			}
		}
	})
	key := apiTestKey(t, a, keyOwner{Kind: actorUser, Name: "Ana"}, "Desk", time.Time{})
	var got []string
	for _, p := range apiBody(t, apiDo(a, "GET", "/api/v1/presets", key, nil))["presets"].([]any) {
		m := p.(map[string]any)
		got = append(got, m["scope"].(string)+":"+m["name"].(string))
	}
	if strings.Join(got, ",") != "everyone:Studio A,me:Mine" {
		t.Errorf("presets %v", got)
	}
	// An API client never sees Me presets, and no key sees a browser's This browser presets.
	for _, p := range apiBody(t, apiDo(a, "GET", "/api/v1/presets", botKey(t, a), nil))["presets"].([]any) {
		if p.(map[string]any)["scope"] != "everyone" {
			t.Errorf("client saw %v", p)
		}
	}
}

func TestAPIKeyUseIsCounted(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := botKey(t, a)
	apiDo(a, "GET", "/api/v1/names", key, nil)
	apiDo(a, "GET", "/api/v1/names", key, nil)
	ks := a.keys.list()
	if len(ks) != 1 || ks[0].Count != 2 || ks[0].LastUsed.IsZero() {
		t.Fatalf("keys %+v", ks)
	}
}

func TestAPIGuestNeedsSwitchAndScopes(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	wantErr(t, apiDo(a, "GET", "/api/v1/names", "", nil), 401, "key_required")

	cfg := a.r().cfg
	cfg.API.GuestEnabled = true
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"names", "tally", "protected", "status"} {
		if rec := apiDo(a, "GET", "/api/v1/"+p, "", nil); rec.Code != 200 {
			t.Errorf("%s: %d %s", p, rec.Code, rec.Body.String())
		}
	}
	for _, p := range []string{"presets", "history", "log", "layouts"} {
		wantErr(t, apiDo(a, "GET", "/api/v1/"+p, "", nil), 403, "scope")
	}
}

func TestAPIRateLimitPerKeyAndPerGuestAddress(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) { c.API.RateLimit = 3; c.API.GuestEnabled = true })
	key := botKey(t, a)
	for i := range 3 {
		if rec := apiDo(a, "GET", "/api/v1/names", key, nil); rec.Code != 200 {
			t.Fatalf("call %d: %d", i, rec.Code)
		}
	}
	rec := apiDo(a, "GET", "/api/v1/names", key, nil)
	wantErr(t, rec, 429, "rate_limited")
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After")
	}
	// another key is unaffected
	other := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Panel 44", time.Time{})
	if rec := apiDo(a, "GET", "/api/v1/names", other, nil); rec.Code != 200 {
		t.Errorf("other key: %d", rec.Code)
	}
	// keyless calls count per address
	for range 3 {
		apiDoFrom(a, "192.0.2.50:1", "GET", "/api/v1/names", "", nil)
	}
	wantErr(t, apiDoFrom(a, "192.0.2.50:1", "GET", "/api/v1/names", "", nil), 429, "rate_limited")
	if rec := apiDoFrom(a, "192.0.2.51:1", "GET", "/api/v1/names", "", nil); rec.Code != 200 {
		t.Errorf("other address: %d", rec.Code)
	}
}

func TestAPIFailedAuthThrottleAndOneRowPerMinute(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	var last *httptest.ResponseRecorder
	for range apiAuthFailMax + 3 {
		last = apiDo(a, "GET", "/api/v1/names", "sode_wrong", nil)
	}
	wantErr(t, last, 429, "rate_limited")
	// the real key from that address waits too: the throttle is per address
	wantErr(t, apiDo(a, "GET", "/api/v1/names", botKey(t, a), nil), 429, "rate_limited")
	// another address is fine
	if rec := apiDoFrom(a, "192.0.2.99:1", "GET", "/api/v1/names", botKey(t, a), nil); rec.Code != 200 {
		t.Errorf("other address: %d", rec.Code)
	}

	a.flushAuthFails(time.Now().Add(2 * time.Minute))
	a.flushAuthFails(time.Now().Add(3 * time.Minute)) // nothing more to write
	rows := waitLogged(t, a, 1)
	time.Sleep(100 * time.Millisecond)
	rows = logged(t, a)
	n := 0
	for _, e := range rows {
		if e.Kind == kindAPI && e.Why == "authFailed" {
			n++
			if e.By != "192.0.2.7" || !strings.Contains(e.Note, "failed") || strings.Contains(e.Note+e.Err, "sode_wrong") {
				t.Errorf("row %+v", e)
			}
			if !strings.Contains(e.Note, "13") && !strings.Contains(e.Note, "14") {
				t.Errorf("count missing: %q", e.Note)
			}
		}
	}
	if n != 1 {
		t.Fatalf("%d authFailed rows: %+v", n, rows)
	}
}

func TestAPILogReadsSwitch(t *testing.T) {
	for _, on := range []bool{false, true} {
		a, _ := apiTestApp(t, func(c *config.Config) { c.API.LogReads = on })
		stop := startLog(t, a)
		key := botKey(t, a)
		apiDo(a, "GET", "/api/v1/names", key, map[string]string{"X-Sode-Via": "wall ui 2"})
		a.apiEvent(sodeActor, "marker", "ok", "", nil)
		rows := waitLogged(t, a, 1)
		time.Sleep(100 * time.Millisecond)
		rows = logged(t, a)
		stop()
		var reads []Sent
		for _, e := range rows {
			if e.Kind == kindAPI && e.Why == "read" {
				reads = append(reads, e)
			}
		}
		if !on && len(reads) != 0 {
			t.Errorf("off: %+v", reads)
		}
		if on {
			if len(reads) != 1 || reads[0].By != "PanelsBot (Panel 43)" || !strings.Contains(reads[0].Note, "/api/v1/names") {
				t.Fatalf("on: %+v", reads)
			}
			if !strings.Contains(reads[0].Note, "unverified via: wall ui 2") {
				t.Errorf("via missing: %q", reads[0].Note)
			}
			if strings.Contains(reads[0].Note+reads[0].By, key) {
				t.Error("key in the row")
			}
		}
	}
}

func TestAPIRequireHTTPSAnswersJSON(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) {
		c.TrustedProxies = []string{"10.0.0.2"}
		c.RequireHTTPS = true
	})
	rec := apiDoFrom(a, "10.0.0.2:4000", "GET", "/api/v1/names", botKey(t, a), nil)
	wantErr(t, rec, 403, "https_required")
	rec = apiDoFrom(a, "10.0.0.2:4000", "GET", "/api/v1/names", botKey(t, a), map[string]string{"X-Forwarded-Proto": "https"})
	if rec.Code != 200 {
		t.Errorf("secure: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAPINoCORSAndUnknownRoutes(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := botKey(t, a)
	rec := apiDo(a, "GET", "/api/v1/names", key, map[string]string{"Origin": "https://evil.example"})
	if rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("CORS header sent")
	}
	if rec := apiDo(a, "OPTIONS", "/api/v1/names", "", map[string]string{"Origin": "https://evil.example", "Access-Control-Request-Method": "GET"}); rec.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Error("preflight allowed")
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/nope", key, nil), 404, "not_found")
	rec = apiDo(a, "DELETE", "/api/v1/names", key, nil)
	wantErr(t, rec, 405, "method_not_allowed")
	if !strings.Contains(rec.Header().Get("Allow"), "GET") {
		t.Errorf("Allow %q", rec.Header().Get("Allow"))
	}
}

// The OpenAPI file is hand-written; this keeps it in step with the route table.
func TestOpenAPIDocumentsEveryRoute(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) { c.API.Enabled = false })
	rec := apiDo(a, "GET", "/api/v1/openapi.yaml", "", nil)
	if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Type"), "yaml") {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var doc struct {
		OpenAPI string                    `yaml:"openapi"`
		Paths   map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(rec.Body.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(doc.OpenAPI, "3.") {
		t.Errorf("openapi %q", doc.OpenAPI)
	}
	if len(apiRoutes) < 7 {
		t.Fatalf("route table has %d routes", len(apiRoutes))
	}
	for _, r := range apiRoutes {
		p := strings.TrimPrefix(r.Path, "/api/v1")
		if _, ok := doc.Paths[p][strings.ToLower(r.Method)]; !ok {
			t.Errorf("OpenAPI lacks %s %s", r.Method, p)
		}
	}
	for p, ops := range doc.Paths {
		for m := range ops {
			if m == "parameters" {
				continue
			}
			found := false
			for _, r := range apiRoutes {
				found = found || (strings.ToLower(r.Method) == m && r.Path == "/api/v1"+p)
			}
			if p != "/openapi.yaml" && !found {
				t.Errorf("OpenAPI documents %s %s but no route serves it", m, p)
			}
		}
	}
}

func TestAPISettingsMessage(t *testing.T) {
	a, _ := apiTestApp(t, func(c *config.Config) { c.API.Enabled = false })
	defer startLog(t, a)()
	rule, ok := policy["apiSettings"]
	if !ok || rule.need&needAdmin == 0 {
		t.Fatalf("apiSettings rule %+v ok=%v", rule, ok)
	}
	c := &client{ip: "10.0.0.9"}
	if err := a.SaveAPISettings(c, apiSettingsReq{Enabled: apiPtr(true)}); err == nil {
		t.Fatal("saved without the Advanced Settings PIN")
	}
	c = adminClient(t, a)
	if err := a.SaveAPISettings(c, apiSettingsReq{Enabled: apiPtr(true), RateLimit: apiPtr(50), GuestEnabled: apiPtr(true), LogReads: apiPtr(true)}); err != nil {
		t.Fatal(err)
	}
	got := a.r().cfg.API
	if !got.Enabled || got.RateLimit != 50 || !got.GuestEnabled || !got.LogReads || got.TakeCap != config.DefaultTakeCap {
		t.Fatalf("settings %+v", got)
	}
	if err := a.SaveAPISettings(c, apiSettingsReq{TakeCap: apiPtr(0)}); err == nil {
		t.Error("take cap 0 accepted")
	}
	if a.r().cfg.API.TakeCap != config.DefaultTakeCap {
		t.Error("refused save changed settings")
	}
	if err := a.SaveAPISettings(c, apiSettingsReq{Enabled: apiPtr(false)}); err != nil {
		t.Fatal(err)
	}
	var whys []string
	for _, e := range waitLogged(t, a, 5) {
		if e.Kind == kindAPI {
			whys = append(whys, e.Why+":"+e.Result)
		}
	}
	for _, want := range []string{"apiEnabled:on", "apiEnabled:off", "guestApi:on", "logApiReads:on"} {
		if !apiHas(whys, want) {
			t.Errorf("no row %q in %v", want, whys)
		}
	}
}

func apiPtr[T any](v T) *T { return &v }

func apiHas(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}
