package app

import (
	"strings"
	"testing"
)

// Panel sharing follows the Users save, and the owner id stays, through the one Save and the REST seam.

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
	mustSave(t, a, admin, map[string]any{ukey(a, "Ana", "name"): "Anna"})
	p, _ := a.panels.get(mine)
	if p.Owner != idOf(a, "Anna") || strings.Join(p.Share.Users, ",") != "Bo,Cy" {
		t.Fatalf("owned panel: owner %q share %v", p.Owner, p.Share.Users)
	}
	if p.Extra["future"] == nil || p.Settings["mode"] != "instant" {
		t.Fatalf("unknown fields lost: %+v", p)
	}
	if p, _ = a.panels.get(shared); strings.Join(p.Share.Users, ",") != "Anna,Bo" || p.Owner != idOf(a, "Bo") {
		t.Fatalf("share did not follow: %+v", p)
	}
	if p, _ = a.panels.get(other); strings.Join(p.Share.Users, ",") != "Bo" || p.Owner != idOf(a, "Bo") {
		t.Fatalf("untouched panel changed: %+v", p)
	}
}

func TestPanelsFollowUserRemoval(t *testing.T) {
	a, admin, mine, shared, _ := followFixture(t)
	anaID, boID := idOf(a, "Ana"), idOf(a, "Bo")
	mustSave(t, a, admin, map[string]any{"user/" + anaID + "/remove": map[string]string{"owner": boID}})
	p, _ := a.panels.get(mine)
	if p.Owner != boID || strings.Join(p.Share.Users, ",") != "Bo,Cy" {
		t.Fatalf("owned panel must go to the new owner: %+v", p)
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
