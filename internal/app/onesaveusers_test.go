package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// usersVersions asks for the user list and returns its field versions, as Manage users loads them.
func (p *wsPeer) usersVersions(id int) map[string]any {
	p.t.Helper()
	r := p.ask("usersList", id, nil)
	v, _ := r["versions"].(map[string]any)
	if v == nil {
		p.t.Fatalf("usersList has no versions: %v", r)
	}
	return v
}

func (p *wsPeer) saveWith(id int, vers map[string]any, set map[string]any) map[string]any {
	p.t.Helper()
	fields := map[string]any{}
	for k, v := range set {
		fields[k] = map[string]any{"value": v, "version": vers[k]}
	}
	return p.ask("settingsSave", id, map[string]any{"fields": fields})
}

func adminPeer(t *testing.T, a *App) *wsPeer {
	t.Helper()
	p := dialPeer(t, a)
	p.signIn("Ada", "2468")
	passed(t, "recheck", p.recheck("2468"))
	return p
}

func TestOneSaveUsersRightsAndClientTogether(t *testing.T) {
	a := userApp(t, false)
	cfg := a.r().cfg
	cfg.APIClients = []config.APIClient{{Name: "Wall", Scopes: []string{config.ScopeNames}}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	p := adminPeer(t, a)
	vers := p.usersVersions(1)
	ana, _ := a.r().cfg.FindUserByName("Ana")
	rights := map[string]any{"sources": []any{map[string]any{"n": 3}}}
	r := p.saveWith(2, vers, map[string]any{
		"user/new:1/add":             map[string]any{"name": "Cy", "pin": "4321", "role": "standard"},
		"user/" + ana.ID + "/rights": rights,
		"client/Wall/scopes":         []string{config.ScopeNames, config.ScopeTally},
	})
	add := resultFor(t, r, "user/new:1/add")
	if add["status"] != "saved" {
		t.Fatalf("add: %v", add)
	}
	wantSaved(t, r, "user/"+ana.ID+"/rights")
	wantSaved(t, r, "client/Wall/scopes")
	got := a.r().cfg
	cy, ok := got.FindUserByName("Cy")
	if !ok || !cy.HasPin() {
		t.Fatalf("Cy not added with a PIN: %+v", cy)
	}
	if v, _ := add["value"].(map[string]any); v["id"] != cy.ID {
		t.Fatalf("add result does not name the new id: %v", add)
	}
	if ana, _ = got.FindUserByName("Ana"); ana.Rights.Sources == nil || len(*ana.Rights.Sources) != 1 {
		t.Fatalf("Ana's rights: %+v", ana.Rights)
	}
	if cl, _ := got.FindAPIClient("Wall"); len(cl.Scopes) != 2 {
		t.Fatalf("Wall scopes: %v", cl.Scopes)
	}
}

func TestOneSaveUserAttributesDoNotConflict(t *testing.T) {
	a := userApp(t, false)
	p1, p2 := adminPeer(t, a), adminPeer(t, a)
	v1, v2 := p1.usersVersions(1), p2.usersVersions(1)
	bo, _ := a.r().cfg.FindUserByName("Bo")
	wantSaved(t, p1.saveWith(2, v1, map[string]any{"user/" + bo.ID + "/persistent": true}), "user/"+bo.ID+"/persistent")
	wantSaved(t, p2.saveWith(2, v2, map[string]any{"user/" + bo.ID + "/name": "Bob"}), "user/"+bo.ID+"/name")
	f := wantRefused(t, p2.saveWith(3, v2, map[string]any{"user/" + bo.ID + "/persistent": false}), "user/"+bo.ID+"/persistent", "conflict")
	if f["value"] != true {
		t.Fatalf("conflict value: %v", f)
	}
	got, _ := a.r().cfg.FindUserByID(bo.ID)
	if got.Name != "Bob" || !got.Persistent {
		t.Fatalf("got %+v", got)
	}
}

func TestOneSavePinConflictHidesTheHash(t *testing.T) {
	a := userApp(t, false)
	p1, p2 := adminPeer(t, a), adminPeer(t, a)
	v1, v2 := p1.usersVersions(1), p2.usersVersions(1)
	bo, _ := a.r().cfg.FindUserByName("Bo")
	key := "user/" + bo.ID + "/pin"
	wantSaved(t, p1.saveWith(2, v1, map[string]any{key: "1111"}), key)
	f := wantRefused(t, p2.saveWith(2, v2, map[string]any{key: "2222"}), key, "conflict")
	if f["value"] != nil {
		t.Fatalf("a PIN conflict sent a value: %v", f)
	}
}

func TestOneSaveRemoveUserNamesTheirClients(t *testing.T) {
	a := userApp(t, false)
	cfg := a.r().cfg
	bo, _ := cfg.FindUserByName("Bo")
	cfg.APIClients = []config.APIClient{{Name: "Bo's wall", Scopes: []string{config.ScopeNames}, Owner: bo.ID}}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	p := adminPeer(t, a)
	list := p.ask("usersList", 1, nil)
	var named []any
	for _, u := range list["users"].([]any) {
		if u := u.(map[string]any); u["id"] == bo.ID {
			named, _ = u["clients"].([]any)
		}
	}
	if len(named) != 1 || named[0] != "Bo's wall" {
		t.Fatalf("usersList clients of Bo: %v", named)
	}
	vers, _ := list["versions"].(map[string]any)
	wantSaved(t, p.saveWith(2, vers, map[string]any{"user/" + bo.ID + "/remove": map[string]any{}}), "user/"+bo.ID+"/remove")
	got := a.r().cfg
	if _, ok := got.FindUserByID(bo.ID); ok {
		t.Fatal("Bo still there")
	}
	if len(got.APIClients) != 0 {
		t.Fatalf("Bo's clients stayed: %v", got.APIClients)
	}
}

func TestOneSaveAPISection(t *testing.T) {
	a := userApp(t, false)
	p := adminPeer(t, a)
	vers := p.usersVersions(1)
	for k, v := range p.hello["versions"].(map[string]any) {
		vers[k] = v
	}
	r := p.saveWith(2, vers, map[string]any{
		"api":            map[string]any{"rateLimit": 7},
		"guestApiScopes": []string{config.ScopeNames},
	})
	wantSaved(t, r, "api")
	wantSaved(t, r, "guestApiScopes")
	if got := a.r().cfg; got.API.RateLimit != 7 || len(got.GuestAPIScopes) != 1 {
		t.Fatalf("api %+v scopes %v", got.API, got.GuestAPIScopes)
	}
	setFlags(t, a, "Ana", true, false)
	q := dialPeer(t, a)
	q.signIn("Ana", "1234")
	wantRefused(t, q.saveFields(1, map[string]any{"api": map[string]any{"rateLimit": 9}}), "api", "Role")
}
