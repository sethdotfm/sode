package app

import (
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

// REST seam for the rest of a panel's sharing: the one-time setup link of secure mode, the
// networks a key may be used from, and the hasKey flag the Share tab reads.

func setupLink(t *testing.T, a *App, owner, id string) (token string, body map[string]any) {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", owner, `{"setup":true}`)
	if rec.Code != 201 {
		t.Fatalf("setup link: %d %s", rec.Code, rec.Body.String())
	}
	body = apiBody(t, rec)
	token, _ = body["setup"].(string)
	if !strings.HasPrefix(token, setupPrefix) {
		t.Fatalf("no setup token in %s", rec.Body.String())
	}
	if _, has := body["key"]; has {
		t.Fatal("a setup link carries no key")
	}
	return token, body
}

func exchangeSetup(a *App, peer, id, token string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/panels/"+id+"/setup", strings.NewReader(`{"setup":"`+token+`"}`))
	r.RemoteAddr = peer
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func TestSetupLinkWorksOnceThenFails(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	token, body := setupLink(t, a, cy, id)
	if want := "panels/" + id + "#setup=" + token; body["link"] != want {
		t.Fatalf("link %v, want %s", body["link"], want)
	}
	rec := exchangeSetup(a, apiPeer, id, token)
	if rec.Code != 201 {
		t.Fatalf("exchange: %d %s", rec.Code, rec.Body.String())
	}
	if !strings.HasPrefix(rec.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("cache header %q", rec.Header().Get("Cache-Control"))
	}
	secret, _ := apiBody(t, rec)["key"].(string)
	if !strings.HasPrefix(secret, keyPrefix) {
		t.Fatalf("no key in %s", rec.Body.String())
	}
	if rec := apiSend(a, "GET", "/api/v1/tally", secret, ""); rec.Code != 200 {
		t.Fatalf("exchanged key: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, exchangeSetup(a, apiPeer, id, token), 401, "invalid_setup")
}

func TestSetupTokenIsNotAKey(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	token, _ := setupLink(t, a, cy, id)
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", token, ""), 401, "invalid_key")
	// A setup record never shows in a key list either.
	for _, k := range a.keys.list() {
		if k.Name == setupName {
			t.Fatal("a setup link is listed as a key")
		}
	}
}

func TestSetupLinkReplacesTheKeyAndAnOlderLink(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	old, _ := mintKey(t, a, cy, id)
	first, _ := setupLink(t, a, cy, id)
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", old, ""), 401, "invalid_key")
	second, _ := setupLink(t, a, cy, id)
	wantErr(t, exchangeSetup(a, apiPeer, id, first), 401, "invalid_setup")
	if rec := exchangeSetup(a, apiPeer, id, second); rec.Code != 201 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
}

func TestSetupLinkEndsWithRevokeAndRotate(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	token, _ := setupLink(t, a, cy, id)
	apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key", cy, "")
	wantErr(t, exchangeSetup(a, apiPeer, id, token), 401, "invalid_setup")
	token, _ = setupLink(t, a, cy, id)
	mintKey(t, a, cy, id)
	wantErr(t, exchangeSetup(a, apiPeer, id, token), 401, "invalid_setup")
}

func TestSetupLinkExpires(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	token, _ := setupLink(t, a, cy, id)
	a.keys.mu.Lock()
	for i := range a.keys.recs {
		if a.keys.recs[i].Setup {
			a.keys.recs[i].Expires = time.Now().Add(-time.Second)
		}
	}
	a.keys.mu.Unlock()
	wantErr(t, exchangeSetup(a, apiPeer, id, token), 401, "invalid_setup")
}

func TestSetupTokenOfAnotherPanelFails(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	other := makePanel(t, a, cy, apiPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	token, _ := setupLink(t, a, cy, id)
	wantErr(t, exchangeSetup(a, apiPeer, other, token), 401, "invalid_setup")
	if rec := exchangeSetup(a, apiPeer, id, token); rec.Code != 201 {
		t.Fatalf("the right panel: %d", rec.Code)
	}
}

func TestSetupExchangeIsThrottled(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	token, _ := setupLink(t, a, cy, id)
	for i := 0; i < apiAuthFailMax; i++ {
		wantErr(t, exchangeSetup(a, apiPeer, id, "sode_setup_wrong"), 401, "invalid_setup")
	}
	wantErr(t, exchangeSetup(a, apiPeer, id, token), 429, "rate_limited")
}

func TestSetupLinkOnlyForOwnersOfAnAPIPanel(t *testing.T) {
	a, _, cy, _ := keyedPanel(t)
	session := makePanel(t, a, cy, `{"name":"Mine"}`)
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+session+"/key", cy, `{"setup":true}`), 409, "conflict")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+session+"/key", userKey(t, a, "Ana"), `{"setup":true}`), 404, "not_found")
}

func putNetworks(a *App, key, id, list string) *httptest.ResponseRecorder {
	return apiSend(a, "PUT", "/api/v1/panels/"+id+"/networks", key, `{"networks":`+list+`}`)
}

func TestKeyFromOutsideTheNetworksIsRefused(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	rec := putNetworks(a, cy, id, `["192.0.2.0/24"," 10.0.0.5 ","2001:db8::/32"]`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	got, _ := apiBody(t, rec)["networks"].([]any)
	if len(got) != 3 || got[0] != "192.0.2.0/24" || got[1] != "10.0.0.5/32" || got[2] != "2001:db8::/32" {
		t.Fatalf("normalized %v", got)
	}
	if rec := apiSend(a, "GET", "/api/v1/tally", secret, ""); rec.Code != 200 {
		t.Fatalf("inside: %d %s", rec.Code, rec.Body.String())
	}
	out := apiDoFrom(a, "198.51.100.9:4000", "GET", "/api/v1/tally", secret, nil)
	wantErr(t, out, 403, "network_not_allowed")
	if !strings.Contains(out.Body.String(), "198.51.100.9") {
		t.Fatalf("the refusal does not name the address: %s", out.Body.String())
	}
	// A refusal is not a failed key, so the address is never throttled for it.
	for i := 0; i < apiAuthFailMax+2; i++ {
		wantErr(t, apiDoFrom(a, "198.51.100.9:4000", "GET", "/api/v1/tally", secret, nil), 403, "network_not_allowed")
	}
}

func TestNetworksEmptyAllowsAnywhere(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	putNetworks(a, cy, id, `["10.9.0.0/16"]`)
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", secret, ""), 403, "network_not_allowed")
	if rec := putNetworks(a, cy, id, `[]`); rec.Code != 200 {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
	if rec := apiSend(a, "GET", "/api/v1/tally", secret, ""); rec.Code != 200 {
		t.Fatalf("after clearing: %d", rec.Code)
	}
}

func TestSetupExchangeFromOutsideTheNetworksFails(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	putNetworks(a, cy, id, `["192.0.2.0/24"]`)
	token, _ := setupLink(t, a, cy, id)
	wantErr(t, exchangeSetup(a, "198.51.100.9:4000", id, token), 403, "network_not_allowed")
	if rec := exchangeSetup(a, apiPeer, id, token); rec.Code != 201 {
		t.Fatalf("the refused exchange spent the link: %d %s", rec.Code, rec.Body.String())
	}
}

func TestNetworksRefuseBadListsAndOtherCallers(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	wantErr(t, putNetworks(a, cy, id, `["not an address"]`), 400, "bad_request")
	wantErr(t, putNetworks(a, cy, id, `["10.0.0.0/33"]`), 400, "bad_request")
	var ips []string
	for i := 0; i <= panelNetworksMax; i++ {
		ips = append(ips, fmt.Sprintf("10.0.0.%d", i))
	}
	many := `["` + strings.Join(ips, `","`) + `"]`
	wantErr(t, putNetworks(a, cy, id, many), 400, "bad_request")
	wantErr(t, putNetworks(a, userKey(t, a, "Ana"), id, `[]`), 404, "not_found")
	session := makePanel(t, a, cy, `{"name":"Mine"}`)
	wantErr(t, putNetworks(a, cy, session, `[]`), 409, "conflict")
}

func TestNetworksSurviveAPanelSaveAndShowToEditors(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	putNetworks(a, cy, id, `["192.0.2.0/24"]`)
	rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, apiPanel+`,"name":"Wall 2"}`)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	c, _ := clientOf(t, a, panelClientName(id))
	if len(c.Networks) != 1 || c.Networks[0] != "192.0.2.0/24" {
		t.Fatalf("client networks %v", c.Networks)
	}
	body := apiBody(t, apiSend(a, "GET", "/api/v1/panels/"+id, cy, ""))
	if n, _ := body["clientNetworks"].([]any); len(n) != 1 {
		t.Fatalf("clientNetworks %v", body["clientNetworks"])
	}
	if body["hasKey"] != false {
		t.Fatalf("hasKey %v before a key", body["hasKey"])
	}
	mintKey(t, a, cy, id)
	if apiBody(t, apiSend(a, "GET", "/api/v1/panels/"+id, cy, ""))["hasKey"] != true {
		t.Fatal("hasKey false after a key")
	}
	// An echoed read must not plant either field in the stored panel.
	echo := apiSend(a, "GET", "/api/v1/panels/"+id, cy, "").Body.String()
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, echo); rec.Code != 200 {
		t.Fatalf("echo: %d %s", rec.Code, rec.Body.String())
	}
	if p, _ := a.panels.get(id); p.Extra["hasKey"] != nil || p.Extra["clientNetworks"] != nil {
		t.Fatalf("extra %v", p.Extra)
	}
	// A setup link is not a key yet.
	setupLink(t, a, cy, id)
	if apiBody(t, apiSend(a, "GET", "/api/v1/panels/"+id, cy, ""))["hasKey"] != false {
		t.Fatal("a pending setup link counts as a key")
	}
}

func TestNetworksEndWithThePanel(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	putNetworks(a, cy, id, `["192.0.2.0/24"]`)
	apiSend(a, "DELETE", "/api/v1/panels/"+id, cy, "")
	if _, ok := clientOf(t, a, panelClientName(id)); ok {
		t.Fatal("client left behind")
	}
}
