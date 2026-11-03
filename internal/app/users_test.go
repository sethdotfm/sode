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

// userApp is an app with users ana (1234) and bo (567890) and, if locks, the
// main and log locks on.
func userApp(t *testing.T, locks bool) *App {
	t.Helper()
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	cfg := config.Defaults()
	cfg.Normalize()
	cfg.Users = []config.User{mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890")}
	cfg.Locks = config.Locks{Main: locks, Log: locks}
	cfg.SettingsPin = mkUser(t, "", "2468").Pin
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
	c.Users, c.Locks, c.AutoLockMinutes = []config.User{good}, config.Locks{Main: true}, 5
	if c.Validate() == nil {
		t.Fatal("lock without a settings PIN accepted")
	}
	c.SettingsPin = good.Pin
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestUserPinRules(t *testing.T) {
	for pin, ok := range map[string]bool{"1234": true, "123456789012": true, "123": false, "1234567890123": false, "12a4": false, "": false} {
		if (config.CheckUserPinRules(pin) == nil) != ok {
			t.Errorf("CheckUserPinRules(%q) ok=%v, want %v", pin, !ok, ok)
		}
	}
}

// Nothing configured: nothing is ever refused, nothing changes.
func TestLocksOffByDefault(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.1"}
	for _, g := range []gate{gateMain, gateGrid, gateLog} {
		if !a.pageAllowed(c, g) {
			t.Errorf("gate %d refused with no locks", g)
		}
	}
	if c.who() != "10.0.0.1" {
		t.Fatalf("who = %q", c.who())
	}
	// Users without locks still don't gate anything.
	a = userApp(t, false)
	if !a.pageAllowed(c, gateMain) || !a.pageAllowed(c, gateLog) {
		t.Fatal("users alone locked something")
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(c, a.r().cfg, "", false); err != nil {
		t.Fatal(err)
	}
}

func TestUserUnlock(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if a.pageAllowed(c, gateMain) || a.pageAllowed(c, gateLog) || !a.pageAllowed(c, gateGrid) {
		t.Fatal("main and log should be locked, grid not")
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	// The settings PIN is the admin credential: with it unlocked, saving
	// needs no user, but the main lock still stops actions.
	if err := a.SaveSettings(c, a.r().cfg, "", false); err != nil {
		t.Fatalf("save with the settings PIN, locked as a user: %v", err)
	}
	if err := a.SaveFilterSets(c, nil); err != nil {
		t.Fatalf("filter sets with the settings PIN: %v", err)
	}
	if err := a.admit(c, policy["batch"]); !errors.Is(err, errPageLocked) {
		t.Fatalf("batch while locked: %v", err)
	}
	if err := a.admit(c, policy["config"]); err != nil {
		t.Fatalf("config with the settings PIN: %v", err)
	}
	// A PIN of a length no user has is an ordinary wrong try.
	if err := a.UserUnlock(c, "12345"); err == nil || a.userPin.n != 1 {
		t.Fatalf("wrong-length PIN: %v, n=%d", err, a.userPin.n)
	}
	if err := a.UserUnlock(c, "4321"); err == nil || a.userPin.n != 2 {
		t.Fatalf("wrong PIN: %v, n=%d", err, a.userPin.n)
	}
	if err := a.UserUnlock(c, "567890"); err != nil {
		t.Fatalf("right PIN: %v", err)
	}
	if a.userPin.n != 0 || c.who() != "Bo" || !a.pageAllowed(c, gateMain) || !a.pageAllowed(c, gateLog) {
		t.Fatalf("not unlocked as Bo: %q", c.who())
	}
	// A different connection stays locked.
	if a.pageAllowed(&client{ip: "10.0.0.2"}, gateMain) {
		t.Fatal("unlock leaked to another connection")
	}
	a.UserLock(c, false)
	if a.pageAllowed(c, gateMain) || c.who() != "10.0.0.1" {
		t.Fatal("lock didn't lock")
	}
	// A user removed or given a new PIN relocks the connection.
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg.Clone()
	cfg.Users = []config.User{mkUser(t, "Bo", "567890")}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if a.pageAllowed(c, gateMain) {
		t.Fatal("removed user still unlocked")
	}
}

// Wrong user PINs share the settings PIN's count and backoff.
func TestUserUnlockBackoff(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for range pinFreeTries {
		if err := a.UserUnlock(c, "0000"); err == nil || strings.Contains(err.Error(), "try again in") {
			t.Fatalf("free try: %v", err)
		}
	}
	err := a.UserUnlock(c, "0000")
	if err == nil || !strings.Contains(err.Error(), "try again in") {
		t.Fatalf("5th wrong: %v", err)
	}
	// Even the right PIN waits, and a wrong settings PIN counts too.
	if a.pin.n != 0 {
		t.Fatal("user failures counted against the settings PIN")
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatalf("settings PIN blocked by user PIN failures: %v", err)
	}
	if err := a.UserUnlock(c, "1234"); err == nil || a.pageAllowed(c, gateMain) {
		t.Fatalf("right PIN during the wait: %v", err)
	}
	if a.userPin.n != pinFreeTries+1 {
		t.Fatalf("count %d", a.userPin.n)
	}
}

func TestAdminTokenAndRevoke(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	a.UserLock(c, false)
	other := &client{ip: "10.0.0.5"}
	if _, err := a.AdminUnlock(other, "0000"); err == nil || a.pageAllowed(other, gateMain) {
		t.Fatalf("wrong admin password: %v", err)
	}
	a.mu.Lock()
	a.pin.until = time.Time{}
	a.mu.Unlock()
	tok, err := a.AdminUnlock(other, "2468")
	if err != nil || tok == "" {
		t.Fatalf("admin unlock: %v", err)
	}
	if !a.pageAllowed(other, gateMain) || !a.pageAllowed(other, gateLog) {
		t.Fatal("token didn't lift the locks")
	}
	// Another connection of the same browser presents the token with hi.
	again := &client{ip: "10.0.0.5"}
	a.Hi(again, "log", tok)
	if !a.pageAllowed(again, gateLog) {
		t.Fatal("token not honored on a new connection")
	}
	if a.pageAllowed(&client{ip: "10.0.0.5", u: connUser{token: "bogus"}}, gateMain) {
		t.Fatal("a made-up token worked")
	}
	// The token is on disk (hashed), and survives a restart of the state.
	b, err := os.ReadFile(a.tokens.path)
	if err != nil || strings.Contains(string(b), tok) {
		t.Fatalf("token file: %v / plaintext token: %v", err, strings.Contains(string(b), tok))
	}
	a.tokens = loadTokens(a.cfgPath) // the load at startup
	if !a.tokens.ok(tok, a.r().cfg.SettingsPin) {
		t.Fatal("token lost with the in-memory state")
	}
	// Changing the settings PIN revokes every token.
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.UserUnlock(admin, "1234"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(admin, a.r().cfg, "1357", false); err != nil {
		t.Fatal(err)
	}
	if a.pageAllowed(other, gateMain) || a.pageAllowed(again, gateLog) {
		t.Fatal("old token still works after the PIN changed")
	}
	if a.tokens.ok(tok, a.r().cfg.SettingsPin) {
		t.Fatal("token valid for the new PIN")
	}
	// And so does removing it.
	tok2, err := a.AdminUnlock(other, "1357")
	if err != nil || !a.pageAllowed(other, gateMain) {
		t.Fatalf("new PIN admin unlock: %v", err)
	}
	// Issuing a token drops the ones the old PIN made.
	if len(a.tokens.toks) != 1 {
		t.Fatalf("%d tokens after the PIN changed and one was issued", len(a.tokens.toks))
	}
	if err := a.Unlock(admin, "1357"); err != nil {
		t.Fatal(err)
	}
	// Locks need the PIN, so they go first.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveSettings(admin, a.r().cfg, "", true); err != nil {
		t.Fatal(err)
	}
	if a.tokens.ok(tok2, "") || a.tokens.ok(tok2, a.r().cfg.SettingsPin) {
		t.Fatal("token survives removing the PIN")
	}
}

func TestSaveUsers(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.1"}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana", Pin: "1234"}}}); !errors.Is(err, errNeedAdminPin) {
		t.Fatalf("users saved with no settings PIN: %v", err)
	}
	if err := a.SaveSettings(c, a.r().cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana"}}}); err == nil {
		t.Fatal("user without a PIN accepted")
	}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana", Pin: "12ab"}}}); err == nil {
		t.Fatal("non-digit PIN accepted")
	}
	if err := a.SaveUsers(c, usersReq{Locks: config.Locks{Main: true}}); err == nil {
		t.Fatal("lock with no users accepted")
	}
	req := usersReq{Users: []userEdit{{Name: "Ana", Pin: "1234"}, {Name: "Bo", Pin: "5678"}}, Locks: config.Locks{Log: true}, AutoLock: 10}
	if err := a.SaveUsers(c, req); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	if len(cfg.Users) != 2 || !cfg.Locks.Log || cfg.AutoLockMinutes != 10 || cfg.Users[0].Len != 4 {
		t.Fatalf("saved %+v", cfg)
	}
	// Hashes never reach browsers.
	if b, _ := json.Marshal(cfg); strings.Contains(string(b), "pbkdf2") {
		t.Fatalf("a hash in the JSON: %s", b)
	}
	// Same PIN twice, new or against a kept user.
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana", Pin: "1234"}, {Name: "Bo", Pin: "1234"}}}); err == nil {
		t.Fatal("two new users with one PIN")
	}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Cy", Pin: "1234"}}}); err == nil {
		t.Fatal("new PIN equal to a kept user's")
	}
	// Keeping, renaming and a name differing only in case.
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Robert", Was: "Bo"}}, Locks: req.Locks, AutoLock: 10}); err != nil {
		t.Fatal(err)
	}
	if u, ok := a.r().cfg.FindUser("robert"); !ok || !config.ValidPinHash(u.Pin) {
		t.Fatal("rename lost the PIN")
	}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "ana", Pin: "9999"}}}); err == nil {
		t.Fatal("names differing in case accepted")
	}
	// Renaming X to inherit Y's hash while Y stays, or two rows keeping one.
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana2", Was: "Ana"}, {Name: "Ana"}, {Name: "Robert"}}, Locks: req.Locks, AutoLock: 10}); err == nil {
		t.Fatal("two rows inherited one user's PIN")
	}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Robert", Was: "Ana"}, {Name: "Ana", Was: "Ana"}}, Locks: req.Locks, AutoLock: 10}); err == nil {
		t.Fatal("duplicate was accepted")
	}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Robert", Pin: "1234"}}, Locks: req.Locks, AutoLock: 10}); err == nil || strings.Contains(err.Error(), "Ana") {
		t.Fatalf("colliding PIN: %v (must not name the other user)", err)
	}
	// A settings save from a browser keeps users, locks and auto-lock.
	stale := a.r().cfg
	stale.Users, stale.Locks, stale.AutoLockMinutes = nil, config.Locks{}, 0
	if err := a.SaveSettings(c, stale, "", false); err != nil {
		t.Fatal(err)
	}
	if got := a.r().cfg; len(got.Users) != 2 || !got.Locks.Log || got.AutoLockMinutes != 10 {
		t.Fatalf("settings save dropped users or locks: %+v", got)
	}
	// Saving users needs the settings PIN unlocked.
	a.Lock(c)
	if err := a.SaveUsers(c, req); !errors.Is(err, errLocked) {
		t.Fatalf("users saved without the settings PIN: %v", err)
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
	// Reads until a message of type t, returning it.
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
	send(map[string]any{"t": "batch", "items": items})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("batch not refused: %v", m)
	}
	send(map[string]any{"t": "config", "config": a.r().cfg})
	until("configResult")
	if cur := a.r().cfg; len(cur.Users) != 2 {
		t.Fatal("config changed users")
	}
	send(map[string]any{"t": "clearHistory"})
	send(map[string]any{"t": "sent", "before": -1, "n": 10})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("clearHistory not refused: %v", m)
	}
	// A PIN of a length nobody has is just wrong; then the real one.
	send(map[string]any{"t": "userUnlock", "pin": "123", "id": 1})
	if m := until("userUnlockResult"); m["ok"] == true {
		t.Fatalf("short PIN: %v", m)
	}
	send(map[string]any{"t": "userUnlock", "pin": "1234", "id": 2})
	// The new lock state is queued ahead of the result.
	if m := until("lockState"); m["locked"] != false || m["user"] != "Ana" {
		t.Fatalf("after unlock: %v", m)
	}
	if m := until("userUnlockResult"); m["ok"] != true {
		t.Fatalf("unlock: %v", m)
	}
	send(map[string]any{"t": "sent", "before": -1, "n": 10})
	until("sentPage")
	send(map[string]any{"t": "userLock"})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("after lock: %v", m)
	}
	send(map[string]any{"t": "sent", "before": -1, "n": 10})
	if m := until("lockState"); m["locked"] != true {
		t.Fatalf("sent not refused after lock: %v", m)
	}
}
