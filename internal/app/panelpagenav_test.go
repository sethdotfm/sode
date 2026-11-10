package app

import "testing"

// Go to Page targets on save (checkPageTargets): a well-formed target is kept, a missing one is stored as is and read as
// unassigned by the pages, and only a malformed one is refused.

const navPages = `"pages":[
 {"id":"one","cells":[{"r":1,"c":1,"kind":"goToPage","page":"two"},{"r":1,"c":2,"kind":"nextPage"},{"r":1,"c":3,"kind":"previousPage"}]},
 {"id":"two","cells":[{"r":1,"c":1,"kind":"goToPage","page":"gone"},{"r":1,"c":2,"kind":"goToPage"},{"r":1,"c":3,"kind":"goToPage","page":"","caption":"Later"}]}]`

func TestPageTargetsAreKeptEvenWhenMissing(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","settings":{"wrapPages":false},`+navPages+`}`)
	got := readPanel(t, a, ana, id)
	pages := pagesOf(t, got)
	first := pages[0]["cells"].([]any)
	second := pages[1]["cells"].([]any)
	if c := first[0].(map[string]any); c["page"] != "two" || c["kind"] != "goToPage" {
		t.Fatalf("target not kept: %v", c)
	}
	if c := second[0].(map[string]any); c["page"] != "gone" {
		t.Fatalf("a missing target was changed: %v", c)
	}
	if len(first) != 3 || len(second) != 3 {
		t.Fatalf("cells dropped: %v %v", first, second)
	}
	if got["settings"].(map[string]any)["wrapPages"] != false {
		t.Fatalf("wrap setting lost: %v", got["settings"])
	}
}

func TestPageTargetMalformedIsRefused(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	for _, target := range []string{`7`, `true`, `"has space"`, `"seventeen-chars-x"`, `{"id":"one"}`} {
		body := `{"name":"Wall","pages":[{"id":"one","cells":[{"r":1,"c":1,"kind":"goToPage","page":` + target + `}]}]}`
		wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body), 400, "bad_request")
	}
	// The field means nothing on another kind, so it is not judged there.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","pages":[{"id":"one","cells":[{"r":1,"c":1,"kind":"take","page":7}]}]}`); rec.Code != 200 {
		t.Fatalf("a take with a stray page field: %d %s", rec.Code, rec.Body)
	}
}

func TestTemplateKeepsPageTargets(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"Nav","scope":"me",`+navPages+`}`)
	if rec.Code != 201 {
		t.Fatalf("template save: %d %s", rec.Code, rec.Body)
	}
	tpl := apiBody(t, rec)
	if c := page1Cells(tpl)[0].(map[string]any); c["page"] != "two" {
		t.Fatalf("a template dropped the Go to Page target: %v", c)
	}
}
