package app

import "testing"

// Page Name and Info cells' show list (checkShowParts): kept when it names that kind's parts, refused otherwise.

func TestPageCellsShowListIsKept(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","pages":[{"id":"one","cells":[
	 {"r":1,"c":1,"kind":"pageName","show":["number","name"]},{"r":1,"c":2,"kind":"info"},{"r":1,"c":3,"kind":"info","show":[]}]}]}`)
	cells := pagesOf(t, readPanel(t, a, ana, id))[0]["cells"].([]any)
	if s, _ := cells[0].(map[string]any)["show"].([]any); len(s) != 2 {
		t.Fatalf("Page Name show not kept: %v", cells[0])
	}
	if _, has := cells[1].(map[string]any)["show"]; has {
		t.Fatalf("an Info cell gained a show list: %v", cells[1])
	}
}

func TestPageCellsBadShowIsRefused(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	for _, c := range []string{
		`{"r":1,"c":1,"kind":"pageName","show":["panel"]}`,
		`{"r":1,"c":1,"kind":"info","show":["name"]}`,
		`{"r":1,"c":1,"kind":"info","show":"page"}`,
		`{"r":1,"c":1,"kind":"info","show":[1]}`,
		`{"r":1,"c":1,"kind":"pageName","show":["name","name"]}`,
	} {
		wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","pages":[{"id":"one","cells":[`+c+`]}]}`), 400, "bad_request")
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","pages":[{"id":"one","cells":[{"r":1,"c":1,"kind":"take","show":7}]}]}`); rec.Code != 200 {
		t.Fatalf("a take with a stray show field: %d %s", rec.Code, rec.Body)
	}
}
