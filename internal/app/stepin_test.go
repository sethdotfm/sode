package app

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Step in (GLOSSARY.md): one connection acts as another user with a PIN until it steps out.
// The tests drive the policy table like the read loop does.

// peerOf registers c with room for every message and returns a reader of all of them.
func peerOf(a *App, c *client) func() []map[string]any {
	a.mu.Lock()
	c.send = make(chan []byte, 512)
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

// sendMsg runs m through admit and its handler; an admit refusal is returned.
func sendMsg(a *App, c *client, m inMsg) error {
	if err := admitMsg(a, c, m); err != nil {
		return err
	}
	policy[m.T].handle(a, c, &m)
	return nil
}

func lastOf(msgs []map[string]any, t string) map[string]any {
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i]["t"] == t {
			return msgs[i]
		}
	}
	return nil
}

// askStepIn sends stepIn and returns the stepInResult.
func askStepIn(t *testing.T, a *App, c *client, read func() []map[string]any, name, pin string) map[string]any {
	t.Helper()
	read()
	if err := sendMsg(a, c, inMsg{T: "stepIn", User: name, Pin: pin, ID: 7}); err != nil {
		t.Fatalf("stepIn not admitted: %v", err)
	}
	r := lastOf(read(), "stepInResult")
	if r == nil {
		t.Fatal("no stepInResult")
	}
	return r
}

func mustStepIn(t *testing.T, a *App, c *client, read func() []map[string]any, name, pin string) {
	t.Helper()
	if r := askStepIn(t, a, c, read, name, pin); r["ok"] != true {
		t.Fatalf("step in as %s refused: %v", name, r)
	}
}

// stepApp: Ana is persistent and signed in on the returned connection with a token; Bo has a
// PIN (567890); locks are off.
func stepApp(t *testing.T) (*App, *client, func() []map[string]any, string) {
	t.Helper()
	a := userApp(t, false)
	setFlags(t, a, "Ana", false, true)
	c := &client{ip: "10.0.0.1"}
	read := peerOf(a, c)
	a.Hi(c, "main", "")
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSession(c)
	if tok == "" {
		t.Fatal("no token for Ana")
	}
	read()
	return a, c, read, tok
}

func steppedIn(a *App, c *client) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return c.u.step != nil
}

func noTokenFor(t *testing.T, a *App, name string) {
	t.Helper()
	id := idOf(a, name)
	for _, r := range a.tokens.toks {
		if r.User == id {
			t.Fatalf("a token was issued to %s", name)
		}
	}
}

func TestStepInActsAsTheUserAndStepOutReturns(t *testing.T) {
	a, c, read, tok := stepApp(t)
	grantRecheck(a, c)
	mustStepIn(t, a, c, read, "Bo", "567890")
	if c.who() != "Bo" {
		t.Fatalf("stepped in as %q", c.who())
	}
	a.mu.Lock()
	st, rechecked := a.lockStateMu(c), a.recheckedMu(c)
	a.mu.Unlock()
	if st.User != "Bo" || !st.StepIn || st.StepGuest {
		t.Fatalf("lockState %+v", st)
	}
	if !rechecked {
		t.Fatal("the step-in PIN does not count as a PIN recheck")
	}
	noTokenFor(t, a, "Bo")

	read()
	if err := sendMsg(a, c, inMsg{T: "stepOut"}); err != nil {
		t.Fatal(err)
	}
	msgs := read()
	if e := lastOf(msgs, "stepEnded"); e == nil || e["why"] != "out" || e["note"] != nil {
		t.Fatalf("stepEnded %v", e)
	}
	if st := lastOf(msgs, "lockState"); st["user"] != "Ana" || st["stepIn"] != nil {
		t.Fatalf("lockState after Step out %v", st)
	}
	a.mu.Lock()
	sameTok, rechecked := c.u.tokenMu() == tok, a.recheckedMu(c)
	a.mu.Unlock()
	if !sameTok || c.who() != "Ana" {
		t.Fatalf("Step out did not return to Ana's session: %q", c.who())
	}
	if rechecked {
		t.Fatal("the recheck stayed after Step out")
	}
}

func TestStepInRecheckLastsOneMinute(t *testing.T) {
	a, c, read, _ := stepApp(t)
	clk := useFakeClock(a)
	mustStepIn(t, a, c, read, "Bo", "567890")
	clk.pass(time.Minute)
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.recheckedMu(c) {
		t.Fatal("the step-in recheck lasted past one minute")
	}
}

