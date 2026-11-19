package app

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/sethdotfm/sode/internal/config"
)

// importApp is exportApp with the signed-in connection kept, so a test can give it a PIN recheck.
func importApp(t *testing.T) (*App, *client, string) {
	t.Helper()
	a := userApp(t, false)
	c := &client{ip: "10.0.0.9", send: make(chan []byte, 4096)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" {
		t.Fatal("no session token for Ada")
	}
	a.mu.Lock()
	a.router.srcNames[2] = "CAM 2"
	a.router.srcNames[3] = "CAM 3"
	a.router.dstNames[4] = "MON 4"
	a.router.dstNames[5] = "MON 5"
	a.mu.Unlock()
	return a, c, tok
}

func postApply(t *testing.T, a *App, tok string, zipBytes []byte, req importReq) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	pw, _ := mw.CreateFormField("plan")
	json.NewEncoder(pw).Encode(req)
	fw, _ := mw.CreateFormFile("file", "export.zip")
	fw.Write(zipBytes)
	mw.Close()
	r := httptest.NewRequest("POST", "/api/import/apply", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set(sessionHeader, tok)
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func applied(t *testing.T, rec *httptest.ResponseRecorder) importResult {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("apply: %d %s", rec.Code, rec.Body)
	}
	var res importResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

func exported(t *testing.T, a *App, tok string, req exportReq) []byte {
	t.Helper()
	rec := postExport(t, a, tok, req)
	if rec.Code != 200 {
		t.Fatalf("export: %d %s", rec.Code, rec.Body)
	}
	return rec.Body.Bytes()
}

func showPreset(c config.Config, name, dst string) config.Preset {
	return config.Preset{Name: name, Items: []config.PresetItem{{Src: c.SourceRef(2, "CAM 2"), Dsts: []config.Ref{c.DestinationRef(4, dst)}}}}
}

func presetNamed(t *testing.T, a *App, name string) (config.Preset, bool) {
	t.Helper()
	for _, p := range a.r().cfg.Presets {
		if p.Name == name {
			return p, true
		}
	}
	return config.Preset{}, false
}

// A file from this install matches by hidden id and preselects Use new. Use new on a global item is
// Critical, so it waits for the PIN recheck.
func TestAddImportSameInstallUpdatesAfterRecheck(t *testing.T) {
	a, c, tok := importApp(t)
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	id := a.r().cfg.Presets[0].ID
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Presets[0].Name = "Show live"
		cfg.Presets[0].Items[0].Dsts = []config.Ref{cfg.DestinationRef(5, "MON 5")}
	})

	rec := postApply(t, a, tok, file, importReq{Mode: importAdd})
	if rec.Code != 403 || !bytes.Contains(rec.Body.Bytes(), []byte(`"recheck"`)) {
		t.Fatalf("without a recheck: %d %s", rec.Code, rec.Body)
	}
	if p := a.r().cfg.Presets[0]; p.Name != "Show live" {
		t.Fatalf("a refused import changed the preset: %+v", p)
	}

	grantRecheck(a, c)
	res := applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if !res.Plan.Critical {
		t.Error("the plan does not say Critical")
	}
	ps := a.r().cfg.Presets
	if len(ps) != 1 || ps[0].ID != id || ps[0].Name != "Show" || ps[0].Items[0].Dsts[0].N != 4 || len(ps[0].Broken) != 0 {
		t.Fatalf("after import: %+v", ps)
	}
}

// The page's way: the token from exportSession, the recheck over the same socket.
func TestAddImportRecheckOverTheSocket(t *testing.T) {
	a := userApp(t, false)
	a.mu.Lock()
	a.router.srcNames[2], a.router.dstNames[4] = "CAM 2", "MON 4"
	a.mu.Unlock()
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets = []config.Preset{showPreset(*cfg, "Show", "MON 4")} })
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	tok, _ := p.ask("exportSession", 7, nil)["token"].(string)
	if tok == "" {
		t.Fatal("no token from exportSession")
	}
	file := exported(t, a, tok, exportReq{Categories: []string{catPresets}})
	lifeCommit(t, a, func(cfg *config.Config) { cfg.Presets[0].Name = "Show live" })
	if rec := postApply(t, a, tok, file, importReq{Mode: importAdd}); rec.Code != 403 {
		t.Fatalf("without a recheck: %d %s", rec.Code, rec.Body)
	}
	passed(t, "recheck", p.recheck("2468"))
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if _, ok := presetNamed(t, a, "Show"); !ok {
		t.Fatalf("presets: %+v", a.r().cfg.Presets)
	}
}
