package app

import (
	"strings"
	"testing"
)

// Panel owner and sharing follow the Users save, through SaveUsers and the REST seam.

func followFixture(t *testing.T) (a *App, admin *client, mine, shared, other string) {
	t.Helper()
	a = panelsApp(t)
	admin = adminClient(t, a)
	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	mine = makePanel(t, a, ana, `{"name":"Mine","settings":{"mode":"instant"},"future":{"x":1},"share":{"mode":"named","users":["Bo","Cy"]}}`)
	shared = makePanel(t, a, bo, `{"name":"Shared","share":{"mode":"named","users":["Ana","Bo"]}}`)
	other = makePanel(t, a, bo, `{"name":"Other","share":{"mode":"named","users":["Bo"]}}`)
	return
}

func TestPanelsFollowUserRename(t *testing.T) {
	a, admin, mine, shared, other := followFixture(t)
	err := a.SaveUsers(admin, usersReq{Users: []userEdit{
		{Name: "Anna", Was: "Ana"}, {Name: "Bo"}, {Name: "Cy", Trusted: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := a.panels.get(mine)
	if p.Owner != "Anna" || strings.Join(p.Share.Users, ",") != "Bo,Cy" {
		t.Fatalf("owned panel: owner %q share %v", p.Owner, p.Share.Users)
	}
	if p.Extra["future"] == nil || p.Settings["mode"] != "instant" {
		t.Fatalf("unknown fields lost: %+v", p)
	}
	if p, _ = a.panels.get(shared); strings.Join(p.Share.Users, ",") != "Anna,Bo" || p.Owner != "Bo" {
		t.Fatalf("share did not follow: %+v", p)
	}
	if p, _ = a.panels.get(other); strings.Join(p.Share.Users, ",") != "Bo" || p.Owner != "Bo" {
		t.Fatalf("untouched panel changed: %+v", p)
	}
}

func TestPanelsFollowUserRemoval(t *testing.T) {
	a, admin, mine, shared, _ := followFixture(t)
	if err := a.SaveUsers(admin, usersReq{Users: []userEdit{{Name: "Bo"}, {Name: "Cy", Trusted: true}}}); err != nil {
		t.Fatal(err)
	}
	p, _ := a.panels.get(mine)
	if p.Owner != "Ana" || strings.Join(p.Share.Users, ",") != "Bo,Cy" {
		t.Fatalf("owned panel must stay with its owner: %+v", p)
	}
	if p, _ = a.panels.get(shared); strings.Join(p.Share.Users, ",") != "Bo" {
		t.Fatalf("removed user still shared: %v", p.Share.Users)
	}
	cy := userKey(t, a, "Cy")
	if rec := apiSend(a, "GET", "/api/v1/panels/"+mine, cy, ""); rec.Code != 200 {
		t.Fatalf("Trusted read: %d", rec.Code)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+mine, cy, ""); rec.Code != 204 && rec.Code != 200 {
		t.Fatalf("Trusted delete: %d", rec.Code)
	}
}
