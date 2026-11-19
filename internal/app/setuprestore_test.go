package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// backupReq is Back up everything (web/exportrules.js backupBody).
func backupReq(pins bool) exportReq {
	return exportReq{
		Categories:  []string{catSettings, catRouters, catUsers, catPresets, catFilterSets, catPanels, catArtwork, catAPIClients, catBrowser},
		IncludePins: pins,
		Browser:     json.RawMessage(`{"theme":"dark","filterSets":[{"name":"Mine","src":"CAM"}],"presets":[]}`),
	}
}

func backupOf(t *testing.T, pins bool) (*App, []byte) {
	t.Helper()
	a, tok := exportApp(t)
	seedEverything(t, a)
	seedPanelExtras(t, a)
	lifeCommit(t, a, func(c *config.Config) { c.BrandText, c.Title, c.Prefix = "Truck A", "Main", "B" })
	a.mu.Lock()
	a.router.namesAt = time.Date(2026, 11, 18, 9, 30, 0, 0, time.UTC)
	a.mu.Unlock()
	return a, exported(t, a, tok, backupReq(pins))
}

func restoreKey(t *testing.T, a *App) (*client, string) {
	t.Helper()
	c := &client{ip: "10.0.0.7", send: make(chan []byte, 4096)}
	register(a, c)
	key, err := a.setupRestoreKey(c)
	if err != nil {
		t.Fatal(err)
	}
	return c, key
}

func postRestore(t *testing.T, a *App, key string, file []byte, pin string) *httptest.ResponseRecorder {
	t.Helper()
	h := map[string]string{}
	if key != "" {
		h[setupKeyHeader] = key
	}
	return postMultipart(t, a, "/api/setup/restore", restoreReq{Pin: pin}, file, h)
}

func restored(t *testing.T, rec *httptest.ResponseRecorder) restoreResult {
	t.Helper()
	if rec.Code != 200 {
		t.Fatalf("restore: %d %s", rec.Code, rec.Body)
	}
	var res restoreResult
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	return res
}

// A fresh install restores Back up everything, then backs up the same file. So the config, users, presets,
// panels and artwork all came back, with their ids, UIDs and names table.
func TestRestoreRoundTrip(t *testing.T) {
	_, file := backupOf(t, true)
	b := setupApp(t)
	_, key := restoreKey(t, b)
	res := restored(t, postRestore(t, b, key, file, ""))
	if len(res.Failed) != 0 {
		t.Fatalf("failed: %v", res.Failed)
	}
	if res.Browser["theme"] != "dark" {
		t.Fatalf("This browser values not answered: %v", res.Browser)
	}
	if b.setupPending() {
		t.Fatal("setup still pending after the restore")
	}
	again := exported(t, b, signInToken(t, b, "Ada", "2468"), backupReq(true))
	want, got := unzipAll(t, file), unzipAll(t, again)
	if len(want) < 12 {
		t.Fatalf("the backup holds only %v", slicesSortedKeys(want))
	}
	for name, w := range want {
		if g, ok := got[name]; !ok {
			t.Errorf("%s is not in the second backup", name)
		} else if !bytes.Equal(w, g) {
			t.Errorf("%s differs:\n--- first\n%s\n--- second\n%s", name, w, g)
		}
	}
	for name := range got {
		if _, ok := want[name]; !ok {
			t.Errorf("%s is only in the second backup", name)
		}
	}
}

// Include PINs off: the restore asks for the first Administrator's PIN, and that user then signs in with it.
func TestRestoreWithoutHashesAsksForTheAdministratorsPin(t *testing.T) {
	_, file := backupOf(t, false)
	b := setupApp(t)
	_, key := restoreKey(t, b)
	rec := postRestore(t, b, key, file, "")
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), "pin_needed") || !strings.Contains(rec.Body.String(), "Ada") {
		t.Fatalf("no PIN asked: %d %s", rec.Code, rec.Body)
	}
	if !b.setupPending() {
		t.Fatal("a refused restore wrote users")
	}
	restored(t, postRestore(t, b, key, file, "97531"))
	signInToken(t, b, "Ada", "97531")
	if u, _ := b.r().cfg.FindUserByName("Ana"); !u.PinNeeded {
		t.Fatalf("Ana arrived with a PIN: %+v", u)
	}
}

// The router is not cabled yet. The file's names table checks the references, so nothing arrives Broken. It
// becomes the saved names table of the file's router.
func TestRestoreRouterOfflineImportsNothingBroken(t *testing.T) {
	src, file := backupOf(t, true)
	b := setupApp(t)
	_, key := restoreKey(t, b)
	res := restored(t, postRestore(t, b, key, file, ""))
	for name, c := range res.Plan.Categories {
		if c.Refused != "" {
			t.Errorf("%s refused: %s", name, c.Refused)
		}
		for _, it := range c.Items {
			if len(it.Broken) > 0 {
				t.Errorf("%s %s arrived Broken: %v", name, it.Name, it.Broken)
			}
		}
	}
	want, cfg := src.r().cfg, b.r().cfg
	if cfg.UID != want.UID || cfg.Prefix != want.Prefix || cfg.Title != want.Title || cfg.InstallUID != want.InstallUID {
		t.Fatalf("router %s %s %q install %s, want %s %s %q install %s", cfg.UID, cfg.Prefix, cfg.Title, cfg.InstallUID,
			want.UID, want.Prefix, want.Title, want.InstallUID)
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(b.cfgPath), namesDir, want.UID+".yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var table namesTable
	if err := yaml.Unmarshal(raw, &table); err != nil || tableMap(table.Sources)[2] != "CAM 2" || tableMap(table.Destinations)[4] != "MON 4" {
		t.Fatalf("saved names table: %v %+v", err, table)
	}
	if n, _ := b.liveNames(); n[2] != "CAM 2" {
		t.Fatalf("live names before the router answers: %v", n)
	}
}

// Only the browser that runs setup restores, and only while setup is pending.
func TestRestoreNeedsTheSetupOwnersKey(t *testing.T) {
	_, file := backupOf(t, true)
	b := setupApp(t)
	if rec := postRestore(t, b, "", file, ""); rec.Code != 403 {
		t.Fatalf("no key: %d %s", rec.Code, rec.Body)
	}
	_, key := restoreKey(t, b)
	other := &client{ip: "10.0.0.6", send: make(chan []byte, 4096)}
	register(b, other)
	if err := b.setupClaim(other, true); err != nil {
		t.Fatal(err)
	}
	if rec := postRestore(t, b, key, file, ""); rec.Code != 403 {
		t.Fatalf("key after Take control: %d %s", rec.Code, rec.Body)
	}
	key2, err := b.setupRestoreKey(other)
	if err != nil {
		t.Fatal(err)
	}
	restored(t, postRestore(t, b, key2, file, ""))
	if rec := postRestore(t, b, key2, file, ""); rec.Code != 409 {
		t.Fatalf("second restore: %d %s", rec.Code, rec.Body)
	}
}

func slicesSortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
