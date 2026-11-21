package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Users without a PIN (GLOSSARY.md) and PIN needed. Seam: the App's messages and the REST API.

func setGuest(t *testing.T, a *App, role string, rights config.Rights) {
	t.Helper()
	cfg := a.r().cfg.Clone()
	cfg.GuestRole, cfg.GuestRights = role, rights
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
}

func TestUserWithoutPinSignsInByName(t *testing.T) {
	a, _ := noPinApp(t)
	cy, ok := a.r().cfg.FindUserByName("Cy")
	if !ok || cy.Pin != "" || cy.Len != 0 {
		t.Fatalf("Cy %+v %v", cy, ok)
	}
	c := &client{ip: "10.0.0.5"}
	registerAll(a, c)
	if err := a.UserUnlockAs(c, "Cy", "1234"); err == nil {
		t.Fatal("a PIN signed in a user without one")
	}
	if err := a.UserUnlockAs(c, "Ana", ""); err == nil {
		t.Fatal("an empty PIN signed in Ana")
	}
	if err := a.UserUnlockAs(c, "Cy", ""); err != nil {
		t.Fatal(err)
	}
	if c.userID() != cy.ID {
		t.Fatalf("signed in as %q", c.userID())
	}
}

// Role and Rights are read when the user acts: a Guest change applies to the next action.
func TestUserWithoutPinActsNoWiderThanGuest(t *testing.T) {
	a, f := noPinApp(t)
	setRole(t, a, "Cy", config.RoleStandard)
	c, _ := signInNoPin(t, a, "10.0.0.5", "Cy")
	if r := a.role(c); r != config.RoleStandard {
		t.Fatalf("role %q", r)
	}
	setGuest(t, a, config.RoleTrusted, config.Rights{Dsts: nums(5)})
	if r := a.role(c); r != config.RoleTrusted {
		t.Fatalf("Guest role Trusted: role %q", r)
	}
	if err := admitMsg(a, c, inMsg{T: "presets"}); err != nil {
		t.Fatalf("a Trusted user action: %v", err)
	}
	if err := admitMsg(a, c, inMsg{T: "usersList"}); err == nil {
		t.Fatal("an Administrator action was let through")
	}
	n := f.sends()
	allFailed(t, takeResults(t, a, c, []routeReq{{ID: 1, Src: 1, Dst: 6, Levels: []int{1}}}), 1, "destination 7")
	if f.sends() != n {
		t.Fatal("sent")
	}
	setGuest(t, a, config.RoleStandard, config.Rights{})
	if err := admitMsg(a, c, inMsg{T: "presets"}); err == nil {
		t.Fatal("Guest role back to Standard: a Trusted user action was let through")
	}
}

// PIN needed: an Administrator with no hash acts as a user without a PIN until they set one.
func TestPinNeededAdministratorActsAsUserWithoutPin(t *testing.T) {
	a, _ := noPinApp(t)
	cfg := a.r().cfg.Clone()
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Cy" {
			cfg.Users[i].Role, cfg.Users[i].PinNeeded = config.RoleAdministrator, true
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c, next := signInNoPin(t, a, "10.0.0.5", "Cy")
	if r := a.role(c); r != config.RoleStandard {
		t.Fatalf("role %q", r)
	}
	if err := admitMsg(a, c, inMsg{T: "usersList"}); err == nil {
		t.Fatal("a PIN needed Administrator acted as one")
	}
	a.syncAccess(c)
	if ls := mrLast(next(), "lockState"); ls == nil || ls["pinNeeded"] != true {
		t.Fatalf("lockState %v", ls)
	}
	if r := send(t, a, c, next, inMsg{T: "userSetPin", ID: 1, New: "4321"}, "userSetPinResult"); r["ok"] != true {
		t.Fatalf("set: %v", r)
	}
	u, _ := a.r().cfg.FindUserByName("Cy")
	if u.PinNeeded || u.Len != 4 {
		t.Fatalf("after set %+v", u)
	}
	if r := a.role(c); r != config.RoleAdministrator {
		t.Fatalf("role after set %q", r)
	}
}

func TestUserWithoutPinOpensNoPageLock(t *testing.T) {
	a, _ := noPinApp(t)
	cfg := a.r().cfg.Clone()
	cfg.Locks.Main = true
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.5"}
	registerAll(a, c)
	err := a.UserUnlockAs(c, "Cy", "")
	if err == nil || !strings.Contains(err.Error(), "Cy") {
		t.Fatalf("sign-in under a page lock: %v", err)
	}
	if canTake(a, c) == nil {
		t.Fatal("took under the page lock")
	}

	// Signed in before the lock went on: the page lock covers them like a Guest.
	cfg.Locks.Main = false
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c2, _ := signInNoPin(t, a, "10.0.0.6", "Cy")
	cfg.Locks.Main = true
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if canTake(a, c2) == nil {
		t.Fatal("a user without a PIN passed the page lock")
	}
}

