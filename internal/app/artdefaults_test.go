package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func artDefaultList(t *testing.T, a *App, key string) []any {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/default-artwork", key, nil)
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	return apiBody(t, rec)["defaults"].([]any)
}

func setSettingsPin(t *testing.T, a *App, pin string) {
	t.Helper()
	h, err := config.HashPin(pin)
	if err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.SettingsPin = h
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestArtDefaultNeedsTrustedOrPin(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	set := `{"kind":"source","n":1,"art":"icon:camera","artB":"2","hideText":true}`

	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", ana, set), 403, "trusted_required")
	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, set); rec.Code != 200 {
		t.Fatalf("Trusted set: %d %s", rec.Code, rec.Body.String())
	}
	got := artDefaultList(t, a, ana)
	if len(got) != 1 {
		t.Fatalf("defaults %v", got)
	}
	// The router's name when it was set, for the editor's warning.
	if d := got[0].(map[string]any); d["name"] != "CAM 1" || d["art"] != "icon:camera" || d["artB"] != "2" || d["hideText"] != true {
		t.Fatalf("default %v", d)
	}

	clear := `{"kind":"source","n":1}`
	wantErr(t, apiSend(a, "DELETE", "/api/v1/default-artwork", ana, clear), 403, "trusted_required")
	setSettingsPin(t, a, "9999")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/default-artwork", ana, `{"kind":"source","n":1,"pin":"1111"}`), 403, "trusted_required")
	if rec := apiSend(a, "DELETE", "/api/v1/default-artwork", ana, `{"kind":"source","n":1,"pin":"9999"}`); rec.Code != 204 {
		t.Fatalf("PIN clear: %d %s", rec.Code, rec.Body.String())
	}
	if got := artDefaultList(t, a, ana); len(got) != 0 {
		t.Fatalf("after clear %v", got)
	}
	wantErr(t, apiSend(a, "DELETE", "/api/v1/default-artwork", cy, clear), 404, "not_found")

	// An API client is refused even with the PIN (the config already refuses a client named like a user).
	cfg := a.r().cfg
	cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, config.ScopePanels, config.ScopePanelsManage)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", botKey(t, a), strings.TrimSuffix(set, "}")+`,"pin":"9999"}`), 403, "user_required")
}

func TestArtDefaultValidation(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	for _, body := range []string{
		`{"kind":"source","art":"icon:camera"}`,
		`{"kind":"robot","n":1,"art":"icon:camera"}`,
		`{"kind":"preset","scope":"elsewhere","name":"X","art":"icon:camera"}`,
		`{"kind":"source","n":1}`,
		`{"kind":"source","n":1,"art":"https://example.com/x.png"}`,
		`{"kind":"source","n":1,"artB":"17"}`,
	} {
		wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", cy, body), 400, "bad_request")
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":1,"art":"d-00000000"}`), 404, "not_found")
}

// A panel's own image is copied by content, outlives the panel, and is served from its own route.
func TestArtDefaultCopiesPanelImage(t *testing.T) {
	a := panelsApp(t)
	ana, cy, bo := userKey(t, a, "Ana"), userKey(t, a, "Cy"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Art"}`)
	png := b64(t, tinyPNG)
	art := apiBody(t, apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, png))["id"].(string)

	// Bo cannot reach Ana's panel, so cannot lift its image.
	wantErr(t, apiSend(a, "PUT", "/api/v1/default-artwork", bo, `{"kind":"destination","n":2,"art":"`+art+`","panel":"`+id+`"}`), 403, "trusted_required")
	rec := apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"destination","n":2,"art":"`+art+`","panel":"`+id+`"}`)
	if rec.Code != 200 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	img, _ := apiBody(t, rec)["art"].(string)
	if !artDefaultIDRe.MatchString(img) {
		t.Fatalf("image id %q", img)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id, ana, ``); rec.Code != 204 {
		t.Fatalf("delete panel: %d", rec.Code)
	}
	got := apiDo(a, "GET", "/api/v1/default-artwork/images/"+img, bo, nil)
	if got.Code != 200 || !bytes.Equal(got.Body.Bytes(), png) || got.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("image: %d %q", got.Code, got.Header().Get("Content-Type"))
	}
	if !strings.Contains(got.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("csp %q", got.Header().Get("Content-Security-Policy"))
	}

	// The file is beside the config and a fresh store reads it back.
	again := loadArtDefaults(a.cfgPath)
	if b, _, ok := again.image(img); !ok || !bytes.Equal(b, png) || len(again.list()) != 1 {
		t.Fatalf("reload: %v %d", ok, len(again.list()))
	}

	// Replacing the default drops the image nothing names any more.
	apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"destination","n":2,"art":"icon:tv"}`)
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.cfgPath), artDefaultsDir, img)); !os.IsNotExist(err) {
		t.Fatalf("old image kept: %v", err)
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/default-artwork/images/"+img, bo, nil), 404, "not_found")
}

