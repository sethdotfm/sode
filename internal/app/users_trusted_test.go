package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"testing/fstest"
	"time"

	"errors"
	"github.com/coder/websocket"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// setFlags changes a user's Trusted/Persistent in the live config, the way a
// users save would (the PIN hash stays).
func setFlags(t *testing.T, a *App, name string, trusted, persistent bool) {
	t.Helper()
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	_, nr, err := a.update(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].Name == name {
				cfg.Users[i].Trusted, cfg.Users[i].Persistent = trusted, persistent
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.mu.Unlock()
}

// admitMsg runs a message through the same two steps as the read loop.
func admitMsg(a *App, c *client, m inMsg) error {
	return a.admit(c, policy[m.T].forMsg(&m))
}

func TestTrustedSavesWithoutPin(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, false)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	cfg.LevelColors = []string{"#aabbcc"}
	for _, m := range []inMsg{{T: "config"}, {T: "filterSets"}, {T: "presets"}} {
		if err := admitMsg(a, c, m); err != nil {
			t.Fatalf("%s refused a trusted user: %v", m.T, err)
		}
	}
	if err := a.SaveSettings(c, cfg, "", false); err != nil {
		t.Fatalf("settings save: %v", err)
	}
	if err := a.SaveFilterSets(c, []config.FilterSet{{Name: "X", Sode: true, Ext: true}}); err != nil {
		t.Fatalf("filter sets: %v", err)
	}
	if err := a.SavePresets(c, []config.Preset{{Name: "S", Items: []config.PresetItem{{Src: 1, Dsts: []int{1}, Levels: []int{1}}}}}); err != nil {
		t.Fatalf("salvos: %v", err)
	}
	if len(a.r().cfg.FilterSets) != 1 || len(a.r().cfg.Presets) != 1 {
		t.Fatal("saves did not land")
	}
}

func TestTrustedStillNeedsPinFor(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, false)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	for name, m := range map[string]inMsg{
		"clearHistory": {T: "clearHistory"},
		"users":        {T: "users"},
		"set PIN":      {T: "config", Pin: "9999"},
		"clear PIN":    {T: "config", ClearPin: true},
	} {
		if err := admitMsg(a, c, m); !errors.Is(err, errLocked) {
			t.Errorf("%s admitted for a trusted user: %v", name, err)
		}
	}
	if err := a.SaveUsers(c, usersReq{Users: []userEdit{{Name: "Ana"}}}); !errors.Is(err, errLocked) {
		t.Errorf("SaveUsers: %v", err)
	}
	if err := a.SaveSettings(c, a.r().cfg, "9999", false); !errors.Is(err, errLocked) {
		t.Errorf("setting the PIN: %v", err)
	}
	if err := a.SaveSettings(c, a.r().cfg, "", true); !errors.Is(err, errLocked) {
		t.Errorf("clearing the PIN: %v", err)
	}
	if a.r().cfg.SettingsPin == "" {
		t.Fatal("PIN gone")
	}
	if err := a.Unlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	for _, m := range []inMsg{{T: "clearHistory"}, {T: "users"}, {T: "config", Pin: "9999"}} {
		if err := admitMsg(a, c, m); err != nil {
			t.Errorf("%s with the PIN: %v", m.T, err)
		}
	}
}

