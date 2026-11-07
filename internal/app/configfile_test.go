package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

func adminClient(t *testing.T, a *App) *client {
	t.Helper()
	c := &client{ip: "10.0.0.9"}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	return c
}

func exportOf(t *testing.T, a *App, pins bool) (string, config.Config) {
	t.Helper()
	_, text, err := a.ExportConfig(adminClient(t, a), pins)
	if err != nil {
		t.Fatal(err)
	}
	c, err := config.Parse([]byte(text))
	if err != nil {
		t.Fatalf("export doesn't load: %v\n%s", err, text)
	}
	return text, c
}

func TestConfigExportPins(t *testing.T) {
	a := userApp(t, false)
	text, c := exportOf(t, a, false)
	if c.SettingsPin != "" || strings.Contains(text, "pbkdf2") {
		t.Fatalf("PIN leaked into a plain export:\n%s", text)
	}
	for _, u := range c.Users {
		if u.Pin != "" {
			t.Fatalf("user %s hash in a plain export", u.Name)
		}
	}
	if !strings.HasPrefix(text, "# Sode server configuration") {
		t.Fatalf("no header: %q", text[:40])
	}
	text, c = exportOf(t, a, true)
	live := a.r().cfg
	if c.SettingsPin != live.SettingsPin || c.Users[0].Pin != live.Users[0].Pin || c.Users[1].Pin != live.Users[1].Pin {
		t.Fatal("a private backup lost a hash")
	}
	if !strings.Contains(text, "PRIVATE BACKUP") {
		t.Fatal("private backup is not marked")
	}
	if a.r().cfg.SettingsPin == "" || a.r().cfg.Users[0].Pin == "" {
		t.Fatal("export blanked the live config")
	}
}

// A file without PINs keeps the live hashes (and lengths) by user name.
func TestConfigImportKeepsLivePins(t *testing.T) {
	a := userApp(t, false)
	text, _ := exportOf(t, a, false)
	live := a.r().cfg
	// Edit: Bo's file length is a lie that must not be believed; Ana renamed
	// in case only; a setting changes.
	file, _ := config.Parse([]byte(text))
	file.BrandText = "Studio B"
	file.Users[0].Name = "ANA"
	file.Users[1].Len = 4
	b, _ := yaml.Marshal(file)
	c := adminClient(t, a)
	if err := a.ImportConfig(c, string(b)); err != nil {
		t.Fatal(err)
	}
	got := a.r().cfg
	if got.BrandText != "Studio B" {
		t.Fatal("setting not imported")
	}
	if got.SettingsPin != live.SettingsPin {
		t.Fatal("Advanced Settings PIN not kept")
	}
	if got.Users[0].Pin != live.Users[0].Pin || got.Users[0].Len != 4 {
		t.Fatalf("Ana: %+v", got.Users[0])
	}
	if got.Users[1].Pin != live.Users[1].Pin || got.Users[1].Len != 6 {
		t.Fatalf("Bo's length came from the file, not with its hash: %+v", got.Users[1])
	}
}

func TestConfigImportNewUserWithoutPin(t *testing.T) {
	a := userApp(t, false)
	text, _ := exportOf(t, a, false)
	file, _ := config.Parse([]byte(text))
	file.Users = append(file.Users, config.User{Name: "Cy", Len: 5})
	b, _ := yaml.Marshal(file)
	before := a.r().cfg
	err := a.ImportConfig(adminClient(t, a), string(b))
	if err == nil || !strings.Contains(err.Error(), `"Cy"`) {
		t.Fatalf("a user with no PIN to keep: %v", err)
	}
	if len(a.r().cfg.Users) != len(before.Users) {
		t.Fatal("config changed by a refused import")
	}
	// Two PIN-less users never clash: with hashes from the file they are fine.
	file.Users[2] = mkUser(t, "Cy", "55555")
	b, _ = yaml.Marshal(file)
	if err := a.ImportConfig(adminClient(t, a), string(b)); err != nil {
		t.Fatal(err)
	}
	if u, ok := a.r().cfg.FindUser("Cy"); !ok || u.Len != 5 {
		t.Fatalf("Cy: %+v", u)
	}
}

