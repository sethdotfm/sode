package app

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Sign-in mode: PIN only is the default; Username and PIN lets users share a PIN and a
// sign-in names the user (docs/agents/users-locks.md).

func sp(s string) *string { return &s }

// usernameApp: Ana (1234) and Bo (1234, the same PIN) in Username and PIN mode.
func usernameApp(t *testing.T) *App {
	t.Helper()
	a := userApp(t, true)
	admin := adminClient(t, a)
	err := a.SaveUsers(admin, usersReq{SignInMode: config.SignInUsername, Users: []userEdit{
		{Name: "Ana", Was: "Ana", Username: sp("ana.k")},
		{Name: "Bo", Was: "Bo", Username: sp("bobby"), Pin: "1234"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestPinOnlyIsTheDefaultAndPinsStayUnique(t *testing.T) {
	a := userApp(t, true)
	if m := a.r().cfg.SignInMode; m != config.SignInPin {
		t.Fatalf("mode %q", m)
	}
	err := a.SaveUsers(adminClient(t, a), usersReq{Users: []userEdit{
		{Name: "Ana", Was: "Ana"}, {Name: "Bo", Was: "Bo"}, {Name: "Cy", Pin: "1234"},
	}})
	if err == nil {
		t.Fatal("a shared PIN was saved in PIN only mode")
	}
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil || c.who() != "Ana" {
		t.Fatalf("PIN alone: %v %q", err, c.who())
	}
	// The mode ignores a username in the message.
	c2 := &client{ip: "10.0.0.2"}
	if err := a.UserUnlockAs(c2, "nobody", "567890"); err != nil || c2.who() != "Bo" {
		t.Fatalf("PIN only with a stray username: %v %q", err, c2.who())
	}
}

func TestUsernameModeSharedPinNeedsBoth(t *testing.T) {
	a := usernameApp(t)
	for user, want := range map[string]string{"ana.k": "Ana", "BOBBY": "Bo"} {
		c := &client{ip: "10.0.0.1"}
		if err := a.UserUnlockAs(c, user, "1234"); err != nil || c.who() != want {
			t.Errorf("%s: %v %q", user, err, c.who())
		}
	}
	for name, args := range map[string][2]string{
		"PIN alone":            {"", "1234"},
		"the displayed name":   {"Ana", "1234"},
		"unknown username":     {"zed", "1234"},
		"another user's PIN":   {"ana.k", "567890"},
		"right user wrong PIN": {"bobby", "9999"},
	} {
		c := &client{ip: "10.0.0.3"}
		if err := a.UserUnlockAs(c, args[0], args[1]); err == nil || c.who() != "10.0.0.3" {
			t.Errorf("%s signed in as %q", name, c.who())
		}
	}
}

func TestUsernameUniqueIgnoringCaseAndIndependentOfName(t *testing.T) {
	a := usernameApp(t)
	admin := adminClient(t, a)
	err := a.SaveUsers(admin, usersReq{Users: []userEdit{
		{Name: "Ana", Was: "Ana"}, {Name: "Bo", Was: "Bo", Username: sp("ANA.K")},
	}})
	if err == nil {
		t.Fatal("two usernames equal ignoring case saved")
	}
	// A rename keeps the username; a username change keeps the name and the PIN.
	err = a.SaveUsers(admin, usersReq{Users: []userEdit{
		{Name: "Anna", Was: "Ana"}, {Name: "Bo", Was: "Bo", Username: sp("bo2")},
	}})
	if err != nil {
		t.Fatal(err)
	}
	users := a.r().cfg.Users
	if users[0].Name != "Anna" || users[0].Username != "ana.k" || users[1].Name != "Bo" || users[1].Username != "bo2" {
		t.Fatalf("users %+v", users)
	}
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlockAs(c, "bo2", "1234"); err != nil || c.who() != "Bo" {
		t.Fatalf("%v %q", err, c.who())
	}
}

func TestUsernameModeNeedsEveryUsername(t *testing.T) {
	a := userApp(t, true)
	err := a.SaveUsers(adminClient(t, a), usersReq{SignInMode: config.SignInUsername, Users: []userEdit{
		{Name: "Ana", Was: "Ana", Username: sp("ana")}, {Name: "Bo", Was: "Bo"},
	}})
	if err == nil {
		t.Fatal("Username and PIN accepted with a user lacking a username")
	}
	if a.r().cfg.SignInMode != config.SignInPin {
		t.Fatal("a refused save changed the mode")
	}
}

func TestChangingModeNeedsThePinAndKeepsEverything(t *testing.T) {
	a := userApp(t, true)
	req := usersReq{SignInMode: config.SignInUsername, Users: []userEdit{
		{Name: "Ana", Was: "Ana", Username: sp("ana")}, {Name: "Bo", Was: "Bo", Username: sp("bo")},
	}}
	// Over the wire the policy refuses it without the Advanced Settings PIN, and a
	// trusted user is not enough.
	setFlags(t, a, "Ana", true, false)
	ana := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(ana, "1234"); err != nil {
		t.Fatal(err)
	}
	if err := admitMsg(a, ana, inMsg{T: "users", UsersReq: &req}); err == nil {
		t.Fatal("a trusted user changed the sign-in mode")
	}
	if err := a.SaveUsers(ana, req); err == nil {
		t.Fatal("SaveUsers took the mode without the PIN")
	}
	// The Settings form can't change it either.
	cfg := a.r().cfg
	cfg.SignInMode = config.SignInUsername
	if err := a.SaveSettings(adminClient(t, a), cfg, "", false); err != nil {
		t.Fatal(err)
	}
	if a.r().cfg.SignInMode != config.SignInPin {
		t.Fatal("the Settings form changed the sign-in mode")
	}

	admin := adminClient(t, a)
	a.cfgMu.Lock()
	_, nr, err := a.update(func(c *config.Config) error {
		c.Users[0].Profile = config.Profile{Prefs: map[string]string{"theme": "dark"}}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	a.rt.Store(nr)
	a.mu.Unlock()
	if err := a.SaveUsers(admin, req); err != nil {
		t.Fatal(err)
	}
	back := usersReq{SignInMode: config.SignInPin, Users: []userEdit{{Name: "Ana", Was: "Ana"}, {Name: "Bo", Was: "Bo"}}}
	if err := a.SaveUsers(admin, back); err != nil {
		t.Fatal(err)
	}
	cfg = a.r().cfg
	if cfg.SignInMode != config.SignInPin || len(cfg.Users) != 2 || cfg.Users[0].Profile.Prefs["theme"] != "dark" || cfg.Users[0].Username != "ana" {
		t.Fatalf("after the round trip: %+v", cfg)
	}
	c := &client{ip: "10.0.0.5"}
	if err := a.UserUnlock(c, "567890"); err != nil || c.who() != "Bo" {
		t.Fatalf("PINs after the round trip: %v %q", err, c.who())
	}
}

// Back to PIN only with a PIN two users share: the PIN alone can't say who it is, so it signs nobody in.
func TestPinOnlyRefusesAnAmbiguousPin(t *testing.T) {
	a := usernameApp(t)
	back := usersReq{SignInMode: config.SignInPin, Users: []userEdit{{Name: "Ana", Was: "Ana"}, {Name: "Bo", Was: "Bo"}}}
	if err := a.SaveUsers(adminClient(t, a), back); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err == nil || c.who() != "10.0.0.1" {
		t.Fatalf("an ambiguous PIN signed in as %q", c.who())
	}
}

func TestSetPinSharingFollowsTheMode(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if err := a.UserSetPin(c, "1234", "567890"); err == nil {
		t.Fatal("PIN only: took another user's PIN")
	}
	a = usernameApp(t)
	c = &client{ip: "10.0.0.2"}
	if err := a.UserUnlockAs(c, "ana.k", "1234"); err != nil {
		t.Fatal(err)
	}
	if err := a.UserSetPin(c, "1234", "567890"); err != nil {
		t.Fatalf("Username and PIN: %v", err)
	}
	// Bo's PIN is still 1234.
	c2 := &client{ip: "10.0.0.3"}
	if err := a.UserUnlockAs(c2, "bobby", "1234"); err != nil || c2.who() != "Bo" {
		t.Fatalf("Bo: %v %q", err, c2.who())
	}
}

func TestHeldLockResumesWithThePinInUsernameMode(t *testing.T) {
	a := usernameApp(t)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlockAs(c, "ana.k", "1234"); err != nil {
		t.Fatal(err)
	}
	a.UserLock(c)
	// Bo shares Ana's PIN; neither name nor PIN may switch a held session to Bo.
	if err := a.UserUnlockAs(c, "bobby", "1234"); err == nil {
		t.Fatal("another user's username resumed a held Lock")
	}
	if err := a.UserUnlockAs(c, "", "1234"); err != nil || c.who() != "Ana" {
		t.Fatalf("resume: %v %q", err, c.who())
	}
}

// Nothing sent before the PIN carries a username.
func TestUsernamesStayOutOfPublicMessages(t *testing.T) {
	a := usernameApp(t)
	a.mu.Lock()
	hello := string(a.hello(helloPage{}))
	locked := a.lockStateLocked(&client{ip: "10.0.0.1"})
	a.mu.Unlock()
	lb, _ := json.Marshal(locked)
	for _, s := range []string{"ana.k", "bobby"} {
		if strings.Contains(hello, s) || strings.Contains(string(lb), s) {
			t.Fatalf("%q leaked: %s %s", s, hello, lb)
		}
	}
	// The sign-in screen needs the mode to know what to ask.
	if locked.Mode != config.SignInUsername {
		t.Fatalf("lockState mode %q", locked.Mode)
	}
	in := &client{ip: "10.0.0.2"}
	if err := a.UserUnlockAs(in, "ana.k", "1234"); err != nil {
		t.Fatal(err)
	}
	a.mu.Lock()
	signed := a.lockStateLocked(in)
	a.mu.Unlock()
	sb, _ := json.Marshal(signed)
	if strings.Contains(string(sb), "ana.k") || strings.Contains(string(sb), "bobby") {
		t.Fatalf("signed-in lockState carries a username: %s", sb)
	}
}

func TestUsersListCarriesUsernamesAndMode(t *testing.T) {
	a := usernameApp(t)
	r := a.usersList(7)
	b, _ := json.Marshal(r)
	if r.Mode != config.SignInUsername || r.Users[0].Username != "ana.k" || r.Users[1].Username != "bobby" || strings.Contains(string(b), "pbkdf2") {
		t.Fatalf("list %s", b)
	}
}

// userUnlock carries user next to pin; the policy keeps it open.
func TestUserUnlockMessageCarriesUsername(t *testing.T) {
	a := usernameApp(t)
	c := &client{ip: "10.0.0.1"}
	read := register(a, c)
	if err := admitMsg(a, c, inMsg{T: "userUnlock"}); err != nil {
		t.Fatalf("userUnlock needs no access: %v", err)
	}
	var m inMsg
	if err := json.Unmarshal([]byte(`{"t":"userUnlock","user":"bobby","pin":"1234","id":4}`), &m); err != nil {
		t.Fatal(err)
	}
	a.serveUserUnlock(c, &m)
	if c.who() != "Bo" {
		t.Fatalf("signed in as %q", c.who())
	}
	if st := lastLockState(t, read); st["locked"] != false || st["mode"] != "username" {
		t.Fatalf("lockState %v", st)
	}
}
