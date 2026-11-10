package app

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func iconUpload(name string, data []byte, pin string) string {
	b, _ := json.Marshal(map[string]any{"name": name, "data": base64.StdEncoding.EncodeToString(data), "pin": pin})
	return string(b)
}

func iconList(t *testing.T, a *App, key string) []any {
	t.Helper()
	rec := apiDo(a, "GET", "/api/v1/icon-library", key, nil)
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	return apiBody(t, rec)["icons"].([]any)
}

func TestIconLibraryUploadNeedsTrustedOrPin(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	png := b64(t, tinyPNG)

	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library", ana, iconUpload("Logo", png, "")), 403, "trusted_required")
	rec := apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload("Logo", png, ""))
	if rec.Code != 201 {
		t.Fatalf("Trusted upload: %d %s", rec.Code, rec.Body.String())
	}
	got := apiBody(t, rec)
	id, _ := got["id"].(string)
	if !iconIDRe.MatchString(id) || got["name"] != "Logo" || got["type"] != "image/png" {
		t.Fatalf("icon %v", got)
	}

	setSettingsPin(t, a, "9999")
	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library", ana, iconUpload("Mark", png, "1111")), 403, "trusted_required")
	if rec := apiSend(a, "POST", "/api/v1/icon-library", ana, iconUpload("Mark", png, "9999")); rec.Code != 201 {
		t.Fatalf("PIN upload: %d %s", rec.Code, rec.Body.String())
	}

	// Anyone who edits panels lists and reads it.
	if n := len(iconList(t, a, ana)); n != 2 {
		t.Fatalf("list has %d", n)
	}
	file := apiDo(a, "GET", "/api/v1/icon-library/"+id, ana, nil)
	if file.Code != 200 || !bytes.Equal(file.Body.Bytes(), png) || file.Header().Get("Content-Type") != "image/png" {
		t.Fatalf("file: %d %q", file.Code, file.Header().Get("Content-Type"))
	}
	if !strings.Contains(file.Header().Get("Content-Security-Policy"), "sandbox") {
		t.Errorf("csp %q", file.Header().Get("Content-Security-Policy"))
	}

	cfg := a.r().cfg
	cfg.APIClients[0].Scopes = append(cfg.APIClients[0].Scopes, config.ScopePanels, config.ScopePanelsManage)
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	bot := botKey(t, a)
	wantErr(t, apiSend(a, "POST", "/api/v1/icon-library", bot, iconUpload("Bot", png, "9999")), 403, "user_required")
	// A panel's own client reads icons, so a panel page draws them.
	if rec := apiDo(a, "GET", "/api/v1/icon-library/"+id, bot, nil); rec.Code != 200 {
		t.Fatalf("client read: %d", rec.Code)
	}

	// The index and the file are beside the config, and a fresh store reads them back.
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.cfgPath), iconLibDir, id+".png")); err != nil {
		t.Fatal(err)
	}
	again := loadIconLib(a.cfgPath)
	if b, _, ok := again.file(id); !ok || !bytes.Equal(b, png) || len(again.list()) != 2 {
		t.Fatalf("reload: %v %d", ok, len(again.list()))
	}
}

