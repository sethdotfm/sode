package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// REST seam for Push, checkpoints and checkpoint sets.

func pushTo(a *App, key, src string, targets ...string) (code int, body map[string]any) {
	b, _ := json.Marshal(map[string]any{"targets": targets})
	rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/push", key, string(b))
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func readPanel(t *testing.T, a *App, key, id string) map[string]any {
	t.Helper()
	return apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, key, nil))
}

func cellCount(p map[string]any) int { return len(page1Cells(p)) }

func TestPushCopiesLayoutAndKeepsIdentity(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"EVS 1","rows":2,"cols":3,"cells":[{"r":1,"c":1,"kind":"take"},{"r":1,"c":2,"kind":"clear"}],"settings":{"mode":"instant"},"future":7}`)
	dst := makePanel(t, a, ana, `{"name":"EVS 2","share":{"mode":"named","users":["Bo"]},"cells":[{"r":1,"c":1,"kind":"current"}]}`)
	before := readPanel(t, a, ana, dst)

	code, body := pushTo(a, ana, src, dst)
	if code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	got := readPanel(t, a, ana, dst)
	if got["name"] != "EVS 2" || got["id"] != dst || got["owner"] != "Ana" || got["acts"] != "session" {
		t.Fatalf("identity lost: %v", got)
	}
	if s := got["share"].(map[string]any); s["mode"] != "named" {
		t.Fatalf("sharing lost: %v", s)
	}
	if cellCount(got) != 2 || got["rows"] != 2.0 || got["cols"] != 3.0 || got["future"] != 7.0 {
		t.Fatalf("layout not copied: %v", got)
	}
	if got["settings"].(map[string]any)["mode"] != "instant" {
		t.Fatalf("settings not copied: %v", got["settings"])
	}
	// Open panels follow the Rev, as for a saved edit.
	if got["rev"].(float64) <= before["rev"].(float64) {
		t.Fatalf("rev did not move: %v then %v", before["rev"], got["rev"])
	}
	if p := body["pushed"].([]any); len(p) != 1 || p[0].(map[string]any)["panel"] != dst {
		t.Fatalf("answer %v", body)
	}
}

func TestPushWhoMayAndAllOrNothing(t *testing.T) {
	a := panelsApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	src := makePanel(t, a, ana, `{"name":"Src","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	mine := makePanel(t, a, ana, `{"name":"Mine"}`)
	private := makePanel(t, a, bo, `{"name":"Bo private"}`)
	open := makePanel(t, a, bo, `{"name":"Bo open","share":{"mode":"everyone"}}`)

	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/push", ana, fmt.Sprintf(`{"targets":[%q,%q]}`, mine, private)), 404, "not_found")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/push", ana, fmt.Sprintf(`{"targets":[%q,%q]}`, mine, open)), 403, "not_owner")
	if cellCount(readPanel(t, a, ana, mine)) != 0 {
		t.Fatal("a refused push changed a panel")
	}

	// A user who may only use the source cannot push it.
	apiSend(a, "PUT", "/api/v1/panels/"+src, ana, `{"name":"Src","share":{"mode":"everyone"},"cells":[{"r":1,"c":1,"kind":"take"}]}`)
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/push", bo, fmt.Sprintf(`{"targets":[%q]}`, open)), 403, "not_owner")

	for _, bad := range []string{`{"targets":[]}`, `{}`, `{"targets":["nope"]}`, fmt.Sprintf(`{"targets":[%q]}`, src)} {
		if rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/push", ana, bad); rec.Code != 400 && rec.Code != 404 {
			t.Fatalf("%s: %d", bad, rec.Code)
		}
	}

	// A Trusted user pushes anybody's panel to anybody's.
	if code, body := pushTo(a, cy, src, private, open, mine); code != 200 {
		t.Fatalf("Trusted push: %d %v", code, body)
	}
	if cellCount(readPanel(t, a, bo, private)) != 1 || cellCount(readPanel(t, a, ana, mine)) != 1 {
		t.Fatal("Trusted push did not land")
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/push", ana, `{"targets":[]`), 400, "bad_request")
	_ = mine
}

