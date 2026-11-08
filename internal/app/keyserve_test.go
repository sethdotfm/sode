package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// keyTestApp: users Ana (1234) and Bo (567890), an API client PanelsBot, Advanced Settings PIN 2468.
func keyTestApp(t *testing.T) *App {
	t.Helper()
	a := userApp(t, false)
	admin := adminClient(t, a)
	if err := a.SaveUsers(admin, usersReq{
		Users:      []userEdit{{Name: "Ana"}, {Name: "Bo"}},
		APIClients: &[]config.APIClient{{Name: "PanelsBot", Scopes: []string{config.ScopeTake}}},
	}); err != nil {
		t.Fatal(err)
	}
	return a
}

func signedIn(t *testing.T, a *App, ip, pin string) (*client, func() []map[string]any) {
	t.Helper()
	c := &client{ip: ip}
	next := registerAll(a, c)
	if err := a.UserUnlock(c, pin); err != nil {
		t.Fatal(err)
	}
	return c, next
}

// registerAll is register but keeps every message type.
func registerAll(a *App, c *client) func() []map[string]any {
	a.mu.Lock()
	c.send = make(chan []byte, 64)
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	return func() []map[string]any {
		var out []map[string]any
		for {
			select {
			case b := <-c.send:
				var m map[string]any
				json.Unmarshal(b, &m)
				out = append(out, m)
			default:
				return out
			}
		}
	}
}

// send runs a message the way the read loop does and returns the reply of type want.
func send(t *testing.T, a *App, c *client, next func() []map[string]any, m inMsg, want string) map[string]any {
	t.Helper()
	next()
	rl := policy[m.T].forMsg(&m)
	if err := a.admit(c, rl); err != nil {
		a.refuse(c, m.T, rl, &m, err)
	} else {
		rl.handle(a, c, &m, &connCtx{})
	}
	for _, r := range next() {
		if r["t"] == want {
			return r
		}
	}
	t.Fatalf("%s: no %s reply", m.T, want)
	return nil
}

func TestOwnKeyNeedsOwnPinAndShowsKeyOnce(t *testing.T) {
	a := keyTestApp(t)
	defer startLog(t, a)()
	c, next := signedIn(t, a, "10.0.0.1", "1234")

	r := send(t, a, c, next, inMsg{T: "keyCreate", ID: 1, Pin: "9999", KeyName: "Panel 21"}, "keyCreateResult")
	if r["ok"] != false {
		t.Fatalf("wrong PIN made a key: %v", r)
	}
	if a.keys.count(keyOwner{Kind: actorUser, Name: "Ana"}) != 0 {
		t.Fatal("key stored after a wrong PIN")
	}
	// The Advanced Settings PIN is not the user's PIN.
	if r := send(t, a, c, next, inMsg{T: "keyCreate", ID: 2, Pin: "2468", KeyName: "Panel 21"}, "keyCreateResult"); r["ok"] != false {
		t.Fatalf("settings PIN made a user key: %v", r)
	}

	r = send(t, a, c, next, inMsg{T: "keyCreate", ID: 3, Pin: "1234", KeyName: "Panel 21"}, "keyCreateResult")
	secret, _ := r["key"].(string)
	if r["ok"] != true || !strings.HasPrefix(secret, "sode_") {
		t.Fatalf("reply %v", r)
	}
	if _, ok := a.keys.lookup(secret, time.Now()); !ok {
		t.Fatal("new key does not look up")
	}

	// The list never repeats the secret.
	l := send(t, a, c, next, inMsg{T: "keyListOwn", ID: 4}, "keyListOwnResult")
	b, _ := json.Marshal(l)
	if strings.Contains(string(b), secret) || strings.Contains(string(b), "hash") || len(l["keys"].([]any)) != 1 {
		t.Fatalf("list %s", b)
	}

	// Rows name the key and never hold it or a PIN.
	for _, e := range waitLogged(t, a, 3) {
		if e.Kind == kindAPI && (strings.Contains(e.Note+e.Err+e.By, secret) || strings.Contains(e.Note+e.Err, "1234")) {
			t.Fatalf("row leaks: %+v", e)
		}
	}
}

