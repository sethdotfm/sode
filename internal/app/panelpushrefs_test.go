package app

import (
	"encoding/json"
	"testing"
)

func cellNum(v any) int {
	switch x := v.(type) {
	case float64:
		return int(x)
	case int:
		return x
	}
	return -1
}

// bareCells stores cells as a panel saved before router references held them, past the API's save.
func bareCells(t *testing.T, a *App, id string, cells ...map[string]any) {
	t.Helper()
	p, _ := a.panels.get(id)
	p.Pages[0].Cells = cells
	if _, err := a.panels.replace(p); err != nil {
		t.Fatal(err)
	}
}

func wantRef(t *testing.T, what string, c map[string]any, uid string, n int, name string) {
	t.Helper()
	if c["router"] != uid || cellNum(c["n"]) != n || c["name"] != name {
		t.Fatalf("%s: cell %v, want {router %s, n %d, name %s}", what, c, uid, n, name)
	}
}

func TestPushCheckpointsAndSetsStoreRouterReferences(t *testing.T) {
	a, f, bot := takerApp(t, nil)
	uid := a.r().cfg.UID
	ana := userKey(t, a, "Ana")
	a.mu.Lock()
	a.first.router.srcNames[3] = "CAM 3"
	a.mu.Unlock()
	from := makePanel(t, a, ana, `{"name":"From"}`)
	to := makePanel(t, a, ana, `{"name":"To"}`)
	bareCells(t, a, from, map[string]any{"kind": "source", "n": 1, "r": 1, "c": 1}, map[string]any{"kind": "destination", "n": 2, "r": 1, "c": 2})
	bareCells(t, a, to, map[string]any{"kind": "source", "n": 3, "r": 1, "c": 1}, map[string]any{"kind": "destination", "n": 2, "r": 1, "c": 2})

	rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, `{"name":"Show","panels":["`+from+`","`+to+`"]}`)
	if rec.Code != 201 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	sid := apiBody(t, rec)["id"].(string)
	set, _ := a.panels.sets.get(sid)
	wantRef(t, "set entry", set.Entries[0].Pages[0].Cells[0], uid, 1, "CAM 1")
	wantRef(t, "set entry", set.Entries[1].Pages[0].Cells[0], uid, 3, "CAM 3")

	if rec := apiSend(a, "POST", "/api/v1/panels/"+from+"/push", ana, `{"targets":["`+to+`"]}`); rec.Code != 200 {
		t.Fatalf("push: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := a.panels.get(to)
	wantRef(t, "pushed", got.Pages[0].Cells[0], uid, 1, "CAM 1")
	cps, _ := a.panels.checkpoints(to)
	if len(cps) != 1 {
		t.Fatalf("checkpoints %v", cps)
	}
	wantRef(t, "checkpoint", cps[0].Layout.Pages[0].Cells[0], uid, 3, "CAM 3")
	wantRef(t, "checkpoint", cps[0].Layout.Pages[0].Cells[1], uid, 2, "MON 2")

	again := loadPanels(a.cfgPath)
	again.by = map[string]Panel{to: got}
	if c := again.cpsLocked(to); len(c) != 1 {
		t.Fatalf("reread %v", c)
	} else {
		wantRef(t, "checkpoint file", c[0].Layout.Pages[0].Cells[0], uid, 3, "CAM 3")
	}

	fire := func(p Panel) map[string]any {
		t.Helper()
		route, _ := json.Marshal(map[string]any{"routes": []any{map[string]any{
			"src": cellNum(p.Pages[0].Cells[0]["n"]), "dst": cellNum(p.Pages[0].Cells[1]["n"]), "levels": []int{1}}}})
		rec := apiPost(a, "/api/v1/takes", bot, string(route), nil)
		if rec.Code != 200 {
			t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
		}
		return apiBody(t, rec)["results"].([]any)[0].(map[string]any)
	}
	f.seed(2, 9, 0)
	rec = apiSend(a, "POST", "/api/v1/panels/"+to+"/checkpoints/"+cps[0].ID+"/restore", ana, ``)
	if rec.Code != 200 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body.String())
	}
	got, _ = a.panels.get(to)
	if r := fire(got); r["src"] != float64(3) || r["dst"] != float64(2) || r["ok"] != true {
		t.Fatalf("restored cells fired %v", r)
	}

	if rec := apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, `{"targets":["`+from+`"]}`); rec.Code != 200 {
		t.Fatalf("set push: %d %s", rec.Code, rec.Body.String())
	}
	got, _ = a.panels.get(from)
	wantRef(t, "set pushed", got.Pages[0].Cells[0], uid, 1, "CAM 1")
	if r := fire(got); r["src"] != float64(1) || r["dst"] != float64(2) || r["ok"] != true {
		t.Fatalf("set cells fired %v", r)
	}
}
