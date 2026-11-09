package app

import (
	"path/filepath"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// REST seam for /api/v1/panel-templates: users Ana and Bo, Cy (Trusted).

const tplBody = `{"name":"Show layout","scope":"me","rows":2,"cols":3,"settings":{"mode":"instant"},"cells":[
 {"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":5,"caption":"CAM 5","levels":[1],"fontSize":2},
 {"r":1,"c":2,"rs":1,"cs":1,"kind":"preset","scope":"everyone","name":"Opening"},
 {"r":2,"c":1,"rs":1,"cs":2,"kind":"take"}]}`

func tplList(t *testing.T, a *App, key string) string {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/panel-templates", key, nil)
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	out := ""
	for _, p := range apiBody(t, rec)["templates"].([]any) {
		m := p.(map[string]any)
		out += m["name"].(string) + "/" + m["scope"].(string) + ","
	}
	return out
}

func TestTemplateRoundTripLeavesCellsUnassigned(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, tplBody)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	made := apiBody(t, rec)
	id, _ := made["id"].(string)
	if !tplIDRe.MatchString(id) || made["owner"] != "Ana" || made["scope"] != "me" || made["contents"] != false {
		t.Fatalf("template %v", made)
	}

	rec = apiDo(a, "GET", "/api/v1/panel-templates/"+id, ana, nil)
	if rec.Code != 200 {
		t.Fatalf("read: %d %s", rec.Code, rec.Body.String())
	}
	got := apiBody(t, rec)
	if got["rows"] != 2.0 || got["cols"] != 3.0 || got["settings"].(map[string]any)["mode"] != "instant" {
		t.Fatalf("layout %v", got)
	}
	cells := got["cells"].([]any)
	if len(cells) != 3 {
		t.Fatalf("cells %v", cells)
	}
	src := cells[0].(map[string]any)
	if src["kind"] != "source" || src["n"] != nil || src["caption"] != nil || src["levels"].([]any)[0] != 1.0 || src["fontSize"] != 2.0 {
		t.Fatalf("source cell kept contents or lost its look: %v", src)
	}
	if pre := cells[1].(map[string]any); pre["scope"] != nil || pre["name"] != nil || pre["kind"] != "preset" {
		t.Fatalf("preset cell %v", pre)
	}
}

func TestTemplateWithContentsKeepsThem(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Full","scope":"me","contents":true,"cells":[{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":5,"caption":"CAM 5","art":"a-0000abcd"}]}`)
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	src := apiBody(t, rec)["cells"].([]any)[0].(map[string]any)
	if src["n"] != 5.0 || src["caption"] != "CAM 5" {
		t.Fatalf("contents lost: %v", src)
	}
	if src["art"] != nil {
		t.Fatalf("artwork of another panel kept: %v", src)
	}
}

func TestTemplateScopesAndVisibility(t *testing.T) {
	a := panelsApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	if rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, tplBody); rec.Code != 201 {
		t.Fatal(rec.Body.String())
	}
	if rec := apiSend(a, "POST", "/api/v1/panel-templates", cy, `{"name":"Shared","scope":"everyone"}`); rec.Code != 201 {
		t.Fatalf("a Trusted user saves to Everyone: %d %s", rec.Code, rec.Body.String())
	}
	if got := tplList(t, a, ana); got != "Shared/everyone,Show layout/me," {
		t.Fatalf("Ana sees %q", got)
	}
	if got := tplList(t, a, bo); got != "Shared/everyone," {
		t.Fatalf("Bo sees %q", got)
	}
}

func TestTemplateEveryoneNeedsTrustedOrPin(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Shared","scope":"everyone"}`)
	if rec.Code != 403 || apiErrCode(t, rec) != "trusted_required" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if got := tplList(t, a, ana); got != "" {
		t.Fatalf("refused template stored: %q", got)
	}

	// With no Advanced Settings PIN set, a pin in the body opens nothing.
	if rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Shared","scope":"everyone","pin":"9999"}`); rec.Code != 403 {
		t.Fatalf("no PIN set: %d", rec.Code)
	}

	h, err := config.HashPin("9999")
	if err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.SettingsPin = h
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Shared","scope":"everyone","pin":"1111"}`); rec.Code != 403 {
		t.Fatalf("wrong PIN: %d", rec.Code)
	}
	if rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Shared","scope":"everyone","pin":"9999"}`); rec.Code != 201 {
		t.Fatalf("right PIN: %d %s", rec.Code, rec.Body.String())
	}
	if got := tplList(t, a, ana); got != "Shared/everyone," {
		t.Fatalf("%q", got)
	}
	if rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Mine","scope":"me"}`); rec.Code != 201 {
		t.Fatalf("a Me template needs no PIN: %d", rec.Code)
	}
}

func TestTemplateNeedsAUsersKey(t *testing.T) {
	a := panelsApp(t)
	bot := botKey(t, a)
	cfg := a.r().cfg
	cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, config.ScopePanelsManage)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	rec := apiSend(a, "POST", "/api/v1/panel-templates", bot, `{"name":"X","scope":"me"}`)
	if rec.Code != 403 || apiErrCode(t, rec) != "user_required" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestTemplateDelete(t *testing.T) {
	a := panelsApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	id, _ := apiBody(t, apiSend(a, "POST", "/api/v1/panel-templates", ana, tplBody))["id"].(string)
	if rec := apiDo(a, "DELETE", "/api/v1/panel-templates/"+id, bo, nil); rec.Code != 404 {
		t.Fatalf("another user's Me template is not there: %d", rec.Code)
	}
	cyID, _ := apiBody(t, apiSend(a, "POST", "/api/v1/panel-templates", cy, `{"name":"Shared","scope":"everyone"}`))["id"].(string)
	if rec := apiDo(a, "DELETE", "/api/v1/panel-templates/"+cyID, ana, nil); rec.Code != 403 {
		t.Fatalf("use only: %d", rec.Code)
	}
	if rec := apiDo(a, "DELETE", "/api/v1/panel-templates/"+id, ana, nil); rec.Code != 204 {
		t.Fatalf("owner delete: %d", rec.Code)
	}
	if rec := apiDo(a, "DELETE", "/api/v1/panel-templates/"+cyID, cy, nil); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
}

func TestTemplatesSurviveRestartInFilesByScope(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "config.yaml")
	s := loadTemplates(cfg)
	me, err := s.create(Template{Name: "A", Scope: templateMe, Owner: "Ana", Rows: 1, Cols: 1})
	if err != nil {
		t.Fatal(err)
	}
	all, err := s.create(Template{Name: "B", Scope: templateEveryone, Owner: "Cy", Rows: 1, Cols: 1})
	if err != nil {
		t.Fatal(err)
	}
	s2 := loadTemplates(cfg)
	if _, ok := s2.get(me.ID); !ok {
		t.Fatal("Me template lost")
	}
	if g, ok := s2.get(all.ID); !ok || g.Owner != "Cy" {
		t.Fatal("Everyone template lost")
	}
	if len(s2.list()) != 2 {
		t.Fatalf("%v", s2.list())
	}
}
