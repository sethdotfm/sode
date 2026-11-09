package app

import (
	"bytes"
	"log"
	"os"
	"strings"
	"testing"
)

// REST seam for a panel's key and its fragment link: Cy (Trusted) owns an API panel, Ana is a
// user with no Rights over it.

func keyedPanel(t *testing.T) (*App, *fakeRouter, string, string) {
	t.Helper()
	a, f := clientApp(t)
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, apiPanel+`,"clientRights":{"sources":[1,2,3],"dsts":[2,4]}}`)
	return a, f, cy, id
}

func mintKey(t *testing.T, a *App, owner, id string) (secret string, body map[string]any) {
	t.Helper()
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", owner, `{"instance":"Lobby"}`)
	if rec.Code != 201 {
		t.Fatalf("mint: %d %s", rec.Code, rec.Body.String())
	}
	body = apiBody(t, rec)
	secret, _ = body["key"].(string)
	if !strings.HasPrefix(secret, keyPrefix) {
		t.Fatalf("no key in %s", rec.Body.String())
	}
	return secret, body
}

func TestPanelKeyLinkRunsAsItsClientWithItsRights(t *testing.T) {
	a, f, cy, id := keyedPanel(t)
	secret, body := mintKey(t, a, cy, id)
	if want := "panels/" + id + "?instance=Lobby#key=" + secret; body["link"] != want {
		t.Fatalf("link %v, want %s", body["link"], want)
	}
	f.seed(2, 1, 0)
	rec := apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 || !strings.Contains(rec.Body.String(), `"result":"connected"`) {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) != 1 || es[0].Actor == nil || es[0].Actor.Kind != actorClient || es[0].Actor.Name != panelClientName(id) {
		t.Fatalf("entries %+v", es)
	}
	// The client's Rights decide, not the owner's.
	n := f.sends()
	rec = apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":3,"dst":5,"levels":[1]}]}`, nil)
	wantErr(t, rec, 403, "outside_rights")
	if f.sends() != n {
		t.Fatal("a refused take reached the router")
	}
}

func TestPanelKeyIsShownOnceAndStoredHashed(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret, body := mintKey(t, a, cy, id)
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", cy, `{"instance":"Lobby"}`); !strings.HasPrefix(rec.Header().Get("Cache-Control"), "no-store") {
		t.Fatalf("cache header %q", rec.Header().Get("Cache-Control"))
	}
	for _, r := range a.keys.recs {
		if strings.Contains(r.Hash, secret) {
			t.Fatal("the secret is stored")
		}
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, cy, ""); strings.Contains(rec.Body.String(), secret) {
		t.Fatal("the panel read gives the key back")
	}
	if body["id"] == "" || body["id"] == nil {
		t.Fatalf("no key id in %v", body)
	}
}

func TestPanelKeyRotateKillsTheOldOne(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	first, b1 := mintKey(t, a, cy, id)
	second, b2 := mintKey(t, a, cy, id)
	if first == second {
		t.Fatal("same key twice")
	}
	if b1["rotated"] == true || b2["rotated"] != true {
		t.Fatalf("rotated flags %v %v", b1["rotated"], b2["rotated"])
	}
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", first, ""), 401, "key_revoked")
	if rec := apiSend(a, "GET", "/api/v1/tally", second, ""); rec.Code != 200 {
		t.Fatalf("new key: %d %s", rec.Code, rec.Body.String())
	}
	n := 0
	for _, k := range a.keys.list() {
		if k.Owner.is(keyOwner{Kind: actorClient, Name: panelClientName(id)}) {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("%d keys on the client", n)
	}
}

func TestPanelKeyRevokeLeavesTheClient(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key?instance=Lobby", cy, ""); rec.Code != 204 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", secret, ""), 401, "key_revoked")
	p, _ := a.panels.get(id)
	if _, ok := clientOfPanel(a.r().cfg, p); !ok || p.Acts != panelActsAPI {
		t.Fatal("revoke removed the client")
	}
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key?instance=Lobby", cy, ""); rec.Code != 204 {
		t.Fatalf("second revoke: %d", rec.Code)
	}
	mintKey(t, a, cy, id)
}

func TestPanelKeyIsForOwnersAndTrustedOnly(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	ana := userKey(t, a, "Ana")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/key", ana, ""), 404, "not_found")
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, apiPanel+`,"share":{"mode":"everyone"}}`); rec.Code != 200 {
		t.Fatalf("share: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/key", ana, ""), 403, "not_owner")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key", ana, ""), 403, "not_owner")
	// The panel's own key cannot make another.
	secret, _ := mintKey(t, a, cy, id)
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", secret, ""); rec.Code != 403 {
		t.Fatalf("a panel key made a key: %d", rec.Code)
	}
}

func TestPanelKeyOnlyForAnAPIPanel(t *testing.T) {
	a, _, cy, _ := keyedPanel(t)
	id := makePanel(t, a, cy, `{"name":"Mine"}`)
	wantErr(t, apiSend(a, "POST", "/api/v1/panels/"+id+"/key", cy, `{"instance":"Lobby"}`), 409, "conflict")
	wantErr(t, apiSend(a, "DELETE", "/api/v1/panels/"+id+"/key?instance=Lobby", cy, ""), 409, "conflict")
}

func TestPanelKeyNeverInALogLineAndNotAQueryCredential(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	a, _, cy, id := keyedPanel(t)
	secret, _ := mintKey(t, a, cy, id)
	apiSend(a, "GET", "/api/v1/tally", secret, "")
	// A key in the query string is not a credential, so a link built that way would not work
	// and its request line would be the leak the fragment avoids.
	wantErr(t, apiDo(a, "GET", "/api/v1/tally?key="+secret, "", nil), 401, "key_required")
	if strings.Contains(buf.String(), secret) {
		t.Fatalf("log has the key: %s", buf.String())
	}
}

func TestPanelWithoutKeyOrSignInRefusesToAct(t *testing.T) {
	a, f, _, id := keyedPanel(t)
	n := f.sends()
	wantErr(t, apiDo(a, "GET", "/api/v1/panels/"+id, "", nil), 401, "key_required")
	wantErr(t, apiPost(a, "/api/v1/takes", "", `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, nil), 401, "key_required")
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", "sode_nope", ""), 401, "invalid_key")
	if f.sends() != n || a.hist.Len() != 0 {
		t.Fatal("an unauthenticated take acted")
	}
}
