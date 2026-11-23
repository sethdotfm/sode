package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Seams: the browser messages and the REST API.

const takeOne = `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`

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
	narrow := config.Rights{Sources: nums(1), Dsts: nums(2)}
	if r := saveOwn(t, a, c, next, myClientReq{Name: "Ana desk", Scopes: []string{config.ScopeTake}, Rights: &narrow}); r["ok"] != true {
		t.Fatalf("refused: %v", r)
	}
	cl, ok := a.r().cfg.FindAPIClient("Ana desk")
	if !ok || cl.Owner != idOf(a, "Ana") || cl.Panel != "" {
		t.Fatalf("client %+v %v", cl, ok)
	}

	wide := config.Rights{Sources: nums(1, 3), Dsts: nums(2)}
	r := saveOwn(t, a, c, next, myClientReq{Name: "Ana wide", Scopes: []string{}, Rights: &wide})
	if r["ok"] != false || !strings.Contains(r["err"].(string), "wider than yours") {
		t.Fatalf("wider Rights: %v", r)
	}
	if r := saveOwn(t, a, c, next, myClientReq{Name: "Ana log", Scopes: []string{config.ScopeLog}}); r["ok"] != false {
		t.Fatalf("a scope Ana's own key lacks: %v", r)
	}
	if _, ok := a.r().cfg.FindAPIClient("Ana wide"); ok {
		t.Fatal("a refused save made a client")
	}

	if r := send(t, a, c, next, inMsg{T: "myClientKey", ID: 3, Name: "Ana desk", KeyName: "Desk", Pin: "9999"}, "myClientKeyResult"); r["ok"] != false {
		t.Fatalf("wrong PIN made a key: %v", r)
	}
	key := ownClientKey(t, a, c, next, "Ana desk")
	f.seed(2, 3, 0)
	if rec := apiPost(a, "/api/v1/takes", key, takeOne, nil); rec.Code != 200 {
		t.Fatalf("inside rights: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":2,"dst":2,"levels":[1]}]}`, nil), 403, "outside_rights")

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
	setRights(t, a, "Ana", config.Rights{Sources: nums(2), Dsts: nums(2)})
	wantErr(t, apiPost(a, "/api/v1/takes", key, takeOne, nil), 403, "outside_rights")

	mustSave(t, a, adminClient(t, a), map[string]any{ukey(a, "Ana", "remove"): true})
	if _, ok := a.r().cfg.FindAPIClient("Ana desk"); ok {
		t.Fatal("client outlived its owner")
	}
	wantErr(t, apiPost(a, "/api/v1/takes", key, takeOne, nil), 401, "key_revoked")
}

func TestAdministratorSaveKeepsOwnClients(t *testing.T) {
	a, _, c, next := ownClientApp(t)
	saveOwn(t, a, c, next, myClientReq{Name: "Ana desk", Scopes: []string{config.ScopeTake}})
	admin := adminClient(t, a)
	if r := mustRefuse(t, a, admin, map[string]any{"client/Ana desk/remove": true}); !strings.Contains(r.Err, "belongs to a panel or a user") {
		t.Fatalf("own client remove: %+v", r)
	}
	if cl, ok := a.r().cfg.FindAPIClient("Ana desk"); !ok || cl.Owner != idOf(a, "Ana") {
		t.Fatalf("removed: %+v %v", cl, ok)
	}
	mustSave(t, a, admin, map[string]any{"client/Ana desk/scopes": []string{config.ScopeNames}, "client/PanelsBot/scopes": []string{}})
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