func TestOwnKeyNeedsSignIn(t *testing.T) {
	a := keyTestApp(t)
	c := &client{ip: "10.0.0.1"}
	next := registerAll(a, c)
	r := send(t, a, c, next, inMsg{T: "keyCreate", ID: 1, Pin: "1234", KeyName: "x"}, "keyCreateResult")
	if r["ok"] != false || a.keys.count(keyOwner{Kind: actorUser, Name: "Ana"}) != 0 {
		t.Fatalf("anonymous connection made a key: %v", r)
	}
	if r := send(t, a, c, next, inMsg{T: "keyListOwn", ID: 2}, "keyListOwnResult"); r["ok"] != false || len(r["keys"].([]any)) != 0 {
		t.Fatalf("list %v", r)
	}
}

func TestOwnKeyUsesChangePinThrottle(t *testing.T) {
	a := keyTestApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	for i := 0; i < 6; i++ {
		send(t, a, c, next, inMsg{T: "keyCreate", ID: i, Pin: "0000", KeyName: "x"}, "keyCreateResult")
	}
	r := send(t, a, c, next, inMsg{T: "keyCreate", ID: 9, Pin: "1234", KeyName: "x"}, "keyCreateResult")
	if r["ok"] != false {
		t.Fatalf("right PIN accepted during a wait: %v", r)
	}
	// Same pool as change-PIN.
	if err := a.UserSetPin(c, "1234", "4321"); err == nil {
		t.Fatal("userSetPin ignored the shared wait")
	}
}

func TestOwnKeyCap(t *testing.T) {
	a := keyTestApp(t)
	a.cfgMu.Lock()
	err := a.commit(func(c *config.Config) error { c.API.MaxUserKeys = 2; return nil }, nil)
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	c, _ := signedIn(t, a, "10.0.0.1", "1234")
	for i := 0; i < 2; i++ {
		if _, _, err := a.CreateOwnKey(c, "1234", "k", 0); err != nil {
			t.Fatal(err)
		}
	}
	if _, _, err := a.CreateOwnKey(c, "1234", "k", 0); err == nil {
		t.Fatal("third key over a cap of 2")
	}
}

func TestKeyExpiryAtCreateAndSweep(t *testing.T) {
	a := keyTestApp(t)
	defer startLog(t, a)()
	c, _ := signedIn(t, a, "10.0.0.1", "1234")
	if _, _, err := a.CreateOwnKey(c, "1234", "old", time.Now().Add(-time.Hour).UnixMilli()); err == nil {
		t.Fatal("a past expiry was accepted")
	}
	soon := time.Now().Add(time.Hour)
	secret, _, err := a.CreateOwnKey(c, "1234", "temp", soon.UnixMilli())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := a.keys.lookup(secret, time.Now()); !ok {
		t.Fatal("live key refused")
	}
	if _, ok := a.keys.lookup(secret, soon.Add(time.Second)); ok {
		t.Fatal("expired key accepted")
	}
	a.expireKeys(soon.Add(time.Second))
	if a.keys.count(keyOwner{Kind: actorUser, Name: "Ana"}) != 0 {
		t.Fatal("expired key kept")
	}
	var found bool
	for _, e := range waitLogged(t, a, 2) {
		found = found || e.Kind == kindAPI && e.Why == "keyExpired" && strings.Contains(e.Note, "temp")
	}
	if !found {
		t.Fatalf("no expiry row: %+v", logged(t, a))
	}
}

func TestRevokeOwnKeyOnlyOwn(t *testing.T) {
	a := keyTestApp(t)
	defer startLog(t, a)()
	ana, nextA := signedIn(t, a, "10.0.0.1", "1234")
	bo, nextB := signedIn(t, a, "10.0.0.2", "567890")
	_, rec, err := a.CreateOwnKey(ana, "1234", "mine", 0)
	if err != nil {
		t.Fatal(err)
	}
	if r := send(t, a, bo, nextB, inMsg{T: "keyRevoke", ID: 1, KeyID: rec.ID}, "keyRevokeResult"); r["ok"] != false {
		t.Fatalf("Bo revoked Ana's key: %v", r)
	}
	if a.keys.count(keyOwner{Kind: actorUser, Name: "Ana"}) != 1 {
		t.Fatal("key gone")
	}
	if r := send(t, a, ana, nextA, inMsg{T: "keyRevoke", ID: 2, KeyID: rec.ID}, "keyRevokeResult"); r["ok"] != true {
		t.Fatalf("own revoke: %v", r)
	}
	var found bool
	for _, e := range waitLogged(t, a, 3) {
		found = found || e.Kind == kindAPI && e.Why == "keyRevoke" && e.Result == "ok" && e.By == "Ana"
	}
	if !found {
		t.Fatalf("no revoke row: %+v", logged(t, a))
	}
}

