package app

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// REST seam for the Fixed destination setting, `settings.fixedDests`.

func fixedOf(p map[string]any) any {
	s, _ := p["settings"].(map[string]any)
	return s["fixedDests"]
}

func TestFixedDestsSaveAndRead(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","settings":{"mode":"take","fixedDests":[12,0,40]}}`)
	if got := fmt.Sprint(fixedOf(readPanel(t, a, ana, id))); got != "[12 0 40]" {
		t.Fatalf("fixed destinations %s", got)
	}
	again := loadPanels(a.cfgPath)
	if p, _ := again.get(id); fmt.Sprint(p.Settings["fixedDests"]) != "[12 0 40]" {
		t.Fatalf("after reload %v", p.Settings)
	}

	plain := makePanel(t, a, ana, `{"name":"Plain","settings":{"mode":"take"}}`)
	if f := fixedOf(readPanel(t, a, ana, plain)); f != nil {
		t.Fatalf("a panel with none stored reads %v", f)
	}
}

// A panel file written before Fixed destination has no fixedDests; it loads as it was and the file is not touched.
func TestFixedDestsOldPanelFileLoadsUnchanged(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "panels"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := []byte("id: p-0000abcd\nname: Old wall\nowner: Ana\nacts: session\nrows: 1\ncols: 2\ncells:\n    - {r: 1, c: 1, rs: 1, cs: 1, kind: destination, n: 7}\n    - {r: 1, c: 2, rs: 1, cs: 1, kind: source, n: 4}\nsettings:\n    mode: instant\n    levels: [1, 2]\n")
	path := filepath.Join(dir, "panels", "p-0000abcd.yaml")
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}
	s := loadPanels(filepath.Join(dir, "config.yaml"))
	p, ok := s.get("p-0000abcd")
	if !ok || len(s.problems()) != 0 {
		t.Fatalf("not loaded: ok=%v problems %v", ok, s.problems())
	}
	if _, has := p.Settings[panelFixedKey]; has || fmt.Sprint(p.Settings) != "map[levels:[1 2] mode:instant]" {
		t.Fatalf("settings %v", p.Settings)
	}
	if got := fmt.Sprint(p.Cells); got != "[map[c:1 cs:1 kind:destination n:7 r:1 rs:1] map[c:2 cs:1 kind:source n:4 r:1 rs:1]]" {
		t.Fatalf("cells %s", got)
	}
	if b, _ := os.ReadFile(path); !bytes.Equal(b, old) {
		t.Fatalf("the file was rewritten:\n%s", b)
	}
}

func TestFixedDestsRefusesABadList(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	upTo := func(n int) string {
		s := make([]string, n)
		for i := range s {
			s[i] = fmt.Sprint(i)
		}
		return "[" + strings.Join(s, ",") + "]"
	}
	for _, list := range []string{`[1.5]`, `["3"]`, `[-1]`, `[4,4]`, upTo(17), `7`, `[null]`} {
		body := `{"name":"Wall","settings":{"fixedDests":` + list + `}}`
		wantErr(t, apiSend(a, "POST", "/api/v1/panels", ana, body), 400, "bad_request")
		wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body), 400, "bad_request")
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","settings":{"fixedDests":`+upTo(16)+`}}`); rec.Code != 200 {
		t.Fatalf("16 entries: %d %s", rec.Code, rec.Body.String())
	}
}

func TestFixedDestsGoInATemplateOnlyWithDests(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	body := `{"name":"%s","scope":"%s",%s"settings":{"mode":"instant","fixedDests":[3,4]},"cells":[]}`

	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, fmt.Sprintf(body, "Plain", "me", `"sources":true,`))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if p := apiBody(t, rec); fixedOf(p) != nil || p["settings"].(map[string]any)["mode"] != "instant" {
		t.Fatalf("a template without dests kept %v", p["settings"])
	}

	rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, fmt.Sprintf(body, "With", "me", `"dests":true,`))
	if rec.Code != 201 {
		t.Fatalf("create: %d %s", rec.Code, rec.Body.String())
	}
	if f := fmt.Sprint(fixedOf(apiBody(t, rec))); f != "[3 4]" {
		t.Fatalf("a template with dests lost the list: %s", f)
	}

	wantErr(t, apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Bad","scope":"me","dests":true,"settings":{"fixedDests":[-2]},"cells":[]}`), 400, "bad_request")
}

func TestFixedDestsStayWithEachPanelOnPushAndComeBackWithASet(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"Wall 1","cells":[{"r":1,"c":1,"kind":"take"}],"settings":{"mode":"instant","fixedDests":[1]}}`)
	two := makePanel(t, a, ana, `{"name":"Wall 2","settings":{"fixedDests":[2,22]}}`)
	none := makePanel(t, a, ana, `{"name":"Wall 3"}`)

	rec := apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Show A", "me", src, two))
	if rec.Code != 201 {
		t.Fatalf("create set: %d %s", rec.Code, rec.Body.String())
	}
	sid := apiBody(t, rec)["id"].(string)

	if code, body := pushTo(a, ana, src, two, none); code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	p := readPanel(t, a, ana, two)
	if f := fmt.Sprint(fixedOf(p)); f != "[2 22]" || p["settings"].(map[string]any)["mode"] != "instant" || cellCount(p) != 1 {
		t.Fatalf("Wall 2 after the Push: %v", p)
	}
	if p := readPanel(t, a, ana, none); fixedOf(p) != nil || p["settings"].(map[string]any)["mode"] != "instant" {
		t.Fatalf("Wall 3 took the source's list: %v", p["settings"])
	}

	apiSend(a, "PUT", "/api/v1/panels/"+src, ana, `{"name":"Wall 1","settings":{"fixedDests":[9]}}`)
	apiSend(a, "PUT", "/api/v1/panels/"+two, ana, `{"name":"Wall 2","settings":{}}`)
	rec = apiSend(a, "POST", "/api/v1/panel-sets/"+sid+"/push", ana, `{}`)
	if rec.Code != 200 {
		t.Fatalf("push set: %d %s", rec.Code, rec.Body.String())
	}
	if f := fmt.Sprint(fixedOf(readPanel(t, a, ana, src))); f != "[1]" {
		t.Fatalf("Wall 1 after the set: %s", f)
	}
	if f := fmt.Sprint(fixedOf(readPanel(t, a, ana, two))); f != "[2 22]" {
		t.Fatalf("Wall 2 after the set: %s", f)
	}
}