func TestPushLeavesTheAPIClientAlone(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	apiSrc := makePanel(t, a, cy, `{"name":"API src","acts":"api","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	plain := makePanel(t, a, cy, `{"name":"Plain"}`)
	apiDst := makePanel(t, a, cy, `{"name":"API dst","acts":"api"}`)
	clientBefore := readPanel(t, a, cy, apiDst)["client"]
	clients := len(a.r().cfg.APIClients)

	if code, body := pushTo(a, cy, apiSrc, plain, apiDst); code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	if p := readPanel(t, a, cy, plain); p["acts"] != "session" || p["client"] != nil || cellCount(p) != 1 {
		t.Fatalf("a Push made a session panel an API panel: %v", p)
	}
	if p := readPanel(t, a, cy, apiDst); p["client"] != clientBefore || p["acts"] != "api" {
		t.Fatalf("the target's client changed: %v", p)
	}
	if got := len(a.r().cfg.APIClients); got != clients {
		t.Fatalf("API clients %d, want %d", got, clients)
	}
}

func TestPushCopiesMissingArtwork(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"Src"}`)
	dst := makePanel(t, a, ana, `{"name":"Dst"}`)
	rec := apiSendRaw(a, "POST", "/api/v1/panels/"+src+"/artwork", ana, b64(t, tinyPNG))
	if rec.Code != 201 {
		t.Fatalf("art: %d %s", rec.Code, rec.Body.String())
	}
	art := apiBody(t, rec)["id"].(string)
	pushTo(a, ana, src, dst)
	if rec := apiDo(a, "GET", "/api/v1/panels/"+dst+"/artwork/"+art, ana, nil); rec.Code != 200 {
		t.Fatalf("target lacks the artwork: %d", rec.Code)
	}
}

func checkpointsOf(t *testing.T, a *App, key, id string) []map[string]any {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/panels/"+id+"/checkpoints", key, nil)
	if rec.Code != 200 {
		t.Fatalf("checkpoints: %d %s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	for _, c := range apiBody(t, rec)["checkpoints"].([]any) {
		out = append(out, c.(map[string]any))
	}
	return out
}

func TestPushKeepsACheckpointAndRestorePutsItBack(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	src := makePanel(t, a, ana, `{"name":"Src","cells":[{"r":1,"c":1,"kind":"take"},{"r":1,"c":2,"kind":"clear"}]}`)
	dst := makePanel(t, a, ana, `{"name":"Dst","rows":3,"cells":[{"r":2,"c":2,"kind":"current"}],"share":{"mode":"everyone"}}`)

	if len(checkpointsOf(t, a, ana, dst)) != 0 {
		t.Fatal("a checkpoint before any Push")
	}
	pushTo(a, ana, src, dst)
	cps := checkpointsOf(t, a, ana, dst)
	if len(cps) != 1 || cps[0]["by"] != "Ana" || cps[0]["id"] == "" {
		t.Fatalf("checkpoints %v", cps)
	}
	if cellCount(readPanel(t, a, ana, dst)) != 2 {
		t.Fatal("push did not land")
	}

	// Use only may neither list nor restore.
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/"+dst+"/checkpoints", bo, nil), 403, "not_owner")
	path := "/api/v1/panels/" + dst + "/checkpoints/" + cps[0]["id"].(string) + "/restore"
	wantErr(t, apiSend(a, "POST", path, bo, ``), 403, "not_owner")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+dst+"/checkpoints/c-00000000/restore", ana, ``), 404, "not_found")

	rec := apiSend(a, "POST", path, ana, ``)
	if rec.Code != 200 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	got := readPanel(t, a, ana, dst)
	if cellCount(got) != 1 || got["rows"] != 3.0 || got["name"] != "Dst" {
		t.Fatalf("restore: %v", got)
	}
	// Restoring is itself kept, so it can be put back too.
	if n := len(checkpointsOf(t, a, ana, dst)); n != 2 {
		t.Fatalf("checkpoints after restore %d", n)
	}
}