func TestConfigImportProfileRoundTrip(t *testing.T) {
	a := userApp(t, false)
	err := a.saveShared(adminClient(t, a), func(c *config.Config) error {
		c.Users[0].Profile = config.Profile{Prefs: map[string]string{"theme": "dark"}}
		return nil
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	text, _ := exportOf(t, a, true)
	a.saveShared(adminClient(t, a), func(c *config.Config) error { c.Users[0].Profile = config.Profile{}; return nil }, nil)
	if err := a.ImportConfig(adminClient(t, a), text); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg.Users[0].Profile.Prefs["theme"]; got != "dark" {
		t.Fatalf("profile after import: %q", got)
	}
	if c, _, err := config.Load(a.cfgPath); err != nil || c.Users[0].Profile.Prefs["theme"] != "dark" {
		t.Fatalf("saved file: %v %+v", err, c.Users)
	}
}

func TestConfigImportRefused(t *testing.T) {
	a := userApp(t, false)
	text, _ := exportOf(t, a, true)
	live := a.r().cfg
	admin := adminClient(t, a)
	for name, in := range map[string]string{
		"empty":        "  \n# nothing\n",
		"unknown key":  text + "\nnonsense: 1\n",
		"not yaml":     "a: [",
		"hand PIN":     strings.Replace(text, "settingsPin: ", "settingsPin: 1234 #", 1),
		"bad user pin": strings.Replace(text, "pin: pbkdf2", "pin: xx", 1),
		"lock no pin":  "locks:\n  main: true\n",
		"too big":      "brandText: " + strings.Repeat("x", maxConfigFile),
	} {
		if err := a.ImportConfig(admin, in); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if got := a.r().cfg; got.SettingsPin != live.SettingsPin || len(got.Users) != 2 || got.BrandText != live.BrandText {
		t.Fatal("a refused import changed the config")
	}
	file, _ := config.Parse([]byte(text))
	file.Locks.Main = true
	b, _ := yaml.Marshal(file)
	if err := a.ImportConfig(admin, string(b)); err != nil {
		t.Fatalf("lock with users and PIN: %v", err)
	}
	file.Users = nil
	b, _ = yaml.Marshal(file)
	if err := a.ImportConfig(admin, string(b)); err == nil || !strings.Contains(err.Error(), "user") {
		t.Fatalf("lock with no user: %v", err)
	}
}

func TestConfigFileNeedsSettingsPin(t *testing.T) {
	a := userApp(t, false)
	cfg := a.r().cfg.Clone()
	cfg.Users[0].Trusted = true
	if err := a.saveShared(adminClient(t, a), func(c *config.Config) error { *c = cfg; return nil }, nil); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	for _, typ := range []string{"configExport", "configImportBegin", "configImport"} {
		if err := a.admit(c, policy[typ]); !errors.Is(err, errLocked) {
			t.Errorf("%s for a trusted user: %v", typ, err)
		}
	}
	text, _ := exportOf(t, a, true)
	if err := a.ImportConfig(c, text); !errors.Is(err, errLocked) {
		t.Fatalf("import as a trusted user: %v", err)
	}
}

func TestReadBudget(t *testing.T) {
	var limit int64
	b := newReadBudget(func(n int64) { limit = n })
	if limit != wsReadLimit {
		t.Fatalf("start %d", limit)
	}
	if b.Grant(0) || b.Grant(maxConfigFile+1) || limit != wsReadLimit {
		t.Fatal("a bad size changed the limit")
	}
	if !b.Grant(1000) || limit != 1000*2+importSlack {
		t.Fatalf("granted %d", limit)
	}
	b.Reset()
	if limit != wsReadLimit {
		t.Fatalf("not restored: %d", limit)
	}
	b.Reset() // harmless
}

func TestImportReadLimit(t *testing.T) {
	for _, n := range []int{0, -1, maxConfigFile + 1} {
		if _, ok := importReadLimit(n); ok {
			t.Errorf("size %d accepted", n)
		}
	}
	if l, ok := importReadLimit(maxConfigFile); !ok || l > 5<<20 || l < maxConfigFile {
		t.Errorf("limit for the largest file: %d %v", l, ok)
	}
}

// The big read is for a settings-PIN connection that asks, once. Everyone
// else keeps 64 KiB, and the limit is back after the one message.
func TestConfigImportReadLimitOverWebSocket(t *testing.T) {
	a := userApp(t, false)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dial := func() (*websocket.Conn, func(map[string]any), func(string) map[string]any) {
		ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
		if err != nil {
			t.Fatal(err)
		}
		ws.SetReadLimit(1 << 24)
		send := func(m map[string]any) {
			b, _ := json.Marshal(m)
			if err := ws.Write(ctx, websocket.MessageText, b); err != nil {
				t.Fatal(err)
			}
		}
		until := func(typ string) map[string]any {
			for {
				_, b, err := ws.Read(ctx)
				if err != nil {
					t.Fatalf("waiting for %s: %v", typ, err)
				}
				var m map[string]any
				json.Unmarshal(b, &m)
				if m["t"] == typ {
					return m
				}
			}
		}
		return ws, send, until
	}
	big := "# " + strings.Repeat("x", 200<<10) + "\nbrandText: Big\n"
	// An oversize message is cut off by the server mid-write, so the client
	// may see the 1009 close or, if the reset wins, a failed write or read.
	// Either way the connection ends; a completed write must show the 1009.
	tooBig := func(ws *websocket.Conn, m map[string]any, what string) {
		t.Helper()
		b, _ := json.Marshal(m)
		werr := ws.Write(ctx, websocket.MessageText, b)
		_, _, rerr := ws.Read(ctx)
		if rerr == nil || (werr == nil && websocket.CloseStatus(rerr) != websocket.StatusMessageTooBig) {
			t.Fatalf("%s: write %v, read %v", what, werr, rerr)
		}
	}

	ws, send, until := dial()
	send(map[string]any{"t": "configImportBegin", "id": 1, "size": len(big)})
	if m := until("configImportBeginResult"); m["ok"] == true || m["locked"] != true || m["id"] != float64(1) {
		t.Fatalf("begin without the PIN: %v", m)
	}
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 9})
	until("unlockResult")
	tooBig(ws, map[string]any{"t": "configImport", "id": 2, "yaml": big}, "a connection without an accepted begin")
	if a.r().cfg.BrandText == "Big" {
		t.Fatal("imported without the PIN")
	}

	ws, send, until = dial()
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 1})
	until("unlockResult")
	tooBig(ws, map[string]any{"t": "configImport", "id": 2, "yaml": big}, "no begin, big message")

	ws, send, until = dial()
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 1})
	until("unlockResult")
	send(map[string]any{"t": "configImportBegin", "id": 3, "size": len(big)})
	if m := until("configImportBeginResult"); m["ok"] != true || m["id"] != float64(3) {
		t.Fatalf("begin: %v", m)
	}
	send(map[string]any{"t": "configImport", "id": 4, "yaml": big})
	if m := until("configImportResult"); m["ok"] != true || m["id"] != float64(4) {
		t.Fatalf("import: %v", m)
	}
	if a.r().cfg.BrandText != "Big" {
		t.Fatal("not applied")
	}
	tooBig(ws, map[string]any{"t": "configImport", "id": 5, "yaml": big}, "limit not restored")
	_, send, until = dial()
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 1})
	until("unlockResult")
	send(map[string]any{"t": "configImportBegin", "id": 6, "size": maxConfigFile + 1})
	if m := until("configImportBeginResult"); m["ok"] == true || m["id"] != float64(6) {
		t.Fatalf("oversize begin: %v", m)
	}
}

