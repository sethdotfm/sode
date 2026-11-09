package app

import (
	"strings"
	"testing"
	"time"
)

// REST seam for sharing: the share field of PUT /api/v1/panels/{id} and the people list a picker reads.

func TestPanelSharingSettingsAndRemoval(t *testing.T) {
	a := panelsApp(t)
	ana, bo, cy := userKey(t, a, "Ana"), userKey(t, a, "Bo"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	put := func(key, share string) int {
		return apiSend(a, "PUT", "/api/v1/panels/"+id, key, `{"name":"Wall","share":`+share+`}`).Code
	}
	shareOf := func() map[string]any {
		return apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil))["share"].(map[string]any)
	}

	if c := put(ana, `{"mode":"named","users":["Bo","Cy"]}`); c != 200 {
		t.Fatalf("named: %d", c)
	}
	if s := shareOf(); s["mode"] != "named" || len(s["users"].([]any)) != 2 {
		t.Fatalf("share %v", s)
	}
	if panelIDs(t, a, bo) != "Wall" {
		t.Fatal("a named user does not see the panel")
	}

	// Dropping Bo takes effect on his next list and on a launch.
	put(ana, `{"mode":"named","users":["Cy"]}`)
	if panelIDs(t, a, bo) != "" {
		t.Fatal("Bo still lists it")
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/"+id, bo, nil), 404, "not_found")

	// Names left over from another mode are not kept.
	put(ana, `{"mode":"everyone","users":["Bo"]}`)
	if s := shareOf(); s["mode"] != "everyone" || s["users"] != nil {
		t.Fatalf("everyone keeps names: %v", s)
	}
	if panelIDs(t, a, bo) != "Wall" {
		t.Fatal("Everyone does not reach Bo")
	}

	put(ana, `{"mode":"me"}`)
	if panelIDs(t, a, bo) != "" {
		t.Fatal("Only me still reaches Bo")
	}
	if panelIDs(t, a, cy) != "Wall" { // Trusted
		t.Fatal("a Trusted user lost it")
	}

	// A named user may use the panel but not change who shares it.
	put(ana, `{"mode":"named","users":["Bo"]}`)
	if c := put(bo, `{"mode":"everyone"}`); c != 403 {
		t.Fatalf("a named user changed sharing: %d", c)
	}

	// Removing a user from Sode removes them from the list.
	a.panels.followUsers(nil, map[string]bool{"bo": true})
	if panelIDs(t, a, bo) != "" {
		t.Fatal("a removed user still sees the panel")
	}
}

func TestPanelPeopleListsUserNamesForThePicker(t *testing.T) {
	a := panelsApp(t)
	rec := apiDo(a, "GET", "/api/v1/panels/people", userKey(t, a, "Ana"), nil)
	if rec.Code != 200 {
		t.Fatalf("people: %d %s", rec.Code, rec.Body.String())
	}
	var names []string
	for _, u := range apiBody(t, rec)["users"].([]any) {
		m := u.(map[string]any)
		names = append(names, m["name"].(string))
		if len(m) != 1 {
			t.Errorf("a person is only a name: %v", m)
		}
	}
	if strings.Join(names, ",") != "Ana,Bo,Cy" {
		t.Fatalf("names %v", names)
	}
}

func TestPanelPeopleIsForUsersOnly(t *testing.T) {
	a := panelsApp(t)
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/people", "sode_"+strings.Repeat("x", 43), nil), 401, "invalid_key")
	cl := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "Bot"}, "k", time.Time{})
	if rec := apiDo(a, "GET", "/api/v1/panels/people", cl, nil); rec.Code == 200 {
		t.Fatalf("a client read user names: %s", rec.Body.String())
	}
}