func TestUserKeysDieWithPinRenameRemoval(t *testing.T) {
	for name, edit := range map[string]func(a *App, admin *client) error{
		"PIN change": func(a *App, admin *client) error {
			return a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana", Pin: "4321"}, {Name: "Bo"}}})
		},
		"rename": func(a *App, admin *client) error {
			return a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Anna", Was: "Ana"}, {Name: "Bo"}}})
		},
		"removal": func(a *App, admin *client) error {
			return a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Bo"}}})
		},
		"own PIN change": func(a *App, _ *client) error {
			c, _ := signedIn(t, a, "10.0.0.7", "1234")
			return a.UserSetPin(c, "1234", "4321")
		},
	} {
		t.Run(name, func(t *testing.T) {
			a := keyTestApp(t)
			defer startLog(t, a)()
			ana, _ := signedIn(t, a, "10.0.0.1", "1234")
			bo, _ := signedIn(t, a, "10.0.0.2", "567890")
			secA, _, _ := a.CreateOwnKey(ana, "1234", "a", 0)
			secB, _, _ := a.CreateOwnKey(bo, "567890", "b", 0)
			secBot, _, err := a.CreateKeyFor(adminClient(t, a), keyOwner{Kind: actorClient, Name: "PanelsBot"}, "bot", 0)
			if err != nil {
				t.Fatal(err)
			}
			if err := edit(a, adminClient(t, a)); err != nil {
				t.Fatal(err)
			}
			if _, ok := a.keys.lookup(secA, time.Now()); ok {
				t.Error("Ana's key survived")
			}
			if _, ok := a.keys.lookup(secB, time.Now()); !ok {
				t.Error("Bo's key died too")
			}
			if _, ok := a.keys.lookup(secBot, time.Now()); !ok {
				t.Error("an API client key died with a user edit")
			}
			var found bool
			for _, e := range waitLogged(t, a, 2) {
				found = found || e.Kind == kindAPI && e.Why == "keyEnded"
			}
			if !found {
				t.Error("no keyEnded row")
			}
		})
	}
}

func TestClientKeysSurviveUserPinsAndDieWithClient(t *testing.T) {
	a := keyTestApp(t)
	admin := adminClient(t, a)
	sec, _, err := a.CreateKeyFor(admin, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Panel 43", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana", Pin: "4321"}, {Name: "Bo", Pin: "135790"}}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.keys.lookup(sec, time.Now()); !ok {
		t.Fatal("client key died with user PINs")
	}
	// A save that leaves the client list unsent keeps the client and its keys.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.keys.lookup(sec, time.Now()); !ok {
		t.Fatal("client key died when the save did not mention clients")
	}
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Bo"}}, APIClients: &[]config.APIClient{}}); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.keys.lookup(sec, time.Now()); ok {
		t.Fatal("client key outlived its client")
	}
}

