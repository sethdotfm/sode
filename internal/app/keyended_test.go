package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// A key that was real and then ended answers `key_revoked` (or `key_expired`) and never counts toward
// the per-address throttle, so a stale page cannot lock out the other pages at its address.

func failCount(a *App, ip string) int {
	a.apiS.mu.Lock()
	defer a.apiS.mu.Unlock()
	if f := a.apiS.fails[ip]; f != nil {
		return f.n
	}
	return 0
}

const apiPeerIP = "192.0.2.7"

func wantEndedUncounted(t *testing.T, a *App, key, code string) {
	t.Helper()
	for range apiAuthFailMax * 3 {
		wantErr(t, apiDo(a, "GET", "/api/v1/names", key, nil), 401, code)
	}
	if n := failCount(a, apiPeerIP); n != 0 {
		t.Fatalf("%d failed attempts counted for an ended key", n)
	}
	if rec := apiDo(a, "GET", "/api/v1/names", botKey(t, a), nil); rec.Code != 200 {
		t.Fatalf("a good key from the same address: %d %s", rec.Code, rec.Body.String())
	}
}

func TestRevokedKeyAnswersKeyRevokedUncounted(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Wall", time.Time{})
	k, _, _ := a.keys.find(key, time.Now())
	if !a.keys.revoke(k.ID) {
		t.Fatal("not revoked")
	}
	wantEndedUncounted(t, a, key, "key_revoked")
	if m := apiBody(t, apiDo(a, "GET", "/api/v1/names", key, nil))["error"].(map[string]any)["message"].(string); !strings.Contains(m, "stop") {
		t.Errorf("message %q does not say to stop", m)
	}
}

func TestOwnerChangeEndsKeyAsRevoked(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := userKey(t, a, "Bo")
	if err := a.SaveUsers(adminClient(t, a), usersReq{Users: []userEdit{{Name: "Ana"}}}); err != nil {
		t.Fatal(err)
	}
	wantEndedUncounted(t, a, key, "key_revoked")
}

func TestRotatedInstanceKeyAnswersKeyRevoked(t *testing.T) {
	a, _, cy, id := keyedPanel(t)
	old, _ := mintKey(t, a, cy, id)
	mintKey(t, a, cy, id) // the same instance again: rotates it
	wantEndedUncounted(t, a, old, "key_revoked")
}

func TestExpiredKeyStaysKeyExpiredAfterTheSweep(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	exp := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Old", time.Now().Add(30*time.Millisecond))
	time.Sleep(60 * time.Millisecond)
	wantEndedUncounted(t, a, exp, "key_expired")
	a.expireKeys(time.Now())
	wantEndedUncounted(t, a, exp, "key_expired")
}

func TestUnknownKeysStillCount(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	for _, k := range []string{"sode_wrong", sessionKeyPrefix + "x", sessionKeyPrefix + strings.Repeat("A", 42) + "!"} {
		wantErr(t, apiDo(a, "GET", "/api/v1/names", k, nil), 401, "invalid_key")
	}
	if n := failCount(a, apiPeerIP); n != 3 {
		t.Fatalf("%d counted, want 3", n)
	}
}

func TestEndedKeyIsAnsweredDuringALockout(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Wall", time.Time{})
	k, _, _ := a.keys.find(key, time.Now())
	a.keys.revoke(k.ID)
	for range apiAuthFailMax {
		apiDo(a, "GET", "/api/v1/names", "sode_wrong", nil)
	}
	before := failCount(a, apiPeerIP)
	wantErr(t, apiDo(a, "GET", "/api/v1/names", key, nil), 401, "key_revoked")
	if failCount(a, apiPeerIP) != before {
		t.Fatal("an ended key counted during the lockout")
	}
	wantErr(t, apiDo(a, "GET", "/api/v1/names", botKey(t, a), nil), 429, "rate_limited")
}

func TestEndedAnswersHaveTheirOwnCap(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Wall", time.Time{})
	k, _, _ := a.keys.find(key, time.Now())
	a.keys.revoke(k.ID)
	for range apiEndedMax {
		wantErr(t, apiDo(a, "GET", "/api/v1/names", key, nil), 401, "key_revoked")
	}
	rec := apiDo(a, "GET", "/api/v1/names", key, nil)
	wantErr(t, rec, 429, "rate_limited")
	if rec.Header().Get("Retry-After") == "" {
		t.Error("no Retry-After")
	}
	if rec := apiDo(a, "GET", "/api/v1/names", botKey(t, a), nil); rec.Code != 200 {
		t.Fatalf("a good key from the same address: %d", rec.Code)
	}
}

