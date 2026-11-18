package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
)

func mkUser(t *testing.T, name, pin string) config.User {
	t.Helper()
	h, err := config.HashPin(pin)
	if err != nil {
		t.Fatal(err)
	}
	return config.User{Name: name, Pin: h, Len: len(pin)}
}

// settledTempDir is t.TempDir for a directory. It is possible that the app still writes to it.
// tokens.json is saved off the lock, so a save can land at the moment the test
// ends. That makes a plain RemoveAll fail with "directory not empty", which
// failed the release image build once. Removal is retried briefly.
func settledTempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "sode-test-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		for i := 0; i < 50; i++ {
			if os.RemoveAll(dir) == nil {
				return
			}
			time.Sleep(20 * time.Millisecond)
		}
	})
	return dir
}

// userApp is an app with Standard users Ana (1234) and Bo (567890), the Administrator Ada
// (2468) and, if locks, the main and log locks on. The Advanced Settings PIN is 2468 too: it
// opens nothing, and the PIN pool tests use it.
func userApp(t *testing.T, locks bool) *App {
	t.Helper()
	cfgPath := filepath.Join(settledTempDir(t), "config.yaml")
	cfg := config.Defaults()
	cfg.Normalize()
	cfg.Users = []config.User{mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890")}
	withAdministrator(&cfg)
	cfg.SettingsPin = mkUser(t, "", "2468").Pin
	cfg.Locks = config.Locks{Main: locks, Log: locks}
	hist, _ := OpenHistory("")
	sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, cfgPath, hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestUsersConfigValidation(t *testing.T) {
	good := mkUser(t, "Ana", "1234")
	cfg := config.Defaults()
	cfg.Normalize()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("default config (no users, no locks): %v", err)
	}
	for name, mut := range map[string]func(*config.Config){
		"lock without users": func(c *config.Config) { c.Locks.Main = true },
		"duplicate name":     func(c *config.Config) { c.Users = []config.User{good, {Name: "ANA", Pin: good.Pin, Len: 4}} },
		"empty name":         func(c *config.Config) { c.Users = []config.User{{Pin: good.Pin, Len: 4}} },
		"hand-typed PIN":     func(c *config.Config) { c.Users = []config.User{{Name: "Ana", Pin: "1234", Len: 4}} },
		"too many": func(c *config.Config) {
			for i := range config.MaxUsers + 1 {
				c.Users = append(c.Users, config.User{Name: string(rune('a' + i)), Pin: good.Pin, Len: 4})
			}
		},
		"control character": func(c *config.Config) { c.Users = []config.User{{Name: "A\nB", Pin: good.Pin, Len: 4}} },
		"auto-lock range":   func(c *config.Config) { c.Users = []config.User{good}; c.AutoLockMinutes = 5000 },
	} {
		c := cfg.Clone()
		mut(&c)
		c.Normalize()
		if c.Validate() == nil {
			t.Errorf("%s accepted", name)
		}
	}
	c := cfg.Clone()
	good.ID = config.NewUserID()
	c.Users, c.Locks, c.AutoLockMinutes = []config.User{good}, config.Locks{Main: true}, 5
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("a lock needs a user, and no Advanced Settings PIN: %v", err)
	}
}

func TestUserPinRules(t *testing.T) {
	for pin, ok := range map[string]bool{"1234": true, "123456789012": true, "123": false, "1234567890123": false, "12a4": false, "": false} {
		if (config.CheckUserPinRules(pin, 12) == nil) != ok {
			t.Errorf("CheckUserPinRules(%q) ok=%v, want %v", pin, !ok, ok)
		}
	}
}

// Nothing configured: nothing is ever refused, nothing changes.
func TestLocksOffByDefault(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.1"}
	for _, g := range []gate{gateMain, gateLog} {
		if !a.pageAllowed(c, g) {
			t.Errorf("gate %d refused with no locks", g)
		}
	}
	if c.who() != "10.0.0.1" {
		t.Fatalf("who = %q", c.who())
	}
	a = userApp(t, false)
	if !a.pageAllowed(c, gateMain) || !a.pageAllowed(c, gateLog) {
		t.Fatal("users alone locked something")
	}
	if err := a.UserUnlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(c, a.r().cfg, "", false); err != nil {
		t.Fatal(err)
	}
}

