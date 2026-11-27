package app

import (
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Every rule says whether it is a Critical action, never Critical, or Critical by the fields
// it changes. The Critical list is closed: a change to it changes this test.
func TestPolicyClassifiesEveryMessageCritical(t *testing.T) {
	var always, byFields []string
	for m, r := range policy {
		switch r.crit {
		case critNever:
		case critAlways:
			always = append(always, m)
		case critByFields:
			byFields = append(byFields, m)
		default:
			t.Errorf("policy rule %q does not say whether it is Critical", m)
		}
	}
	slices.Sort(always)
	slices.Sort(byFields)
	if want := []string{"clearHistory", "keyRevokeAll", "protectClearAll", "userSignOutAll"}; !slices.Equal(always, want) {
		t.Errorf("always Critical: %v, want %v", always, want)
	}
	if want := []string{"config", "settingsSave"}; !slices.Equal(byFields, want) {
		t.Errorf("Critical by fields: %v, want %v", byFields, want)
	}
}

// grantRecheck gives c's signed-in user a PIN recheck, for tests that save Critical fields.
// The test-only sign-ins (UserUnlock, adminClient) call it; the tests here do not.
func grantRecheck(a *App, c *client) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if u := c.u.user.Load(); u != nil {
		c.u.recheck = recheck{user: u.ID, hash: u.Pin, until: a.now().Add(time.Hour)}
	}
}

func (p *wsPeer) signInRechecked(name, pin string) {
	p.t.Helper()
	p.signIn(name, pin)
	if r := p.recheck(pin); r["ok"] != true {
		p.t.Fatalf("recheck: %v", r)
	}
}

func recheckClock(a *App) *atomic.Int64 {
	var ahead atomic.Int64
	a.now = func() time.Time { return time.Now().Add(time.Duration(ahead.Load())) }
	return &ahead
}

func needsRecheck(t *testing.T, what string, r map[string]any) {
	t.Helper()
	if r["ok"] == true || r["recheck"] != true {
		t.Fatalf("%s: want a refusal that needs a PIN recheck, got %v", what, r)
	}
	if e, _ := r["err"].(string); !strings.Contains(e, "needs PIN recheck") {
		t.Fatalf("%s: refusal %q does not say it needs a PIN recheck", what, e)
	}
}

func passed(t *testing.T, what string, r map[string]any) {
	t.Helper()
	if r["ok"] != true {
		t.Fatalf("%s: refused: %v", what, r)
	}
}

func (p *wsPeer) recheck(pin string) map[string]any {
	p.t.Helper()
	return p.ask("pinRecheck", 900, map[string]any{"pin": pin})
}

func TestCriticalActionNeedsRecheckForOneMinute(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	ahead := recheckClock(a)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")

	p.send(map[string]any{"t": "clearHistory"}) // the page waits for a result with no id
	needsRecheck(t, "clear History", p.until("clearHistoryResult"))
	if r := p.recheck("1111"); r["ok"] == true {
		t.Fatalf("a wrong recheck PIN passed: %v", r)
	}
	needsRecheck(t, "after a wrong recheck", p.ask("keyRevokeAll", 2, map[string]any{"owner": map[string]any{"user": "x"}}))
	passed(t, "recheck", p.recheck("2468"))
	passed(t, "Revoke all keys after a recheck", p.ask("keyRevokeAll", 3, map[string]any{"owner": map[string]any{"user": "x"}}))
	p.send(map[string]any{"t": "clearHistory"})
	p.until("historyCleared")

	ahead.Store(int64(61 * time.Second))
	needsRecheck(t, "after one minute", p.ask("keyRevokeAll", 5, map[string]any{"owner": map[string]any{"user": "x"}}))
}

func TestLockEndsRecheck(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	passed(t, "recheck", p.recheck("2468"))
	p.send(map[string]any{"t": "userLock"})
	p.until("lockState")
	p.signIn("", "2468") // the held Lock resumes with the holder's PIN
	needsRecheck(t, "after a Lock", p.ask("userSignOutAll", 1, map[string]any{"name": "Ada"}))
}

