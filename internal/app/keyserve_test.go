package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// keyTestApp: users Ana (1234) and Bo (567890), an API client PanelsBot, the Administrator Ada (2468).
func keyTestApp(t *testing.T) *App {
	t.Helper()
	a := userApp(t, false)
	admin := adminClient(t, a)
	if err := saveUsers(a, admin, usersReq{
		Users:      []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {ID: idOf(a, "Bo"), Name: "Bo"}},
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
		rl.handle(a, c, &m)
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
	if keyCount(a.keys, ownerOf(a, "Ana")) != 0 {
		t.Fatal("key stored after a wrong PIN")
	}
	// Another user's PIN is not Ana's.
	if r := send(t, a, c, next, inMsg{T: "keyCreate", ID: 2, Pin: "2468", KeyName: "Panel 21"}, "keyCreateResult"); r["ok"] != false {
		t.Fatalf("Ada's PIN made a key for Ana: %v", r)
	}

	r = send(t, a, c, next, inMsg{T: "keyCreate", ID: 3, Pin: "1234", KeyName: "Panel 21"}, "keyCreateResult")
	secret, _ := r["key"].(string)
	if r["ok"] != true || !strings.HasPrefix(secret, "sode_") {
		t.Fatalf("reply %v", r)
	}
	if _, _, ok := a.keys.find(secret, time.Now()); !ok {
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
	if r["ok"] != false || keyCount(a.keys, ownerOf(a, "Ana")) != 0 {
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
	c, _ := signedIn(t, a, "10.0.0.1", "1234")
	for range 50 {
		if _, _, err := a.CreateOwnKey(c, "1234", "k", 0); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err := a.CreateOwnKey(c, "1234", "k", 0)
	if err == nil || !strings.Contains(err.Error(), "50 keys") || !strings.Contains(err.Error(), "Revoke a key") {
		t.Fatalf("51st key: %v", err)
	}
}

func TestAPIClientKeyCap(t *testing.T) {
	a := keyTestApp(t)
	admin := adminClient(t, a)
	bot := keyOwner{Kind: actorClient, Name: "PanelsBot"}
	for range 200 {
		if _, _, err := a.CreateKeyFor(admin, bot, "k", 0); err != nil {
			t.Fatal(err)
		}
	}
	_, _, err := a.CreateKeyFor(admin, bot, "k", 0)
	if err == nil || !strings.Contains(err.Error(), "200 keys") || !strings.Contains(err.Error(), "Revoke a key") {
		t.Fatalf("201st key: %v", err)
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
	if _, _, ok := a.keys.find(secret, time.Now()); !ok {
		t.Fatal("live key refused")
	}
	if _, _, ok := a.keys.find(secret, soon.Add(time.Second)); ok {
		t.Fatal("expired key accepted")
	}
	a.expireKeys(soon.Add(time.Second))
	if keyCount(a.keys, ownerOf(a, "Ana")) != 0 {
		t.Fatal("expired key kept")
	}
	var found bool
	for _, e := range waitLogged(t, a, 1) {
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
	if keyCount(a.keys, ownerOf(a, "Ana")) != 1 {
		t.Fatal("key gone")
	}
	if r := send(t, a, ana, nextA, inMsg{T: "keyRevoke", ID: 2, KeyID: rec.ID}, "keyRevokeResult"); r["ok"] != true {
		t.Fatalf("own revoke: %v", r)
	}
	var found bool
	for _, e := range waitLogged(t, a, 2) {
		found = found || e.Kind == kindAPI && e.Why == "keyRevoke" && e.Result == "ok" && e.By == "Ana"
	}
	if !found {
		t.Fatalf("no revoke row: %+v", logged(t, a))
	}
}

func TestUserKeysDieWithPinChangeAndRemoval(t *testing.T) {
	for name, edit := range map[string]func(a *App, admin *client) error{
		"PIN change": func(a *App, admin *client) error {
			return saveUsers(a, admin, usersReq{Users: []userEdit{{Name: "Ana", Pin: "4321"}, {ID: idOf(a, "Bo"), Name: "Bo"}}})
		},
		"removal": func(a *App, admin *client) error {
			return saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Bo"), Name: "Bo"}}})
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
			if _, _, ok := a.keys.find(secA, time.Now()); ok {
				t.Error("Ana's key survived")
			}
			if _, _, ok := a.keys.find(secB, time.Now()); !ok {
				t.Error("Bo's key died too")
			}
			if _, _, ok := a.keys.find(secBot, time.Now()); !ok {
				t.Error("an API client key died with a user edit")
			}
			var found bool
			for _, e := range waitLogged(t, a, 1) {
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
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{{Name: "Ana", Pin: "4321"}, {Name: "Bo", Pin: "135790"}}}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := a.keys.find(sec, time.Now()); !ok {
		t.Fatal("client key died with user PINs")
	}
	// A save that leaves the client list unsent keeps the client and its keys.
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {ID: idOf(a, "Bo"), Name: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := a.keys.find(sec, time.Now()); !ok {
		t.Fatal("client key died when the save did not mention clients")
	}
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {ID: idOf(a, "Bo"), Name: "Bo"}}, APIClients: &[]config.APIClient{}}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok := a.keys.find(sec, time.Now()); ok {
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
	if r := send(t, a, admin, register, inMsg{T: "keyCreateFor", ID: 2, KeyName: "x", Owner: ptr(ownerOf(a, "Nobody"))}, "keyCreateForResult"); r["ok"] != false {
		t.Fatalf("key for a missing user: %v", r)
	}
	r = send(t, a, admin, register, inMsg{T: "keyCreateFor", ID: 3, KeyName: "Ana's panel", Owner: ptr(ownerOf(a, "Ana"))}, "keyCreateForResult")
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

// Every Administrator key message refuses a lower Role, a Trusted user too.
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
	srcs := *nums(1, 2)
	tr := true
	err := saveUsers(a, admin, usersReq{
		Users:       []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Rights: &config.Rights{Sources: &srcs, Visibility: config.VisHidden}, NoAutoLock: &tr}, {ID: idOf(a, "Bo"), Name: "Bo"}},
		GuestRights: &config.Rights{Dsts: nums()},
	})
	if err != nil {
		t.Fatal(err)
	}
	cfg := a.r().cfg
	ana, _ := cfg.FindUserByName("Ana")
	if !ana.NoAutoLock || !ana.Rights.Hidden() || !ana.Rights.AllowsSource(config.Ref{N: 2}) || ana.Rights.AllowsSource(config.Ref{N: 3}) {
		t.Fatalf("Ana %+v", ana)
	}
	if cfg.GuestRights.Dsts == nil || cfg.GuestRights.AllowsDst(config.Ref{N: 0}) {
		t.Fatalf("guest %+v %v", cfg.GuestRights, cfg.GuestAPIScopes)
	}

	// A save from a page that knows none of it keeps all of it.
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {ID: idOf(a, "Bo"), Name: "Bo"}}}); err != nil {
		t.Fatal(err)
	}
	cfg = a.r().cfg
	ana, _ = cfg.FindUserByName("Ana")
	if !ana.NoAutoLock || ana.Rights.Sources == nil || cfg.GuestRights.Dsts == nil || len(cfg.APIClients) != 1 {
		t.Fatalf("a stale save erased: %+v %+v", ana, cfg.GuestRights)
	}

	// The list carries them, to the PIN holder only.
	b, _ := json.Marshal(a.usersList(&client{}, 1))
	for _, want := range []string{`"noAutoLock":true`, `"visibility":"hidden"`, `"apiClients"`, `"PanelsBot"`, `"guestRights"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("usersListResult lacks %s: %s", want, b)
		}
	}

	// The fixture's API client add, then the first save; the stale save changed nothing.
	waitSaves(t, a, 2)
	time.Sleep(50 * time.Millisecond)
	rows := a.saves.Between(-1, 1<<62)
	if len(rows) != 2 {
		t.Fatalf("%d Settings saves rows, want 2", len(rows))
	}
	got := map[string]bool{}
	for _, f := range rows[0].Save.Fields {
		parts := strings.SplitN(f.Label, ":", 3)
		got[parts[0]+":"+parts[1]] = true
	}
	for _, want := range []string{"User Ana: Rights", "User Ana: No auto-lock", "Guest rights: dsts"} {
		if !got[want] {
			t.Errorf("no %q field in %v", want, got)
		}
	}
}

func TestUsersSaveRefusesBadRights(t *testing.T) {
	a := keyTestApp(t)
	admin := adminClient(t, a)
	err := saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Rights: &config.Rights{Visibility: "secret"}}, {ID: idOf(a, "Bo"), Name: "Bo"}}})
	if err == nil {
		t.Fatal("bad visibility saved")
	}
	err = saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "Ana"}, {ID: idOf(a, "Bo"), Name: "Bo"}}, APIClients: &[]config.APIClient{{Name: "ana"}}})
	if err == nil {
		t.Fatal("API client with a user's name saved")
	}
}

func TestRightsAndKeysStayOutOfPublicMessages(t *testing.T) {
	a := keyTestApp(t)
	admin := adminClient(t, a)
	tr := true
	srcs := *nums(7)
	if err := saveUsers(a, admin, usersReq{
		Users:      []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", NoAutoLock: &tr, Rights: &config.Rights{Sources: &srcs}}, {ID: idOf(a, "Bo"), Name: "Bo"}},
		APIClients: &[]config.APIClient{{Name: "PanelsBot", Rights: config.Rights{Dsts: nums(42)}}},
	}); err != nil {
		t.Fatal(err)
	}
	ana, _ := signedIn(t, a, "10.0.0.1", "1234")
	secret, rec, err := a.CreateOwnKey(ana, "1234", "Panel 21", 0)
	if err != nil {
		t.Fatal(err)
	}

	a.mu.Lock()
	hello := withoutAccessSettings(t, a.hello(helloPage{}))
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

	a.mu.Lock()
	m := a.lockStateMu(ana)
	a.mu.Unlock()
	b, _ := json.Marshal(m)
	if !m.NoAutoLock || strings.Contains(string(b), "rights") || strings.Contains(string(b), "Panel 21") {
		t.Fatalf("own lock state %s", b)
	}
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