func TestEndedKeyUseIsOneRowPerAddress(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	defer startLog(t, a)()
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Wall", time.Time{})
	k, _, _ := a.keys.find(key, time.Now())
	a.keys.revoke(k.ID)
	for range 7 {
		apiDo(a, "GET", "/api/v1/names", key, nil)
	}
	a.flushAuthFails(time.Now().Add(2 * time.Minute))
	a.flushAuthFails(time.Now().Add(3 * time.Minute))
	waitLogged(t, a, 1)
	time.Sleep(100 * time.Millisecond)
	n := 0
	for _, e := range logged(t, a) {
		if e.Kind == kindAPI && e.Why == "endedKeyUsed" {
			n++
			if e.By != apiPeerIP || !strings.Contains(e.Note, "7") || strings.Contains(e.Note, keyPrefix) {
				t.Errorf("row %+v", e)
			}
		}
		if e.Why == "authFailed" {
			t.Errorf("authFailed row %+v", e)
		}
	}
	if n != 1 {
		t.Fatalf("%d endedKeyUsed rows", n)
	}
}

func TestEndedKeysOutliveARestart(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Wall", time.Time{})
	k, _, _ := a.keys.find(key, time.Now())
	a.keys.revoke(k.ID)
	again := loadKeys(a.cfgPath)
	if _, ended := again.endedAs(key); !ended {
		t.Fatal("the ended key was forgotten by a restart")
	}
}

func TestEndedKeysAreBounded(t *testing.T) {
	s := &keyStore{}
	o := keyOwner{Kind: actorClient, Name: "Bot"}
	var first string
	for i := range endedKeyMax + 5 {
		secret, k, err := s.createCapped(o, fmt.Sprint("k", i), time.Time{}, 0, "")
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			first = secret
		}
		s.revoke(k.ID)
	}
	if len(s.ended) != endedKeyMax || len(s.endedAt) != endedKeyMax {
		t.Fatalf("%d ended held", len(s.ended))
	}
	if _, ended := s.endedAs(first); ended {
		t.Fatal("the oldest was kept past the bound")
	}
}

// A memory-only Sode forgets every key on a restart; a session key still reads as ended by its shape.
func TestSessionShapedKeyReadsAsEnded(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	key := newSessionSecret()
	if !isSessionShape(key) || isSessionShape(newKeySecret()) || isSessionShape(newShortSecret()) {
		t.Fatal("shapes overlap")
	}
	wantEndedUncounted(t, a, key, "key_revoked")
}

func TestSessionKeyEndsAsRevokedOnSignOut(t *testing.T) {
	a := sessionKeyApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	key := sessionKeyOf(t, a, c, next)
	if !isSessionShape(key) {
		t.Fatalf("session key %q lacks the session shape", key)
	}
	a.SignOut(c)
	wantErr(t, apiSend(a, "GET", "/api/v1/panels", key, ""), 401, "key_revoked")
	if n := failCount(a, apiPeerIP); n != 0 {
		t.Fatalf("%d counted", n)
	}
}

// Tabs that share a session each hold a key; a new one does not end another tab's.
func TestSessionKeysPerSessionAreCapped(t *testing.T) {
	a := sessionKeyApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	keys := []string{}
	for range sessionKeyMax {
		keys = append(keys, sessionKeyOf(t, a, c, next))
	}
	for i, k := range keys {
		if panelsStatus(a, k) != 200 {
			t.Fatalf("key %d of %d ended early", i, sessionKeyMax)
		}
	}
	sessionKeyOf(t, a, c, next)
	wantErr(t, apiSend(a, "GET", "/api/v1/panels", keys[0], ""), 401, "key_revoked")
	if panelsStatus(a, keys[1]) != 200 {
		t.Fatal("more than the oldest went")
	}
}

func TestEndedKeyCodeIsDocumented(t *testing.T) {
	if !strings.Contains(string(openAPIFile), "- key_revoked\n") {
		t.Error("the Error.code enum lacks key_revoked")
	}
}