func TestCheckpointsAreBoundedAndGoWithThePanel(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"Src","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	dst := makePanel(t, a, ana, `{"name":"Dst"}`)
	for i := 0; i < panelCheckpointMax+3; i++ {
		pushTo(a, ana, src, dst)
	}
	if n := len(checkpointsOf(t, a, ana, dst)); n != panelCheckpointMax {
		t.Fatalf("kept %d, want %d", n, panelCheckpointMax)
	}
	apiDo(a, "DELETE", "/api/v1/panels/"+dst, ana, nil)
	if _, ok := a.panels.checkpoints(dst); ok && len(mustCps(a, dst)) != 0 {
		t.Fatal("checkpoints outlived the panel")
	}
}

func mustCps(a *App, id string) []panelCheckpoint {
	c, _ := a.panels.checkpoints(id)
	return c
}

func TestCheckpointsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	s := loadPanels(dir + "/config.yaml")
	p, _ := s.create(Panel{Name: "A", Owner: "Ana", Rows: 1, Cols: 1})
	if _, _, err := s.applyLayout(p.ID, panelLayout{Rows: 2, Cols: 2}, "Ana", "test"); err != nil {
		t.Fatal(err)
	}
	again := loadPanels(dir + "/config.yaml")
	if len(again.by) != 1 {
		t.Fatalf("the checkpoint file was loaded as a panel: %v", again.problems())
	}
	if cps, _ := again.checkpoints(p.ID); len(cps) != 1 || cps[0].Layout.Rows != 1 {
		t.Fatalf("after reload %v", cps)
	}
}

func setBody(name, scope string, ids ...string) string {
	b, _ := json.Marshal(map[string]any{"name": name, "scope": scope, "panels": ids})
	return string(b)
}

func TestCheckpointSetGivesEachPanelItsOwnLayout(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	e1 := makePanel(t, a, ana, `{"name":"EVS 1","cells":[{"r":1,"c":1,"kind":"source","n":1}]}`)
	e2 := makePanel(t, a, ana, `{"name":"EVS 2","cells":[{"r":1,"c":1,"kind":"source","n":9},{"r":1,"c":2,"kind":"source","n":10}]}`)

	rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Show A", "me", e1, e2))
	if rec.Code != 201 {
		t.Fatalf("create set: %d %s", rec.Code, rec.Body.String())
	}
	set := apiBody(t, rec)
	sid := set["id"].(string)
	if set["scope"] != "me" || set["owner"] != "Ana" || len(set["entries"].([]any)) != 2 {
		t.Fatalf("set %v", set)
	}

	// The panels move on, then the set puts each one's own layout back.
	apiSend(a, "PUT", "/api/v1/panels/"+e1, ana, `{"name":"EVS 1","cells":[]}`)
	apiSend(a, "PUT", "/api/v1/panels/"+e2, ana, `{"name":"EVS 2","cells":[{"r":1,"c":1,"kind":"clear"}]}`)

	rec = apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, `{}`)
	if rec.Code != 200 {
		t.Fatalf("push set: %d %s", rec.Code, rec.Body.String())
	}
	if p := readPanel(t, a, ana, e1); cellCount(p) != 1 || page1Cells(p)[0].(map[string]any)["n"] != 1.0 || p["name"] != "EVS 1" {
		t.Fatalf("EVS 1 %v", p)
	}
	if p := readPanel(t, a, ana, e2); cellCount(p) != 2 || p["name"] != "EVS 2" {
		t.Fatalf("EVS 2 %v", p)
	}
	if len(checkpointsOf(t, a, ana, e1)) != 1 {
		t.Fatal("a set push keeps no checkpoint")
	}

	// A subset of targets pushes only those.
	apiSend(a, "PUT", "/api/v1/panels/"+e1, ana, `{"name":"EVS 1","cells":[]}`)
	apiSend(a, "PUT", "/api/v1/panels/"+e2, ana, `{"name":"EVS 2","cells":[]}`)
	apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, fmt.Sprintf(`{"targets":[%q]}`, e2))
	if cellCount(readPanel(t, a, ana, e1)) != 0 || cellCount(readPanel(t, a, ana, e2)) != 2 {
		t.Fatal("subset push touched the wrong panels")
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, `{"targets":["p-00000000"]}`), 400, "bad_request")

	// A panel that was deleted is skipped and named.
	apiDo(a, "DELETE", "/api/v1/panels/"+e1, ana, nil)
	rec = apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, `{}`)
	body := apiBody(t, rec)
	if rec.Code != 200 || len(body["pushed"].([]any)) != 1 || len(body["skipped"].([]any)) != 1 {
		t.Fatalf("after a delete: %d %v", rec.Code, body)
	}
}

