package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// API clients a user makes from My user: within their own Rights, owned by them, capped live by
// their Rights and deleted with them. Seams: the browser messages and the REST API.

const takeOne = `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`

// ownClientApp: Ana is a Standard user with sources 1-2 and destination 2, signed in.
func ownClientApp(t *testing.T) (*App, *fakeRouter, *client, func() []map[string]any) {
	t.Helper()
	a, f := apiTestApp(t, nil)
	setRights(t, a, "Ana", config.Rights{Sources: config.Nums(1, 2), Dsts: config.Nums(2)})
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	return a, f, c, next
}

func saveOwn(t *testing.T, a *App, c *client, next func() []map[string]any, cl myClientReq) map[string]any {
	t.Helper()
	return send(t, a, c, next, inMsg{T: "myClientSave", ID: 1, MyClient: &cl}, "myClientSaveResult")
}

func ownClientKey(t *testing.T, a *App, c *client, next func() []map[string]any, name string) string {
	t.Helper()
	r := send(t, a, c, next, inMsg{T: "myClientKey", ID: 2, Name: name, KeyName: "Desk", Pin: "1234"}, "myClientKeyResult")
	if r["ok"] != true {
		t.Fatalf("key refused: %v", r)
	}
	return r["key"].(string)
}

func TestStandardUserMakesAPIClientWithinOwnRights(t *testing.T) {
	a, f, c, next := ownClientApp(t)
	narrow := config.Rights{Sources: config.Nums(1), Dsts: config.Nums(2)}
	if r := saveOwn(t, a, c, next, myClientReq{Name: "Ana desk", Scopes: []string{config.ScopeTake}, Rights: &narrow}); r["ok"] != true {
		t.Fatalf("refused: %v", r)
	}
	cl, ok := a.r().cfg.FindAPIClient("Ana desk")
	if !ok || cl.Owner != idOf(a, "Ana") || cl.Panel != "" {
		t.Fatalf("client %+v %v", cl, ok)
	}

	wide := config.Rights{Sources: config.Nums(1, 3), Dsts: config.Nums(2)}
	r := saveOwn(t, a, c, next, myClientReq{Name: "Ana wide", Scopes: []string{}, Rights: &wide})
	if r["ok"] != false || !strings.Contains(r["err"].(string), "Settings") {
		t.Fatalf("wider Rights: %v", r)
	}
	if r := saveOwn(t, a, c, next, myClientReq{Name: "Ana log", Scopes: []string{config.ScopeLog}}); r["ok"] != false {
		t.Fatalf("a scope Ana's own key lacks: %v", r)
	}
	if _, ok := a.r().cfg.FindAPIClient("Ana wide"); ok {
		t.Fatal("a refused save made a client")
	}

	// A key needs Ana's PIN, and acts with the client's Rights.
	if r := send(t, a, c, next, inMsg{T: "myClientKey", ID: 3, Name: "Ana desk", KeyName: "Desk", Pin: "9999"}, "myClientKeyResult"); r["ok"] != false {
		t.Fatalf("wrong PIN made a key: %v", r)
	}
	key := ownClientKey(t, a, c, next, "Ana desk")
	f.seed(2, 3, 0)
	if rec := apiPost(a, "/api/v1/takes", key, takeOne, nil); rec.Code != 200 {
		t.Fatalf("inside rights: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":2,"dst":2,"levels":[1]}]}`, nil), 403, "outside_rights")

	// Someone else's client is not Ana's to change.
	if r := saveOwn(t, a, c, next, myClientReq{Name: "PanelsBot", Scopes: []string{}}); r["ok"] != false {
		t.Fatalf("took over an Administrator's client: %v", r)
	}
	l := send(t, a, c, next, inMsg{T: "myClients", ID: 4}, "myClientsResult")
	if cs := l["clients"].([]any); len(cs) != 1 || cs[0].(map[string]any)["name"] != "Ana desk" {
		t.Fatalf("list %v", l)
	}
	if ks := l["keys"].([]any); len(ks) != 1 {
		t.Fatalf("keys %v", l)
	}
}

func TestOwnClientNarrowsWithOwnerAndGoesWithThem(t *testing.T) {
	a, f, c, next := ownClientApp(t)
	saveOwn(t, a, c, next, myClientReq{Name: "Ana desk", Scopes: []string{config.ScopeTake}})
	cl, _ := a.r().cfg.FindAPIClient("Ana desk")
	if !refsAre(cl.Rights.Sources, 1, 2) || !refsAre(cl.Rights.Dsts, 2) {
		t.Fatalf("default is not Ana's Rights: %+v", cl.Rights)
	}
	key := ownClientKey(t, a, c, next, "Ana desk")
	f.seed(2, 3, 0)
	if rec := apiPost(a, "/api/v1/takes", key, takeOne, nil); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	setRights(t, a, "Ana", config.Rights{Sources: config.Nums(2), Dsts: config.Nums(2)})
	wantErr(t, apiPost(a, "/api/v1/takes", key, takeOne, nil), 403, "outside_rights")

	// Deleting Ana deletes the client and ends its key.
	keep := []userEdit{}
	for _, u := range a.r().cfg.Users {
		if u.Name != "Ana" {
			keep = append(keep, userEdit{ID: u.ID, Name: u.Name, Role: u.Role})
		}
	}
	if err := saveUsers(a, adminClient(t, a), usersReq{Users: keep}); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.r().cfg.FindAPIClient("Ana desk"); ok {
		t.Fatal("client outlived its owner")
	}
	wantErr(t, apiPost(a, "/api/v1/takes", key, takeOne, nil), 401, "key_revoked")
}

// An Administrator's Users save lists clients by name. It keeps a user's client and its owner, and
// a request cannot make a client someone's.
func TestAdministratorSaveKeepsOwnClients(t *testing.T) {
	a, _, c, next := ownClientApp(t)
	saveOwn(t, a, c, next, myClientReq{Name: "Ana desk", Scopes: []string{config.ScopeTake}})
	admin := adminClient(t, a)
	users := []userEdit{}
	for _, u := range a.r().cfg.Users {
		users = append(users, userEdit{ID: u.ID, Name: u.Name, Role: u.Role})
	}
	if err := saveUsers(a, admin, usersReq{Users: users, APIClients: &[]config.APIClient{{Name: "PanelsBot", Scopes: []string{}}}}); err != nil {
		t.Fatal(err)
	}
	if cl, ok := a.r().cfg.FindAPIClient("Ana desk"); !ok || cl.Owner != idOf(a, "Ana") {
		t.Fatalf("left out: %+v %v", cl, ok)
	}
	edited := []config.APIClient{{Name: "Ana desk", Scopes: []string{config.ScopeNames}}, {Name: "PanelsBot", Scopes: []string{}}}
	if err := saveUsers(a, admin, usersReq{Users: users, APIClients: &edited}); err != nil {
		t.Fatal(err)
	}
	if cl, _ := a.r().cfg.FindAPIClient("Ana desk"); cl.Owner != idOf(a, "Ana") || !slicesHas(cl.Scopes, config.ScopeNames) {
		t.Fatalf("edited: %+v", cl)
	}
	if cl, _ := a.r().cfg.FindAPIClient("PanelsBot"); cl.Owner != "" {
		t.Fatalf("PanelsBot got an owner: %+v", cl)
	}
	l := a.usersList(admin, 1)
	for _, cl := range l.APIClients {
		if cl.Name == "Ana desk" && cl.OwnerName != "Ana" {
			t.Fatalf("the Administrator's list does not name the owner: %+v", cl)
		}
	}
}

func TestOwnClientsNeedSignIn(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	guest := &client{ip: "10.0.0.7"}
	next := registerAll(a, guest)
	cl := myClientReq{Name: "Mine", Scopes: []string{}}
	if r := send(t, a, guest, next, inMsg{T: "myClientSave", ID: 1, MyClient: &cl}, "myClientSaveResult"); r["ok"] != false {
		t.Fatalf("Guest made a client: %v", r)
	}
	if r := send(t, a, guest, next, inMsg{T: "myClients", ID: 2}, "myClientsResult"); r["ok"] != false {
		t.Fatalf("Guest listed clients: %v", r)
	}
}

func TestOwnClientDelete(t *testing.T) {
	a, _, c, next := ownClientApp(t)
	saveOwn(t, a, c, next, myClientReq{Name: "Ana desk", Scopes: []string{config.ScopeTake}})
	key := ownClientKey(t, a, c, next, "Ana desk")
	if r := send(t, a, c, next, inMsg{T: "myClientDelete", ID: 5, Name: "PanelsBot"}, "myClientDeleteResult"); r["ok"] != false {
		t.Fatalf("deleted an Administrator's client: %v", r)
	}
	if r := send(t, a, c, next, inMsg{T: "myClientDelete", ID: 6, Name: "ana DESK"}, "myClientDeleteResult"); r["ok"] != true {
		t.Fatalf("%v", r)
	}
	if _, ok := a.r().cfg.FindAPIClient("Ana desk"); ok {
		t.Fatal("not deleted")
	}
	wantErr(t, apiPost(a, "/api/v1/takes", key, takeOne, nil), 401, "key_revoked")
}
