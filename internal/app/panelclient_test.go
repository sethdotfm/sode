package app

import (
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// REST seam for a panel's own API client (ADR 0006): Cy is Trusted with narrow Rights, Ana is not.

func clientApp(t *testing.T) (*App, *fakeRouter) {
	t.Helper()
	return apiTestApp(t, func(c *config.Config) {
		cy := mkUser(t, "Cy", "4321")
		cy.Trusted = true
		cy.Rights = config.Rights{Sources: ints(1, 2, 3), Dsts: ints(2, 4)}
		c.Users = append(c.Users, cy)
	})
}

const apiPanel = `{"name":"Wall","acts":"api"`

func clientOf(t *testing.T, a *App, name string) (config.APIClient, bool) {
	t.Helper()
	return a.r().cfg.FindAPIClient(name)
}

func TestPanelClientCreatedWithinRights(t *testing.T) {
	a, _ := clientApp(t)
	key := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`,"clientRights":{"sources":[1,2],"dsts":[2]}}`)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	body := apiBody(t, rec)
	id, name := body["id"].(string), body["client"].(string)
	if name != "Panel "+id {
		t.Fatalf("client %q for %s", name, id)
	}
	cl, ok := clientOf(t, a, name)
	if !ok || cl.Panel != id || !reflect.DeepEqual(cl.Rights.Sources, ints(1, 2)) || !reflect.DeepEqual(cl.Rights.Dsts, ints(2)) {
		t.Fatalf("client %+v %v", cl, ok)
	}
	if cl.Rights.Levels != nil || slicesHas(cl.Scopes, config.ScopePanelsManage) || !slicesHas(cl.Scopes, config.ScopePanels) {
		t.Fatalf("scopes or levels: %+v", cl)
	}
	if got, _ := body["clientRights"].(map[string]any); got == nil || got["sources"] == nil {
		t.Fatalf("clientRights not shown to its editor: %s", rec.Body.String())
	}
	// The creator's rights are the default.
	rec = apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`}`)
	if rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	cl, _ = clientOf(t, a, apiBody(t, rec)["client"].(string))
	if !reflect.DeepEqual(cl.Rights.Sources, ints(1, 2, 3)) || !reflect.DeepEqual(cl.Rights.Dsts, ints(2, 4)) {
		t.Fatalf("default rights %+v", cl.Rights)
	}
}

func slicesHas(l []string, s string) bool {
	for _, v := range l {
		if v == s {
			return true
		}
	}
	return false
}

func TestPanelClientWiderThanCreatorRefused(t *testing.T) {
	a, _ := clientApp(t)
	key := userKey(t, a, "Cy")
	for _, rights := range []string{
		`{"sources":[1,9]}`,
		`{"dsts":[7]}`,
		`{}`, // every list unset is wider than Cy's
		`{"sources":[1],"dsts":[2],"presets":["Studio A"],"levels":null}`,
	} {
		rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`,"clientRights":`+rights+`}`)
		want := 403
		if rights == `{"sources":[1],"dsts":[2],"presets":["Studio A"],"levels":null}` {
			want = 201 // within: lists only narrow
		}
		if rec.Code != want || (want == 403 && apiErrCode(t, rec) != "outside_rights") {
			t.Fatalf("%s: %d %s", rights, rec.Code, rec.Body.String())
		}
	}
	// Only the 201 left anything behind.
	if n := len(a.panels.list()); n != 1 {
		t.Fatalf("%d panels", n)
	}
	n := 0
	for _, c := range a.r().cfg.APIClients {
		if c.Panel != "" {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d panel clients", n)
	}
}

