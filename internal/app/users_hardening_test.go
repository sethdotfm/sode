package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func setPinHash(t *testing.T, a *App, name, hash string) {
	t.Helper()
	a.cfgMu.Lock()
	_, nr, err := a.update(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].Name == name {
				cfg.Users[i].Pin = hash
			}
		}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.mu.Unlock()
}

func tokenAlive(a *App, tok string) bool {
	_, ok := a.tokens.user(tok, a.r().cfg)
	return ok
}

func TestRevocationIsDurable(t *testing.T) {
	t.Run("untick then re-tick Persistent", func(t *testing.T) {
		a, _, tok := persistentApp(t)
		setFlags(t, a, "Ana", false, false)
		setFlags(t, a, "Ana", false, true)
		if tokenAlive(a, tok) {
			t.Fatal("token revived")
		}
		if len(a.tokens.toks) != 0 {
			t.Fatal("record kept")
		}
	})
	t.Run("removed and added again under the same name and PIN", func(t *testing.T) {
		a, _, tok := persistentApp(t)
		admin := &client{ip: "10.0.0.9"}
		if err := a.UserUnlock(admin, "2468"); err != nil {
			t.Fatal(err)
		}
		mustSave(t, a, admin, map[string]any{ukey(a, "Ana", "remove"): true})
		mustSave(t, a, admin, map[string]any{"user/new:0/add": newUser{Name: "Ana", Pin: "1234", Persistent: new(true)}})
		if tokenAlive(a, tok) {
			t.Fatal("token revived by a new user of the same name")
		}
	})
	t.Run("PIN changed and changed back", func(t *testing.T) {
		a, _, tok := persistentApp(t)
		orig := a.r().cfg.Users[0].Pin
		setPinHash(t, a, "Ana", mkUser(t, "", "4321").Pin)
		setPinHash(t, a, "Ana", orig)
		if tokenAlive(a, tok) {
			t.Fatal("token revived by restoring the PIN")
		}
	})
	t.Run("a startup prune", func(t *testing.T) {
		a, _, tok := persistentApp(t)
		cfg := a.r().cfg
		cfg.Users[0].Persistent = false
		a.tokens.prune(cfg)
		cfg.Users[0].Persistent = true
		if _, ok := a.tokens.user(tok, cfg); ok {
			t.Fatal("pruned token alive")
		}
	})
}

func TestTokenMaxAge(t *testing.T) {
	a, _, tok := persistentApp(t)
	a.tokens.mu.Lock()
	for k, r := range a.tokens.toks {
		r.At = time.Now().Add(-tokenMaxAge - time.Hour)
		a.tokens.toks[k] = r
	}
	a.tokens.mu.Unlock()
	if tokenAlive(a, tok) {
		t.Fatal("an old token works")
	}
	a.tokens.prune(a.r().cfg)
	if len(a.tokens.toks) != 0 {
		t.Fatal("an old token was not pruned")
	}
}

func TestTrustedCannotWeakenGuards(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, false)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	changes := map[string]func(*config.Config){
		"pinWaits":       func(x *config.Config) { x.PinWaits = false },
		"trustedProxies": func(x *config.Config) { x.TrustedProxies = []string{"0.0.0.0/0"} },
		"routerShow":     func(x *config.Config) { x.RouterShow = "none" },
	}
	before := a.r().cfg
	for name, ch := range changes {
		cfg := a.r().cfg.Clone()
		ch(&cfg)
		err := a.SaveSettings(c, cfg)
		if !isRoleErr(err) || !strings.Contains(err.Error(), "Administrator") {
			t.Errorf("trusted changed %s: %v", name, err)
		}
	}
	// The router settings save only as router/<uid>/<attribute>, in sections only an Administrator edits.
	for attr, v := range map[string]any{"address": "10.9.9.9:8910", "matrix": 5, "levels": 4, "levelWireOffset": 3,
		"nameRequests": "ext", "fireMode": "native", "salvoGroup": 5, "protectDevice": 7} {
		if r := mustRefuse(t, a, c, map[string]any{aKey(a, attr): v}); r.Reason != reasonRole && r.Reason != reasonApproval {
			t.Errorf("trusted changed %s: %+v", attr, r)
		}
	}
	if a.r().cfg.Routers[0].Address != before.Routers[0].Address || a.r().cfg.PinWaits != before.PinWaits || len(a.r().cfg.TrustedProxies) != 0 {
		t.Fatal("a refused save changed the config")
	}
	cfg := a.r().cfg.Clone()
	cfg.BrandText = "Studio B"
	if err := a.SaveSettings(c, cfg); err != nil {
		t.Fatalf("plain trusted save: %v", err)
	}
	if err := a.UserUnlock(c, "2468"); err != nil {
		t.Fatal(err)
	}
	cfg = a.r().cfg.Clone()
	cfg.PinWaits = false
	cfg.TrustedProxies = []string{"10.0.0.2"}
	if err := a.SaveSettings(c, cfg); err != nil {
		t.Fatalf("admin save: %v", err)
	}
	grantRecheck(a, c)
	mustSave(t, a, c, map[string]any{aKey(a, "address"): "127.0.0.1:9999"})
	if got := a.r().cfg; got.PinWaits || got.Routers[0].Address != "127.0.0.1:9999" || len(got.TrustedProxies) != 1 {
		t.Fatalf("admin changes lost: %+v", got.Routers[0].Address)
	}
}