func TestStepInRefusals(t *testing.T) {
	a, c, read, _ := stepApp(t)
	if r := askStepIn(t, a, c, read, "Ana", "1234"); r["ok"] == true {
		t.Fatal("stepped in as the user already signed in")
	}
	if r := askStepIn(t, a, c, read, "bo", "111111"); r["ok"] == true {
		t.Fatal("a wrong PIN stepped in")
	}
	mustStepIn(t, a, c, read, "Bo", "567890")
	if r := askStepIn(t, a, c, read, "Ada", "2468"); r["ok"] == true || c.who() != "Bo" {
		t.Fatalf("a second step-in: %v as %q", r, c.who())
	}
	if err := sendMsg(a, c, inMsg{T: "userSignIn", User: "Ada", Pin: "2468"}); err != nil {
		t.Fatal(err)
	}
	if r := lastOf(read(), "userSignInResult"); r["ok"] == true || c.who() != "Bo" {
		t.Fatalf("a sign-in while stepped in: %v as %q", r, c.who())
	}

	// The Lock screen: a held Lock, and a page lock with nobody signed in.
	a2, c2, read2, _ := stepApp(t)
	a2.UserLock(c2)
	if r := askStepIn(t, a2, c2, read2, "Bo", "567890"); r["ok"] == true {
		t.Fatal("stepped in from the held Lock")
	}
	a3 := userApp(t, true)
	c3 := &client{ip: "10.0.0.3"}
	read3 := peerOf(a3, c3)
	a3.Hi(c3, "main", "")
	if r := askStepIn(t, a3, c3, read3, "Bo", "567890"); r["ok"] == true {
		t.Fatal("stepped in from the main page lock")
	}
	// Main page and Settings only.
	a4 := userApp(t, false)
	c4 := &client{ip: "10.0.0.4"}
	read4 := peerOf(a4, c4)
	a4.Hi(c4, "log", "")
	if r := askStepIn(t, a4, c4, read4, "Bo", "567890"); r["ok"] == true {
		t.Fatal("stepped in on the Log page")
	}
}

func TestStepInWrongPinsCountTowardTheNamesWait(t *testing.T) {
	a, c, read, _ := stepApp(t)
	a.cfgMu.Lock()
	_, nr, err := a.update(func(cfg *config.Config) error { cfg.PinWaits = true; return nil })
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	a.rt.Store(nr)
	for range pinFreeTries + 1 {
		if r := askStepIn(t, a, c, read, "Bo", "000000"); r["ok"] == true {
			t.Fatal("a wrong PIN stepped in")
		}
	}
	r := askStepIn(t, a, c, read, "Bo", "567890")
	if e, _ := r["err"].(string); r["ok"] == true || !strings.Contains(e, "try again in") {
		t.Fatalf("a right PIN during the wait: %v", r)
	}
	// The wait is Bo's: a sign-in as Bo elsewhere waits too.
	other := &client{ip: "10.0.0.8"}
	if err := a.UserUnlockAs(other, "Bo", "567890"); err == nil {
		t.Fatal("Bo's sign-in did not wait")
	}
}

func TestStepInEndsWhenIdle(t *testing.T) {
	a, c, read, tok := stepApp(t)
	clk := useFakeClock(a)
	setAutoLock(t, a, 10, "")
	mustStepIn(t, a, c, read, "Bo", "567890")
	for range 3 { // twelve minutes with activity: the step-in and Ana's session stay
		clk.pass(4 * time.Minute)
		ping(t, a, c)
		a.idleSweep()
	}
	if !steppedIn(a, c) {
		t.Fatal("the step-in ended although the tab was active")
	}
	if a.tokens.lastSeen(tok).Before(clk.now()) {
		t.Fatal("step-in activity did not count for Ana's session")
	}
	read()
	clk.pass(5 * time.Minute)
	a.idleSweep()
	if steppedIn(a, c) || c.who() != "Ana" {
		t.Fatalf("after 5 idle minutes: stepped in %v as %q", steppedIn(a, c), c.who())
	}
	msgs := read()
	if e := lastOf(msgs, "stepEnded"); e == nil || e["why"] != "idle" {
		t.Fatalf("stepEnded %v", e)
	}
	if err := canTake(a, c); err != nil {
		t.Fatalf("the idle end locked Ana: %v", err)
	}
}

func TestStepInIdleIgnoresNoAutoLock(t *testing.T) {
	a, c, read, _ := stepApp(t)
	clk := useFakeClock(a)
	setAutoLock(t, a, 0, "")
	mustStepIn(t, a, c, read, "Bo", "567890")
	clk.pass(5 * time.Minute)
	a.idleSweep()
	if steppedIn(a, c) {
		t.Fatal("no auto-lock extended the step-in")
	}
}

