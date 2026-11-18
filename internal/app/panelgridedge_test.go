package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func gridEdgeJSON(t *testing.T, p map[string]any, edge string) string {
	t.Helper()
	b, err := json.Marshal(p[edge])
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// A pin or a hidden item never stops an "all" edge from following the router.
func TestGridPanelPinOnAllEdgeKeepsFollowingTheRouter(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","sources":{"members":"all","hide":[17,18],"items":{"812":{"pinned":true,"art":"camera"}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	got := gridEdgeJSON(t, readPanel(t, a, ana, id), "sources")
	if want := `{"hide":[17,18],"items":{"812":{"art":"camera","pinned":true}},"members":"all"}`; got != want {
		t.Errorf("sources %s, want %s", got, want)
	}
}

// The editor sends a reorder as an explicit list. The list keeps its order and its pins.
func TestGridPanelReorderStoresAList(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","destinations":{"members":[40,41,12],"items":{"41":{"pinned":true}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	p := readPanel(t, a, ana, id)
	if got, want := gridEdgeJSON(t, p, "destinations"), `{"items":{"41":{"pinned":true}},"members":[40,41,12]}`; got != want {
		t.Errorf("destinations %s, want %s", got, want)
	}
	if got := gridEdgeJSON(t, p, "sources"); got != `{"members":"all"}` {
		t.Errorf("sources %s, want all", got)
	}
}

func TestGridPanelRefusesBadEdges(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	before := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.String()
	for _, edge := range []string{
		`{"members":[1,2,1]}`,
		`{"members":"all","hide":[3,3]}`,
		`{"members":[1,2],"hide":[3]}`,
		`{"members":"all","items":{"007":{"pinned":true}}}`,
		`{"members":"all","items":{"cam":{}}}`,
		`{"members":"all","items":{"-1":{}}}`,
		`{"items":{"1":{"pinned":true}}}`,
		`{"members":[-1]}`,
		`{"members":[65536]}`,
		`{"members":"all","hide":[70000]}`,
		`{"members":null}`,
	} {
		for _, side := range []string{"sources", "destinations"} {
			rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"G","`+side+`":`+edge+`}`)
			if rec.Code != 400 || !strings.Contains(rec.Body.String(), "bad_request") {
				t.Errorf("%s %s: %d %s", side, edge, rec.Code, rec.Body.String())
			}
		}
	}
	if after := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.String(); after != before {
		t.Errorf("a refused PUT changed the panel:\n%s\nwas\n%s", after, before)
	}
	// An edge pins as many items as it likes, members or not.
	pins := make([]string, 50)
	for i := range pins {
		pins[i] = fmt.Sprintf(`"%d":{"pinned":true}`, i+1)
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"G","sources":{"members":[1],"items":{`+strings.Join(pins, ",")+`}}}`); rec.Code != 200 {
		t.Errorf("50 pins: %d %s", rec.Code, rec.Body.String())
	}
}

// The editor's Clear stores an empty list. It must load again after a restart and go back unchanged.
func TestGridPanelEmptyListSurvivesARestart(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"G","sources":{"members":[]}}`); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	b := fixRestart(t, a, func(*config.Config) {})
	p, ok := b.panels.get(id)
	if !ok {
		t.Fatal("the panel did not load")
	}
	if p.Sources == nil || p.Sources.Members.All || p.Sources.Members.List == nil || len(p.Sources.Members.List) != 0 {
		t.Fatalf("sources after restart %+v", p.Sources)
	}
	out, _ := json.Marshal(p.Sources)
	if string(out) != `{"members":[]}` {
		t.Errorf("sources JSON %s", out)
	}
}

func TestGridPanelCopyKeepsListsHiddenItemsAndPins(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","sources":{"members":"all","hide":[3],"items":{"4":{"pinned":true}}},"destinations":{"members":[9,2]}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body.String())
	}
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", ana, `{}`)
	if rec.Code != 201 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body.String())
	}
	c := apiBody(t, rec)
	if got := gridEdgeJSON(t, c, "sources"); got != `{"hide":[3],"items":{"4":{"pinned":true}},"members":"all"}` {
		t.Errorf("copy sources %s", got)
	}
	if got := gridEdgeJSON(t, c, "destinations"); got != `{"members":[9,2]}` {
		t.Errorf("copy destinations %s", got)
	}
}

// An editor whose Rights are narrower than the panel's lists saves what the read gave back. The members, hidden
// items and item settings outside those Rights are still there for a user who can see them.
func TestGridSaveKeepsItemsOutsideTheEditorsRights(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"G","type":"grid",
		"sources":{"members":[1,50],"items":{"1":{"pinned":true},"50":{"pinned":true,"art":"icon:mic"}}},
		"destinations":{"members":"all","hide":[60],"items":{"70":{"art":"icon:mic"}}}}`)
	cfg := a.r().cfg
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Ana" {
			cfg.Users[i].Rights = config.Rights{Sources: &[]int{0, 1, 2}, Dsts: &[]int{0, 1, 2}}
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	read := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, read.Body.String()); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	p := readPanel(t, a, cy, id)
	src, _ := json.Marshal(p["sources"])
	dst, _ := json.Marshal(p["destinations"])
	for _, want := range []string{`"members":[1,50]`, `"50":{"art":"icon:mic","pinned":true}`} {
		if !strings.Contains(string(src), want) {
			t.Errorf("sources %s, want %s", src, want)
		}
	}
	for _, want := range []string{`"hide":[60]`, `"70":{"art":"icon:mic"}`} {
		if !strings.Contains(string(dst), want) {
			t.Errorf("destinations %s, want %s", dst, want)
		}
	}
}
