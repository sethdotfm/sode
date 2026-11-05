package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
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
	_, text, err := a.ExportConfig(pins)
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
	// The live config is untouched by exporting.
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
		t.Fatal("settings PIN not kept")
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
	// Wipe the profile, then restore from the file.
	a.saveShared(adminClient(t, a), func(c *config.Config) error { c.Users[0].Profile = config.Profile{}; return nil }, nil)
	if err := a.ImportConfig(adminClient(t, a), text); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg.Users[0].Profile.Prefs["theme"]; got != "dark" {
		t.Fatalf("profile after import: %q", got)
	}
	// And it was saved to disk as the loadable file.
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
	// A lock needs a user and a settings PIN: with the file's PIN-less
	// users the live PIN is kept, so a lock with users imports fine.
	file, _ := config.Parse([]byte(text))
	file.Locks.Main = true
	b, _ := yaml.Marshal(file)
	if err := a.ImportConfig(admin, string(b)); err != nil {
		t.Fatalf("lock with users and PIN: %v", err)
	}
	// ...but not with no users.
	file.Users = nil
	b, _ = yaml.Marshal(file)
	if err := a.ImportConfig(admin, string(b)); err == nil || !strings.Contains(err.Error(), "user") {
		t.Fatalf("lock with no user: %v", err)
	}
}

// Only the settings PIN: not a trusted user, not an unlocked user.
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

	// Not unlocked: begin is refused, and the big message closes the socket.
	ws, send, until := dial()
	send(map[string]any{"t": "configImportBegin", "id": 1, "size": len(big)})
	if m := until("configImportResult"); m["ok"] == true || m["locked"] != true {
		t.Fatalf("begin without the PIN: %v", m)
	}
	send(map[string]any{"t": "configImport", "id": 2, "yaml": big})
	if _, _, err := ws.Read(ctx); websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("a locked connection sent %d bytes: %v", len(big), err)
	}
	if a.r().cfg.BrandText == "Big" {
		t.Fatal("imported without the PIN")
	}

	// Unlocked and skipping the begin step: still 64 KiB.
	ws, send, until = dial()
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 1})
	until("unlockResult")
	send(map[string]any{"t": "configImport", "id": 2, "yaml": big})
	if _, _, err := ws.Read(ctx); websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("no begin, big message: %v", err)
	}

	// Unlocked, begin then the file: accepted. Then the limit is back.
	ws, send, until = dial()
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 1})
	until("unlockResult")
	send(map[string]any{"t": "configImportBegin", "id": 3, "size": len(big)})
	send(map[string]any{"t": "configImport", "id": 4, "yaml": big})
	if m := until("configImportResult"); m["ok"] != true || m["id"] != float64(4) {
		t.Fatalf("import: %v", m)
	}
	if a.r().cfg.BrandText != "Big" {
		t.Fatal("not applied")
	}
	// The PIN hashes were left out, so the connection is still unlocked and a
	// second big message with no new begin is refused by the limit again.
	send(map[string]any{"t": "configImport", "id": 5, "yaml": big})
	if _, _, err := ws.Read(ctx); websocket.CloseStatus(err) != websocket.StatusMessageTooBig {
		t.Fatalf("limit not restored: %v", err)
	}
	// An oversize claim is answered, and widens nothing.
	_, send, until = dial()
	send(map[string]any{"t": "unlock", "pin": "2468", "id": 1})
	until("unlockResult")
	send(map[string]any{"t": "configImportBegin", "id": 6, "size": maxConfigFile + 1})
	if m := until("configImportResult"); m["ok"] == true || m["id"] != float64(6) {
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