func TestStepInShorterAutoLockWins(t *testing.T) {
	a, c, read, tok := stepApp(t)
	clk := useFakeClock(a)
	setAutoLock(t, a, 3, "")
	mustStepIn(t, a, c, read, "Bo", "567890")
	clk.pass(3 * time.Minute)
	a.idleSweep()
	if steppedIn(a, c) {
		t.Fatal("the step-in outlasted Ana's auto-lock")
	}
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("Ana's auto-lock did not lock: %v", err)
	}
	if !a.tokens.locked(tok) {
		t.Fatal("the Lock is not on Ana's token")
	}
}

func TestStepInLockLocksTheOriginalEverywhere(t *testing.T) {
	a, c, read, tok := stepApp(t)
	sib := &client{ip: "10.0.0.2"}
	peerOf(a, sib)
	a.Hi(sib, "main", tok)
	mustStepIn(t, a, c, read, "Bo", "567890")
	if err := sendMsg(a, c, inMsg{T: "userLock"}); err != nil {
		t.Fatal(err)
	}
	if steppedIn(a, c) {
		t.Fatal("the Lock did not end the step-in")
	}
	if e := lastOf(read(), "stepEnded"); e == nil || e["why"] != "lock" {
		t.Fatalf("stepEnded %v", e)
	}
	for _, cl := range []*client{c, sib} {
		if err := canTake(a, cl); !errors.Is(err, errPageLocked) {
			t.Fatalf("%s is not locked: %v", cl.ip, err)
		}
	}
	if !a.tokens.locked(tok) {
		t.Fatal("the Lock is not on Ana's token")
	}
}

func TestLockOfTheOriginalElsewhereEndsTheStepIn(t *testing.T) {
	a, c, read, tok := stepApp(t)
	sib := &client{ip: "10.0.0.2"}
	peerOf(a, sib)
	a.Hi(sib, "main", tok)
	mustStepIn(t, a, c, read, "Bo", "567890")
	a.UserLock(sib)
	if steppedIn(a, c) {
		t.Fatal("a Lock of Ana's session left the step-in")
	}
	if err := canTake(a, c); !errors.Is(err, errPageLocked) {
		t.Fatalf("the stepped-in tab did not land on the Lock: %v", err)
	}
}

func TestStepOutAfterTheOriginalSignedOut(t *testing.T) {
	a, c, read, tok := stepApp(t)
	sib := &client{ip: "10.0.0.2"}
	peerOf(a, sib)
	a.Hi(sib, "main", tok)
	mustStepIn(t, a, c, read, "Bo", "567890")
	if err := sendMsg(a, sib, inMsg{T: "userSignOut"}); err != nil {
		t.Fatal(err)
	}
	if !steppedIn(a, c) || c.who() != "Bo" {
		t.Fatal("Ana's sign-out ended the step-in")
	}
	read()
	if err := sendMsg(a, c, inMsg{T: "stepOut"}); err != nil {
		t.Fatal(err)
	}
	msgs := read()
	if e := lastOf(msgs, "stepEnded"); e == nil || e["note"] == nil {
		t.Fatalf("no status text: %v", e)
	}
	if st := lastOf(msgs, "lockState"); st["user"] != nil {
		t.Fatalf("Step out did not land signed out: %v", st)
	}
}

func TestStepOutAfterTheOriginalSignedOutUnderAPageLock(t *testing.T) {
	a := userApp(t, true)
	setFlags(t, a, "Ana", false, true)
	c := &client{ip: "10.0.0.1"}
	read := peerOf(a, c)
	a.Hi(c, "main", "")
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSession(c)
	mustStepIn(t, a, c, read, "Bo", "567890")
	a.cfgMu.Lock()
	a.tokens.dropUser(idOf(a, "Ana"))
	a.cfgMu.Unlock()
	if tokenAlive(a, tok) {
		t.Fatal("token still alive")
	}
	read()
	if err := sendMsg(a, c, inMsg{T: "stepOut"}); err != nil {
		t.Fatal(err)
	}
	if st := lastOf(read(), "lockState"); st["user"] != nil || st["locked"] != true {
		t.Fatalf("Step out did not land on the PIN pad: %v", st)
	}
}

