package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// idOf is the live id of the user named name ("" for none), as the Users editor sends for a row it loaded.
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

// A rename keeps everything that refers to the user: the session token, API keys, panels, templates,
// Checkpoint sets and Rights. Only the shown name changes. The reads below use the key made before.
func TestRenameKeepsEverythingOfTheUser(t *testing.T) {
	a := panelsApp(t)
	setFlags(t, a, "Ana", false, true)
	ana := userKey(t, a, "Ana")
	srcs := *config.Nums(1, 2)
	admin := adminClient(t, a)
	if err := saveUsers(a, admin, usersReq{Users: []userEdit{
		{ID: idOf(a, "Ana"), Name: "Ana", Persistent: true, Rights: &config.Rights{Sources: &srcs}},
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
		{ID: id, Name: "Anna", Persistent: true}, {ID: idOf(a, "Bo"), Name: "Bo"}, {ID: idOf(a, "Cy"), Name: "Cy", Role: config.RoleTrusted},
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

// A config file without user ids, such as one written by hand, keeps each user's id by name.
// So their keys and panels stay theirs.
func TestConfigImportWithoutIDsKeepsUsers(t *testing.T) {
	a := panelsApp(t)
	ana := userKey(t, a, "Ana")
	panel := makePanel(t, a, ana, `{"name":"Mine"}`)
	id := idOf(a, "Ana")
	text, _ := exportOf(t, a, false)
	var lines []string
	for _, l := range strings.Split(text, "\n") {
		if i := strings.Index(l, "id: u-"); i >= 0 {
			l = l[:i] // "- id: u-..." keeps its "- ", and the item's other keys follow
		}
		lines = append(lines, l)
	}
	stripped := strings.Join(lines, "\n")
	if strings.Contains(stripped, id) {
		t.Fatal("the id is still in the file")
	}
	if err := a.ImportConfig(adminClient(t, a), stripped); err != nil {
		t.Fatal(err)
	}
	if got := idOf(a, "Ana"); got != id {
		t.Fatalf("Ana's id %q, was %q", got, id)
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+panel, ana, ""); rec.Code != 200 || apiBody(t, rec)["owner"] != "Ana" {
		t.Fatalf("after import: %d %s", rec.Code, rec.Body.String())
	}
}

// Name rules are refused on save with words a person can act on.
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