func TestUserUnlock(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if a.pageAllowed(c, gateMain) || a.pageAllowed(c, gateLog) {
		t.Fatal("main and log should be locked")
	}
	if err := a.admit(c, policy["take"]); !errors.Is(err, errPageLocked) {
		t.Fatalf("batch while locked: %v", err)
	}
	if err := a.UserUnlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(c, a.r().cfg, "", false); err != nil {
		t.Fatalf("save as the Administrator: %v", err)
	}
	if err := a.SaveFilterSets(c, nil); err != nil {
		t.Fatalf("filter sets as the Administrator: %v", err)
	}
	if err := a.admit(c, policy["config"]); err != nil {
		t.Fatalf("config as the Administrator: %v", err)
	}
	if err := a.UserUnlockAs(c, "Bo", "12345"); err == nil || userPinCount(a, "Bo") != 1 {
		t.Fatalf("wrong-length PIN: %v, n=%d", err, userPinCount(a, "Bo"))
	}
	if err := a.UserUnlockAs(c, "Bo", "4321"); err == nil || userPinCount(a, "Bo") != 2 {
		t.Fatalf("wrong PIN: %v, n=%d", err, userPinCount(a, "Bo"))
	}
	if err := a.UserUnlockAs(c, "Bo", "567890"); err != nil {
		t.Fatalf("right PIN: %v", err)
	}
	if userPinCount(a, "Bo") != 0 || c.who() != "Bo" || !a.pageAllowed(c, gateMain) || !a.pageAllowed(c, gateLog) {
		t.Fatalf("not unlocked as Bo: %q", c.who())
	}
	if a.pageAllowed(&client{ip: "10.0.0.2"}, gateMain) {
		t.Fatal("unlock leaked to another connection")
	}
	a.SignOut(c)
	if a.pageAllowed(c, gateMain) || c.who() != "10.0.0.1" {
		t.Fatal("lock didn't lock")
	}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg.Clone()
	cfg.Users = cfg.Users[1:] // Ana goes
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if a.pageAllowed(c, gateMain) {
		t.Fatal("removed user still unlocked")
	}
}

func TestUserUnlockBackoff(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range pinFreeTries {
		if err := a.UserUnlockAs(c, "Ana", "0000"); err == nil || strings.Contains(err.Error(), "try again in") {
			t.Fatalf("free try: %v", err)
		}
	}
	err := a.UserUnlockAs(c, "Ana", "0000")
	if err == nil || !strings.Contains(err.Error(), "try again in") {
		t.Fatalf("5th wrong: %v", err)
	}
	if a.pin.n != 0 {
		t.Fatal("user failures counted against the Advanced Settings PIN")
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatalf("Advanced Settings PIN blocked by user PIN failures: %v", err)
	}
	if err := a.UserUnlockAs(c, "Ana", "1234"); err == nil || a.pageAllowed(c, gateMain) {
		t.Fatalf("right PIN during the wait: %v", err)
	}
	if userPinCount(a, "Ana") != pinFreeTries+1 {
		t.Fatalf("count %d", userPinCount(a, "Ana"))
	}
}

