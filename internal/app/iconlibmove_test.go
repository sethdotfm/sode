package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func libIcon(t *testing.T, a *App, key, name string) string {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/icon-library", key, iconUpload(name, b64(t, tinyPNG), ""))
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	return apiBody(t, rec)["id"].(string)
}

func cellArts(t *testing.T, cells []map[string]any) []any {
	t.Helper()
	out := make([]any, len(cells))
	for i, c := range cells {
		out[i] = c["art"]
	}
	return out
}

// Push, Copy and a template are on the same install, so a `lib:<id>` goes as the reference and is never copied
// or skipped as artwork.
func TestIconLibraryRefsSurvivePushCopyAndTemplate(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	ref := "lib:" + libIcon(t, a, cy, "Logo")
	cells := `[{"kind":"source","n":1,"r":1,"c":1,"art":"` + ref + `","artB":"2"},{"kind":"source","n":2,"r":1,"c":2,"art":"lib:0badc0de"}]`
	src := makePanel(t, a, ana, `{"name":"From","cells":`+cells+`}`)
	dst := makePanel(t, a, ana, `{"name":"To"}`)

	rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/push", ana, `{"targets":["`+dst+`"]}`)
	if rec.Code != 200 {
		t.Fatalf("push: %d %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "lib:") {
		t.Errorf("push reported a library icon as skipped artwork: %s", rec.Body.String())
	}
	got, _ := a.panels.get(dst)
	if arts := cellArts(t, got.Pages[0].Cells); len(arts) != 2 || arts[0] != ref || arts[1] != "lib:0badc0de" || got.Pages[0].Cells[0]["artB"] != "2" {
		t.Fatalf("pushed cells %v", got.Pages[0].Cells)
	}
	if len(got.Art) != 0 {
		t.Errorf("push stored artwork for library icons: %v", got.Art)
	}

	rec = apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ana, `{}`)
	if rec.Code != 201 {
		t.Fatalf("copy: %d %s", rec.Code, rec.Body.String())
	}
	cp := apiBody(t, rec)
	if cp["artSkipped"] != nil {
		t.Errorf("copy skipped %v", cp["artSkipped"])
	}
	if c := page1Cells(cp)[0].(map[string]any); c["art"] != ref {
		t.Fatalf("copied cell %v", c)
	}

	rec = apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"T","scope":"me","sources":true,"cells":`+cells+`}`)
	if rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	tid := apiBody(t, rec)["id"].(string)
	tc := page1Cells(apiBody(t, apiDo(a, "GET", "/api/v1/panel-templates/"+tid, ana, nil)))
	if tc[0].(map[string]any)["art"] != ref || tc[1].(map[string]any)["art"] != "lib:0badc0de" {
		t.Fatalf("template cells %v", tc)
	}
}

// A Default artwork names a library icon by reference: nothing is copied into artdefaults.art, so the 64 MB
// cap is not spent on it, and a delete from the library leaves the default drawing nothing.
func TestArtDefaultMayBeLibraryIcon(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := libIcon(t, a, cy, "Logo")

	rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":1,"art":"lib:`+id+`","artB":"3"}`)
	if rec.Code != 200 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	if got := apiBody(t, rec); got["art"] != "lib:"+id || got["artB"] != "3" {
		t.Fatalf("saved %v", got)
	}
	if d := artDefaultList(t, a, ana)[0].(map[string]any); d["art"] != "lib:"+id {
		t.Fatalf("listed %v", d)
	}
	if len(a.artDefaults.data.Images) != 0 {
		t.Errorf("library icon copied into Default artwork: %v", a.artDefaults.data.Images)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.cfgPath), artDefaultsDir)); !os.IsNotExist(err) {
		t.Errorf("artdefaults.art made for a reference: %v", err)
	}

	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":2,"art":"lib:0badc0de"}`), 404, "not_found")
	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":2,"art":"lib:0BADC0DE"}`), 400, "bad_request")
	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":2,"art":"lib:../x"}`), 400, "bad_request")

	if rec := apiSend(a, "DELETE", "/api/v1/icon-library/"+id, cy, `{}`); rec.Code != 204 {
		t.Fatalf("delete icon: %d %s", rec.Code, rec.Body.String())
	}
	if d := artDefaultList(t, a, ana)[0].(map[string]any); d["art"] != "lib:"+id {
		t.Fatalf("default after delete %v", d)
	}
}

// An export is the panel read; an import is that body made a panel on another install. Only the reference
// travels, the panel saves without the icon, and the Default artwork on the first install is untouched.
func TestIconLibraryRefsRoundTripWithoutTheLibrary(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	ref := "lib:" + libIcon(t, a, cy, "Logo")
	src := makePanel(t, a, ana, `{"name":"Wall","cells":[{"kind":"source","n":1,"r":1,"c":1,"art":"`+ref+`"},{"kind":"source","n":2,"r":1,"c":2}]}`)
	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":2,"art":"`+ref+`"}`); rec.Code != 200 {
		t.Fatalf("default: %d %s", rec.Code, rec.Body.String())
	}
	out := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+src, ana, nil))
	if defs, _ := json.Marshal(out["artDefaults"]); !strings.Contains(string(defs), ref) {
		t.Fatalf("panel read lacks the default: %s", defs)
	}
	body, _ := json.Marshal(map[string]any{"name": "Wall", "pages": out["pages"], "settings": out["settings"]})

	b := panelsApp(t)
	bo := userKey(t, b, "Ana")
	id := makePanel(t, b, bo, string(body))
	cells := page1Cells(apiBody(t, apiDo(b, "GET", "/api/v1/panels/"+id, bo, nil)))
	if cells[0].(map[string]any)["art"] != ref || cells[1].(map[string]any)["art"] != nil {
		t.Fatalf("imported cells %v", cells)
	}
	wantErr(t, apiDo(b, "GET", "/api/v1/icon-library/"+strings.TrimPrefix(ref, "lib:"), bo, nil), 404, "not_found")
}