func TestUntrustedAndLockedNeedPin(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	for _, m := range []inMsg{{T: "config"}, {T: "filterSets"}, {T: "presets"}} {
		if err := admitMsg(a, c, m); !errors.Is(err, errLocked) {
			t.Errorf("%s: locked connection admitted: %v", m.T, err)
		}
	}
	if err := a.UserUnlock(c, "1234"); err != nil { // Ana, not trusted
		t.Fatal(err)
	}
	for _, m := range []inMsg{{T: "config"}, {T: "filterSets"}, {T: "presets"}} {
		if err := admitMsg(a, c, m); !errors.Is(err, errLocked) {
			t.Errorf("%s: untrusted user admitted: %v", m.T, err)
		}
	}
	if err := a.SaveFilterSets(c, nil); !errors.Is(err, errLocked) {
		t.Errorf("untrusted SaveFilterSets: %v", err)
	}
	setFlags(t, a, "Ana", true, false)
	if err := admitMsg(a, c, inMsg{T: "presets"}); err != nil {
		t.Fatal(err)
	}
	setFlags(t, a, "Ana", false, false)
	if err := admitMsg(a, c, inMsg{T: "presets"}); !errors.Is(err, errLocked) {
		t.Errorf("after unticking Trusted: %v", err)
	}
	setFlags(t, a, "Ana", true, false)
	a.cfgMu.Lock()
	_, nr, _ := a.update(func(cfg *config.Config) error {
		cfg.Users[0].Pin = mkUser(t, "", "4321").Pin
		return nil
	})
	a.cfgMu.Unlock()
	a.mu.Lock()
	a.rt.Store(nr)
	a.mu.Unlock()
	if err := admitMsg(a, c, inMsg{T: "presets"}); !errors.Is(err, errLocked) {
		t.Errorf("after the user's PIN changed: %v", err)
	}
}

func TestLockStateCarriesFlags(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	st := a.lockStateMu(c)
	a.mu.Unlock()
	if st.User != "Ana" || !st.Trusted || !st.Persistent || st.Locked {
		t.Fatalf("state %+v", st)
	}
}

func TestSaveUsersFlags(t *testing.T) {
	a := userApp(t, false)
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{
		{Name: "Ana", Trusted: true}, {Name: "Bo", Persistent: true},
	}}); err != nil {
		t.Fatal(err)
	}
	u := a.r().cfg.Users
	if !u[0].Trusted || u[0].Persistent || u[1].Trusted || !u[1].Persistent {
		t.Fatalf("flags %+v", u)
	}
}

// persistentApp: Ana is persistent (and the first connection unlocks her).
func persistentApp(t *testing.T) (*App, *client, string) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", false, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSession(c)
	if tok == "" {
		t.Fatal("no token for a persistent user")
	}
	return a, c, tok
}

func TestSessionTokenIssuedOnlyForPersistent(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if tok := a.issueSession(c); tok != "" || len(a.tokens.toks) != 0 {
		t.Fatalf("token for a non-persistent user: %q", tok)
	}
}

func TestSessionTokenSurvivesNewConnection(t *testing.T) {
	a, _, tok := persistentApp(t)
	fresh := &client{ip: "10.0.0.2"}
	a.Hi(fresh, "main", tok)
	if !a.pageAllowed(fresh, gateMain) || fresh.who() != "Ana" {
		t.Fatal("token not honored on a new connection")
	}
	other := &client{ip: "10.0.0.3"}
	a.Hi(other, "main", "bogus")
	if a.pageAllowed(other, gateMain) {
		t.Fatal("a made-up token worked")
	}
	b, err := os.ReadFile(a.tokens.path)
	if err != nil || strings.Contains(string(b), tok) {
		t.Fatalf("token file: %v / plaintext: %v", err, strings.Contains(string(b), tok))
	}
	a.tokens = loadTokens(a.cfgPath)
	again := &client{ip: "10.0.0.4"}
	a.Hi(again, "log", tok)
	if !a.pageAllowed(again, gateLog) {
		t.Fatal("token lost across a restart")
	}
}

func TestSessionTokenDies(t *testing.T) {
	for name, kill := range map[string]func(*testing.T, *App){
		"PIN changed": func(t *testing.T, a *App) {
			a.cfgMu.Lock()
			_, nr, _ := a.update(func(cfg *config.Config) error {
				cfg.Users[0].Pin = mkUser(t, "", "4321").Pin
				return nil
			})
			a.cfgMu.Unlock()
			a.mu.Lock()
			a.rt.Store(nr)
			a.mu.Unlock()
		},
		"user removed": func(t *testing.T, a *App) {
			admin := &client{ip: "10.0.0.9"}
			if err := a.Unlock(admin, "2468"); err != nil {
				t.Fatal(err)
			}
			if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Bo"}}}); err != nil {
				t.Fatal(err)
			}
		},
		"Persistent unticked": func(t *testing.T, a *App) { setFlags(t, a, "Ana", false, false) },
	} {
		a, _, tok := persistentApp(t)
		kill(t, a)
		fresh := &client{ip: "10.0.0.2"}
		a.Hi(fresh, "main", tok)
		if a.pageAllowed(fresh, gateMain) {
			t.Errorf("%s: token still works", name)
		}
	}
}