func TestAdminKeyMessages(t *testing.T) {
	a := keyTestApp(t)
	defer startLog(t, a)()
	admin := adminClient(t, a)
	register := registerAll(a, admin)
	r := send(t, a, admin, register, inMsg{T: "keyCreateFor", ID: 1, KeyName: "Panel 43", Owner: &keyOwner{Kind: actorClient, Name: "panelsbot"}}, "keyCreateForResult")
	if r["ok"] != true || !strings.HasPrefix(r["key"].(string), "sode_") {
		t.Fatalf("create for client: %v", r)
	}
	if r := send(t, a, admin, register, inMsg{T: "keyCreateFor", ID: 2, KeyName: "x", Owner: &keyOwner{Kind: actorUser, Name: "Nobody"}}, "keyCreateForResult"); r["ok"] != false {
		t.Fatalf("key for a missing user: %v", r)
	}
	r = send(t, a, admin, register, inMsg{T: "keyCreateFor", ID: 3, KeyName: "Ana's panel", Owner: &keyOwner{Kind: actorUser, Name: "Ana"}}, "keyCreateForResult")
	anaID := r["info"].(map[string]any)["id"].(string)
	l := send(t, a, admin, register, inMsg{T: "keyListAll", ID: 4}, "keyListAllResult")
	if len(l["keys"].([]any)) != 2 {
		t.Fatalf("list %v", l)
	}
	if r := send(t, a, admin, register, inMsg{T: "keyRevokeAny", ID: 5, KeyID: anaID}, "keyRevokeAnyResult"); r["ok"] != true {
		t.Fatalf("revoke any: %v", r)
	}
	if r := send(t, a, admin, register, inMsg{T: "keyRevokeAny", ID: 6, KeyID: "nope"}, "keyRevokeAnyResult"); r["ok"] != false {
		t.Fatalf("revoke missing: %v", r)
	}
	if _, _, err := a.CreateKeyFor(admin, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "second", 0); err != nil {
		t.Fatal(err)
	}
	if r := send(t, a, admin, register, inMsg{T: "keyRevokeAll", ID: 7, Owner: &keyOwner{Kind: actorClient, Name: "PanelsBot"}}, "keyRevokeAllResult"); r["ok"] != true {
		t.Fatalf("revoke all: %v", r)
	}
	if n := len(a.keys.list()); n != 0 {
		t.Fatalf("%d keys left", n)
	}
	waitLogged(t, a, 5)
}

// Every admin key message refuses without the Advanced Settings PIN, a trusted user too.
func TestAdminKeyMessagesNeedPin(t *testing.T) {
	a := keyTestApp(t)
	setFlags(t, a, "Ana", true, false)
	c, _ := signedIn(t, a, "10.0.0.1", "1234")
	for _, typ := range []string{"keyListAll", "keyCreateFor", "keyRevokeAny", "keyRevokeAll"} {
		if err := admitMsg(a, c, inMsg{T: typ}); err == nil {
			t.Errorf("%s admitted a trusted user without the PIN", typ)
		}
		if err := admitMsg(a, &client{ip: "10.0.0.5"}, inMsg{T: typ}); err == nil {
			t.Errorf("%s admitted an anonymous connection", typ)
		}
		if err := admitMsg(a, adminClient(t, a), inMsg{T: typ}); err != nil {
			t.Errorf("%s refused the PIN holder: %v", typ, err)
		}
	}
}

func TestKeyMessagesHaveRules(t *testing.T) {
	for _, typ := range []string{"keyCreate", "keyRevoke", "keyListOwn", "keyListAll", "keyCreateFor", "keyRevokeAny", "keyRevokeAll"} {
		if _, ok := policy[typ]; !ok {
			t.Errorf("no policy rule for %s", typ)
		}
	}
}

