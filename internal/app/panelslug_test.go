package app

import (
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

// REST seam for a panel's short name: validation, uniqueness, reading by it and the HTML route.

func slugPut(a *App, key, id, body string) int {
	return apiSend(a, "PUT", "/api/v1/panels/"+id, key, body).Code
}

func TestPanelSlugValidation(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	for _, bad := range []string{"Studio", "a b", "-x", "x-", "a_b", "editor", "p-0123abcd", strings.Repeat("a", 33), "é"} {
		rec := apiSend(a, "POST", "/api/v1/panels", ana, `{"name":"X","slug":"`+bad+`"}`)
		if rec.Code != 400 {
			t.Errorf("slug %q: %d %s", bad, rec.Code, rec.Body.String())
		}
	}
	for _, good := range []string{"a", "studio-a", "s1", "9", strings.Repeat("a", 32)} {
		rec := apiSend(a, "POST", "/api/v1/panels", ana, `{"name":"X","slug":"`+good+`"}`)
		if rec.Code != 201 {
			t.Errorf("slug %q: %d %s", good, rec.Code, rec.Body.String())
			continue
		}
		if got := apiBody(t, rec)["slug"]; got != good {
			t.Errorf("slug %q came back %v", good, got)
		}
	}
}

func TestPanelSlugIsUnique(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	one := makePanel(t, a, ana, `{"name":"One","slug":"lobby"}`)
	two := makePanel(t, a, bo, `{"name":"Two"}`)
	if c := apiSend(a, "POST", "/api/v1/panels", bo, `{"name":"Three","slug":"lobby"}`).Code; c != 409 {
		t.Errorf("create with a used slug: %d", c)
	}
	if c := slugPut(a, bo, two, `{"name":"Two","slug":"lobby"}`); c != 409 {
		t.Errorf("update to a used slug: %d", c)
	}
	// Keeping its own slug on a save is not a clash.
	if c := slugPut(a, ana, one, `{"name":"One","slug":"lobby"}`); c != 200 {
		t.Errorf("resave: %d", c)
	}
	// Clearing frees it.
	if c := slugPut(a, ana, one, `{"name":"One"}`); c != 200 {
		t.Errorf("clear: %d", c)
	}
	if c := slugPut(a, bo, two, `{"name":"Two","slug":"lobby"}`); c != 200 {
		t.Errorf("reuse after clear: %d", c)
	}
}

func TestPanelReadBySlug(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Lobby","slug":"lobby"}`)
	rec := apiSend(a, "GET", "/api/v1/panels/lobby", ana, "")
	if rec.Code != 200 || apiBody(t, rec)["id"] != id {
		t.Fatalf("by slug: %d %s", rec.Code, rec.Body.String())
	}
	if c := apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Code; c != 200 {
		t.Errorf("by id: %d", c)
	}
	// Bo may not read it, and the answer is the one for a slug nobody has.
	hidden := apiSend(a, "GET", "/api/v1/panels/lobby", bo, "")
	none := apiSend(a, "GET", "/api/v1/panels/nothing-here", bo, "")
	if hidden.Code != 404 || none.Code != 404 {
		t.Fatalf("hidden %d, unknown %d", hidden.Code, none.Code)
	}
	if strings.Contains(hidden.Body.String(), id) || strings.Contains(hidden.Body.String(), "Lobby") {
		t.Errorf("a refusal names the panel: %s", hidden.Body.String())
	}
	if strings.ReplaceAll(hidden.Body.String(), "lobby", "x") != strings.ReplaceAll(none.Body.String(), "nothing-here", "x") {
		t.Errorf("a refused slug and an unknown one answer differently:\n%s\n%s", hidden.Body.String(), none.Body.String())
	}
}

func TestPanelSlugCopyAndPush(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Lobby","slug":"lobby"}`)
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", ana, "")
	if rec.Code != 201 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body.String())
	}
	if s, _ := apiBody(t, rec)["slug"].(string); s != "" {
		t.Errorf("a copy kept the slug %q", s)
	}
}

func TestPanelPageRouteShape(t *testing.T) {
	a := panelsApp(t)
	fs := fstest.MapFS{"panel.html": {Data: []byte("panel page")}, "panels.html": {Data: []byte("list")}}
	get := func(path string) (int, string) {
		rec := httptest.NewRecorder()
		a.Handler(fs).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
		return rec.Code, rec.Body.String()
	}
	for _, p := range []string{"/panels/lobby", "/panels/p-0123abcd", "/panels/studio-a"} {
		if c, b := get(p); c != 200 || b != "panel page" {
			t.Errorf("%s: %d %q", p, c, b)
		}
	}
	// A segment that can be neither an id nor a slug never reaches the page.
	for _, p := range []string{"/panels/Studio", "/panels/a_b", "/panels/-x"} {
		if c, _ := get(p); c != 404 {
			t.Errorf("%s: %d", p, c)
		}
	}
}

// A panel key and a setup token carry 128 random bits: enough for a secret that is throttled and
// revocable, and half the characters in the link.
func TestPanelLinkSecretsAreShort(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	if n := len(strings.TrimPrefix(secret, keyPrefix)); n != 22 {
		t.Errorf("panel key body is %d characters", n)
	}
	token, _ := setupLink(t, a, cy, id)
	if n := len(strings.TrimPrefix(token, setupPrefix)); n != 22 {
		t.Errorf("setup token body is %d characters", n)
	}
	rec := exchangeSetup(a, apiPeer, id, token)
	if rec.Code != 201 {
		t.Fatalf("exchange: %d %s", rec.Code, rec.Body.String())
	}
	if n := len(strings.TrimPrefix(apiBody(t, rec)["key"].(string), keyPrefix)); n != 22 {
		t.Errorf("exchanged key body is %d characters", n)
	}
	// Setup by short name trades the same token.
	made := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, apiPanel+`,"slug":"desk"}`)
	if made.Code != 200 {
		t.Fatalf("slug: %d %s", made.Code, made.Body.String())
	}
	token, _ = setupLink(t, a, cy, id)
	if rec := exchangeSetup(a, apiPeer, "desk", token); rec.Code != 201 {
		t.Errorf("exchange by slug: %d %s", rec.Code, rec.Body.String())
	}
}
