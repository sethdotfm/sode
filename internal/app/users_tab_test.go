package app

import "testing"

// A login with no lock on (userUnlock with tab) gets a tab token.
func TestTabSession(t *testing.T) {
	a := userApp(t, false)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if tok := a.issueSessionTab(c, false); tok != "" {
		t.Fatal("token without tab")
	}
	tok := a.issueSessionTab(c, true)
	if tok == "" || !tokenAlive(a, tok) {
		t.Fatal("no tab token")
	}
	// A reload of that tab is logged in again.
	fresh := &client{ip: "10.0.0.2"}
	a.Hi(fresh, "main", tok)
	if fresh.who() != "Ana" {
		t.Fatal("tab token not honored")
	}
	// A lock revokes it.
	a.UserLock(fresh)
	if tokenAlive(a, tok) {
		t.Fatal("lock did not revoke")
	}
	// Persistent behaviour is unchanged: tab asked, ordinary token given.
	setFlags(t, a, "Ana", false, true)
	d := &client{ip: "10.0.0.3"}
	if err := a.UserUnlock(d, "1234"); err != nil {
		t.Fatal(err)
	}
	if a.issueSessionTab(d, true) == "" {
		t.Fatal("persistent user got no token")
	}
	for _, r := range a.tokens.toks {
		if r.Tab {
			t.Fatal("persistent token marked tab")
		}
	}
}

// With a lock on, a tab token is refused (the idle lock must stay in force).
func TestTabSessionRefusedWhenLocked(t *testing.T) {
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	if a.issueSessionTab(c, true) != "" {
		t.Fatal("tab token issued for a locked page")
	}
}