func TestSignOutAll(t *testing.T) {
	a, c, tok := persistentApp(t)
	a.mu.Lock()
	c.send = make(chan []byte, 8)
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	setFlags(t, a, "Ana", true, true) // trusted is not enough
	if err := a.SignOutAll(c, "Ana"); !isRoleErr(err) {
		t.Fatalf("trusted user signed everyone out: %v", err)
	}
	admin := &client{ip: "10.0.0.9"}
	if err := a.UserUnlock(admin, "2468"); err != nil {
		t.Fatal(err)
	}
	if err := a.SignOutAll(admin, "Nobody"); err == nil {
		t.Fatal("unknown user accepted")
	}
	if err := a.SignOutAll(admin, "Ana"); err != nil {
		t.Fatal(err)
	}
	if tokenAlive(a, tok) || a.pageAllowed(c, gateMain) {
		t.Fatal("a session survived")
	}
	if err := a.UserUnlock(c, "1234"); err != nil || a.issueSession(c) == "" {
		t.Fatalf("login after sign out: %v", err)
	}
	if err := admitMsg(a, &client{}, inMsg{T: "userSignOutAll"}); err == nil {
		t.Fatalf("policy: %v", err)
	}
}

func TestUserUnlockClearsToken(t *testing.T) {
	a, _, tok := persistentApp(t)
	c := &client{ip: "10.0.0.2", send: make(chan []byte, 8)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	a.Hi(c, "main", tok)
	if c.who() != "Ana" {
		t.Fatal("token hi failed")
	}
	if err := a.UserUnlock(c, "567890"); err != nil { // Bo, by PIN
		t.Fatal(err)
	}
	if c.who() != "Bo" || c.u.tokenMu() != "" {
		t.Fatalf("who %s token %q", c.who(), c.u.tokenMu())
	}
	a.UserLock(c)
	if !tokenAlive(a, tok) {
		t.Fatal("Bo's lock revoked Ana's token")
	}
}

func TestTokenFileWrite(t *testing.T) {
	a, _, _ := persistentApp(t)
	fi, err := os.Stat(a.tokens.path)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("token file: %v %v", err, fi)
	}
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(a.tokens.path), ".sode-*"))
	if len(left) != 0 {
		t.Fatalf("temp files left: %v", left)
	}
}

func TestSecurityHeaders(t *testing.T) {
	a := userApp(t, false)
	srv := httptest.NewServer(a.Handler(web.FS))
	defer srv.Close()
	for _, p := range []string{"/", "/log", "/main.js", "/manifest.webmanifest", "/icon.png", "/healthz"} {
		res, err := http.Get(srv.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Errorf("%s: %d", p, res.StatusCode)
		}
		if h := res.Header.Get("Content-Security-Policy"); !strings.Contains(h, "script-src 'self'") || !strings.Contains(h, "frame-ancestors 'none'") {
			t.Errorf("%s: CSP %q", p, h)
		}
		if res.Header.Get("X-Content-Type-Options") != "nosniff" || res.Header.Get("Referrer-Policy") != "same-origin" {
			t.Errorf("%s: missing headers", p)
		}
	}
}