func TestUserWithoutPinHeldLockResumesByName(t *testing.T) {
	a, _ := noPinApp(t)
	c, _ := signInNoPin(t, a, "10.0.0.5", "Cy")
	a.UserLock(c)
	if canTake(a, c) == nil {
		t.Fatal("took under the held Lock")
	}
	if err := a.UserUnlockAs(c, "Ana", "1234"); err == nil {
		t.Fatal("another user resumed Cy's Lock")
	}
	if err := a.UserUnlockAs(c, "Cy", ""); err != nil {
		t.Fatal(err)
	}
	if err := canTake(a, c); err != nil {
		t.Fatalf("after resume: %v", err)
	}
}

func TestUserWithoutPinHoldsNoPersonalKeys(t *testing.T) {
	a, _ := noPinApp(t)
	c, next := signInNoPin(t, a, "10.0.0.5", "Cy")
	if r := send(t, a, c, next, inMsg{T: "keyCreate", ID: 1, KeyName: "Desk"}, "keyCreateResult"); r["ok"] != false {
		t.Fatalf("own key: %v", r)
	}
	admin := adminClient(t, a)
	anext := registerAll(a, admin)
	r := send(t, a, admin, anext, inMsg{T: "keyCreateFor", ID: 2, Owner: &keyOwner{Kind: actorUser, User: idOf(a, "Cy")}, KeyName: "Desk"}, "keyCreateForResult")
	if r["ok"] != false {
		t.Fatalf("Administrator made a key for Cy: %v", r)
	}
	if r := send(t, a, c, next, inMsg{T: "myClientSave", ID: 3, MyClient: &myClientReq{Name: "Cy desk", Scopes: []string{}}}, "myClientSaveResult"); r["ok"] != false {
		t.Fatalf("own client: %v", r)
	}
	setGuest(t, a, config.RoleStandard, config.Rights{Dsts: nums(5)})
	r = send(t, a, c, next, inMsg{T: "sessionKey", ID: 4, KeyName: "tab"}, "sessionKeyResult")
	if r["ok"] != true {
		t.Fatalf("session key: %v", r)
	}
	wantErr(t, apiPost(a, "/api/v1/takes", r["key"].(string), takeOne, nil), 403, "outside_rights")
}

func TestRemoveOwnPinEndsKeysAndClients(t *testing.T) {
	a, _, c, next := ownClientApp(t)
	if r := saveOwn(t, a, c, next, myClientReq{Name: "Ana desk", Scopes: []string{config.ScopeTake}}); r["ok"] != true {
		t.Fatalf("client: %v", r)
	}
	ownClientKey(t, a, c, next, "Ana desk")
	k := send(t, a, c, next, inMsg{T: "keyCreate", ID: 5, Pin: "1234", KeyName: "Desk"}, "keyCreateResult")
	if k["ok"] != true {
		t.Fatalf("key: %v", k)
	}
	if r := send(t, a, c, next, inMsg{T: "userSetPin", ID: 6, Old: "9999", Remove: true}, "userSetPinResult"); r["ok"] != false {
		t.Fatalf("wrong old PIN removed it: %v", r)
	}
	if r := send(t, a, c, next, inMsg{T: "userSetPin", ID: 7, Old: "1234", Remove: true}, "userSetPinResult"); r["ok"] != true {
		t.Fatalf("remove: %v", r)
	}
	if u, _ := a.r().cfg.FindUserByName("Ana"); u.Pin != "" || u.Len != 0 {
		t.Fatalf("Ana %+v", u)
	}
	wantErr(t, apiPost(a, "/api/v1/takes", k["key"].(string), takeOne, nil), 401, "key_revoked")
	if _, ok := a.r().cfg.FindAPIClient("Ana desk"); ok {
		t.Fatal("Ana's own client outlived her PIN")
	}
	if c.userID() != idOf(a, "Ana") || canTake(a, c) != nil {
		t.Fatal("this browser was signed out")
	}

	setRole(t, a, "Bo", config.RoleTrusted)
	bo, bnext := signedIn(t, a, "10.0.0.2", "567890")
	if r := send(t, a, bo, bnext, inMsg{T: "userSetPin", ID: 8, Old: "567890", Remove: true}, "userSetPinResult"); r["ok"] != false {
		t.Fatalf("a Trusted user removed their PIN: %v", r)
	}
}