func TestPanelClientNeedsTrustedUser(t *testing.T) {
	a, _ := clientApp(t)
	rec := apiSend(a, "POST", "/api/v1/panels", userKey(t, a, "Ana"), apiPanel+`}`)
	if rec.Code != 403 || apiErrCode(t, rec) != "trusted_required" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if len(a.panels.list()) != 0 || len(a.r().cfg.APIClients) != 3 {
		t.Fatal("a refused create left a panel or client")
	}
	// A session panel needs no client, for anyone.
	if rec := apiSend(a, "POST", "/api/v1/panels", userKey(t, a, "Ana"), `{"name":"Mine"}`); rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestPanelClientCheckedOnSave(t *testing.T) {
	a, _ := clientApp(t)
	key := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	id, name := apiBody(t, rec)["id"].(string), apiBody(t, rec)["client"].(string)
	path := "/api/v1/panels/" + id

	if rec := apiSend(a, "PUT", path, key, apiPanel+`,"clientRights":{"sources":[1,2],"dsts":[2]}}`); rec.Code != 200 {
		t.Fatalf("narrower or equal change refused: %d %s", rec.Code, rec.Body.String())
	}
	if cl, _ := clientOf(t, a, name); !reflect.DeepEqual(cl.Rights.Sources, ints(1, 2)) {
		t.Fatalf("not saved: %+v", cl.Rights)
	}
	if rec := apiSend(a, "PUT", path, key, apiPanel+`,"clientRights":{"sources":[1,2,8],"dsts":[2]}}`); rec.Code != 403 || apiErrCode(t, rec) != "outside_rights" {
		t.Fatalf("widening on save: %d %s", rec.Code, rec.Body.String())
	}
	if cl, _ := clientOf(t, a, name); !reflect.DeepEqual(cl.Rights.Sources, ints(1, 2)) {
		t.Fatalf("refused save changed rights: %+v", cl.Rights)
	}
	// A save that leaves clientRights out keeps them, and the client keeps its name.
	rec = apiSend(a, "PUT", path, key, apiPanel+`}`)
	if rec.Code != 200 || apiBody(t, rec)["client"] != name {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if cl, _ := clientOf(t, a, name); !reflect.DeepEqual(cl.Rights.Sources, ints(1, 2)) {
		t.Fatalf("rights lost: %+v", cl.Rights)
	}
	// Losing Trusted ends the right to change them, not to save the panel.
	cfg := a.r().cfg
	cfg.Users[2].Trusted = false
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if rec := apiSend(a, "PUT", path, key, apiPanel+`,"clientRights":{"sources":[1]}}`); rec.Code != 403 || apiErrCode(t, rec) != "trusted_required" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestPanelClientAdminWidens(t *testing.T) {
	a, _ := clientApp(t)
	key := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	id, name := apiBody(t, rec)["id"].(string), apiBody(t, rec)["client"].(string)

	list := a.usersList(1).APIClients
	var shown *config.APIClient
	for i := range list {
		if list[i].Name == name {
			shown = &list[i]
		}
	}
	if shown == nil || shown.Panel != id {
		t.Fatalf("not in the admin list: %+v", list)
	}

	admin := adminClient(t, a)
	widened := *shown
	widened.Rights = config.Rights{Sources: ints(1, 50)}
	// The page may drop the panel link; the save still treats the client as the panel's, and
	// leaving a panel client out never removes it.
	widened.Panel = ""
	clients := []config.APIClient{widened}
	for _, c := range list {
		if c.Panel == "" {
			clients = append(clients, c)
		}
	}
	users := []userEdit{{Name: "Ana"}, {Name: "Bo"}, {Name: "Cy", Trusted: true}}
	if err := a.SaveUsers(admin, usersReq{Users: users, APIClients: &clients}); err != nil {
		t.Fatal(err)
	}
	cl, _ := clientOf(t, a, name)
	if cl.Panel != id || !reflect.DeepEqual(cl.Rights.Sources, ints(1, 50)) {
		t.Fatalf("after widening: %+v", cl)
	}
	none := []config.APIClient{}
	if err := a.SaveUsers(admin, usersReq{Users: users, APIClients: &none}); err != nil {
		t.Fatal(err)
	}
	if _, ok := clientOf(t, a, name); !ok {
		t.Fatal("an admin save without the panel client removed it")
	}
	// Widened past the creator, a plain re-save of the panel is not blocked.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, key, apiPanel+`}`); rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	// A client of that name cannot be added beside it.
	dup := []config.APIClient{{Name: name, Scopes: []string{}}}
	if err := a.SaveUsers(admin, usersReq{Users: users, APIClients: &dup}); err != nil {
		t.Fatal(err)
	}
	if n := 0; true {
		for _, c := range a.r().cfg.APIClients {
			if strings.EqualFold(c.Name, name) {
				n++
			}
		}
		if n != 1 {
			t.Fatalf("%d clients named %s", n, name)
		}
	}
}

func TestPanelClientGoesWithPanel(t *testing.T) {
	a, _ := clientApp(t)
	key := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`}`)
	id, name := apiBody(t, rec)["id"].(string), apiBody(t, rec)["client"].(string)
	secret := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: name}, "Wall tablet", time.Time{})

	// The client reads its panel, and nothing else of the panels.
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, secret, ""); rec.Code != 200 {
		t.Fatalf("client reads own panel: %d %s", rec.Code, rec.Body.String())
	} else if _, shown := apiBody(t, rec)["clientRights"]; shown {
		t.Fatal("clientRights shown to a use-only reader")
	}

	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id, key, ""); rec.Code != 204 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if _, ok := clientOf(t, a, name); ok {
		t.Fatal("client outlived its panel")
	}
	for _, k := range a.keys.list() {
		if k.Owner.Kind == actorClient && k.Owner.Name == name {
			t.Fatalf("key outlived its panel: %+v", k)
		}
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, secret, ""); rec.Code != 401 {
		t.Fatalf("key still works: %d", rec.Code)
	}
}

