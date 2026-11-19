package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func idOf(a *App, name string) string {
	u, _ := a.r().cfg.FindUserByName(name)
	return u.ID
}

// ownerOf is the key owner for the user named name; a missing user gives an owner nobody is.
func ownerOf(a *App, name string) keyOwner {
	if u, ok := a.r().cfg.FindUserByName(name); ok {
		return userOwner(u)
	}
	return keyOwner{Kind: actorUser, Name: name}
}

func ptr[T any](v T) *T { return &v }

// The reads below use the key made before the rename.
func TestRenameKeepsEverythingOfTheUser(t *testing.T) {
	a := panelsApp(t)
	setFlags(t, a, "Ana", false, true)
	ana := userKey(t, a, "Ana")
	srcs := *nums(1, 2)
	admin := adminClient(t, a)
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{
		{ID: idOf(a, "Ana"), Name: "Ana", Persistent: new(true), Rights: &config.Rights{Sources: &srcs}},
		{ID: idOf(a, "Bo"), Name: "Bo"}, {ID: idOf(a, "Cy"), Name: "Cy", Role: config.RoleTrusted},
	}}); err != nil {
		t.Fatal(err)
	}
	c := &client{ip: "10.0.0.1"}
	register(a, c)
	if err := a.UserUnlock(c, "1234"); err != nil {
		t.Fatal(err)
	}
	tok := a.issueSession(c)
	if tok == "" {
		t.Fatal("no token for a persistent user")
	}
	panel := makePanel(t, a, ana, `{"name":"Mine"}`)
	rec := apiSend(a, "POST", "/api/v1/panel-templates", ana, `{"name":"T","scope":"me","sources":true,"cells":[]}`)
	if rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	tid, _ := apiBody(t, rec)["id"].(string)
	rec = apiSend(a, "POST", "/api/v1/panel-sets", ana, setBody("Show", "me", panel))
	if rec.Code != 201 {
		t.Fatalf("set: %d %s", rec.Code, rec.Body.String())
	}
	id := idOf(a, "Ana")

	if err := saveUsers(a, admin, usersReq{Users: []userEdit{
		{ID: id, Name: "Anna", Persistent: new(true)}, {ID: idOf(a, "Bo"), Name: "Bo"}, {ID: idOf(a, "Cy"), Name: "Cy", Role: config.RoleTrusted},
	}}); err != nil {
		t.Fatal(err)
	}

	fresh := &client{ip: "10.0.0.2"}
	register(a, fresh)
	a.Hi(fresh, "main", tok)
	if fresh.who() != "Anna" || c.who() != "Anna" {
		t.Fatalf("token after rename signs in as %q, open connection is %q", fresh.who(), c.who())
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+panel, ana, ""); rec.Code != 200 || apiBody(t, rec)["owner"] != "Anna" || apiBody(t, rec)["canEdit"] != true {
		t.Fatalf("panel after rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "GET", "/api/v1/panel-templates/"+tid, ana, ""); rec.Code != 200 || apiBody(t, rec)["owner"] != "Anna" {
		t.Fatalf("template after rename: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "GET", "/api/v1/panel-sets", ana, ""); !strings.Contains(rec.Body.String(), `"owner":"Anna"`) {
		t.Fatalf("set after rename: %d %s", rec.Code, rec.Body.String())
	}
	u, _ := a.r().cfg.FindUserByID(id)
	if u.Name != "Anna" || u.Rights.Sources == nil || len(*u.Rights.Sources) != 2 {
		t.Fatalf("user after rename: %+v", u)
	}
}

func TestUsersSaveRefusesBadNames(t *testing.T) {
	a := userApp(t, false)
	admin := adminClient(t, a)
	bo := userEdit{ID: idOf(a, "Bo"), Name: "Bo"}
	for name, tc := range map[string]struct {
		set, want string
	}{
		"25 runes":           {strings.Repeat("é", 25), "too long"},
		"not printable":      {"An\u0007a", "cannot be shown"},
		"same as Bo by case": {"BO", "two users are named"},
		"blank":              {"  ", "needs a name"},
	} {
		err := saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: tc.set}, bo}})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: %v, want %q", name, err, tc.want)
		}
	}
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{{ID: idOf(a, "Ana"), Name: "ANA"}, bo}}); err != nil {
		t.Fatalf("a change of case only: %v", err)
	}
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{{ID: "u-gone", Name: "Ana"}, bo}}); err == nil {
		t.Fatal("a row for a user who is gone was saved")
	}
}