func TestAdministratorResetEndsSessionsAndKeys(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	admin := adminClient(t, a)
	mustSave(t, a, admin, map[string]any{ukey(a, "Ana", "persistent"): true})
	ana, anext := signedIn(t, a, "10.0.0.1", "1234")
	tok := a.issueToken(ana, false)
	if tok == "" {
		t.Fatal("no token")
	}
	k := send(t, a, ana, anext, inMsg{T: "keyCreate", ID: 1, Pin: "1234", KeyName: "Desk"}, "keyCreateResult")
	sk := send(t, a, ana, anext, inMsg{T: "sessionKey", ID: 2, KeyName: "tab"}, "sessionKeyResult")
	if k["ok"] != true || sk["ok"] != true {
		t.Fatalf("keys %v %v", k, sk)
	}
	mustSave(t, a, admin, map[string]any{ukey(a, "Ana", "pin"): "8642"})
	if _, ok := a.tokens.user(tok, a.r().cfg); ok {
		t.Fatal("the token outlived the reset")
	}
	for _, key := range []string{k["key"].(string), sk["key"].(string)} {
		if rec := apiPost(a, "/api/v1/takes", key, takeOne, nil); rec.Code != 401 {
			t.Fatalf("a key outlived the reset: %d", rec.Code)
		}
	}
	if act, _ := a.callerOf(ana); act.Kind != actorGuest {
		t.Fatal("Ana's open browser stayed signed in")
	}

	mustSave(t, a, admin, map[string]any{ukey(a, "Ana", "pin"): ""})
	if u, _ := a.r().cfg.FindUserByName("Ana"); u.Pin != "" || u.Len != 0 {
		t.Fatalf("Ana %+v", u)
	}
}

func TestSaveRefusesTrustedUserWithoutPin(t *testing.T) {
	a, _ := noPinApp(t)
	admin := adminClient(t, a)
	for _, c := range []struct {
		what, name string
		kv         map[string]any
	}{
		{"raise a user without a PIN", "Cy", map[string]any{ukey(a, "Cy", "role"): config.RoleTrusted}},
		{"new Trusted user", "Dee", map[string]any{"user/new:0/add": newUser{Name: "Dee", Role: config.RoleTrusted}}},
		{"new Administrator", "Dee", map[string]any{"user/new:0/add": newUser{Name: "Dee", Role: config.RoleAdministrator}}},
	} {
		if r := mustRefuse(t, a, admin, c.kv); !strings.Contains(r.Err, c.name) {
			t.Errorf("%s: %+v", c.what, r)
		}
	}
	setRole(t, a, "Bo", config.RoleTrusted)
	if r := mustRefuse(t, a, admin, map[string]any{ukey(a, "Bo", "pin"): ""}); !strings.Contains(r.Err, "Bo") {
		t.Fatalf("removing a Trusted user's PIN: %+v", r)
	}
	if u, _ := a.r().cfg.FindUserByName("Bo"); u.Pin == "" {
		t.Fatal("the refused save removed Bo's PIN")
	}
}

func TestSaveKeepsPinNeededTrustedUser(t *testing.T) {
	a, _ := noPinApp(t)
	cfg := a.r().cfg.Clone()
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Cy" {
			cfg.Users[i].Role, cfg.Users[i].PinNeeded = config.RoleTrusted, true
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	mustSave(t, a, adminClient(t, a), map[string]any{ukey(a, "Ana", "name"): "Anna", ukey(a, "Cy", "role"): config.RoleTrusted})
	if u, _ := a.r().cfg.FindUserByName("Cy"); !u.PinNeeded || u.Role != config.RoleTrusted {
		t.Fatalf("Cy %+v", u)
	}
}

func TestUsersListAndHelloNameUsersWithoutPin(t *testing.T) {
	a, _ := noPinApp(t)
	l := a.usersList(adminClient(t, a), 1)
	found := false
	for _, u := range l.Users {
		if u.Name == "Cy" {
			found = u.ID != "" && u.Len == 0
		}
	}
	if !found {
		t.Fatalf("usersList %+v", l.Users)
	}
}

// The page type comes from the browser's own hi, so it must not weaken a lock. Here a user without
// a PIN says they are on the Communication Log page, and its lock is off. Under the main page lock
// they still count as nobody: no take, no Protect, no key that acts as them.
func TestUserWithoutPinOnLogPagePassesNoMainLock(t *testing.T) {
	a, f := noPinApp(t)
	setRole(t, a, "Cy", config.RoleStandard)
	cfg := a.r().cfg.Clone()
	cfg.Locks.Main, cfg.Locks.Log = true, false
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.5"}
	next := registerAll(a, c)
	a.Hi(c, "log", "")
	if err := a.UserUnlockAs(c, "Cy", ""); err != nil {
		t.Fatalf("sign-in on the unlocked Log page: %v", err)
	}
	if err := admitMsg(a, c, inMsg{T: "log"}); err != nil {
		t.Fatalf("the Log page itself: %v", err)
	}
	n := f.sends()
	if canTake(a, c) == nil {
		t.Fatal("took under the main page lock")
	}
	if f.sends() != n {
		t.Fatal("sent")
	}
	if err := admitMsg(a, c, inMsg{T: "protectSet"}); err == nil {
		t.Fatal("Protect under the main page lock")
	}
	for _, kind := range []string{"sessionKey", "keyCreate"} {
		if r := send(t, a, c, next, inMsg{T: kind, ID: 1, KeyName: "x"}, kind+"Result"); r["ok"] != false {
			t.Fatalf("%s under the main page lock: %v", kind, r)
		}
	}

}