// The recheck belongs to the connection: a page opened again on the same session token
// starts without one.
func TestPageCloseEndsRecheck(t *testing.T) {
	a := newTestApp(t, withAdministrator, func(c *config.Config) { c.Users[0].Persistent = true })
	p := dialPeer(t, a)
	p.send(map[string]any{"t": "userSignIn", "user": "Ada", "pin": "2468", "id": 1})
	tok, _ := p.until("userSignInResult")["token"].(string)
	if tok == "" {
		t.Fatal("no session token")
	}
	passed(t, "recheck", p.recheck("2468"))
	p.ws.CloseNow()

	q := dialPeer(t, a)
	q.send(map[string]any{"t": "hi", "page": "main", "token": tok})
	if m := q.until("lockState"); m["user"] != "Ada" {
		t.Fatalf("the token did not sign the new page in: %v", m)
	}
	needsRecheck(t, "on the page opened again", q.ask("keyRevokeAll", 2, map[string]any{"owner": map[string]any{"user": "x"}}))
}

// Wrong recheck PINs count toward the signed-in user's name, as a sign-in does.
func TestWrongRechecksCountTowardTheNamesWait(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	a.pinFirstWait = time.Minute
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	for range pinFreeTries + 1 {
		p.recheck("0000")
	}
	if r := p.recheck("2468"); r["ok"] == true || !strings.Contains(r["err"].(string), "try again in") {
		t.Fatalf("a right PIN during the wait: %v", r)
	}
	q := dialPeer(t, a)
	q.send(map[string]any{"t": "userSignIn", "user": "Ada", "pin": "2468", "id": 1})
	if r := q.until("userSignInResult"); r["ok"] == true {
		t.Fatalf("sign-in as Ada passed during her wait: %v", r)
	}
}

func TestNoRecheckWithoutUsers(t *testing.T) {
	a := newTestApp(t)
	p := dialPeer(t, a)
	passed(t, "Revoke all keys with no users", p.ask("keyRevokeAll", 1, map[string]any{"owner": map[string]any{"user": "x"}}))
	cfg := a.r().cfg.Clone()
	cfg.Routers[0].Address = "127.0.0.1:9"
	p.send(map[string]any{"t": "config", "config": cfg})
	passed(t, "router address with no users", p.until("configResult"))
}

// Each save path refuses a Critical field without a recheck and saves it after one. A field
// that is not Critical saves without one.
func TestSavePathsAskRecheckForCriticalFields(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	api := func(edit func(*config.APISettings)) map[string]any {
		s := a.r().cfg.API
		edit(&s)
		return map[string]any{"api": s}
	}
	saveForm := func(edit func(*config.Config)) map[string]any {
		cfg := a.r().cfg.Clone()
		edit(&cfg)
		p.send(map[string]any{"t": "config", "config": cfg})
		return p.until("configResult")
	}
	passed(t, "brand text", saveForm(func(c *config.Config) { c.BrandText = "Studio" }))
	needsRecheck(t, "router address", saveForm(func(c *config.Config) { c.Routers[0].Address = "127.0.0.1:9" }))
	needsRecheck(t, "a page lock", saveForm(func(c *config.Config) { c.Locks.Log = true }))
	needsRecheck(t, "Wait after wrong PINs", saveForm(func(c *config.Config) { c.PinWaits = !c.PinWaits }))
	needsRecheck(t, "Guest role", saveForm(func(c *config.Config) { c.GuestRole = config.RoleTrusted }))

	wantSaved(t, p.saveLive(a, 1, api(func(s *config.APISettings) { s.RateLimit = 7 })), "api")
	keyless := func(s *config.APISettings) { s.GuestEnabled = !s.GuestEnabled }
	wantRefused(t, p.saveLive(a, 2, api(keyless)), "api", reasonRecheck)

	addBob := map[string]any{"user/new:0/add": map[string]any{"name": "Bob", "pin": "1357"}}
	wantSaved(t, p.saveLive(a, 3, map[string]any{"maxPinLen": 9}), "maxPinLen")
	wantRefused(t, p.saveLive(a, 4, addBob), "user/new:0/add", reasonRecheck)

	passed(t, "recheck", p.recheck("2468"))
	passed(t, "router address after a recheck", saveForm(func(c *config.Config) { c.Routers[0].Address = "127.0.0.1:9" }))
	wantSaved(t, p.saveLive(a, 6, api(keyless)), "api")
	wantSaved(t, p.saveLive(a, 7, addBob), "user/new:0/add")
	if _, ok := a.r().cfg.FindUserByName("Bob"); !ok {
		t.Fatal("Bob was not added")
	}
}

