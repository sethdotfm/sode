package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REST seam for a panel's Pages (ADR 0007): the stored shape, its checks, and every copy of a layout carrying them.

func pagesOf(t *testing.T, p map[string]any) []map[string]any {
	t.Helper()
	raw, ok := p["pages"].([]any)
	if !ok {
		t.Fatalf("no pages in %v", p)
	}
	out := make([]map[string]any, len(raw))
	for i, g := range raw {
		out[i] = g.(map[string]any)
	}
	return out
}

// page1Cells is the cells of a read panel or template's first Page.
func page1Cells(p map[string]any) []any {
	return p["pages"].([]any)[0].(map[string]any)["cells"].([]any)
}

// pageSum is "id:cells" for each Page, in order.
func pageSum(t *testing.T, p map[string]any) string {
	t.Helper()
	var parts []string
	for _, g := range pagesOf(t, p) {
		parts = append(parts, fmt.Sprintf("%v:%d", g["id"], len(g["cells"].([]any))))
	}
	return strings.Join(parts, " ")
}

const twoPages = `"pages":[
 {"id":"cams","name":"Cameras","enabled":true,"cells":[{"r":1,"c":1,"kind":"source","n":1},{"r":1,"c":2,"kind":"take"}]},
 {"id":"vts","enabled":false,"cells":[{"r":1,"c":1,"kind":"source","n":9}]}]`

func TestPagesOldFileLoadsAsOnePageAndSavesOnlyPages(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "panels"), 0o755)
	old := []byte("id: p-0000abcd\nname: Old wall\nowner: Ana\nacts: session\nrows: 1\ncols: 2\ncells:\n    - {r: 1, c: 1, rs: 1, cs: 1, kind: destination, n: 7}\n    - {r: 1, c: 2, rs: 1, cs: 1, kind: source, n: 4}\nsettings:\n    mode: instant\n")
	path := filepath.Join(dir, "panels", "p-0000abcd.yaml")
	os.WriteFile(path, old, 0o644)

	s := loadPanels(filepath.Join(dir, "config.yaml"))
	p, ok := s.get("p-0000abcd")
	if !ok || len(p.Pages) != 1 || !p.Pages[0].Enabled || len(p.Pages[0].Cells) != 2 || p.Pages[0].ID == "" || p.LegacyCells != nil {
		t.Fatalf("not one enabled Page: %+v", p)
	}
	// A restart before the first save names the Page the same, so nothing that saw its id goes stale.
	if again, _ := loadPanels(filepath.Join(dir, "config.yaml")).get("p-0000abcd"); again.Pages[0].ID != p.Pages[0].ID {
		t.Fatalf("Page id moved across loads: %q then %q", p.Pages[0].ID, again.Pages[0].ID)
	}
	if _, err := s.replace(p); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	if !strings.Contains(string(b), "\npages:\n") || strings.Contains(string(b), "\ncells:") {
		t.Fatalf("the save did not write only pages:\n%s", b)
	}
}

func TestPagesRoundTripOverREST(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall",`+twoPages+`}`)
	got := readPanel(t, a, ana, id)
	if _, has := got["cells"]; has {
		t.Fatalf("a read mirrors cells: %v", got)
	}
	pages := pagesOf(t, got)
	if pageSum(t, got) != "cams:2 vts:1" || pages[0]["name"] != "Cameras" || pages[0]["enabled"] != true || pages[1]["enabled"] != false {
		t.Fatalf("pages %v", pages)
	}
	if _, has := pages[1]["name"]; has {
		t.Fatalf("an unnamed Page carries a name: %v", pages[1])
	}

	// A Page sent without id or enabled gets a fresh id and is on; one with no pages at all is one empty Page.
	id2 := makePanel(t, a, ana, `{"name":"New","pages":[{"cells":[]},{"name":" Two "}]}`)
	p2 := pagesOf(t, readPanel(t, a, ana, id2))
	if len(p2) != 2 || p2[0]["id"] == "" || p2[0]["id"] == p2[1]["id"] || p2[0]["enabled"] != true || p2[1]["name"] != "Two" {
		t.Fatalf("defaults %v", p2)
	}
	if p3 := pagesOf(t, readPanel(t, a, ana, makePanel(t, a, ana, `{"name":"Bare"}`))); len(p3) != 1 || p3[0]["enabled"] != true {
		t.Fatalf("bare panel %v", p3)
	}
}