func TestStepInEndsOnASaveThatChangesTheUser(t *testing.T) {
	for _, tc := range []struct {
		what string
		edit func(a *App) []userEdit
		ends bool
	}{
		{"delete", func(a *App) []userEdit {
			return []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Persistent: true}}
		}, true},
		{"rename", func(a *App) []userEdit {
			return []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Persistent: true}, {ID: idOf(a, "Bo"), Name: "Bob", Role: config.RoleTrusted}}
		}, true},
		{"PIN reset", func(a *App) []userEdit {
			return []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Persistent: true}, {ID: idOf(a, "Bo"), Name: "Bo", Pin: "222222", Role: config.RoleTrusted}}
		}, true},
		{"Role lowered", func(a *App) []userEdit {
			return []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Persistent: true}, {ID: idOf(a, "Bo"), Name: "Bo"}}
		}, true},
		{"Persistent", func(a *App) []userEdit {
			return []userEdit{{ID: idOf(a, "Ana"), Name: "Ana", Persistent: true}, {ID: idOf(a, "Bo"), Name: "Bo", Role: config.RoleTrusted, Persistent: true}}
		}, false},
	} {
		t.Run(tc.what, func(t *testing.T) {
			a, c, read, _ := stepApp(t)
			setFlags(t, a, "Bo", true, false)
			mustStepIn(t, a, c, read, "Bo", "567890")
			if err := saveUsers(a, adminClient(t, a), usersReq{Users: tc.edit(a)}); err != nil {
				t.Fatal(err)
			}
			if steppedIn(a, c) == tc.ends {
				t.Fatalf("stepped in after the save: %v", steppedIn(a, c))
			}
			if !tc.ends {
				noTokenFor(t, a, "Bo") // Persistent ticked: catchUpSessions skips a step-in
				return
			}
			if c.who() != "Ana" {
				t.Fatalf("returned as %q", c.who())
			}
			if e := lastOf(read(), "stepEnded"); e == nil || e["why"] != "save" {
				t.Fatalf("stepEnded %v", e)
			}
		})
	}
}

func TestStepInFromAGuestBrowser(t *testing.T) {
	a := userApp(t, false)
	c := &client{ip: "10.0.0.1"}
	read := peerOf(a, c)
	a.Hi(c, "main", "")
	mustStepIn(t, a, c, read, "Bo", "567890")
	a.mu.Lock()
	st := a.lockStateMu(c)
	a.mu.Unlock()
	if !st.StepIn || !st.StepGuest || st.User != "Bo" {
		t.Fatalf("lockState %+v", st)
	}
	if err := sendMsg(a, c, inMsg{T: "userLock"}); err != nil {
		t.Fatal(err)
	}
	if steppedIn(a, c) || c.who() != c.ip {
		t.Fatalf("the Lock did not return to Guest: %q", c.who())
	}
	noTokenFor(t, a, "Bo")
}

func TestStepInFromAUserWithoutAPin(t *testing.T) {
	a := userApp(t, false)
	a.cfgMu.Lock()
	_, nr, err := a.update(func(cfg *config.Config) error {
		for i := range cfg.Users {
			if cfg.Users[i].Name == "Ana" {
				cfg.Users[i].Pin, cfg.Users[i].Len, cfg.Users[i].Role = "", 0, config.RoleStandard
			}
		}
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	a.rt.Store(nr)
	c := &client{ip: "10.0.0.1"}
	read := peerOf(a, c)
	a.Hi(c, "main", "")
	if err := a.UserUnlockAs(c, "Ana", ""); err != nil {
		t.Fatal(err)
	}
	mustStepIn(t, a, c, read, "Bo", "567890")
	read()
	if err := sendMsg(a, c, inMsg{T: "stepOut"}); err != nil {
		t.Fatal(err)
	}
	if e := lastOf(read(), "stepEnded"); c.who() != "Ana" || e == nil || e["note"] != nil {
		t.Fatalf("Step out returned to %q with %v", c.who(), e)
	}
}

// Reload, page close and a dropped connection end the step-in with the connection: the next
// connection says hi with Ana's token and is Ana.
func TestStepInEndsWithTheConnection(t *testing.T) {
	a, c, read, tok := stepApp(t)
	setFlags(t, a, "Bo", false, true)
	mustStepIn(t, a, c, read, "Bo", "567890")
	a.catchUpSessions()
	noTokenFor(t, a, "Bo")
	if lastOf(read(), "session") != nil {
		t.Fatal("a session token reached the stepped-in tab")
	}
	if err := sendMsg(a, c, inMsg{T: "sessionKey"}); err != nil {
		t.Fatal(err)
	}
	noTokenFor(t, a, "Bo")
	dropClient(a, c)
	fresh := &client{ip: "10.0.0.1"}
	peerOf(a, fresh)
	a.Hi(fresh, "main", tok)
	if fresh.who() != "Ana" || steppedIn(a, fresh) {
		t.Fatalf("the reload is %q", fresh.who())
	}
}
