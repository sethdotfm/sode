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