func TestSaveUsers(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.1"}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{Name: "Ana", Pin: "1234"}}}); !errors.Is(err, errNoAdministrator) {
		t.Fatalf("first users with no Administrator: %v", err)
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}}}); err == nil {
		t.Fatal("user without a PIN accepted")
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{Name: "Ana", Pin: "12ab"}}}); err == nil {
		t.Fatal("non-digit PIN accepted")
	}
	withLock := a.r().cfg
	withLock.Locks.Main = true
	if err := a.SaveSettings(c, withLock, "", false); err == nil || !strings.Contains(err.Error(), "user") {
		t.Fatalf("lock with no users: %v", err)
	}
	req := usersReq{Users: []userEdit{{Name: "Ana", Pin: "1234", Role: config.RoleAdministrator}, {Name: "Bo", Pin: "5678"}}}
	if err := saveUsers(a, c, req); err != nil {
		t.Fatal(err)
	}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	req.Users[0].ID, req.Users[0].Pin = idOf(a, "Ana"), ""
	cfg := a.r().cfg
	if len(cfg.Users) != 2 || cfg.Locks.Any() || cfg.Users[0].Len != 4 {
		t.Fatalf("saved %+v (SaveUsers must not touch the locks)", cfg)
	}
	withLock = a.r().cfg
	withLock.Locks.Log, withLock.AutoLockMinutes = true, 10
	if err := a.SaveSettings(c, withLock, "", false); err != nil {
		t.Fatal(err)
	}
	if err := saveUsers(a, c, req); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg; !got.Locks.Log || got.AutoLockMinutes != 10 {
		t.Fatalf("SaveUsers changed the locks: %+v", got)
	}
	// Hashes never reach browsers.
	if b, _ := json.Marshal(cfg); strings.Contains(string(b), "pbkdf2") {
		t.Fatalf("a hash in the JSON: %s", b)
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{Name: "Ana", Pin: "1234"}, {Name: "Bo", Pin: "1234"}}}); err == nil {
		t.Fatal("two new users with one PIN")
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {Name: "Cy", Pin: "1234"}}}); err == nil {
		t.Fatal("new PIN equal to a kept user's")
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Role: config.RoleAdministrator}, {ID: idOf(a, "Bo"), Name: "Robert"}}}); err != nil {
		t.Fatal(err)
	}
	if u, ok := a.r().cfg.FindUserByName("robert"); !ok || !config.ValidPinHash(u.Pin) {
		t.Fatal("rename lost the PIN")
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {Name: "ana", Pin: "9999"}}}); err == nil {
		t.Fatal("names differing in case accepted")
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana2"}, {ID: idOf(a, "Ana"), Name: "Ana"}, {ID: idOf(a, "Robert"), Name: "Robert"}}}); err == nil {
		t.Fatal("two rows inherited one user's PIN")
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Robert"}, {ID: idOf(a, "Ana"), Name: "Ana"}}}); err == nil {
		t.Fatal("duplicate was accepted")
	}
	if err := saveUsers(a, c, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {Name: "Robert", Pin: "1234"}}}); err == nil || strings.Contains(err.Error(), "Ana") {
		t.Fatalf("colliding PIN: %v (must not name the other user)", err)
	}
	if err := pageSave(t, a, c, map[string]any{"users": nil}, "locks", "autoLockMinutes"); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg; len(got.Users) != 2 || !got.Locks.Log || got.AutoLockMinutes != 10 {
		t.Fatalf("settings save dropped users or locks: %+v", got)
	}
	a.SignOut(c)
	if err := saveUsers(a, c, req); !isRoleErr(err) {
		t.Fatalf("users saved by a Guest: %v", err)
	}
}

// The server refuses over a real WebSocket, and says why.
func TestLockOverWebSocket(t *testing.T) {
	a := userApp(t, true)
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
	send(map[string]any{"t": "hi", "page": "main"})
	if m := until("lockState"); m["locked"] != true || m["enforced"] != true {
		t.Fatalf("lockState %v", m)
	}
	items := []map[string]any{{"src": 1, "dst": 1, "levels": []int{1}}}
	send(map[string]any{"t": "take", "items": items})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("batch not refused: %v", m)
	}
	send(map[string]any{"t": "config", "config": a.r().cfg})
	until("configResult")
	if cur := a.r().cfg; len(cur.Users) != 3 {
		t.Fatal("config changed users")
	}
	send(map[string]any{"t": "clearHistory"})
	send(map[string]any{"t": "log", "before": -1, "n": 10})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("clearHistory not refused: %v", m)
	}
	send(map[string]any{"t": "userSignIn", "user": "Ana", "pin": "123", "id": 1})
	if m := until("userSignInResult"); m["ok"] == true {
		t.Fatalf("short PIN: %v", m)
	}
	send(map[string]any{"t": "userSignIn", "user": "Ana", "pin": "1234", "id": 2})
	if m := until("lockState"); m["locked"] != false || m["user"] != "Ana" {
		t.Fatalf("after unlock: %v", m)
	}
	if m := until("userSignInResult"); m["ok"] != true {
		t.Fatalf("unlock: %v", m)
	}
	send(map[string]any{"t": "log", "before": -1, "n": 10})
	until("logPage")
	send(map[string]any{"t": "userLock"})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("after lock: %v", m)
	}
	send(map[string]any{"t": "log", "before": -1, "n": 10})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("sent not refused after lock: %v", m)
	}
}

// UserUnlock signs in as the one user whose PIN this is, so tests need not name them. Test PINs
// are unique; with no match or several, the name is left empty and the try is a wrong one.
func (a *App) UserUnlock(c *client, pin string) error {
	name := ""
	for _, u := range a.r().cfg.Users {
		if u.Len == len(pin) && config.CheckPin(u.Pin, pin) {
			if name != "" {
				name = ""
				break
			}
			name = u.Name
		}
	}
	if err := a.UserUnlockAs(c, name, pin); err != nil {
		return err
	}
	grantRecheck(a, c) // tests of the PIN recheck sign in over the socket (recheck_test.go)
	return nil
}