func TestIconLibraryLimits(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	up := func(name string, data []byte) *httptest.ResponseRecorder {
		return apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload(name, data, ""))
	}

	wantErr(t, up("Photo", jpegBytes()), 415, "unsupported_type")
	wantErr(t, up("Gif", b64(t, tinyGIF)), 415, "unsupported_type")
	wantErr(t, up("Text", []byte("hello")), 415, "unsupported_type")
	rec := up("Wide", imgBytes("png", 1025, 4))
	wantErr(t, rec, 400, "bad_request")
	if !strings.Contains(rec.Body.String(), "1024") {
		t.Errorf("side limit not stated: %s", rec.Body.String())
	}
	rec = up("Huge", append(bigSVG(), bytes.Repeat([]byte(" "), 200)...))
	wantErr(t, rec, 413, "too_large")
	if !strings.Contains(rec.Body.String(), "2 MB") {
		t.Errorf("byte limit not stated: %s", rec.Body.String())
	}

	for i := 0; i < 32; i++ {
		if rec := up("Big", bigSVG()); rec.Code != 201 {
			t.Fatalf("file %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	rec = up("Big", bigSVG())
	wantErr(t, rec, 409, "conflict")
	if !strings.Contains(rec.Body.String(), "64 MB") {
		t.Errorf("total not stated: %s", rec.Body.String())
	}
	if rec := up("Small", b64(t, tinyPNG)); rec.Code != 201 {
		t.Errorf("small icon after the big ones: %d %s", rec.Code, rec.Body.String())
	}
}

func TestIconLibraryNames(t *testing.T) {
	a := panelsApp(t)
	cy := userKey(t, a, "Cy")
	png := b64(t, tinyPNG)
	name := func(n string) string {
		t.Helper()
		rec := apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload(n, png, ""))
		if rec.Code != 201 {
			t.Fatalf("upload %q: %d %s", n, rec.Code, rec.Body.String())
		}
		return apiBody(t, rec)["name"].(string)
	}
	if got := name("  Logo "); got != "Logo" {
		t.Errorf("trimmed %q", got)
	}
	if got := name("logo"); got != "logo 2" {
		t.Errorf("clash %q", got)
	}
	if got := name("LOGO"); got != "LOGO 3" {
		t.Errorf("second clash %q", got)
	}
	forty := strings.Repeat("é", iconNameMax)
	name(forty)
	if got := name(forty); got != strings.Repeat("é", iconNameMax-2)+" 2" {
		t.Errorf("long clash %q", got)
	}
	for _, bad := range []string{"", "   ", strings.Repeat("x", iconNameMax+1)} {
		wantErr(t, apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload(bad, png, "")), 400, "bad_request")
	}
}

// Old panels and a reference to an icon that is not in the library both save; the cell keeps it and draws nothing.
func TestIconLibraryUnknownRefSaves(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Lib"}`)
	body := `{"name":"Lib","cells":[{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":1,"art":"lib:0badc0de"}]}`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	cells := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))["cells"].([]any)
	if art := cells[0].(map[string]any)["art"]; art != "lib:0badc0de" {
		t.Fatalf("art %v", art)
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/icon-library/0badc0de", ana, nil), 404, "not_found")
}