func TestUsersSaveCriticalFields(t *testing.T) {
	bobPin, _ := config.HashPin("1357")
	a := newTestApp(t, withAdministrator, func(c *config.Config) {
		c.Users = append(c.Users, config.User{Name: "Bob", Pin: bobPin, Len: 4, Role: config.RoleStandard})
		one := []config.Ref{{N: 1}}
		c.Users[0].Rights = config.Rights{Sources: &one}
	})
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	bob := "user/" + idOf(a, "Bob") + "/"
	src := func(n int) map[string]any { return map[string]any{"sources": []any{map[string]any{"n": n}}} }
	cases := []struct {
		what     string
		key      string
		value    any
		critical bool
	}{
		{"rename", bob + "name", "Robert", false},
		{"remove a user", bob + "remove", true, true},
		{"change a role", bob + "role", config.RoleTrusted, true},
		{"set another's PIN", bob + "pin", "9753", true},
		{"remove another's PIN", bob + "pin", "", true},
		{"Persistent", bob + "persistent", true, true},
		{"No auto-lock", bob + "noAutoLock", true, true},
		{"Rights within Ada's", bob + "rights", src(1), false},
		{"Rights wider than Ada's", bob + "rights", src(2), true},
		{"nothing", bob + "name", "Robert", false},
		{"Guest Rights wider than Ada's", "guestRights", src(3), true},
		{"an API client wider than Ada's Rights", "client/Wall/add", map[string]any{"scopes": []string{"tally"}}, true},
	}
	for i, c := range cases {
		r := p.saveLive(a, 10+i, map[string]any{c.key: c.value})
		if c.critical {
			wantRefused(t, r, c.key, reasonRecheck)
		} else {
			allSaved(t, c.what, r)
		}
	}
	own := ukey(a, "Ada", "pin") // only Change my PIN, which asks for the current PIN (ownpin_test.go)
	wantRefused(t, p.saveLive(a, 99, map[string]any{own: "8642"}), own, reasonOwnPin)
}

// Manage users sends the loaded API clients and Guest Rights back as they came, and Settings > API the
// keyless scopes. Untouched, they are no Critical change, even in another order.
func TestUsersSaveEchoNeedsNoRecheck(t *testing.T) {
	a := newTestApp(t, withAdministrator, func(c *config.Config) {
		two := []config.Ref{{N: 1}, {N: 2}}
		c.Users[0].Rights = config.Rights{Sources: &[]config.Ref{{N: 1}}}
		c.GuestRights = config.Rights{Sources: &two}
		c.GuestAPIScopes = []string{"tally", "names"}
		c.APIClients = []config.APIClient{{Name: "Wall", Scopes: []string{"tally"}, Rights: config.Rights{Dsts: &two}}}
	})
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	list := p.ask("usersList", 1, nil)
	back := map[string]any{"guestRights": list["guestRights"]}
	for _, u := range list["users"].([]any) {
		row := u.(map[string]any)
		for _, at := range []string{"name", "role", "persistent", "rights", "noAutoLock"} {
			back["user/"+row["id"].(string)+"/"+at] = row[at]
		}
	}
	for _, c := range list["apiClients"].([]any) {
		cl := c.(map[string]any)
		back["client/"+cl["name"].(string)+"/scopes"] = cl["scopes"]
		back["client/"+cl["name"].(string)+"/rights"] = cl["rights"]
	}
	allSaved(t, "the loaded lists sent back", p.saveLive(a, 2, back))
	scopes := list["guestScopes"].([]any)
	slices.Reverse(scopes)
	allSaved(t, "the keyless scopes in another order", p.saveLive(a, 3, map[string]any{"guestApiScopes": scopes}))
}