func TestUsersSaveCarriesRightsClientsAndNoAutoLock(t *testing.T) {
	a := keyTestApp(t)
	defer startLog(t, a)()
	admin := adminClient(t, a)
	srcs := []int{1, 2}
	tr := true
	err := a.SaveUsers(admin, usersReq{
		Users:       []userEdit{{Name: "Ana", Rights: &config.Rights{Sources: &srcs, Visibility: config.VisHidden}, NoAutoLock: &tr}, {Name: "Bo"}},
		GuestRights: &config.Rights{Dsts: &[]int{}},
		GuestScopes: &[]string{config.ScopeNames},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	ana, _ := cfg.FindUser("Ana")
	if !ana.NoAutoLock || !ana.Rights.Hidden() || !ana.Rights.AllowsSource(2) || ana.Rights.AllowsSource(3) {
		t.Fatalf("Ana %+v", ana)
	}
	if cfg.GuestRights.Dsts == nil || cfg.GuestRights.AllowsDst(0) || len(cfg.GuestAPIScopes) != 1 {
		t.Fatalf("guest %+v %v", cfg.GuestRights, cfg.GuestAPIScopes)
	}

	// A save from a page that knows none of it keeps all of it.
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	cfg = a.r().cfg
	ana, _ = cfg.FindUser("Ana")
	if !ana.NoAutoLock || ana.Rights.Sources == nil || cfg.GuestRights.Dsts == nil || len(cfg.APIClients) != 1 {
		t.Fatalf("a stale save erased: %+v %+v", ana, cfg.GuestRights)
	}

	// The list carries them, to the PIN holder only.
	b, _ := json.Marshal(a.usersList(1))
	for _, want := range []string{`"noAutoLock":true`, `"visibility":"hidden"`, `"apiClients"`, `"PanelsBot"`, `"guestRights"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("usersListResult lacks %s: %s", want, b)
		}
	}

	rows := waitLogged(t, a, 4)
	got := map[string]bool{}
	for _, e := range rows {
		if e.Kind == kindAPI {
			got[e.Why] = true
		}
	}
	for _, why := range []string{"rightsEdit", "noAutoLockEdit", "clientEdit"} {
		if !got[why] {
			t.Errorf("no %s row in %v", why, got)
		}
	}
}

func TestUsersSaveRefusesBadRights(t *testing.T) {
	a := keyTestApp(t)
	admin := adminClient(t, a)
	err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana", Rights: &config.Rights{Visibility: "secret"}}, {Name: "Bo"}}})
	if err == nil {
		t.Fatal("bad visibility saved")
	}
	err = a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Ana"}, {Name: "Bo"}}, APIClients: &[]config.APIClient{{Name: "ana"}}})
	if err == nil {
		t.Fatal("API client with a user's name saved")
	}
}

// Rights, key lists and No auto-lock stay out of everything sent before the PIN.
func TestRightsAndKeysStayOutOfPublicMessages(t *testing.T) {
	a := keyTestApp(t)
	admin := adminClient(t, a)
	tr := true
	srcs := []int{7}
	if err := a.SaveUsers(admin, usersReq{
		Users:      []userEdit{{Name: "Ana", NoAutoLock: &tr, Rights: &config.Rights{Sources: &srcs}}, {Name: "Bo"}},
		APIClients: &[]config.APIClient{{Name: "PanelsBot", Rights: config.Rights{Dsts: &[]int{42}}}},
	}); err != nil {
		t.Fatal(err)
	}
	ana, _ := signedIn(t, a, "10.0.0.1", "1234")
	secret, rec, err := a.CreateOwnKey(ana, "1234", "Panel 21", 0)
	if err != nil {
		t.Fatal(err)
	}

	a.mu.Lock()
	hello := string(a.hello(helloPage{}))
	locked := a.lockStateMu(&client{ip: "10.0.0.8"})
	a.mu.Unlock()
	lockedJSON, _ := json.Marshal(locked)
	for name, s := range map[string]string{"hello": hello, "lockState": string(lockedJSON)} {
		for _, bad := range []string{"noAutoLock", "rights", "PanelsBot", "apiClients", "guestRights", secret, rec.ID, "Panel 21", "keyHash"} {
			if strings.Contains(s, bad) {
				t.Errorf("%s mentions %q: %s", name, bad, s)
			}
		}
	}
	for _, typ := range []string{"keyListAll"} {
		if err := admitMsg(a, &client{ip: "10.0.0.8"}, inMsg{T: typ}); err == nil {
			t.Errorf("%s reachable without the PIN", typ)
		}
	}

	// A signed-in user is told their own No auto-lock flag and nothing of rights or keys.
	a.mu.Lock()
	m := a.lockStateMu(ana)
	a.mu.Unlock()
	b, _ := json.Marshal(m)
	if !m.NoAutoLock || strings.Contains(string(b), "rights") || strings.Contains(string(b), "Panel 21") {
		t.Fatalf("own lock state %s", b)
	}
	// Another user is not.
	bo, _ := signedIn(t, a, "10.0.0.2", "567890")
	a.mu.Lock()
	m = a.lockStateMu(bo)
	a.mu.Unlock()
	if m.NoAutoLock {
		t.Fatal("Bo got Ana's flag")
	}
}

func TestKeysLoadedAtStartDropOrphans(t *testing.T) {
	a := keyTestApp(t)
	admin := adminClient(t, a)
	if _, _, err := a.CreateKeyFor(admin, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "k", 0); err != nil {
		t.Fatal(err)
	}
	a.keys.flush()
	cfg := a.r().cfg
	cfg.APIClients = nil
	h, _ := OpenHistory("")
	b, err := New(cfg, a.cfgPath, h, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	if n := len(b.keys.list()); n != 0 {
		t.Fatalf("%d orphan keys survived a restart", n)
	}
}