func TestNoInlineScript(t *testing.T) {
	scripts := regexp.MustCompile(`(?is)<script\b([^>]*)>(.*?)</script>`)
	handler := regexp.MustCompile(`(?i)\son[a-z]+\s*=`)
	javascriptURL := regexp.MustCompile(`(?i)(href|src|action)\s*=\s*["']\s*javascript:`)
	for _, name := range []string{"index.html", "log.html", "panels.html", "paneleditor.html", "panel.html"} {
		b, err := web.FS.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range scripts.FindAllStringSubmatch(string(b), -1) {
			if !strings.Contains(m[1], "src=") || strings.TrimSpace(m[2]) != "" {
				t.Errorf("%s: inline script %q", name, m[0])
			}
		}
		if handler.Match(b) || javascriptURL.Match(b) {
			t.Errorf("%s: inline event handler or javascript: URL", name)
		}
	}
}

// Over a real WebSocket: odd key casing, a cross-origin upgrade, and a token
// connection that then signs in by PIN as someone else.
func TestHardeningOverWebSocket(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, true)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	dial := func(opts *websocket.DialOptions) (func(map[string]any), func(string) map[string]any) {
		ws, _, err := websocket.Dial(ctx, url, opts)
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

	send, until := dial(nil)
	send(map[string]any{"t": "hi", "page": "main"})
	until("lockState")
	send(map[string]any{"t": "userSignIn", "user": "Ana", "pin": "1234", "id": 1})
	tok, _ := until("userSignInResult")["token"].(string)
	if tok == "" {
		t.Fatal("no token")
	}
	send2, until2 := dial(nil)
	send2(map[string]any{"t": "hi", "page": "main", "token": tok})
	if m := until2("lockState"); m["user"] != "Ana" {
		t.Fatalf("token hi: %v", m)
	}

	send2(map[string]any{"t": "userSignIn", "user": "Bo", "pin": "567890", "id": 2})
	until2("userSignInResult")
	a.mu.Lock()
	var tokens []string
	for cl := range a.clients {
		if cl.who() == "Bo" {
			tokens = append(tokens, cl.u.tokenMu())
		}
	}
	a.mu.Unlock()
	if len(tokens) != 1 || tokens[0] != "" {
		t.Fatalf("Bo's connection tokens %q", tokens)
	}

	_, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: http.Header{"Origin": {"https://evil.example"}}})
	if err == nil {
		t.Fatal("cross-origin WebSocket accepted")
	}
	if resp != nil && resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d", resp.StatusCode)
	}
}

// The return value reads the lockState messages sent to c.
func register(a *App, c *client) func() []map[string]any {
	a.mu.Lock()
	c.send = make(chan []byte, 16)
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	return func() []map[string]any {
		var out []map[string]any
		for {
			select {
			case b := <-c.send:
				var m map[string]any
				json.Unmarshal(b, &m)
				if m["t"] == "lockState" {
					out = append(out, m)
				}
			default:
				return out
			}
		}
	}
}

// The pad's lengths come with every lock state, so a PIN change reaches every
// open page (the lock screen of a wall panel included).
func TestLockStateCarriesPinLengthsAfterPinChange(t *testing.T) {
	a := userApp(t, true)
	c, other := &client{ip: "10.0.0.1"}, &client{ip: "10.0.0.2"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	read := register(a, other)
	a.syncAccess(other)
	if got := read(); len(got) != 1 || !reflect.DeepEqual(got[0]["lens"], []any{float64(4), float64(6)}) {
		t.Fatalf("lens before: %v", got)
	}
	if err := a.UserSetPin(c, "1234", "12345678"); err != nil {
		t.Fatal(err)
	}
	got := read()
	if len(got) == 0 || !reflect.DeepEqual(got[len(got)-1]["lens"], []any{float64(4), float64(6), float64(8)}) {
		t.Fatalf("lens after the change: %v", got)
	}
}

func TestOnlyTheAnswerToHiIsMarkedHi(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	read := register(a, c)
	a.syncAccess(c) // someone else's config save reaches it first
	a.Hi(c, "main", "no such token")
	got := read()
	if len(got) != 2 || got[0]["hi"] != nil || got[1]["hi"] != true {
		t.Fatalf("lockStates: %v", got)
	}
}
