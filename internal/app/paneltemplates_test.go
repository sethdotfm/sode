package app

import (
	"fmt"
	"maps"
	"os"
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
	if !tplIDRe.MatchString(id) || made["owner"] != "Ana" || made["scope"] != "me" || made["sources"] != false || made["dests"] != false || made["contents"] != nil {
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
	cells := page1Cells(got)
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

const tplMixed = `{"name":"%s","scope":"me",%s"cells":[
 {"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":5,"caption":"CAM 5","art":"a-0000abcd"},
 {"r":1,"c":2,"rs":1,"cs":1,"kind":"destination","n":7,"caption":"MON 7"},
 {"r":1,"c":3,"rs":1,"cs":1,"kind":"preset","scope":"everyone","name":"Opening"},
 {"r":2,"c":1,"rs":1,"cs":1,"kind":"take","caption":"GO"}]}`

// tplCells saves a template of tplMixed with the given flags and returns its cells by kind.
func tplCells(t *testing.T, a *App, key, name, flags string) (map[string]map[string]any, map[string]any) {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/panel-templates", key, fmt.Sprintf(tplMixed, name, flags))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	made := apiBody(t, rec)
	out := map[string]map[string]any{}
	for _, c := range page1Cells(made) {
		m := c.(map[string]any)
		out[m["kind"].(string)] = m
	}
	return out, made
}

func TestTemplateSourcesAndDestsAreStoredApart(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")

	both, made := tplCells(t, a, ana, "Both", `"sources":true,"dests":true,`)
	if made["sources"] != true || made["dests"] != true {
		t.Fatalf("flags %v", made)
	}
	if both["source"]["n"] != 5.0 || both["source"]["caption"] != "CAM 5" || both["destination"]["n"] != 7.0 || both["preset"]["name"] != "Opening" || both["take"]["caption"] != "GO" {
		t.Fatalf("contents lost: %v", both)
	}
	if both["source"]["art"] != nil {
		t.Fatalf("artwork of another panel kept: %v", both["source"])
	}

	// Presets follow Sources, since a Preset is chosen from the same list of what to route.
	src, _ := tplCells(t, a, ana, "Sources", `"sources":true,`)
	if src["source"]["n"] != 5.0 || src["preset"]["name"] != "Opening" || src["destination"]["n"] != nil || src["destination"]["caption"] != nil {
		t.Fatalf("Sources only: %v", src)
	}
	dst, _ := tplCells(t, a, ana, "Dests", `"dests":true,`)
	if dst["destination"]["n"] != 7.0 || dst["destination"]["caption"] != "MON 7" || dst["source"]["n"] != nil || dst["preset"]["name"] != nil {
		t.Fatalf("Destinations only: %v", dst)
	}
	if dst["take"]["caption"] != "GO" {
		t.Fatalf("a Take caption is layout and stays once either option is on: %v", dst["take"])
	}
}

// Undo is layout as Clear is: its caption goes once either option is on, and nothing else of it changes.
func TestTemplateTreatsUndoAsClear(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	for _, flags := range []string{``, `"dests":true,`} {
		rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Undo `+fmt.Sprint(len(flags))+`","scope":"me",`+flags+`"cells":[
 {"r":1,"c":1,"rs":1,"cs":1,"kind":"clear","caption":"BACK","fontSize":2},
 {"r":1,"c":2,"rs":1,"cs":1,"kind":"undo","caption":"BACK","fontSize":2}]}`)
		if rec.Code != 201 {
			t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
		}
		cells := page1Cells(apiBody(t, rec))
		if len(cells) != 2 {
			t.Fatalf("cells %v", cells)
		}
		clr, undo := maps.Clone(cells[0].(map[string]any)), maps.Clone(cells[1].(map[string]any))
		if undo["kind"] != "undo" || (undo["caption"] != nil) != (flags != ``) {
			t.Fatalf("flags %q: undo cell %v", flags, undo)
		}
		delete(clr, "kind")
		delete(undo, "kind")
		delete(clr, "c")
		delete(undo, "c")
		if !maps.EqualFunc(clr, undo, func(x, y any) bool { return fmt.Sprint(x) == fmt.Sprint(y) }) {
			t.Fatalf("flags %q: clear %v, undo %v", flags, clr, undo)
		}
	}
}

// A request from before the split sends contents:true and means both.
func TestTemplateLegacyContentsMeansBoth(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	cells, made := tplCells(t, a, ana, "Old", `"contents":true,`)
	if made["sources"] != true || made["dests"] != true || made["contents"] != nil {
		t.Fatalf("flags %v", made)
	}
	if cells["source"]["n"] != 5.0 || cells["destination"]["n"] != 7.0 {
		t.Fatalf("contents lost: %v", cells)
	}
}

// A template file written before the split holds contents: true; it loads as both.
func TestTemplateStoredBeforeTheSplitLoadsAsBoth(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "config.yaml")
	if err := os.MkdirAll(filepath.Join(dir, "templates", "me"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := "id: t-0000abcd\nname: Old file\nscope: me\nowner: Ana\ncontents: true\nrows: 1\ncols: 2\ncells:\n    - {r: 1, c: 1, rs: 1, cs: 1, kind: source, n: 4}\n"
	if err := os.WriteFile(filepath.Join(dir, "templates", "me", "t-0000abcd.yaml"), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	s := loadTemplates(cfg)
	got, ok := s.get("t-0000abcd")
	if !ok || !got.Sources || !got.Dests || got.Contents {
		t.Fatalf("loaded %+v ok=%v", got, ok)
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
