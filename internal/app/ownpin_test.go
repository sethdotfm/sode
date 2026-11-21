package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A user who changes or removes their own PIN gives the current PIN first, whatever their Role,
// session kind or Lock settings. An Administrator who changes another user's PIN does not.

func pinOf(a *App, name string) string {
	u, _ := a.r().cfg.FindUserByName(name)
	return u.Pin
}

// setPinAsks: userSetPin with no and a wrong current PIN is refused, each a wrong try in the user's pool.
func setPinAsks(t *testing.T, a *App, p *wsPeer, name, wrong string, remove bool) {
	t.Helper()
	was := pinOf(a, name)
	for i, old := range []string{"", wrong} {
		n := userPinCount(a, name)
		r := p.ask("userSetPin", 50+i, map[string]any{"old": old, "new": "8642", "remove": remove})
		if r["ok"] == true {
			t.Fatalf("%s: changed with current PIN %q: %v", name, old, r)
		}
		if userPinCount(a, name) != n+1 {
			t.Fatalf("%s: current PIN %q did not count as a wrong try", name, old)
		}
	}
	if pinOf(a, name) != was {
		t.Fatalf("%s: the PIN changed", name)
	}
}

func TestOwnPinNeedsCurrentPinForEveryRole(t *testing.T) {
	for _, role := range []string{config.RoleStandard, config.RoleTrusted, config.RoleAdministrator} {
		t.Run(role, func(t *testing.T) {
			a := userApp(t, true)
			setRole(t, a, "Ana", role)
			p := dialPeer(t, a)
			p.signIn("Ana", "1234")
			passed(t, "recheck", p.recheck("1234")) // a fresh PIN recheck is not the current PIN
			setPinAsks(t, a, p, "Ana", "0000", false)
			passed(t, "change", p.ask("userSetPin", 60, map[string]any{"old": "1234", "new": "8642"}))
			if pinOf(a, "Ana") == "" || config.CheckPin(pinOf(a, "Ana"), "1234") {
				t.Fatal("the PIN did not change")
			}
		})
	}
}

// The reported case: a Trusted user with Never sign out and No auto-lock, back by the session token.
func TestOwnPinNeedsCurrentPinInPersistentSession(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", true, true)
	lifeCommit(t, a, func(cfg *config.Config) {
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Ana" {
				cfg.Users[i].NoAutoLock = true
			}
		}
	})
	c := watched(t, a, "10.0.0.1", "1234")
	tok := a.issueSession(c)
	back := &client{ip: "10.0.0.2", send: make(chan []byte, 64)}
	a.Hi(back, "main", tok)
	if back.who() != "Ana" {
		t.Fatal("the token did not sign in")
	}
	was := pinOf(a, "Ana")
	for _, old := range []string{"", "0000"} {
		n := userPinCount(a, "Ana")
		if err := a.UserSetPin(back, old, "8642"); err == nil || userPinCount(a, "Ana") != n+1 {
			t.Fatalf("current PIN %q: %v, tries %d", old, err, userPinCount(a, "Ana"))
		}
	}
	if pinOf(a, "Ana") != was {
		t.Fatal("the PIN changed")
	}
	if err := a.UserSetPin(back, "1234", "8642"); err != nil {
		t.Fatal(err)
	}
}

func TestRemoveOwnPinNeedsCurrentPin(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	setPinAsks(t, a, p, "Ana", "0000", true)
	passed(t, "remove", p.ask("userSetPin", 60, map[string]any{"old": "1234", "remove": true}))
	if pinOf(a, "Ana") != "" {
		t.Fatal("the PIN stayed")
	}
}

// A first PIN is not a change, so it needs no current PIN.
func TestFirstPinNeedsNoCurrentPin(t *testing.T) {
	a := userApp(t, false)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users = append(cfg.Users, config.User{ID: config.NewUserID(), Name: "Cy", PinNeeded: true})
	})
	p := dialPeer(t, a)
	p.signIn("Cy", "")
	passed(t, "first PIN", p.ask("userSetPin", 60, map[string]any{"new": "8642"}))
	if !config.CheckPin(pinOf(a, "Cy"), "8642") {
		t.Fatal("no PIN set")
	}
}

func TestGuestCannotSetAPin(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	if r := p.ask("userSetPin", 60, map[string]any{"new": "8642"}); r["ok"] == true {
		t.Fatalf("a Guest set a PIN: %v", r)
	}
}