// Rename keeps the id, so a cell's `lib:<id>` still draws; a case-only rename is not a clash with itself.
func TestIconLibraryRename(t *testing.T) {
	a := panelsApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	png := b64(t, tinyPNG)
	add := func(n string) string {
		t.Helper()
		rec := apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload(n, png, ""))
		if rec.Code != 201 {
			t.Fatalf("upload %q: %d %s", n, rec.Code, rec.Body.String())
		}
		return apiBody(t, rec)["id"].(string)
	}
	id := add("Logo")
	add("Camera")
	rename := func(key, n, pin string) *httptest.ResponseRecorder {
		b, _ := json.Marshal(map[string]string{"name": n, "pin": pin})
		return apiSend(a, "PATCH", "/api/v1/icon-library/"+id, key, string(b))
	}

	wantErr(t, rename(ana, "Mark", ""), 403, "trusted_required")
	for _, c := range [][2]string{{"logo", "logo"}, {" Station ", "Station"}, {"camera", "camera 2"}} {
		rec := rename(cy, c[0], "")
		got := apiBody(t, rec)
		if rec.Code != 200 || got["name"] != c[1] || got["id"] != id {
			t.Fatalf("rename %q: %d %s, want %q", c[0], rec.Code, rec.Body.String(), c[1])
		}
	}
	setSettingsPin(t, a, "9999")
	if rec := rename(ana, "By PIN", "9999"); rec.Code != 200 {
		t.Fatalf("PIN rename: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, rename(cy, "", ""), 400, "bad_request")
	wantErr(t, apiSend(a, "PATCH", "/api/v1/icon-library/0badc0de", cy, `{"name":"X"}`), 404, "not_found")

	if rec := apiDo(a, "GET", "/api/v1/icon-library/"+id, ana, nil); rec.Code != 200 || !bytes.Equal(rec.Body.Bytes(), png) {
		t.Fatalf("file after rename: %d", rec.Code)
	}
	if got := loadIconLib(a.cfgPath).list(); got[0].ID != id || got[0].Name != "By PIN" {
		t.Fatalf("reloaded %+v", got)
	}
}

// Delete needs a Trusted user or the PIN. Cells that named the icon keep `lib:<id>` and draw no Icon A, as any
// reference the library does not hold; their Icon B stays. The use count spans every owner's panels.
func TestIconLibraryDeleteAndUses(t *testing.T) {
	a := panelsApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload("Logo", b64(t, tinyPNG), ""))
	id := apiBody(t, rec)["id"].(string)
	lib := "lib:" + id
	makePanel(t, a, ana, `{"name":"A","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"source","n":1,"art":"`+lib+`","artB":"2"},{"r":1,"c":2,"kind":"source","n":2,"art":"`+lib+`"},{"r":2,"c":1,"kind":"source","n":3,"art":"icon:camera"}]}`)
	pb := makePanel(t, a, bo, `{"name":"B","cells":[{"r":1,"c":1,"kind":"dest","n":1,"art":"`+lib+`"}]}`)
	makePanel(t, a, bo, `{"name":"C","cells":[{"r":1,"c":1,"kind":"dest","n":1}]}`)

	uses := func(key, icon string) map[string]any {
		t.Helper()
		rec := apiDo(a, "GET", "/api/v1/icon-library/"+icon+"/uses", key, nil)
		if rec.Code != 200 {
			t.Fatalf("uses: %d %s", rec.Code, rec.Body.String())
		}
		return apiBody(t, rec)
	}
	if got := uses(cy, id); got["cells"] != 3.0 || got["panels"] != 2.0 {
		t.Fatalf("uses %v, want 3 cells on 2 panels", got)
	}

	wantErr(t, apiSend(a, "DELETE", "/api/v1/icon-library/"+id, ana, ``), 403, "trusted_required")
	setSettingsPin(t, a, "9999")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/icon-library/"+id, ana, `{"pin":"1111"}`), 403, "trusted_required")
	if len(a.icons.list()) != 1 {
		t.Fatal("a refused delete removed the icon")
	}
	if rec := apiSend(a, "DELETE", "/api/v1/icon-library/"+id, cy, ``); rec.Code != 204 {
		t.Fatalf("Trusted delete: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/icon-library/"+id, ana, nil), 404, "not_found")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/icon-library/"+id, cy, ``), 404, "not_found")
	if _, err := os.Stat(filepath.Join(filepath.Dir(a.cfgPath), iconLibDir, id+".png")); !os.IsNotExist(err) {
		t.Errorf("file left behind: %v", err)
	}
	if len(loadIconLib(a.cfgPath).list()) != 0 {
		t.Error("the index still holds it")
	}
	if got := uses(cy, id); got["cells"] != 3.0 {
		t.Errorf("cells still naming it %v", got)
	}
	if p, _ := a.panels.get(pb); p.Cells[0]["art"] != lib {
		t.Errorf("cell art %v, want %s kept", p.Cells[0]["art"], lib)
	}

	// The PIN deletes too.
	id2 := apiBody(t, apiSend(a, "POST", "/api/v1/icon-library", cy, iconUpload("Mark", b64(t, tinyPNG), "")))["id"].(string)
	if rec := apiSend(a, "DELETE", "/api/v1/icon-library/"+id2, ana, `{"pin":"9999"}`); rec.Code != 204 {
		t.Fatalf("PIN delete: %d %s", rec.Code, rec.Body.String())
	}
}

// A hand-edited icons.yaml: an entry whose type is not PNG or SVG, or whose name is empty, too long or holds control
// characters, is dropped, since the type becomes the Content-Type and the name reaches every page.
func TestIconLibraryLoadDropsBadEntries(t *testing.T) {
	dir := t.TempDir()
	file := "rev: 2\nicons:\n" +
		"  - {id: 0000000a, name: Good, type: image/png, size: 4}\n" +
		"  - {id: 0000000b, name: Page, type: text/html, size: 4}\n" +
		"  - {id: 0000000c, name: Photo, type: image/jpeg, size: 4}\n" +
		"  - {id: 0000000d, name: \"\", type: image/png, size: 4}\n" +
		"  - {id: 0000000e, name: \"" + strings.Repeat("x", iconNameMax+1) + "\", type: image/png, size: 4}\n" +
		"  - {id: 0000000f, name: \"Bell\\a\", type: image/svg+xml, size: 4}\n" +
		"  - {id: 00000010, name: Vector, type: image/svg+xml, size: 4}\n"
	if err := os.WriteFile(filepath.Join(dir, iconLibFile), []byte(file), 0o600); err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, ic := range loadIconLib(filepath.Join(dir, "config.yaml")).list() {
		names = append(names, ic.Name)
	}
	if got := strings.Join(names, ","); got != "Good,Vector" {
		t.Fatalf("icons %s", got)
	}
}