func TestConfigExportOverWebSocket(t *testing.T) {
	a := userApp(t, false)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 24)
	send := func(m map[string]any) {
		b, _ := json.Marshal(m)
		ws.Write(ctx, websocket.MessageText, b)
	}
	until := func(typ string) map[string]any {
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				t.Fatalf("waiting for %s: %v", typ, err)
			}
			var m map[string]any
			json.Unmarshal(b, &m)
			if m["t"] == typ {
				return m
			}
		}
	}
	send(map[string]any{"t": "configExport", "id": 1, "pins": true})
	if m := until("configExportResult"); m["ok"] == true || m["yaml"] != nil {
		t.Fatalf("export without the PIN: %v", m)
	}
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 2})
	until("unlockResult")
	send(map[string]any{"t": "configExport", "id": 3, "pins": true})
	m := until("configExportResult")
	if m["ok"] != true || m["id"] != float64(3) || !strings.Contains(m["yaml"].(string), "PRIVATE BACKUP") || m["name"] == "" {
		t.Fatalf("export: %v", m["ok"])
	}
}

// The export re-checks the Advanced Settings PIN when it runs: a lock (or a changed
// PIN) between admit and the export refuses it and returns no YAML.
func TestConfigExportRechecksPin(t *testing.T) {
	a := userApp(t, false)
	c := adminClient(t, a)
	if err := a.admit(c, policy["configExport"]); err != nil {
		t.Fatal(err)
	}
	a.Lock(c)
	_, text, err := a.ExportConfig(c, true)
	if !errors.Is(err, errLocked) || text != "" {
		t.Fatalf("export after lock: %q %v", text, err)
	}
	if _, text, err := a.ExportConfig(adminClient(t, a), true); err != nil || text == "" {
		t.Fatalf("unlocked export: %v", err)
	}
}

// A blank install (no Advanced Settings PIN, no users) importing a multiuser file that
// carries no PINs (an export without PINs): refused with a plain message, the
// live config untouched, nothing half-applied. A file with no users, or with
// PIN hashes, goes in.
func TestConfigImportIntoBlankInstall(t *testing.T) {
	src := userApp(t, true) // users, locks on, an Advanced Settings PIN
	text, _ := exportOf(t, src, false)

	blank := func() *App {
		cfg := config.Defaults()
		cfg.Normalize()
		hist, _ := OpenHistory("")
		sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
		if err != nil {
			t.Fatal(err)
		}
		a, err := New(cfg, filepath.Join(settledTempDir(t), "config.yaml"), hist, sl, false)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}

	a := blank()
	err := a.ImportConfig(&client{ip: "10.0.0.9"}, text)
	if err == nil || !strings.Contains(err.Error(), "no PIN") {
		t.Fatalf("PIN-less multiuser file on a blank install: %v", err)
	}
	if len(a.r().cfg.Users) != 0 || a.r().cfg.Locks.Main {
		t.Fatalf("a refused import changed the config: %+v", a.r().cfg.Users)
	}

	full, _ := exportOf(t, src, true)
	if err := blank().ImportConfig(&client{ip: "10.0.0.9"}, full); err != nil {
		t.Fatalf("export with PINs: %v", err)
	}

	file, _ := config.Parse([]byte(text))
	file.Users, file.Locks = nil, config.Locks{}
	b, _ := yaml.Marshal(file)
	if err := blank().ImportConfig(&client{ip: "10.0.0.9"}, string(b)); err != nil {
		t.Fatalf("file without users: %v", err)
	}
}