func TestCheckpointSetScopeAndAccess(t *testing.T) {
	a := panelsApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	p1 := makePanel(t, a, ana, `{"name":"P1"}`)
	bp := makePanel(t, a, bo, `{"name":"Bo's"}`)

	// Everyone sets need a Trusted user, and a set holds only panels its maker may change.
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("All", "everyone", p1)), 403, "trusted_required")
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Bad", "me", p1, bp)), 404, "not_found")
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("", "me", p1)), 400, "bad_request")
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("None", "me")), 400, "bad_request")
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Odd", "team", p1)), 400, "bad_request")

	mine := apiBody(t, apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Mine", "me", p1)))["id"].(string)
	all := apiBody(t, apiSend(a, "POST", "/api/v1/panel-sets", cy, setBody("Shared", "everyone", p1)))["id"].(string)

	names := func(key string) string {
		var n []string
		for _, s := range apiBody(t, apiDo(a, "GET", "/api/v1/panel-sets", key, nil))["sets"].([]any) {
			n = append(n, s.(map[string]any)["name"].(string))
		}
		return strings.Join(n, ",")
	}
	if names(ana) != "Mine,Shared" || names(bo) != "Shared" || names(cy) != "Shared" {
		t.Fatalf("lists: %q %q %q", names(ana), names(bo), names(cy))
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets/"+mine+"/push", bo, `{}`), 404, "not_found")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panel-sets/"+mine, bo, ``), 404, "not_found")
	// Bo may use the Everyone set but not change it, and may not push to Ana's panel.
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panel-sets/"+all, bo, ``), 403, "not_owner")
	wantErr(t, apiSend(a, "POST", "/api/v1/panel-sets/"+all+"/push", bo, `{}`), 404, "not_found")

	if rec := apiSend(a, "PUT", "/api/v1/panel-sets/"+mine, ana, `{"name":"Renamed"}`); rec.Code != 200 || apiBody(t, rec)["name"] != "Renamed" {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "DELETE", "/api/v1/panel-sets/"+mine, ana, ``); rec.Code != 204 {
		t.Fatalf("delete: %d", rec.Code)
	}
	// Trusted may change an Everyone set that someone else made.
	if rec := apiSend(a, "DELETE", "/api/v1/panel-sets/"+all, cy, ``); rec.Code != 204 {
		t.Fatalf("Trusted delete: %d", rec.Code)
	}
	if names(ana) != "" {
		t.Fatalf("left over: %q", names(ana))
	}
}

func TestCheckpointSetsSurviveRestart(t *testing.T) {
	dir := t.TempDir()
	s := loadPanels(dir + "/config.yaml")
	if _, err := s.sets.create(PanelSet{Name: "Mine", Scope: panelShareMe, Owner: "Ana", Entries: []PanelSetEntry{{Panel: "p-00000001"}}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.sets.create(PanelSet{Name: "All", Scope: panelShareAll, Owner: "Cy"}); err != nil {
		t.Fatal(err)
	}
	again := loadPanels(dir + "/config.yaml")
	if n := len(again.sets.visible("Ana")); n != 2 {
		t.Fatalf("Ana sees %d sets", n)
	}
	if n := len(again.sets.visible("Bo")); n != 1 {
		t.Fatalf("Bo sees %d sets", n)
	}
}