func TestSessionTokenDiesOnLock(t *testing.T) {
	a, c, tok := persistentApp(t)
	twin := &client{ip: "10.0.0.2", send: make(chan []byte, 8)} // another tab of the same browser
	c.send = make(chan []byte, 8)
	a.Hi(twin, "main", tok)
	a.mu.Lock()
	a.clients[c], a.clients[twin] = struct{}{}, struct{}{}
	a.mu.Unlock()
	a.UserLock(c)
	if a.pageAllowed(c, gateMain) || a.pageAllowed(twin, gateMain) {
		t.Fatal("lock left a connection open")
	}
	fresh := &client{ip: "10.0.0.3"}
	a.Hi(fresh, "main", tok)
	if a.pageAllowed(fresh, gateMain) {
		t.Fatal("a reload would undo the lock")
	}
}

func TestSessionTokenCapPerUser(t *testing.T) {
	a, _, first := persistentApp(t)
	var last string
	for range perUserTokens + 3 {
		c := &client{ip: "10.0.0.7"}
		clearWaits(a)
		if err := a.UserUnlock(c, "1234"); err != nil {
			t.Fatal(err)
		}
		last = a.issueSession(c)
	}
	n := 0
	for _, r := range a.tokens.toks {
		if r.User == "Ana" {
			n++
		}
	}
	if n != perUserTokens {
		t.Fatalf("%d tokens for one user", n)
	}
	if _, ok := a.tokens.user(first, a.r().cfg); ok {
		t.Fatal("the oldest token survived")
	}
	if _, ok := a.tokens.user(last, a.r().cfg); !ok {
		t.Fatal("the newest token was dropped")
	}
}

// The first version stored admin tokens ({pin, at}); they must load as
// nothing, not crash, and not open anything.
func TestOldAdminTokensIgnored(t *testing.T) {
	dir := t.TempDir()
	old := `{"abc123":{"pin":"hash","at":"2026-10-01T00:00:00Z"}}`
	if err := os.WriteFile(filepath.Join(dir, "tokens.json"), []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	s := loadTokens(filepath.Join(dir, "config.yaml"))
	if len(s.toks) != 0 {
		t.Fatalf("old tokens kept: %v", s.toks)
	}
	if err := os.WriteFile(filepath.Join(dir, "tokens.json"), []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if s := loadTokens(filepath.Join(dir, "config.yaml")); len(s.toks) != 0 {
		t.Fatal("damaged file kept tokens")
	}
}

// Over the wire: a persistent user's unlock returns a token, a second socket
// that presents it in hi is unlocked, and userLock revokes it.
func TestSessionOverWebSocket(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, true)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	dial := func() (func(map[string]any), func(string) map[string]any) {
		ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { ws.CloseNow() })
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
		return send, until
	}
	send, until := dial()
	send(map[string]any{"t": "hi", "page": "main"})
	until("lockState")
	send(map[string]any{"t": "userUnlock", "pin": "1234", "id": 1})
	res := until("userUnlockResult")
	tok, _ := res["token"].(string)
	if res["ok"] != true || tok == "" {
		t.Fatalf("unlock result %v", res)
	}
	send2, until2 := dial()
	send2(map[string]any{"t": "hi", "page": "main", "token": tok})
	if m := until2("lockState"); m["locked"] != false || m["user"] != "Ana" || m["trusted"] != true || m["persistent"] != true {
		t.Fatalf("token hi: %v", m)
	}
	send2(map[string]any{"t": "userSignOut"})
	if m := until2("lockState"); m["locked"] != true {
		t.Fatalf("after lock: %v", m)
	}
	send3, until3 := dial()
	send3(map[string]any{"t": "hi", "page": "main", "token": tok})
	if m := until3("lockState"); m["locked"] != true {
		t.Fatalf("revoked token worked: %v", m)
	}
}

