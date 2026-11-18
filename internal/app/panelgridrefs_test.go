package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// A saved Grid panel holds {router, n, name} for its members, hidden items and item settings. A number the router
// has no name for keeps the name it came with, also through a resave that sends no name. The stored numbers fire
// the same route as before references.
func TestGridPanelEdgesStoreRouterReferencesAndFireSameRoutes(t *testing.T) {
	a, f, bot := takerApp(t, nil)
	uid := a.r().cfg.UID
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"G","type":"grid",
		"sources":{"members":"all","hide":[{"n":3}],"items":[{"n":1,"pinned":true,"art":"icon:mic"}]},
		"destinations":{"members":[{"n":2},{"n":7,"name":"OLD 7"}],"items":[{"n":7,"pinned":true}]}}`)

	p := readPanel(t, a, ana, id)
	src, dst := gridEdgeRaw(t, p, "sources"), gridEdgeRaw(t, p, "destinations")
	wantSrc := `{"hide":[{"n":3,"router":"` + uid + `"}],"items":[{"art":"icon:mic","n":1,"name":"CAM 1","pinned":true,"router":"` + uid + `"}],"members":"all"}`
	if src != wantSrc {
		t.Errorf("sources\n%s\nwant\n%s", src, wantSrc)
	}
	wantDst := `{"items":[{"n":7,"name":"OLD 7","pinned":true,"router":"` + uid + `"}],"members":[{"n":2,"name":"MON 2","router":"` + uid + `"},{"n":7,"name":"OLD 7","router":"` + uid + `"}]}`
	if dst != wantDst {
		t.Errorf("destinations\n%s\nwant\n%s", dst, wantDst)
	}

	// The editor sends numbers with no names. The router has no name for 7, so the stored one stays.
	body := `{"name":"G","type":"grid","sources":{"members":"all"},"destinations":{"members":[{"n":7},{"n":2}]}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	p = readPanel(t, a, ana, id)
	wantDst = `{"members":[{"n":7,"name":"OLD 7","router":"` + uid + `"},{"n":2,"name":"MON 2","router":"` + uid + `"}]}`
	if got := gridEdgeRaw(t, p, "destinations"); got != wantDst {
		t.Errorf("destinations after resave\n%s\nwant\n%s", got, wantDst)
	}

	raw, err := os.ReadFile(filepath.Join(a.panels.dir, id+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Destinations struct {
			Members []map[string]any `yaml:"members"`
		} `yaml:"destinations"`
	}
	if err := yaml.Unmarshal(raw, &file); err != nil {
		t.Fatalf("file: %v\n%s", err, raw)
	}
	if m := file.Destinations.Members; len(m) != 2 || m[0]["router"] != uid || m[0]["n"] != 7 || m[0]["name"] != "OLD 7" {
		t.Fatalf("file members %v\n%s", m, raw)
	}

	f.seed(2, 3, 0)
	stored, _ := a.panels.get(id)
	route, _ := json.Marshal(map[string]any{"routes": []any{map[string]any{"src": 1, "dst": stored.Destinations.Members.List[1].N, "levels": []int{1}}}})
	rec := apiPost(a, "/api/v1/takes", bot, string(route), nil)
	r0 := apiBody(t, rec)["results"].([]any)[0].(map[string]any)
	if rec.Code != 200 || r0["dst"] != float64(2) || r0["ok"] != true {
		t.Fatalf("fired %d %s", rec.Code, rec.Body.String())
	}
}

// Two entries for one router item are refused, as two equal numbers were.
func TestGridPanelRefusesTwoReferencesToOneItem(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	uid := a.r().cfg.UID
	for _, edge := range []string{
		`{"members":[{"n":1},{"router":"` + uid + `","n":1}]}`,
		`{"members":"all","items":[{"n":4},{"n":4,"pinned":true}]}`,
		`{"members":"all","items":{"4":{"pinned":true}}}`,
		`{"members":[1,2]}`,
	} {
		rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"G","sources":`+edge+`}`)
		if rec.Code != 400 || !strings.Contains(rec.Body.String(), "bad_request") {
			t.Errorf("%s: %d %s", edge, rec.Code, rec.Body.String())
		}
	}
}
