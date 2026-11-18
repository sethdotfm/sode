package app

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
	"time"
)

// One sign-in: a name, then that user's PIN. PINs need not be unique (docs/agents/users-locks.md).

// sharedPinApp: Ana and Bo share PIN 1234; Ada (Administrator) has 2468.
func sharedPinApp(t *testing.T) *App {
	t.Helper()
	a := userApp(t, true)
	err := saveUsers(a, adminClient(t, a), usersReq{Users: []userEdit{
		{ID: idOf(a, "Ana"), Name: "Ana"},
		{ID: idOf(a, "Bo"), Name: "Bo", Pin: "1234"},
	}})
	if err != nil {
		t.Fatalf("a shared PIN was refused: %v", err)
	}
	return a
}

func TestSharedPinSignsInTheNamedUser(t *testing.T) {
	a := sharedPinApp(t)
	for user, want := range map[string]string{"ana": "Ana", "BO": "Bo"} {
		c := &client{ip: "10.0.0.1"}
		if err := a.UserUnlockAs(c, user, "1234"); err != nil || c.who() != want {
			t.Errorf("%s: %v %q", user, err, c.who())
		}
	}
	for name, args := range map[string][2]string{
		"PIN alone":            {"", "1234"},
		"unknown name":         {"zed", "1234"},
		"another user's PIN":   {"Ana", "567890"},
		"right user wrong PIN": {"Bo", "9999"},
	} {
		c := &client{ip: "10.0.0.3"}
		if err := a.UserUnlockAs(c, args[0], args[1]); err == nil || c.who() != "10.0.0.3" {
			t.Errorf("%s signed in as %q", name, c.who())
		}
	}
}

// A unique PIN typed with no name signs nobody in: there is no PIN only sign-in.
func TestPinAloneSignsNobodyIn(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlockAs(c, "", "567890"); err == nil || c.who() != "10.0.0.1" {
		t.Fatalf("PIN alone signed in as %q", c.who())
	}
}

func TestSetPinMayShareAnotherUsersPin(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.2"}
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatal(err)
	}
	if err := a.UserSetPin(c, "1234", "567890"); err != nil {
		t.Fatalf("took Bo's PIN: %v", err)
	}
	c2 := &client{ip: "10.0.0.3"}
	if err := a.UserUnlockAs(c2, "Bo", "567890"); err != nil || c2.who() != "Bo" {
		t.Fatalf("Bo: %v %q", err, c2.who())
	}
}

func TestHeldLockResumesOnlyWithTheHoldersPin(t *testing.T) {
	a := sharedPinApp(t)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatal(err)
	}
	a.UserLock(c)
	// Bo shares Ana's PIN; the name and the PIN must not switch a held session to Bo.
	if err := a.UserUnlockAs(c, "Bo", "1234"); err == nil {
		t.Fatal("another user's name resumed a held Lock")
	}
	if err := a.UserUnlockAs(c, "", "1234"); err != nil || c.who() != "Ana" {
		t.Fatalf("resume: %v %q", err, c.who())
	}
}