func TestPanelClientEndsWhenPanelStopsActingAsOne(t *testing.T) {
	a, _ := clientApp(t)
	key := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`}`)
	id, name := apiBody(t, rec)["id"].(string), apiBody(t, rec)["client"].(string)
	apiTestKey(t, a, keyOwner{Kind: actorClient, Name: name}, "Wall tablet", time.Time{})

	rec = apiSend(a, "PUT", "/api/v1/panels/"+id, key, `{"name":"Wall","acts":"session"}`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if _, has := apiBody(t, rec)["client"]; has {
		t.Fatalf("client still named: %s", rec.Body.String())
	}
	if _, ok := clientOf(t, a, name); ok || len(a.keys.list()) != 1 { // only Cy's own key is left
		t.Fatalf("client or its key kept: %v", a.keys.list())
	}
	// And back again: a new client.
	rec = apiSend(a, "PUT", "/api/v1/panels/"+id, key, apiPanel+`}`)
	if rec.Code != 200 || apiBody(t, rec)["client"] != name {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if _, ok := clientOf(t, a, name); !ok {
		t.Fatal("no client after switching back to api")
	}
}

func TestPanelClientsDoNotCountAgainstTheClientLimit(t *testing.T) {
	a, _ := clientApp(t)
	key := userKey(t, a, "Cy")
	cfg := a.r().cfg
	for len(cfg.APIClients) < config.MaxAPIClients {
		cfg.APIClients = append(cfg.APIClients, config.APIClient{Name: fmt.Sprintf("Bot %d", len(cfg.APIClients)), Scopes: []string{}})
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		if rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`}`); rec.Code != 201 {
			t.Fatalf("panel client %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	// The limit still stops an administrator's own clients.
	cfg = a.r().cfg
	cfg.APIClients = append(cfg.APIClients, config.APIClient{Name: "One too many", Scopes: []string{}})
	if err := a.ApplyConfig(cfg); err == nil {
		t.Fatal("limit gone")
	}
}

func TestRightsChangeListsPanelClients(t *testing.T) {
	a, _ := clientApp(t)
	defer startLog(t, a)()
	key := userKey(t, a, "Cy")
	rec := apiSend(a, "POST", "/api/v1/panels", key, apiPanel+`,"clientRights":{"sources":[1,2,3],"dsts":[2]}}`)
	id, name := apiBody(t, rec)["id"].(string), apiBody(t, rec)["client"].(string)
	apiSend(a, "POST", "/api/v1/panels", userKey(t, a, "Ana"), `{"name":"Not a client"}`)

	var cy userView
	for _, u := range a.usersList(1).Users {
		if u.Name == "Cy" {
			cy = u
		}
	}
	if len(cy.PanelClients) != 1 || cy.PanelClients[0].Client != name || cy.PanelClients[0].Panel != id || cy.PanelClients[0].Wider {
		t.Fatalf("panel clients %+v", cy.PanelClients)
	}

	admin := adminClient(t, a)
	narrow := config.Rights{Sources: ints(1), Dsts: ints(2, 4)}
	users := []userEdit{{Name: "Ana"}, {Name: "Bo"}, {Name: "Cy", Trusted: true, Rights: &narrow}}
	if err := a.SaveUsers(admin, usersReq{Users: users}); err != nil {
		t.Fatal(err)
	}
	for _, u := range a.usersList(2).Users {
		if u.Name == "Cy" && (len(u.PanelClients) != 1 || !u.PanelClients[0].Wider) {
			t.Fatalf("after narrowing: %+v", u.PanelClients)
		}
	}
	found := false
	for end := time.Now().Add(5 * time.Second); !found && time.Now().Before(end); time.Sleep(10 * time.Millisecond) {
		for _, row := range logged(t, a) {
			if row.Why == "rightsEdit" && strings.Contains(row.Note, "rights of user Cy") && strings.Contains(row.Note, name) {
				found = true
			}
		}
	}
	if !found {
		t.Fatal("no rightsEdit row listing the panel client")
	}
}

func TestPanelActorShowsKeyName(t *testing.T) {
	a, f, _ := takerApp(t, func(c *config.Config) {
		cy := mkUser(t, "Cy", "4321")
		cy.Trusted = true
		c.Users = append(c.Users, cy)
	})
	f.seed(2, 1, 0)
	key := apiTestKey(t, a, keyOwner{Kind: actorUser, Name: "Cy"}, "Panel 21", time.Time{})
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 1 || es[0].Client != "Cy (Panel 21)" {
		t.Fatalf("entries %+v", es)
	}
}

func TestOrphanPanelClientsDropAtStartup(t *testing.T) {
	a, _ := clientApp(t)
	rec := apiSend(a, "POST", "/api/v1/panels", userKey(t, a, "Cy"), apiPanel+`}`)
	name := apiBody(t, rec)["client"].(string)
	cfg := a.r().cfg.Clone()
	cfg.APIClients = append(cfg.APIClients, config.APIClient{Name: "Panel p-deadbeef", Panel: "p-deadbeef", Scopes: []string{}})
	dropOrphanPanelClients(&cfg, a.panels)
	if _, ok := cfg.FindAPIClient("Panel p-deadbeef"); ok {
		t.Fatal("orphan kept")
	}
	if _, ok := cfg.FindAPIClient(name); !ok || len(cfg.APIClients) != 4 {
		t.Fatalf("live clients dropped: %+v", cfg.APIClients)
	}
}
