package app

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func gridEdgeRaw(t *testing.T, p map[string]any, edge string) string {
	t.Helper()
	b, err := json.Marshal(p[edge])
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// gridEdgeJSON is an edge with each reference as its number and items keyed by it, so a test about pins, hiding or
// artwork reads short. Every reference must name a router; panelgridrefs_test.go checks the full shape.
func gridEdgeJSON(t *testing.T, p map[string]any, edge string) string {
	t.Helper()
	e, _ := p[edge].(map[string]any)
	num := func(x any) float64 {
		r := x.(map[string]any)
		if s, _ := r["router"].(string); s == "" {
			t.Fatalf("%s: a reference with no router: %v", edge, r)
		}
		return r["n"].(float64)
	}
	out := map[string]any{}
	for k, v := range e {
		switch k {
		case "members", "hide":
			list, ok := v.([]any)
			if !ok {
				out[k] = v
				continue
			}
			ns := []float64{}
			for _, x := range list {
				ns = append(ns, num(x))
			}
			out[k] = ns
		case "items":
			items := map[string]any{}
			for _, x := range v.([]any) {
				it := maps.Clone(x.(map[string]any))
				n := num(it)
				for _, ref := range []string{"router", "n", "name"} {
					delete(it, ref)
				}
				items[fmt.Sprint(n)] = it
			}
			out[k] = items
		default:
			out[k] = v
		}
	}
	b, err := json.Marshal(out)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// gridBody turns a panel body's numeric edges ({"members":[1],"hide":[2],"items":{"1":{...}}}) into references
// with no router, as the editor sends them.
func gridBody(t *testing.T, body string) string {
	t.Helper()
	var p map[string]any
	if err := json.Unmarshal([]byte(body), &p); err != nil {
		t.Fatalf("gridBody %s: %v", body, err)
	}
	for _, edge := range []string{"sources", "destinations"} {
		e, ok := p[edge].(map[string]any)
		if !ok {
			continue
		}
		for _, k := range []string{"members", "hide"} {
			if list, ok := e[k].([]any); ok {
				refs := []any{}
				for _, n := range list {
					refs = append(refs, map[string]any{"n": n})
				}
				e[k] = refs
			}
		}
		if items, ok := e["items"].(map[string]any); ok {
			keys := slices.Sorted(maps.Keys(items))
			list := []any{}
			for _, k := range keys {
				it := maps.Clone(items[k].(map[string]any))
				n, err := strconv.Atoi(k)
				if err != nil {
					t.Fatalf("gridBody key %q", k)
				}
				it["n"] = n
				list = append(list, it)
			}
			e["items"] = list
		}
	}
	b, _ := json.Marshal(p)
	return string(b)
}

// A pin or a hidden item never stops an "all" edge from following the router.
func TestGridPanelPinOnAllEdgeKeepsFollowingTheRouter(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makeGridPanel(t, a, ana, "G")
	body := `{"name":"G","sources":{"members":"all","hide":[17,18],"items":{"812":{"pinned":true,"art":"camera"}}}}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, gridBody(t, body)); rec.Code != 200 {
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
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, gridBody(t, body)); rec.Code != 200 {
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
		`{"members":[{"n":1},{"n":2},{"n":1}]}`,
		`{"members":"all","hide":[{"n":3},{"n":3}]}`,
		`{"members":[{"n":1},{"n":2}],"hide":[{"n":3}]}`,
		`{"members":"all","items":[{"n":"cam"}]}`,
		`{"members":"all","items":[{"n":-1}]}`,
		`{"items":[{"n":1,"pinned":true}]}`,
		`{"members":[{"n":-1}]}`,
		`{"members":[{"n":65536}]}`,
		`{"members":"all","hide":[{"n":70000}]}`,
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
		pins[i] = fmt.Sprintf(`{"n":%d,"pinned":true}`, i+1)
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"G","sources":{"members":[{"n":1}],"items":[`+strings.Join(pins, ",")+`]}}`); rec.Code != 200 {
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
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, gridBody(t, body)); rec.Code != 200 {
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
	id := makePanel(t, a, ana, gridBody(t, `{"name":"G","type":"grid",
		"sources":{"members":[1,50],"items":{"1":{"pinned":true},"50":{"pinned":true,"art":"icon:mic"}}},
		"destinations":{"members":"all","hide":[60],"items":{"70":{"art":"icon:mic"}}}}`))
	cfg := a.r().cfg
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Ana" {
			cfg.Users[i].Rights = config.Rights{Sources: nums(0, 1, 2), Dsts: nums(0, 1, 2)}
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
	src := gridEdgeJSON(t, p, "sources")
	dst := gridEdgeJSON(t, p, "destinations")
	for _, want := range []string{`"members":[1,50]`, `"50":{"art":"icon:mic","pinned":true}`} {
		if !strings.Contains(src, want) {
			t.Errorf("sources %s, want %s", src, want)
		}
	}
	for _, want := range []string{`"hide":[60]`, `"70":{"art":"icon:mic"}`} {
		if !strings.Contains(dst, want) {
			t.Errorf("destinations %s, want %s", dst, want)
		}
	}
}

// gridEdgeNums is gridEdgeJSON decoded, for a test that looks at one part.
func gridEdgeNums(t *testing.T, p map[string]any, edge string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(gridEdgeJSON(t, p, edge)), &out); err != nil {
		t.Fatal(err)
	}
	return out
}
