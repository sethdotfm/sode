package app

import (
	"fmt"
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