// watched: a connection the app can deliver to, unlocked as the user whose PIN is pin.
func watched(t *testing.T, a *App, ip, pin string) *client {
	t.Helper()
	c := &client{ip: ip, send: make(chan []byte, 16)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	if pin != "" {
		if err := a.UserUnlock(c, pin); err != nil {
			t.Fatal(err)
		}
	}
	return c
}

// sessionSent: the token of a queued `session` message, "" if none.
func sessionSent(c *client) string {
	for {
		select {
		case b := <-c.send:
			var m sessionMsg
			if json.Unmarshal(b, &m) == nil && m.T == "session" {
				return m.Token
			}
		default:
			return ""
		}
	}
}

// Ticking Never log out for someone already logged in gives their open
// connection a token at once (only the PIN did before), so a reload keeps
// them in; a connection that is not logged in gets nothing.
func TestNeverLogOutTickedWhileLoggedIn(t *testing.T) {
	a := userApp(t, true)
	ana := watched(t, a, "10.0.0.1", "1234")
	bo := watched(t, a, "10.0.0.2", "567890") // not persistent
	idle := watched(t, a, "10.0.0.3", "")
	admin := &client{ip: "10.0.0.9"}
	if err := a.Unlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{
		{Name: "Ana", Persistent: true}, {Name: "Bo"},
	}}); err != nil {
		t.Fatal(err)
	}
	tok := sessionSent(ana)
	if tok == "" {
		t.Fatal("Ana was not given a token")
	}
	if sessionSent(bo) != "" || sessionSent(idle) != "" {
		t.Fatal("a token went to someone who is not persistent or not logged in")
	}
	fresh := &client{ip: "10.0.0.4"}
	a.Hi(fresh, "main", tok)
	if !a.pageAllowed(fresh, gateMain) || fresh.who() != "Ana" {
		t.Fatal("the pushed token does not log in")
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{
		{Name: "Ana", Persistent: true}, {Name: "Bo"},
	}}); err != nil {
		t.Fatal(err)
	}
	if sessionSent(ana) != "" || len(a.tokens.toks) != 1 {
		t.Fatalf("a second token was issued (%d on file)", len(a.tokens.toks))
	}
}

// Changing your own PIN ends your old tokens; the connection that changed it
// is given a new one, or the next reload would ask for the new PIN.
func TestChangedPinKeepsSessionLoggedIn(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", false, true)
	c := watched(t, a, "10.0.0.1", "1234")
	old := a.issueSession(c)
	if old == "" {
		t.Fatal("no token")
	}
	if err := a.UserSetPin(c, "1234", "4321"); err != nil {
		t.Fatal(err)
	}
	tok := sessionSent(c)
	if tok == "" || tok == old {
		t.Fatalf("no new token after the PIN change (%q)", tok)
	}
	fresh := &client{ip: "10.0.0.2"}
	a.Hi(fresh, "main", old)
	if a.pageAllowed(fresh, gateMain) {
		t.Fatal("the old token survived the PIN change")
	}
	again := &client{ip: "10.0.0.3"}
	a.Hi(again, "main", tok)
	if !a.pageAllowed(again, gateMain) {
		t.Fatal("the new token does not log in")
	}
}

func TestSessionIssuedToLockedConnectionIsDropped(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", false, true)
	c := watched(t, a, "10.0.0.1", "1234")
	a.UserLock(c)
	if tok := a.issueSession(c); tok != "" || len(a.tokens.toks) != 0 {
		t.Fatalf("token for a locked connection: %q (%d on file)", tok, len(a.tokens.toks))
	}
}