func TestPagesValidation(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	many := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = fmt.Sprintf(`{"id":"g%d"}`, i)
		}
		return `[` + strings.Join(s, ",") + `]`
	}
	name := func(n int) string { return `[{"id":"a","name":"` + strings.Repeat("x", n) + `"}]` }
	for what, pages := range map[string]string{
		"17 Pages":        many(17),
		"a repeated id":   `[{"id":"a"},{"id":"a"}]`,
		"a bad id":        `[{"id":"has space"}]`,
		"a long name":     name(41),
		"no enabled Page": `[{"id":"a","enabled":false},{"id":"b","enabled":false}]`,
	} {
		body := `{"name":"Wall","pages":` + pages + `}`
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 400 {
			t.Errorf("%s: PUT %d %s", what, rec.Code, rec.Body.String())
		}
		if rec := apiSend(a, "POST", "/api/v1/panels", ana, body); rec.Code != 400 {
			t.Errorf("%s: POST %d", what, rec.Code)
		}
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","pages":[],"cells":[]}`), 400, "bad_request")
	for _, pages := range []string{many(16), name(40)} {
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","pages":`+pages+`}`); rec.Code != 200 {
			t.Errorf("refused %s: %d %s", pages, rec.Code, rec.Body.String())
		}
	}
}

// An API client from before Pages sends cells: they are Page 1's, and the other Pages stay.
func TestPagesCellsOnlyBodyIsPageOne(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	old := makePanel(t, a, ana, `{"name":"Old","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	if p := pagesOf(t, readPanel(t, a, ana, old)); len(p) != 1 || len(p[0]["cells"].([]any)) != 1 {
		t.Fatalf("cells did not become one Page: %v", p)
	}
	id := makePanel(t, a, ana, `{"name":"Wall",`+twoPages+`}`)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","cells":[]}`); rec.Code != 200 {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	if got := pageSum(t, readPanel(t, a, ana, id)); got != "cams:0 vts:1" {
		t.Fatalf("after a cells-only PUT %s", got)
	}
}

func TestPagesResizeDropsCellsOnEveryPage(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","rows":4,"cols":4,"pages":[
	 {"id":"a","cells":[{"r":1,"c":1,"rs":2,"cs":3,"kind":"take"},{"r":3,"c":1,"kind":"clear"}]},
	 {"id":"b","cells":[{"r":2,"c":2,"kind":"take"},{"r":1,"c":4,"kind":"clear"}]}]}`)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","rows":2,"cols":2,"pages":[
	 {"id":"a","cells":[{"r":1,"c":1,"rs":2,"cs":3,"kind":"take"},{"r":3,"c":1,"kind":"clear"}]},
	 {"id":"b","cells":[{"r":2,"c":2,"kind":"take"},{"r":1,"c":4,"kind":"clear"}]}]}`); rec.Code != 200 {
		t.Fatalf("put %d %s", rec.Code, rec.Body.String())
	}
	got := readPanel(t, a, ana, id)
	if pageSum(t, got) != "a:1 b:1" {
		t.Fatalf("cells outside the grid kept: %s", pageSum(t, got))
	}
	if c := pagesOf(t, got)[0]["cells"].([]any)[0].(map[string]any); c["rs"] != 2.0 || c["cs"] != 2.0 {
		t.Fatalf("a merged cell was not clipped: %v", c)
	}
}

