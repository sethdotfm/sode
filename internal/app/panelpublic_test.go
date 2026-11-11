package app

import (
	"fmt"
	"net/http/httptest"
	"os"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// REST seam for a Public panel: anyone with its URL gets a short-lived key for the panel's own
// client, asked for with the panel id alone.

const publicPanel = `{"name":"Lobby","acts":"public"`

func TestPublicNeedsATrustedUserWithinTheirRights(t *testing.T) {
	a, _ := clientApp(t)
	cy, ana := userKey(t, a, "Cy"), userKey(t, a, "Ana")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", ana, publicPanel+`}`), 403, "trusted_required")
	wantErr(t, apiSend(a, "POST", "/api/v1/panels", cy, publicPanel+`,"clientRights":{"sources":[1,8],"dsts":[2]}}`), 403, "outside_rights")
	rec := apiSend(a, "POST", "/api/v1/panels", cy, publicPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	if rec.Code != 201 || apiBody(t, rec)["acts"] != "public" {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if _, ok := clientOf(t, a, apiBody(t, rec)["client"].(string)); !ok {
		t.Fatal("a Public panel owns a client")
	}
}

// An administrator may widen an API panel's client past Cy's Rights; Cy may still save it as
// API, but opening it to anyone checks Cy's Rights again.
func TestSwitchingToPublicChecksTheCallerAgain(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, apiPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	cfg := a.r().cfg
	for i, c := range cfg.APIClients {
		if c.Panel == id {
			cfg.APIClients[i].Rights = config.Rights{Sources: ints(1, 9), Dsts: ints(2)}
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, apiPanel+`}`); rec.Code != 200 {
		t.Fatalf("api save: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, cy, publicPanel+`}`), 403, "outside_rights")

	ana := userKey(t, a, "Ana")
	own := makePanel(t, a, ana, `{"name":"Mine"}`)
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+own, ana, publicPanel+`}`), 403, "trusted_required")
}

func askPublic(a *App, peer, id string) *httptest.ResponseRecorder {
	r := httptest.NewRequest("POST", "/api/v1/panels/"+id+"/public", nil)
	r.RemoteAddr = peer
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

// Every keyless open asks, so "not Public" is a plain answer, not an error the browser logs. A
// panel that does not exist gets the same answer, so the route does not say which ids exist.
func wantNotPublic(t *testing.T, rec *httptest.ResponseRecorder) {
	t.Helper()
	if rec.Code != 200 || rec.Body.String() != "{\"public\":false}\n" {
		t.Fatalf("got %d %s, want 200 and public false", rec.Code, rec.Body.String())
	}
}

func publicKeyOf(t *testing.T, a *App, id string) string {
	t.Helper()
	rec := askPublic(a, apiPeer, id)
	if rec.Code != 201 {
		t.Fatalf("public key: %d %s", rec.Code, rec.Body.String())
	}
	body := apiBody(t, rec)
	k, _ := body["key"].(string)
	if !strings.HasPrefix(k, keyPrefix) || body["expires"] == nil || body["public"] != true {
		t.Fatalf("no key or expiry in %s", rec.Body.String())
	}
	return k
}

func TestPublicPanelHandsOutAKeyWithTheClientsRights(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	key := publicKeyOf(t, a, id)
	rec := apiSend(a, "GET", "/api/v1/panels/"+id, key, "")
	if rec.Code != 200 {
		t.Fatalf("read with the public key: %d %s", rec.Code, rec.Body.String())
	}
	r, _ := apiBody(t, rec)["rights"].(map[string]any)
	if r == nil || !reflect.DeepEqual(r["sources"], []any{1.0}) || !reflect.DeepEqual(r["dsts"], []any{2.0}) {
		t.Fatalf("rights %v", r)
	}
	if n := len(keyList(t, a, cy, id)); n != 0 {
		t.Fatalf("%d instances listed, want the public key left out", n)
	}
}

func TestOnlyAPublicPanelHandsOutAKey(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	for _, body := range []string{`{"name":"S"}`, apiPanel + `}`, `{"name":"B","acts":"both"}`} {
		id := makePanel(t, a, cy, body)
		wantNotPublic(t, askPublic(a, apiPeer, id))
	}
}

// Every keyless open of a panel asks first, so a panel that exists and is not Public is not a guess.
func TestAskingANonPublicPanelIsNotCountedAsAFailedKey(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"S"}`)
	for i := 0; i < apiAuthFailMax+5; i++ {
		wantNotPublic(t, askPublic(a, apiPeer, id))
	}
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, cy, ""); rec.Code != 200 {
		t.Fatalf("after the asks: %d %s", rec.Code, rec.Body.String())
	}
}

// Asks are capped per address whatever the id, and an unknown id, a panel that is not Public and a
// Public panel asked from outside its networks get the same answers, so no run of asks says which
// ids exist.
func TestAskingSaysNothingAboutWhichIdsExist(t *testing.T) {
	seq := func(mk func(a *App, cy string) string) []string {
		a, _ := clientApp(t)
		cy := userKey(t, a, "Cy")
		id := mk(a, cy)
		var out []string
		for i := 0; i < publicAskMax+2; i++ {
			rec := askPublic(a, apiPeer, id)
			out = append(out, fmt.Sprint(rec.Code, " ", rec.Body.String()))
		}
		return out
	}
	unknown := seq(func(*App, string) string { return "p-00000000" })
	session := seq(func(a *App, cy string) string { return makePanel(t, a, cy, `{"name":"S"}`) })
	outside := seq(func(a *App, cy string) string {
		id := makePanel(t, a, cy, publicPanel+`}`)
		if rec := putNetworks(a, cy, id, `["10.0.0.0/8"]`); rec.Code != 200 {
			t.Fatalf("networks: %d %s", rec.Code, rec.Body.String())
		}
		return id
	})
	if !reflect.DeepEqual(unknown, session) || !reflect.DeepEqual(unknown, outside) {
		t.Fatalf("answers differ:\nunknown %v\nsession %v\noutside %v", unknown, session, outside)
	}
	if !strings.HasPrefix(unknown[len(unknown)-1], "429") {
		t.Fatalf("no cap: %v", unknown[len(unknown)-1])
	}
}

// The cap on asking does not depend on the API rate limit, so with the limit off a caller still
// cannot loop the route.
func TestAskingIsCappedWithTheRateLimitOff(t *testing.T) {
	a, _ := clientApp(t)
	cfg := a.r().cfg
	cfg.API.RateLimit = 0
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	for i := 0; i < publicAskMax; i++ {
		if rec := askPublic(a, apiPeer, id); rec.Code != 201 {
			t.Fatalf("ask %d: %d %s", i, rec.Code, rec.Body.String())
		}
	}
	wantErr(t, askPublic(a, apiPeer, id), 429, "rate_limited")
	if rec := askPublic(a, "10.9.9.9:5000", id); rec.Code != 201 {
		t.Fatalf("another address: %d %s", rec.Code, rec.Body.String())
	}
}

// Past the cap an address loses its own oldest key first, so one caller looping the route cannot
// push every other page's key out; the key that ends takes its streams with it.
func TestPastTheCapAnAddressLosesItsOwnKeyFirst(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	other := askPublic(a, "10.1.1.1:5000", id)
	if other.Code != 201 {
		t.Fatalf("other: %d %s", other.Code, other.Body.String())
	}
	otherKey := apiBody(t, other)["key"].(string)
	first := publicKeyOf(t, a, id)
	s := strmDial(t, strmServer(t, a), first, "status")
	s.next()
	for i := 0; i < publicKeyMax; i++ {
		publicKeyOf(t, a, id)
	}
	rvkClosed(t, s)
	wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+id, first, ""), 401, "key_revoked")
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, otherKey, ""); rec.Code != 200 {
		t.Fatalf("the other address's key: %d %s", rec.Code, rec.Body.String())
	}
}

func TestLeavingPublicEndsItsKeys(t *testing.T) {
	for _, to := range []string{`{"name":"Lobby","acts":"session"}`, apiPanel + `}`, `{"name":"Lobby","acts":"both"}`} {
		a, _ := clientApp(t)
		cy := userKey(t, a, "Cy")
		id := makePanel(t, a, cy, publicPanel+`}`)
		key := publicKeyOf(t, a, id)
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, to); rec.Code != 200 {
			t.Fatalf("switch: %d %s", rec.Code, rec.Body.String())
		}
		wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+id, key, ""), 401, "key_revoked")
		wantNotPublic(t, askPublic(a, apiPeer, id))
		_, has := clientOf(t, a, "Panel "+id)
		if strings.Contains(to, "session") == has {
			t.Fatalf("after %s the client is there: %v", to, has)
		}
	}
}

func TestDeletingAPublicPanelEndsItsKeys(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	key := publicKeyOf(t, a, id)
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id, cy, ""); rec.Code != 204 {
		t.Fatalf("delete: %d %s", rec.Code, rec.Body.String())
	}
	wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+id, key, ""), 401, "key_revoked")
}

// A key this Sode no longer holds (a restart forgets them) must read as ended, so a wall of
// tablets behind one address asks again instead of locking itself out.
func TestAForgottenPublicKeyReadsAsEnded(t *testing.T) {
	a, _ := clientApp(t)
	key := newPublicSecret()
	for i := 0; i < apiAuthFailMax+2; i++ {
		wantErr(t, apiDo(a, "GET", "/api/v1/tally", key, nil), 401, "key_revoked")
	}
}

// Once the start is longer ago than a key lives, a key of that shape this Sode never handed out is
// a guess and counts toward the failed-key throttle. It still reads as ended, so a page that slept
// through a restart asks again.
func TestAnUnknownPublicShapedKeyCountsOnceNoKeyCanPredateTheStart(t *testing.T) {
	a, _ := clientApp(t)
	a.started = time.Now().Add(-publicKeyTTL - time.Minute)
	cy := userKey(t, a, "Cy")
	for i := 0; i < apiAuthFailMax; i++ {
		wantErr(t, apiDo(a, "GET", "/api/v1/tally", newPublicSecret(), nil), 401, "key_revoked")
	}
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", cy, ""), 429, "rate_limited")
}

// A key Sode handed out and then dropped (expired and swept, pushed out past the cap, or ended when
// the panel left Public) still reads as ended and is never counted.
func TestADroppedPublicKeyIsNeverCounted(t *testing.T) {
	a, _ := clientApp(t)
	a.started = time.Now().Add(-publicKeyTTL - time.Minute)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	swept := publicKeyOf(t, a, id)
	a.expireKeys(time.Now().Add(publicKeyTTL + time.Second))
	pushed := publicKeyOf(t, a, id)
	for i := 0; i < publicKeyMax; i++ {
		publicKeyOf(t, a, id)
	}
	left := publicKeyOf(t, a, id)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, cy, `{"name":"Lobby"}`); rec.Code != 200 {
		t.Fatalf("leave Public: %d %s", rec.Code, rec.Body.String())
	}
	for i := 0; i < apiAuthFailMax; i++ {
		for _, k := range []string{swept, pushed, left} {
			if rec := apiDo(a, "GET", "/api/v1/tally", k, nil); rec.Code != 401 {
				t.Fatalf("dropped key: %d %s", rec.Code, rec.Body.String())
			}
		}
	}
	if rec := apiSend(a, "GET", "/api/v1/tally", cy, ""); rec.Code != 200 {
		t.Fatalf("after the dropped keys: %d %s", rec.Code, rec.Body.String())
	}
}

func TestAPublicKeyReachesOnlyItsPanel(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	other := makePanel(t, a, cy, publicPanel+`}`)
	key := publicKeyOf(t, a, id)
	wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+other, key, ""), 404, "not_found")
}

func TestPublicKeysAreNotWrittenToDisk(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	key := publicKeyOf(t, a, id)
	a.keys.save()
	b, _ := os.ReadFile(a.keys.path)
	if strings.Contains(string(b), keyHash(key)) {
		t.Fatal("keys.json holds a public key")
	}
}

func TestPublicKeysPerPanelAreCapped(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	first := publicKeyOf(t, a, id)
	for i := 0; i < publicKeyMax; i++ {
		publicKeyOf(t, a, id)
	}
	wantErr(t, apiSend(a, "GET", "/api/v1/panels/"+id, first, ""), 401, "key_revoked")
}

// Outside the client's networks a Public panel reads as not Public, as an unknown id does.
func TestPublicKeyOnlyFromTheClientsNetworks(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`}`)
	if rec := putNetworks(a, cy, id, `["10.0.0.0/8"]`); rec.Code != 200 {
		t.Fatalf("networks: %d %s", rec.Code, rec.Body.String())
	}
	wantNotPublic(t, askPublic(a, apiPeer, id))
	if rec := askPublic(a, "10.1.2.3:5000", id); rec.Code != 201 {
		t.Fatalf("from inside: %d %s", rec.Code, rec.Body.String())
	}
}

// Every page holds a key named Public, so an unnamed page is told apart by its address, as the
// monitor does, not by the key's name.
func TestAnUnnamedPublicPageIsLoggedByItsAddress(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	key := publicKeyOf(t, a, id)
	f.seed(2, 3, 0)
	if rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	if e := lastRow(t, a, "take"); !strings.Contains(e.Note, "instance 192.0.2.7") || strings.Contains(e.Note, "instance Public") {
		t.Fatalf("take row %+v", e)
	}
}

func TestPublicTakeOutsideTheRightsIsRefusedAndHistoryNamesTheClient(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	key := publicKeyOf(t, a, id)
	n := f.sends()
	wantErr(t, apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, nil), 403, "outside_rights")
	if f.sends() != n {
		t.Fatal("a refused take reached the router")
	}
	f.seed(2, 3, 0)
	rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) == 0 || es[0].Actor == nil || es[0].Actor.Name != "Panel "+id {
		t.Fatalf("history %+v", es)
	}
}

// The page lock does not stand between a Public panel and its key: the page drops the PIN pad
// itself (lock.js). The server still counts the panel page locked, so it sends it no names.
func TestPageLockLeavesThePublicKeyAndSendsNoNames(t *testing.T) {
	a, f := apiTestApp(t, func(c *config.Config) {
		cy := mkUser(t, "Cy", "4321")
		cy.Trusted = true
		cy.Rights = config.Rights{Sources: ints(1, 2, 3), Dsts: ints(2, 4)}
		c.Users = append(c.Users, cy)
		c.Locks = config.Locks{Panel: true}
		c.SettingsPin = mkUser(t, "", "2468").Pin
	})
	shortTimers(a)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	key := publicKeyOf(t, a, id)
	if rec := apiSend(a, "GET", "/api/v1/panels/"+id, key, ""); rec.Code != 200 || apiBody(t, rec)["acts"] != "public" {
		t.Fatalf("read under the page lock: %d %s", rec.Code, rec.Body.String())
	}
	f.seed(2, 3, 0)
	if rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":1,"dst":2,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("take under the page lock: %d %s", rec.Code, rec.Body.String())
	}
	wantNotPublic(t, askPublic(a, apiPeer, makePanel(t, a, cy, `{"name":"S"}`)))

	c := &client{ip: "10.0.0.1"}
	c.u.page = "panel"
	a.mu.Lock()
	m := a.lockStateMu(c)
	a.mu.Unlock()
	if !m.Enforced || !m.Locked || len(m.Names) != 0 || m.User != "" {
		t.Fatalf("panel page lockState %+v, want locked with no names", m)
	}
}

// A Public key is anyone's, so its panel reads name no user.
func TestPublicKeyReadsNoUserNames(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`,"share":{"mode":"named","users":["Ana"]},"clientRights":{"sources":[1],"dsts":[2]}}`)
	key := publicKeyOf(t, a, id)
	for _, path := range []string{"/api/v1/panels/" + id, "/api/v1/panels"} {
		rec := apiSend(a, "GET", path, key, "")
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
		}
		if b := rec.Body.String(); strings.Contains(b, `"Cy"`) || strings.Contains(b, `"Ana"`) {
			t.Fatalf("%s names a user: %s", path, b)
		}
	}
}

func TestPublicKeyReadsNoTemplateOwner(t *testing.T) {
	a, _ := clientApp(t)
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, publicPanel+`,"clientRights":{"sources":[1],"dsts":[2]}}`)
	if rec := apiSend(a, "POST", "/api/v1/panel-templates", cy, `{"name":"Grid","scope":"everyone","from":"`+id+`"}`); rec.Code != 201 {
		t.Fatalf("template: %d %s", rec.Code, rec.Body.String())
	}
	rec := apiSend(a, "GET", "/api/v1/panel-templates", publicKeyOf(t, a, id), "")
	if rec.Code != 200 || strings.Contains(rec.Body.String(), `"Cy"`) {
		t.Fatalf("templates for a Public key: %d %s", rec.Code, rec.Body.String())
	}
}