// Hidden: a default for an item outside the Rights is not in the list, the panel read or the image route.
func TestArtDefaultHiddenFromHiddenKey(t *testing.T) {
	for _, vis := range []string{config.VisHidden, ""} {
		a, bot := viewApp(t, vis)
		ana := userKey(t, a, "Ana")
		for _, body := range []string{
			`{"kind":"source","n":1,"art":"icon:camera"}`,
			`{"kind":"source","n":5,"art":"icon:lock"}`,
			`{"kind":"destination","n":7,"artB":"7"}`,
			`{"kind":"preset","scope":"everyone","name":"Secret mix","artB":"S"}`,
			`{"kind":"preset","scope":"everyone","name":"Studio A","artB":"A"}`,
		} {
			// userApp's Advanced Settings PIN is 2468.
			body = strings.TrimSuffix(body, "}") + `,"pin":"2468"}`
			if rec := apiSend(a, "PUT", "/api/v1/default-artwork", ana, body); rec.Code != 200 {
				t.Fatalf("set %s: %d %s", body, rec.Code, rec.Body.String())
			}
		}
		cfg := a.r().cfg
		cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, config.ScopePanels)
		if err := a.ApplyConfig(cfg); err != nil {
			t.Fatal(err)
		}
		want := 5
		if vis == config.VisHidden {
			want = 2
		}
		if got := artDefaultList(t, a, bot); len(got) != want {
			t.Errorf("%q: list %v", vis, got)
		}
	}
}

func TestArtDefaultRidesPanelRead(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Wall","cells":[{"kind":"source","n":1,"r":1,"c":1},{"kind":"preset","scope":"everyone","name":"Studio A","r":1,"c":2}]}`)
	apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":1,"art":"icon:camera"}`)
	apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":9,"art":"icon:mic"}`)
	p := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))
	ds := p["artDefaults"].([]any)
	if len(ds) != 1 || ds[0].(map[string]any)["art"] != "icon:camera" {
		t.Fatalf("artDefaults %v", ds)
	}
	// Stored cells never take the default: a save of the read leaves the cell bare.
	if c := p["cells"].([]any)[0].(map[string]any); c["art"] != nil {
		t.Fatalf("cell froze the default: %v", c)
	}
	back, _ := json.Marshal(p)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, string(back)); rec.Code != 200 {
		t.Fatalf("save the read back: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := a.panels.get(id)
	if _, ok := got.Extra["artDefaults"]; ok || got.Cells[0]["art"] != nil {
		t.Fatalf("stored %+v", got)
	}
}

// Push and templates copy a cell's own artwork only, never the default it shows.
func TestArtDefaultNotFrozenByPushOrTemplate(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	cells := `[{"kind":"source","n":1,"r":1,"c":1},{"kind":"source","n":2,"r":1,"c":2,"art":"icon:mic"}]`
	src := makePanel(t, a, ana, `{"name":"From","cells":`+cells+`}`)
	dst := makePanel(t, a, ana, `{"name":"To"}`)
	apiSend(a, "PUT", "/api/v1/default-artwork", cy, `{"kind":"source","n":1,"art":"icon:camera","hideText":true}`)

	if rec := apiSend(a, "POST", "/api/v1/panels/"+src+"/push", ana, `{"targets":["`+dst+`"]}`); rec.Code != 200 {
		t.Fatalf("push: %d %s", rec.Code, rec.Body.String())
	}
	got, _ := a.panels.get(dst)
	if len(got.Cells) != 2 || got.Cells[0]["art"] != nil || got.Cells[0]["hideText"] != nil || got.Cells[1]["art"] != "icon:mic" {
		t.Fatalf("pushed cells %v", got.Cells)
	}

	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"T","scope":"me","sources":true,"cells":`+cells+`}`)
	if rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	tc := apiBody(t, rec)["cells"].([]any)
	if tc[0].(map[string]any)["art"] != nil || tc[1].(map[string]any)["art"] != "icon:mic" {
		t.Fatalf("template cells %v", tc)
	}
}