func TestPagesTravelWithPushCopyRestoreSetAndTemplate(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"Src",`+twoPages+`}`)
	dst := makePanel(t, a, ana, `{"name":"Dst","pages":[{"id":"mine","cells":[{"r":1,"c":1,"kind":"current"}]}],"settings":{"fixedDests":[3]}}`)

	if code, body := pushTo(a, ana, src, dst); code != 200 {
		t.Fatalf("push %d %v", code, body)
	}
	got := readPanel(t, a, ana, dst)
	if pageSum(t, got) != "cams:2 vts:1" || pagesOf(t, got)[1]["enabled"] != false {
		t.Fatalf("push: %s", pageSum(t, got))
	}
	if fmt.Sprint(fixedOf(got)) != "[3]" {
		t.Fatalf("push took the target's fixed destinations: %v", fixedOf(got))
	}
	cps := checkpointsOf(t, a, ana, dst)
	if len(cps) != 1 || cps[0]["cells"] != 1.0 || cps[0]["pages"] != 1.0 {
		t.Fatalf("checkpoint summary %v", cps)
	}
	if rec := apiSend(a, "POST", "/api/v1/panels/"+dst+"/checkpoints/"+cps[0]["id"].(string)+"/restore", ana, ``); rec.Code != 200 {
		t.Fatalf("restore %d %s", rec.Code, rec.Body.String())
	}
	if s := pageSum(t, readPanel(t, a, ana, dst)); s != "mine:1" {
		t.Fatalf("restore: %s", s)
	}

	rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ana, `{}`)
	if rec.Code != 201 || pageSum(t, apiBody(t, rec)) != "cams:2 vts:1" {
		t.Fatalf("copy %d %s", rec.Code, rec.Body.String())
	}

	rec = apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Show", "me", src))
	set := apiBody(t, rec)
	if e := set["entries"].([]any)[0].(map[string]any); e["cells"] != 3.0 || e["pages"] != 2.0 {
		t.Fatalf("set entry %v", e)
	}
	apiSend(a, "PUT", "/api/v1/panels/"+src, ana, `{"name":"Src","pages":[{"id":"other"}]}`)
	if rec := apiSend(a, "POST", "/api/v1/panel-sets/"+set["id"].(string)+"/push", ana, `{}`); rec.Code != 200 {
		t.Fatalf("set push %d %s", rec.Code, rec.Body.String())
	}
	if s := pageSum(t, readPanel(t, a, ana, src)); s != "cams:2 vts:1" {
		t.Fatalf("set push: %s", s)
	}

	rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Show","scope":"me",`+twoPages+`}`)
	if rec.Code != 201 {
		t.Fatalf("template %d %s", rec.Code, rec.Body.String())
	}
	tpl := apiBody(t, rec)
	if _, has := tpl["cells"]; has || pageSum(t, tpl) != "cams:2 vts:1" {
		t.Fatalf("template %v", tpl)
	}
	if c := pagesOf(t, tpl)[1]["cells"].([]any)[0].(map[string]any); c["n"] != nil {
		t.Fatalf("a template kept Page 2's contents: %v", c)
	}
	again := loadTemplates(a.cfgPath)
	if t2, _ := again.get(tpl["id"].(string)); len(t2.Pages) != 2 || t2.Pages[0].ID != "cams" {
		t.Fatalf("template after reload %+v", t2)
	}
}

// An old template file and an old checkpoint hold cells: each comes back as one Page.
func TestPagesOldTemplateAndCheckpointLoadAsOnePage(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "templates", "me"), 0o755)
	os.WriteFile(filepath.Join(dir, "templates", "me", "t-0000abcd.yaml"), []byte("id: t-0000abcd\nname: Old\nscope: me\nowner: Ana\nrows: 1\ncols: 1\ncells:\n    - {r: 1, c: 1, kind: take}\n"), 0o644)
	ts := loadTemplates(filepath.Join(dir, "config.yaml"))
	if tp, ok := ts.get("t-0000abcd"); !ok || len(tp.Pages) != 1 || len(tp.Pages[0].Cells) != 1 || tp.LegacyCells != nil {
		t.Fatalf("old template %+v", tp)
	}

	s := loadPanels(filepath.Join(dir, "config.yaml"))
	p, _ := s.create(Panel{Name: "A", Owner: "Ana", Rows: 1, Cols: 1})
	lay := panelLayout{Rows: 1, Cols: 1, LegacyCells: []map[string]any{{"r": 1, "c": 1, "kind": "take"}}}
	q, _, err := s.applyLayout(p.ID, lay, "Ana", "test")
	if err != nil || len(q.Pages) != 1 || len(q.Pages[0].Cells) != 1 || q.LegacyCells != nil {
		t.Fatalf("old layout %+v %v", q, err)
	}
}

// Icon uses and Default artwork read every Page, not only the first.
func TestPagesIconUsesCountEveryPage(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	makePanel(t, a, ana, `{"name":"Wall","pages":[{"id":"a"},{"id":"b","cells":[{"r":1,"c":1,"kind":"take","art":"lib:i-1"}]}]}`)
	var got map[string]int
	rec := apiSend(a, "GET", "/api/v1/icon-library/i-1/uses", ana, ``)
	json.Unmarshal(rec.Body.Bytes(), &got)
	if got["cells"] != 1 || got["panels"] != 1 {
		t.Fatalf("uses %d %s", rec.Code, rec.Body.String())
	}
}