// The one Save has no current PIN, so it refuses the saver's own PIN, also with a PIN recheck.
func TestUsersSaveRefusesOwnPin(t *testing.T) {
	a := userApp(t, false)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users = append(cfg.Users, mkUser(t, "Bea", "9753"))
		cfg.Users[len(cfg.Users)-1].Role = config.RoleAdministrator
	})
	p := adminPeer(t, a)
	was := pinOf(a, "Ada")
	for i, v := range []string{"1357", ""} { // a new PIN, then removal (Bea keeps an Administrator)
		key := ukey(a, "Ada", "pin")
		f := resultFor(t, p.saveWith(10+i, p.usersVersions(20+i), map[string]any{key: v}), key)
		if f["status"] != "refused" || !strings.Contains(f["err"].(string), "Change my PIN") {
			t.Fatalf("own PIN %q: %v", v, f)
		}
	}
	if pinOf(a, "Ada") != was {
		t.Fatal("Ada's PIN changed")
	}
}

// Another user's PIN needs the Administrator's PIN recheck and never that user's current PIN.
func TestUsersSaveSetsAnotherUsersPinWithoutTheirCurrentPin(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	key := ukey(a, "Ana", "pin")
	if f := resultFor(t, p.saveWith(10, p.usersVersions(20), map[string]any{key: "1357"}), key); f["reason"] != reasonRecheck {
		t.Fatalf("without a recheck: %v", f)
	}
	passed(t, "recheck", p.recheck("2468"))
	wantSaved(t, p.saveWith(11, p.usersVersions(21), map[string]any{key: "1357"}), key)
	if !config.CheckPin(pinOf(a, "Ana"), "1357") {
		t.Fatal("Ana's PIN did not change")
	}
	wantSaved(t, p.saveWith(12, p.usersVersions(22), map[string]any{key: ""}), key)
	if pinOf(a, "Ana") != "" {
		t.Fatal("Ana's PIN stayed")
	}
}

// Stepped in, the user is the stepped-in user: their own PIN needs their current PIN, and the
// original user's PIN is another user's.
func TestSteppedInOwnPinNeedsCurrentPin(t *testing.T) {
	a := userApp(t, false)
	lifeCommit(t, a, func(cfg *config.Config) {
		cfg.Users = append(cfg.Users, mkUser(t, "Bea", "9753"))
		cfg.Users[len(cfg.Users)-1].Role = config.RoleAdministrator
	})
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	p.stepIn("Bea", "9753") // the step-in PIN counts as the PIN recheck
	key := ukey(a, "Bea", "pin")
	if f := resultFor(t, p.saveWith(10, p.usersVersions(20), map[string]any{key: "1357"}), key); f["status"] != "refused" {
		t.Fatalf("own PIN in the one Save: %v", f)
	}
	anaKey := ukey(a, "Ana", "pin")
	wantSaved(t, p.saveWith(11, p.usersVersions(21), map[string]any{anaKey: "1357"}), anaKey)
	setPinAsks(t, a, p, "Bea", "1234", false) // the original's PIN is a wrong current PIN
	passed(t, "change", p.ask("userSetPin", 60, map[string]any{"old": "9753", "new": "8642"}))
	if !config.CheckPin(pinOf(a, "Bea"), "8642") {
		t.Fatal("Bea's PIN did not change")
	}
}

// An import never writes over the importer's own PIN; other users get the file's hash.
func TestImportKeepsImportersOwnPin(t *testing.T) {
	a, c, tok := importApp(t)
	file := exported(t, a, tok, exportReq{Categories: []string{catUsers}, IncludePins: true})
	lifeCommit(t, a, func(cfg *config.Config) {
		for i, name := range map[int]string{0: "1111", 2: "3333"} { // Ana, Ada
			h, err := config.HashPin(name)
			if err != nil {
				t.Fatal(err)
			}
			cfg.Users[i].Pin = h
		}
	})
	if a.r().cfg.Users[2].Name != "Ada" {
		t.Fatalf("users: %+v", a.r().cfg.Users)
	}
	if err := a.UserUnlockAs(c, "Ada", "3333"); err != nil {
		t.Fatal(err)
	}
	tok = a.issueSessionTab(c, true)
	grantRecheck(a, c)
	adaPin := pinOf(a, "Ada")
	applied(t, postApply(t, a, tok, file, importReq{Mode: importAdd}))
	if pinOf(a, "Ada") != adaPin {
		t.Fatal("the import changed the importer's own PIN")
	}
	if !config.CheckPin(pinOf(a, "Ana"), "1234") {
		t.Fatal("Ana did not get the file's PIN")
	}
}

func TestOwnPinChange(t *testing.T) {
	cfg := config.Config{Users: []config.User{{ID: "u-1", Pin: "h", Len: 4}, {ID: "u-2"}}}
	for _, c := range []struct {
		key, self string
		want      bool
	}{
		{"user/u-1/pin", "u-1", true},
		{"user/u-1/pin", "u-2", false},
		{"user/u-2/pin", "u-2", false}, // a first PIN
		{"user/u-1/role", "u-1", false},
		{"user/u-1/pin", "", false},
	} {
		if got := ownPinChange(cfg, c.key, c.self); got != c.want {
			t.Errorf("%s as %q: %v", c.key, c.self, got)
		}
	}
}
