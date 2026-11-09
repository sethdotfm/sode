package app

import (
	"bytes"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

const (
	tinyPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
	tinyGIF = "R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7"
)

func b64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func jpegBytes() []byte {
	return append([]byte{0xff, 0xd8, 0xff, 0xe0, 0, 16, 'J', 'F', 'I', 'F', 0}, make([]byte, 20)...)
}

func TestPanelArtworkTypesAreDecidedByContent(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Art"}`)
	svg := `<?xml version="1.0"?><!-- c --><svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"/>`
	for name, c := range map[string]struct {
		body []byte
		typ  string
	}{
		"png":  {b64(t, tinyPNG), "image/png"},
		"gif":  {b64(t, tinyGIF), "image/gif"},
		"jpeg": {jpegBytes(), "image/jpeg"},
		"svg":  {[]byte(svg), "image/svg+xml"},
	} {
		rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, c.body)
		if rec.Code != 201 || apiBody(t, rec)["type"] != c.typ {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
			continue
		}
		art := apiBody(t, rec)["id"].(string)
		got := apiDo(a, "GET", "/api/v1/panels/"+id+"/artwork/"+art, ana, nil)
		if got.Code != 200 || !bytes.Equal(got.Body.Bytes(), c.body) || got.Header().Get("Content-Type") != c.typ {
			t.Errorf("%s fetch: %d %q", name, got.Code, got.Header().Get("Content-Type"))
		}
	}
	// The panel lists what it holds.
	if n := len(apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))["art"].([]any)); n != 4 {
		t.Errorf("panel lists %d artwork files", n)
	}
}

func TestPanelArtworkRefusals(t *testing.T) {
	a := panelsApp(t)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Art","share":{"mode":"everyone"}}`)
	up := "/api/v1/panels/" + id + "/artwork"
	for name, body := range map[string][]byte{
		"html":        []byte("<html><script>alert(1)</script></html>"),
		"text":        []byte("hello"),
		"empty":       nil,
		"xml not svg": []byte(`<?xml version="1.0"?><note/>`),
		"html first":  []byte(`<div><svg xmlns="http://www.w3.org/2000/svg"/></div>`),
		"svg in text": []byte("see <svg/>"),
		"pdf":         []byte("%PDF-1.4"),
	} {
		if rec := apiSendRaw(a, "POST", up, ana, body); rec.Code != 415 || apiErrCode(t, rec) != "unsupported_type" {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	// Over 1 MB is refused whatever it is.
	big := append(b64(t, tinyPNG), make([]byte, 1<<20)...)
	wantErr(t, apiSendRaw(a, "POST", up, ana, big), 413, "too_large")
	// A user of a shared panel may fetch artwork but not add it.
	ok := apiSendRaw(a, "POST", up, ana, b64(t, tinyPNG))
	art := apiBody(t, ok)["id"].(string)
	if apiDo(a, "GET", up+"/"+art, bo, nil).Code != 200 {
		t.Error("use-only user cannot read artwork")
	}
	wantErr(t, apiSendRaw(a, "POST", up, bo, b64(t, tinyPNG)), 403, "not_owner")
	wantErr(t, apiSend(a, "DELETE", up+"/"+art, bo, ""), 403, "not_owner")
	wantErr(t, apiDo(a, "GET", up+"/a-00000000", ana, nil), 404, "not_found")
	if rec := apiSend(a, "DELETE", up+"/"+art, ana, ""); rec.Code != 204 {
		t.Errorf("delete artwork: %d", rec.Code)
	}
	wantErr(t, apiDo(a, "GET", up+"/"+art, ana, nil), 404, "not_found")
}

// An SVG with a script is only ever served as an image under a sandbox that blocks scripts,
// with no sniffing, so opening the URL directly runs nothing.
func TestPanelArtworkSVGWithScriptNeverRuns(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Art"}`)
	evil := `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(2)</script></svg>`
	rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, []byte(evil))
	if rec.Code != 201 {
		t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
	}
	got := apiDo(a, "GET", "/api/v1/panels/"+id+"/artwork/"+apiBody(t, rec)["id"].(string), ana, nil)
	h := got.Header()
	csp := h.Get("Content-Security-Policy")
	if h.Get("Content-Type") != "image/svg+xml" || h.Get("X-Content-Type-Options") != "nosniff" ||
		!strings.Contains(csp, "default-src 'none'") || !strings.Contains(csp, "sandbox") || strings.Contains(csp, "allow-scripts") {
		t.Fatalf("headers %v", h)
	}
}

func TestPanelArtworkLandsBesideThePanelAndGoesWithIt(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Art"}`)
	rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, b64(t, tinyPNG))
	art := apiBody(t, rec)["id"].(string)
	dir := filepath.Join(filepath.Dir(a.cfgPath), "panels")
	if _, err := os.Stat(filepath.Join(dir, id+".art", art)); err != nil {
		t.Fatal(err)
	}
	// Save a copy carries the artwork under the same id, so cells that name it still work.
	bo := userKey(t, a, "Bo")
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Art","share":{"mode":"everyone"}}`)
	cp := apiBody(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/copy", bo, ""))["id"].(string)
	if got := apiDo(a, "GET", "/api/v1/panels/"+cp+"/artwork/"+art, bo, nil); got.Code != 200 {
		t.Fatalf("copy artwork: %d", got.Code)
	}
	apiSend(a, "DELETE", "/api/v1/panels/"+id, ana, "")
	if _, err := os.Stat(filepath.Join(dir, id+".art")); !os.IsNotExist(err) {
		t.Errorf("artwork folder left: %v", err)
	}
	if got := apiDo(a, "GET", "/api/v1/panels/"+cp+"/artwork/"+art, bo, nil); got.Code != 200 {
		t.Fatalf("copy artwork after the original went: %d", got.Code)
	}
}

// openapi.yaml names every panel route (TestOpenAPIDocumentsEveryRoute), every scope and
// every error code the API can send.
func TestOpenAPIListsScopesAndErrorCodes(t *testing.T) {
	var doc struct {
		Components struct {
			Schemas struct {
				Error struct {
					Properties struct {
						Error struct {
							Properties struct {
								Code struct {
									Enum []string `yaml:"enum"`
								} `yaml:"code"`
							} `yaml:"properties"`
						} `yaml:"error"`
					} `yaml:"properties"`
				} `yaml:"Error"`
			} `yaml:"schemas"`
		} `yaml:"components"`
	}
	raw := string(openAPIFile)
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		t.Fatal(err)
	}
	codes := doc.Components.Schemas.Error.Properties.Error.Properties.Code.Enum
	for _, c := range []string{"not_owner", "user_required", "unsupported_type", "internal"} {
		if !apiHas(codes, c) {
			t.Errorf("error code %s is not in openapi.yaml", c)
		}
	}
	for _, s := range config.AllScopes {
		if !strings.Contains(raw, "`"+s+"`") {
			t.Errorf("scope %s is not named in openapi.yaml", s)
		}
	}
	for _, r := range apiRoutes {
		if strings.Contains(r.Path, "/panels") && !strings.Contains(raw, "`"+r.Scope[:strings.IndexAny(r.Scope+" ", " ")]+"`") {
			t.Errorf("%s %s: scope %s is not named", r.Method, r.Path, r.Scope)
		}
	}
	if !strings.Contains(raw, "Panels need the API switch") {
		t.Error("openapi.yaml does not say panels ride the API switch")
	}
}