// The hello reaches locked pages, so it carries each user's name and PIN length and nothing else.
func TestUsernamesStayOutOfPublicMessages(t *testing.T) {
	a := userApp(t, true)
	a.mu.Lock()
	b := a.hello(helloPage{})
	a.mu.Unlock()
	var h struct {
		Users []map[string]any `json:"users"`
		Cfg   struct {
			Users []any `json:"users"`
		} `json:"cfg"`
	}
	if err := json.Unmarshal(b, &h); err != nil {
		t.Fatal(err)
	}
	if len(h.Cfg.Users) != 0 {
		t.Fatalf("cfg.users in the hello: %v", h.Cfg.Users)
	}
	want := []map[string]any{{"name": "Ana", "len": 4.0}, {"name": "Bo", "len": 6.0}, {"name": "Ada", "len": 4.0}}
	if len(h.Users) != len(want) {
		t.Fatalf("hello users %v", h.Users)
	}
	for i, u := range h.Users {
		keys := []string{}
		for k := range u {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		if !slices.Equal(keys, []string{"len", "name"}) || u["name"] != want[i]["name"] || u["len"] != want[i]["len"] {
			t.Fatalf("hello user %d: %v", i, u)
		}
	}
	for _, bad := range []string{`"u-`, `"role"`, `"administrator"`, `"trusted"`, `"persistent"`, `"noAutoLock"`, `"rights"`, "pbkdf2", `"signInMode"`} {
		if strings.Contains(string(b), bad) {
			t.Fatalf("hello mentions %s: %s", bad, b)
		}
	}
}

// A users save sends every open page a fresh hello, so a locked page's name list follows it.
func TestUsersSaveSendsAFreshHello(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.7", send: make(chan []byte, 64)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	err := saveUsers(a, adminClient(t, a), usersReq{Users: []userEdit{
		{ID: idOf(a, "Ana"), Name: "Anna"}, {ID: idOf(a, "Bo"), Name: "Bo"}, {Name: "Cy", Pin: "13579"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for {
		select {
		case b := <-c.send:
			var m struct {
				T     string `json:"t"`
				Users []struct {
					Name string `json:"name"`
					Len  int    `json:"len"`
				} `json:"users"`
			}
			json.Unmarshal(b, &m)
			if m.T != "hello" {
				continue
			}
			if len(m.Users) != 4 || m.Users[0].Name != "Anna" || m.Users[2].Name != "Cy" || m.Users[2].Len != 5 {
				t.Fatalf("hello users %+v", m.Users)
			}
			return
		default:
			t.Fatal("no hello after the users save")
		}
	}
}

// The Users editor gets each user's hidden id, so a save keeps the user through a rename.
func TestUsersListCarriesIDs(t *testing.T) {
	a := userApp(t, true)
	r := a.usersList(&client{}, 7)
	b, _ := json.Marshal(r)
	if r.Users[0].ID != idOf(a, "Ana") || r.Users[0].ID == "" || strings.Contains(string(b), "pbkdf2") || strings.Contains(string(b), "signInMode") {
		t.Fatalf("list %s", b)
	}
}

// userUnlock carries user next to pin; the policy keeps it open.
func TestUserUnlockMessageCarriesName(t *testing.T) {
	a := sharedPinApp(t)
	c := &client{ip: "10.0.0.1"}
	read := register(a, c)
	if err := admitMsg(a, c, inMsg{T: "userUnlock"}); err != nil {
		t.Fatalf("userUnlock needs no access: %v", err)
	}
	var m inMsg
	if err := json.Unmarshal([]byte(`{"t":"userUnlock","user":"bo","pin":"1234","id":4}`), &m); err != nil {
		t.Fatal(err)
	}
	a.serveUserUnlock(c, &m)
	if c.who() != "Bo" {
		t.Fatalf("signed in as %q", c.who())
	}
	if st := lastLockState(t, read); st["locked"] != false || st["mode"] != nil || st["names"] != nil {
		t.Fatalf("lockState %v", st)
	}
}

// An unknown name must cost about what a known one does, or response time tells
// which names exist.
func TestUnknownNameCostsAsMuchAsAKnownOne(t *testing.T) {
	a := sharedPinApp(t)
	try := func(user string) time.Duration {
		return fastestOf(t, a, func() error { return a.UserUnlockAs(&client{ip: "10.0.0.9"}, user, "9999") })
	}
	known, unknown := try("Ana"), try("nobody")
	if unknown < known/2 {
		t.Fatalf("unknown name answered in %v, a known one in %v", unknown, known)
	}
}

// PIN lengths are public (the hello). So a wrong-length PIN for a known name must
// still pay the hash cost, or a fast answer says the name exists.
func TestWrongLengthPinCostsAsMuchAsARightLengthOne(t *testing.T) {
	a := sharedPinApp(t)
	try := func(user, pin string) time.Duration {
		return fastestOf(t, a, func() error { return a.UserUnlockAs(&client{ip: "10.0.0.9"}, user, pin) })
	}
	short, unknown := try("Ana", "99"), try("nobody", "9999")
	if short < unknown/2 {
		t.Fatalf("a wrong-length PIN answered in %v, an unknown name in %v", short, unknown)
	}
}

// fastestOf times a refused sign-in several times and keeps the fastest try. Test hash
// costs make one try well under a millisecond, so one slow schedule would decide a single try.
func fastestOf(t *testing.T, a *App, signIn func() error) time.Duration {
	t.Helper()
	best := time.Duration(1<<63 - 1)
	for range 15 {
		// Wrong PINs lock the pool after a few tries, and a locked pool answers before any hash.
		a.mu.Lock()
		a.userPin.pinState = pinState{}
		a.mu.Unlock()
		start := time.Now()
		if signIn() == nil {
			t.Fatal("a wrong PIN was accepted")
		}
		best = min(best, time.Since(start))
	}
	return best
}
