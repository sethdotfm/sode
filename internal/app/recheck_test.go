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
	if want := []string{"clearHistory", "configImport", "keyRevokeAll", "userSignOutAll"}; !slices.Equal(always, want) {
		t.Errorf("always Critical: %v, want %v", always, want)
	}
	if want := []string{"apiSettings", "config", "users"}; !slices.Equal(byFields, want) {
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

// signInRechecked is signIn plus a PIN recheck over the socket.
func (p *wsPeer) signInRechecked(name, pin string) {
	p.t.Helper()
	p.signIn(name, pin)
	if r := p.recheck(pin); r["ok"] != true {
		p.t.Fatalf("recheck: %v", r)
	}
}

// recheckClock lets a test move the App's clock forward.
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
	cfg.Address = "127.0.0.1:9"
	p.send(map[string]any{"t": "config", "config": cfg})
	passed(t, "router address with no users", p.until("configResult"))
}

// Each save path refuses a Critical field without a recheck and saves it after one. A field
// that is not Critical saves without one.
func TestSavePathsAskRecheckForCriticalFields(t *testing.T) {
	a := newTestApp(t, withAdministrator)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	ada, _ := a.r().cfg.FindUserByName("Ada")
	adaRow := map[string]any{"id": ada.ID, "name": "Ada", "role": config.RoleAdministrator}

	// The Settings form
	saveForm := func(edit func(*config.Config)) map[string]any {
		cfg := a.r().cfg.Clone()
		edit(&cfg)
		p.send(map[string]any{"t": "config", "config": cfg})
		return p.until("configResult")
	}
	passed(t, "brand text", saveForm(func(c *config.Config) { c.BrandText = "Studio" }))
	needsRecheck(t, "router address", saveForm(func(c *config.Config) { c.Address = "127.0.0.1:9" }))
	needsRecheck(t, "a page lock", saveForm(func(c *config.Config) { c.Locks.Log = true }))
	needsRecheck(t, "Wait after wrong PINs", saveForm(func(c *config.Config) { c.PinWaits = !c.PinWaits }))

	// API settings
	passed(t, "rate limit", p.ask("apiSettings", 1, map[string]any{"apiSettings": map[string]any{"rateLimit": 7}}))
	needsRecheck(t, "keyless API", p.ask("apiSettings", 2, map[string]any{"apiSettings": map[string]any{"guestEnabled": !a.r().cfg.API.GuestEnabled}}))

	// Manage users
	addBob := map[string]any{"users": []any{adaRow, map[string]any{"name": "Bob", "pin": "1357"}}}
	passed(t, "longest PIN", p.ask("users", 3, map[string]any{"usersReq": map[string]any{"users": []any{adaRow}, "maxPinLen": 9}}))
	needsRecheck(t, "add a user", p.ask("users", 4, map[string]any{"usersReq": addBob}))
	needsRecheck(t, "Guest role", p.ask("users", 5, map[string]any{"usersReq": map[string]any{"users": []any{adaRow}, "guestRole": config.RoleTrusted}}))

	passed(t, "recheck", p.recheck("2468"))
	passed(t, "router address after a recheck", saveForm(func(c *config.Config) { c.Address = "127.0.0.1:9" }))
	passed(t, "keyless API after a recheck", p.ask("apiSettings", 6, map[string]any{"apiSettings": map[string]any{"guestEnabled": !a.r().cfg.API.GuestEnabled}}))
	passed(t, "add a user after a recheck", p.ask("users", 7, map[string]any{"usersReq": addBob}))
	if _, ok := a.r().cfg.FindUserByName("Bob"); !ok {
		t.Fatal("Bob was not added")
	}
}

// The users save: each user field on the Critical list, and one that is not.
func TestUsersSaveCriticalFields(t *testing.T) {
	bobPin, _ := config.HashPin("1357")
	a := newTestApp(t, withAdministrator, func(c *config.Config) {
		c.Users = append(c.Users, config.User{Name: "Bob", Pin: bobPin, Len: 4, Role: config.RoleStandard})
		one := []config.Ref{{N: 1}}
		c.Users[0].Rights = config.Rights{Sources: &one}
	})
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	cfg := a.r().cfg
	ada, _ := cfg.FindUserByName("Ada")
	bob, _ := cfg.FindUserByName("Bob")
	row := func(u config.User, edit func(map[string]any)) []any {
		m := map[string]any{"id": u.ID, "name": u.Name, "role": u.Role, "persistent": u.Persistent}
		edit(m)
		ar := map[string]any{"id": ada.ID, "name": "Ada", "role": config.RoleAdministrator}
		return []any{ar, m}
	}
	none := func(map[string]any) {}
	cases := []struct {
		what     string
		users    []any
		critical bool
	}{
		{"rename", row(bob, func(m map[string]any) { m["name"] = "Robert" }), false},
		{"remove a user", []any{map[string]any{"id": ada.ID, "name": "Ada", "role": config.RoleAdministrator}}, true},
		{"change a role", row(bob, func(m map[string]any) { m["role"] = config.RoleTrusted }), true},
		{"set another's PIN", row(bob, func(m map[string]any) { m["pin"] = "9753" }), true},
		{"remove another's PIN", row(bob, func(m map[string]any) { m["clearPin"] = true }), true},
		{"Persistent", row(bob, func(m map[string]any) { m["persistent"] = true }), true},
		{"No auto-lock", row(bob, func(m map[string]any) { m["noAutoLock"] = true }), true},
		{"Rights within Ada's", row(bob, func(m map[string]any) { m["rights"] = map[string]any{"sources": []any{map[string]any{"n": 1}}} }), false},
		{"Rights wider than Ada's", row(bob, func(m map[string]any) { m["rights"] = map[string]any{"sources": []any{map[string]any{"n": 2}}} }), true},
		{"nothing", row(bob, none), false},
		// last: it signs this page out
		{"Ada's own PIN", []any{map[string]any{"id": ada.ID, "name": "Ada", "role": config.RoleAdministrator, "pin": "8642"}, map[string]any{"id": bob.ID, "name": "Bob", "role": config.RoleStandard}}, false},
	}
	needsRecheck(t, "Guest Rights wider than Ada's", p.ask("users", 30, map[string]any{"usersReq": map[string]any{
		"users": row(bob, none), "guestRights": map[string]any{"sources": []any{map[string]any{"n": 3}}}}}))
	needsRecheck(t, "an API client wider than Ada's Rights", p.ask("users", 31, map[string]any{"usersReq": map[string]any{
		"users": row(bob, none), "apiClients": []any{map[string]any{"name": "Wall", "scopes": []string{"tally"}}}}}))
	for i, c := range cases {
		r := p.ask("users", 10+i, map[string]any{"usersReq": map[string]any{"users": c.users}})
		if c.critical {
			needsRecheck(t, c.what, r)
		} else {
			passed(t, c.what, r)
		}
	}
}
