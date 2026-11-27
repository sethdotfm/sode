package app

import (
	"encoding/json"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func jsonOf(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// saveKeys: kv maps a key to its new value, sent at the live version.
func (a *App) saveKeys(c *client, kv map[string]any) (map[string]fieldResult, error) {
	live := a.r().cfg
	edits := make(map[string]fieldEdit, len(kv))
	for k, v := range kv {
		b, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		edits[k] = fieldEdit{Value: b, Version: fieldVersion(live, k)}
	}
	res, _, err := a.saveSettingsFields(c, edits, nil, "", "")
	return res, err
}

// firstRefused: "" for none.
func firstRefused(res map[string]fieldResult) (string, fieldResult) {
	for _, k := range slices.Sorted(maps.Keys(res)) {
		if res[k].Status == "refused" {
			return k, res[k]
		}
	}
	return "", fieldResult{}
}

func mustSave(t *testing.T, a *App, c *client, kv map[string]any) {
	t.Helper()
	res, err := a.saveKeys(c, kv)
	if err != nil {
		t.Fatal(err)
	}
	if k, r := firstRefused(res); k != "" {
		t.Fatalf("%s refused: %s: %s", k, r.Reason, r.Err)
	}
}

// mustRefuse returns the first refusal; a save refused as a whole comes back as Err.
func mustRefuse(t *testing.T, a *App, c *client, kv map[string]any) fieldResult {
	t.Helper()
	res, err := a.saveKeys(c, kv)
	if err != nil {
		return fieldResult{Status: "refused", Err: err.Error()}
	}
	k, r := firstRefused(res)
	if k == "" {
		t.Fatalf("saved, want a refusal: %v", res)
	}
	return r
}

func ukey(a *App, name, attr string) string { return "user/" + idOf(a, name) + "/" + attr }

// saveLive: a peer has the versions of user and client keys only after usersList, so this takes
// them from the live config.
func (p *wsPeer) saveLive(a *App, id int, kv map[string]any) map[string]any {
	p.t.Helper()
	live := a.r().cfg
	fields := map[string]any{}
	for k, v := range kv {
		fields[k] = map[string]any{"value": v, "version": fieldVersion(live, k)}
	}
	return p.ask("settingsSave", id, map[string]any{"fields": fields})
}

func allSaved(t *testing.T, what string, r map[string]any) {
	t.Helper()
	res, _ := r["results"].(map[string]any)
	if r["ok"] != true || res == nil {
		t.Fatalf("%s: refused: %v", what, r)
	}
	for k, v := range res {
		if f, _ := v.(map[string]any); f["status"] == "refused" {
			t.Fatalf("%s: %s refused: %v", what, k, f)
		}
	}
}

// newTestApp has no users, so every browser acts as an Administrator. Pass withAdministrator
// for an app where Roles apply.
func newTestApp(t *testing.T, opts ...func(*config.Config)) *App {
	t.Helper()
	h, _ := OpenHistory("")
	cfg := config.Defaults()
	cfg.Routers[0].Levels = 3
	for _, o := range opts {
		o(&cfg)
	}
	cfg.Normalize()
	a, err := New(cfg, "", h, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	shortTimers(a)
	return a
}

// withAdministrator seeds Ada, an Administrator with PIN 2468.
func withAdministrator(cfg *config.Config) {
	h, err := config.HashPin("2468")
	if err != nil {
		panic(err)
	}
	cfg.Users = append(cfg.Users, config.User{Name: "Ada", Pin: h, Len: 4, Role: config.RoleAdministrator})
}

// moreRouter is a router to add to the settings: the defaults with this prefix, title, address and level count.
func moreRouter(prefix, title, addr string, levels int) config.Router {
	r := config.Defaults().DefaultRouter()
	r.UID, r.Prefix, r.Title, r.Address, r.Levels, r.DefaultLevels = "", prefix, title, addr, levels, ""
	return r
}

// shortTimers keeps tests that wait on an unanswered router fast, on every router of a.
func shortTimers(a *App) {
	for _, u := range a.units() {
		shortUnitTimers(u)
	}
	a.drainLimit = 2 * time.Second
	a.pinFirstWait = 20 * time.Millisecond
}

// shortUnitTimers: queueMax stays well above the others so a held reply still counts as queued, not lost.
func shortUnitTimers(u *routerUnit) {
	u.checkTimeout = 40 * time.Millisecond
	u.routeTimeout = 40 * time.Millisecond
	u.routeGrace = 40 * time.Millisecond
	u.queueMax = 500 * time.Millisecond
}

func mkUser(t *testing.T, name, pin string) config.User {
	t.Helper()
	h, err := config.HashPin(pin)
	if err != nil {
		t.Fatal(err)
	}
	return config.User{Name: name, Pin: h, Len: len(pin)}
}

// The app saves tokens.json off the lock, so a save can land when the test ends.
// A plain RemoveAll then fails with "directory not empty".
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
// (2468) and, if locks, the main and log locks on.
func userApp(t *testing.T, locks bool) *App {
	t.Helper()
	cfgPath := filepath.Join(settledTempDir(t), "config.yaml")
	cfg := config.Defaults()
	cfg.Normalize()
	cfg.Users = []config.User{mkUser(t, "Ana", "1234"), mkUser(t, "Bo", "567890")}
	withAdministrator(&cfg)
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

// adminClient is a connection signed in as Ada (withAdministrator), or a Guest on an app with
// no users, where every browser acts as an Administrator.
func adminClient(t *testing.T, a *App) *client {
	t.Helper()
	c := &client{ip: "10.0.0.9"}
	if len(a.r().cfg.Users) == 0 {
		return c
	}
	if err := a.UserUnlockAs(c, "Ada", "2468"); err != nil {
		t.Fatal(err)
	}
	grantRecheck(a, c)
	return c
}

func idOf(a *App, name string) string {
	u, _ := a.r().cfg.FindUserByName(name)
	return u.ID
}

// The PIN hash stays.
func setFlags(t *testing.T, a *App, name string, trusted, persistent bool) {
	t.Helper()
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	_, nr, err := a.update(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].Name == name {
				cfg.Users[i].Role, cfg.Users[i].Persistent = config.RoleStandard, persistent
				if trusted {
					cfg.Users[i].Role = config.RoleTrusted
				}
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

func setRole(t *testing.T, a *App, name, role string) {
	t.Helper()
	cfg := a.r().cfg.Clone()
	for i := range cfg.Users {
		if cfg.Users[i].Name == name {
			cfg.Users[i].Role = role
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

// keyTestApp: users Ana (1234) and Bo (567890), an API client PanelsBot, the Administrator Ada (2468).
func keyTestApp(t *testing.T) *App {
	t.Helper()
	a := userApp(t, false)
	cfg := a.r().cfg.Clone()
	cfg.APIClients = []config.APIClient{{Name: "PanelsBot", Scopes: []string{config.ScopeTake}}}
	if err := a.ApplyConfig(cfg); err != nil {
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

// noPinApp: apiTestApp plus Cy, a Standard user an Administrator saved with no PIN.
func noPinApp(t *testing.T) (*App, *fakeRouter) {
	t.Helper()
	a, f := apiTestApp(t, nil)
	mustSave(t, a, adminClient(t, a), map[string]any{"user/new:0/add": newUser{Name: "Cy"}})
	return a, f
}

func signInNoPin(t *testing.T, a *App, ip, name string) (*client, func() []map[string]any) {
	t.Helper()
	c := &client{ip: ip}
	next := registerAll(a, c)
	if err := a.UserUnlockAs(c, name, ""); err != nil {
		t.Fatal(err)
	}
	return c, next
}

func clientApp(t *testing.T) (*App, *fakeRouter) {
	t.Helper()
	return apiTestApp(t, func(c *config.Config) {
		cy := mkUser(t, "Cy", "4321")
		cy.Role = config.RoleTrusted
		cy.Rights = config.Rights{Sources: nums(1, 2, 3), Dsts: nums(2, 4)}
		c.Users = append(c.Users, cy)
	})
}

// ownClientApp: Ana is a Standard user with sources 1-2 and destination 2, signed in.
func ownClientApp(t *testing.T) (*App, *fakeRouter, *client, func() []map[string]any) {
	t.Helper()
	a, f := apiTestApp(t, nil)
	setRights(t, a, "Ana", config.Rights{Sources: nums(1, 2), Dsts: nums(2)})
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	return a, f, c, next
}

func rightsSignedIn(t *testing.T, a *App, name string) *client {
	t.Helper()
	u, _ := a.r().cfg.FindUserByName(name)
	c := &client{ip: "10.0.0.1", send: make(chan []byte, 32)}
	register(a, c)
	a.mu.Lock()
	c.u.user.Store(&u)
	a.mu.Unlock()
	return c
}

func stateClient(t *testing.T, st int) (*App, *client) {
	t.Helper()
	if st == stNoUsers {
		return newTestApp(t), &client{ip: "10.0.0.1"}
	}
	a := userApp(t, st == stNoAccess || st == stLocked)
	c := &client{ip: "10.0.0.1"}
	switch st {
	case stGuestTrusted:
		cfg := a.r().cfg.Clone()
		cfg.GuestRole = config.RoleTrusted
		if err := a.ApplyConfig(cfg); err != nil {
			t.Fatal(err)
		}
	case stTrusted:
		setFlags(t, a, "Ana", true, false)
	case stNoPin:
		cfg := a.r().cfg.Clone()
		cfg.Users = append(cfg.Users, config.User{Name: "Cy", Role: config.RoleAdministrator, PinNeeded: true})
		if err := a.ApplyConfig(cfg); err != nil {
			t.Fatal(err)
		}
		if err := a.UserUnlockAs(c, "Cy", ""); err != nil {
			t.Fatal(err)
		}
	}
	pin := map[int]string{stStandard: "1234", stTrusted: "1234", stAdmin: "2468", stLocked: "2468"}[st]
	if pin != "" {
		if err := a.UserUnlock(c, pin); err != nil {
			t.Fatal(err)
		}
	}
	if st == stLocked {
		a.UserLock(c)
	}
	return a, c
}

func setUsers(t *testing.T, a *App, fn func(*config.Config)) {
	t.Helper()
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.applyConfig(func(c *config.Config) error { fn(c); return nil }); err != nil {
		t.Fatal(err)
	}
}

// aKey is the one Save key of attribute attr of the first router: router/<uid>/<attr>.
func aKey(a *App, attr string) string { return "router/" + a.r().cfg.DefaultRouter().UID + "/" + attr }
