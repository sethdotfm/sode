package app

import (
	"fmt"
	"strings"
	"testing"
)

// REST seam for a panel's named instances: one key per instance name, listed and revoked or
// rotated one at a time.

func mintFor(t *testing.T, a *App, owner, id, name string) string {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", owner, fmt.Sprintf(`{"instance":%q}`, name))
	if rec.Code != 201 {
		t.Fatalf("mint %s: %d %s", name, rec.Code, rec.Body.String())
	}
	return apiBody(t, rec)["key"].(string)
}

func keyList(t *testing.T, a *App, owner, id string) []map[string]any {
	t.Helper()
	rec := apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", owner, "")
	if rec.Code != 200 {
		t.Fatalf("list: %d %s", rec.Code, rec.Body.String())
	}
	var out []map[string]any
	for _, k := range apiBody(t, rec)["keys"].([]any) {
		out = append(out, k.(map[string]any))
	}
	return out
}

func TestPanelLinkNeedsAnInstanceName(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	for _, body := range []string{``, `{}`, `{"instance":"  "}`, `{"setup":true}`} {
		wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/key", cy, body), 400, "bad_request")
	}
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key", cy, ""), 400, "bad_request")
	if len(keyList(t, a, cy, id)) != 0 {
		t.Fatal("a refused request made a key")
	}
}

func TestPanelHoldsOneKeyPerInstance(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	lobby := mintFor(t, a, cy, id, "Lobby")
	studio := mintFor(t, a, cy, id, "Studio")
	for _, k := range []string{lobby, studio} {
		if rec := apiSend(a, "GET", "/api/v1/tally", k, ""); rec.Code != 200 {
			t.Fatalf("a key of the panel: %d", rec.Code)
		}
	}
	// Rotating Lobby, by any case, leaves Studio alone.
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", cy, `{"instance":"LOBBY"}`)
	body := apiBody(t, rec)
	if rec.Code != 201 || body["rotated"] != true || body["instance"] != "LOBBY" {
		t.Fatalf("rotate: %d %v", rec.Code, body)
	}
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", lobby, ""), 401, "key_revoked")
	if rec := apiSend(a, "GET", "/api/v1/tally", studio, ""); rec.Code != 200 {
		t.Fatalf("Studio was rotated with Lobby: %d", rec.Code)
	}
	if got := len(keyList(t, a, cy, id)); got != 2 {
		t.Fatalf("%d instances, want 2", got)
	}
}

func TestPanelRevokeEndsOneInstance(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	lobby := mintFor(t, a, cy, id, "Lobby")
	studio := mintFor(t, a, cy, id, "Studio")
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key?instance=lobby", cy, ""); rec.Code != 204 {
		t.Fatalf("revoke: %d", rec.Code)
	}
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", lobby, ""), 401, "key_revoked")
	if rec := apiSend(a, "GET", "/api/v1/tally", studio, ""); rec.Code != 200 {
		t.Fatalf("Studio was revoked with Lobby: %d", rec.Code)
	}
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key?instance=Nobody", cy, ""); rec.Code != 204 {
		t.Fatalf("revoking an unknown instance: %d", rec.Code)
	}
	p, _ := a.panels.get(id)
	if _, ok := clientOfPanel(a.r().cfg, p); !ok {
		t.Fatal("revoke removed the client")
	}
}

func TestPanelKeyListShowsInstancesNotSecrets(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret := mintFor(t, a, cy, id, "Studio")
	mintFor(t, a, cy, id, "Lobby")
	token, _ := setupLink(t, a, cy, id)
	a.mon.beat(id, "studio")
	apiSend(a, "GET", "/api/v1/tally", secret, "")

	rec := apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", cy, "")
	if strings.Contains(rec.Body.String(), secret) || strings.Contains(rec.Body.String(), token) {
		t.Fatal("the list gives a secret back")
	}
	list := keyList(t, a, cy, id)
	// Setup replaced Lobby's key (same name), so the list has Lobby (setup) and Studio.
	if len(list) != 2 || list[0]["name"] != "Lobby" || list[0]["setup"] != true || list[0]["expires"] == nil || list[1]["name"] != "Studio" {
		t.Fatalf("list %v", list)
	}
	if list[1]["online"] != true || list[1]["lastUsed"] == nil || list[1]["created"] == nil || list[0]["online"] != false {
		t.Fatalf("Studio state %v", list)
	}
	// Exchanging the setup link makes a key under the same name.
	if r := exchangeSetup(a, apiPeer, id, token); r.Code != 201 || apiBody(t, r)["instance"] != "Lobby" {
		t.Fatalf("exchange: %d %s", r.Code, r.Body.String())
	}
	if list = keyList(t, a, cy, id); len(list) != 2 || list[0]["name"] != "Lobby" || list[0]["setup"] == true {
		t.Fatalf("after exchange %v", list)
	}
}

func TestRotatingOneInstanceKeepsTheOthersOnline(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	mintFor(t, a, cy, id, "Lobby")
	mintFor(t, a, cy, id, "Studio")
	a.mon.beat(id, "Lobby")
	a.mon.beat(id, "Studio")
	mintFor(t, a, cy, id, "Lobby")
	if a.mon.deviceOnline(id, "Lobby") || !a.mon.deviceOnline(id, "Studio") {
		t.Fatal("rotating forgot the wrong instance")
	}
}

func TestSetupLinkForOneInstanceLeavesOthersKeys(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	studio := mintFor(t, a, cy, id, "Studio")
	setupLink(t, a, cy, id)
	if rec := apiSend(a, "GET", "/api/v1/tally", studio, ""); rec.Code != 200 {
		t.Fatalf("a setup link for Lobby ended Studio's key: %d", rec.Code)
	}
}

func TestPanelInstancesAreCapped(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	for i := 0; i < panelKeyMax; i++ {
		mintFor(t, a, cy, id, fmt.Sprintf("Screen %d", i))
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/key", cy, `{"instance":"One more"}`), 409, "conflict")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/key", cy, `{"instance":"One more","setup":true}`), 409, "conflict")
	// Rotating a held name still works, and nothing was lost to the refusals.
	mintFor(t, a, cy, id, "screen 3")
	if got := len(keyList(t, a, cy, id)); got != panelKeyMax {
		t.Fatalf("%d instances, want %d", got, panelKeyMax)
	}
}

func TestPanelKeyListIsForOwnersOfAnAPIPanel(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	ana := userKey(t, a, "Ana")
	wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", ana, ""), 404, "not_found")
	session := makePanel(t, a, cy, `{"name":"Mine"}`)
	wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+session+"/keys", cy, ""), 409, "conflict")
	secret := mintFor(t, a, cy, id, "Lobby")
	wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", secret, ""), 403, "scope")
}
