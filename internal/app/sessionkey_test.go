package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Seam: the `sessionKey` message from a signed-in connection, then /api/v1 with the key.

func sessionKeyApp(t *testing.T) *App {
	t.Helper()
	a := keyTestApp(t)
	cfg := a.r().cfg
	cfg.API.Enabled = true
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	return a
}

func sessionKeyOf(t *testing.T, a *App, c *client, next func() []map[string]any) string {
	t.Helper()
	r := send(t, a, c, next, inMsg{T: "sessionKey", ID: 1}, "sessionKeyResult")
	key, _ := r["key"].(string)
	if r["ok"] != true || !strings.HasPrefix(key, "sode_") {
		t.Fatalf("reply %v", r)
	}
	return key
}

func panelsStatus(a *App, key string) int {
	return apiSend(a, "GET", "/api/v1/panels", key, "").Code
}

func TestSessionKeyActsAsTheSignedInUser(t *testing.T) {
	a := sessionKeyApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	key := sessionKeyOf(t, a, c, next)
	id := makePanel(t, a, key, `{"name":"Mine"}`)
	p, _ := a.panels.get(id)
	if p.Owner != "Ana" {
		t.Fatalf("owner %q", p.Owner)
	}
	// A key shows in the user's own key list like any other, with its key name.
	l := send(t, a, c, next, inMsg{T: "keyListOwn", ID: 2}, "keyListOwnResult")
	if len(l["keys"].([]any)) != 1 {
		t.Fatalf("list %v", l)
	}
}

func TestSessionKeyNeedsASignIn(t *testing.T) {
	a := sessionKeyApp(t)
	c := &client{ip: "10.0.0.1"}
	next := registerAll(a, c)
	r := send(t, a, c, next, inMsg{T: "sessionKey", ID: 1}, "sessionKeyResult")
	if r["ok"] != false || r["key"] != nil {
		t.Fatalf("anonymous connection got a key: %v", r)
	}
}

func TestSessionKeyEndsOnSignOut(t *testing.T) {
	a := sessionKeyApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	key := sessionKeyOf(t, a, c, next)
	if panelsStatus(a, key) != 200 {
		t.Fatal("key refused before sign out")
	}
	a.SignOut(c)
	if got := panelsStatus(a, key); got != 401 {
		t.Fatalf("after sign out: %d", got)
	}
	if keyCount(a.keys, keyOwner{Kind: actorUser, Name: "Ana"}) != 0 {
		t.Fatal("record kept after sign out")
	}
}

func TestSessionKeyEndsWithItsSessionToken(t *testing.T) {
	a := sessionKeyApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	key := sessionKeyOf(t, a, c, next)
	a.tokens.mu.Lock()
	for k, r := range a.tokens.toks {
		r.At = time.Now().Add(-tabTokenMaxAge - time.Hour)
		a.tokens.toks[k] = r
	}
	a.tokens.mu.Unlock()
	// Refused at once, before the sweep runs.
	if got := panelsStatus(a, key); got != 401 {
		t.Fatalf("expired session: %d", got)
	}
	a.expireKeys(time.Now())
	if keyCount(a.keys, keyOwner{Kind: actorUser, Name: "Ana"}) != 0 {
		t.Fatal("record kept after the sweep")
	}
}

func TestSessionKeyEndsOnPinChange(t *testing.T) {
	a := sessionKeyApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	key := sessionKeyOf(t, a, c, next)
	cfg := a.r().cfg
	for i := range cfg.Users {
		if cfg.Users[i].Name == "Ana" {
			cfg.Users[i] = mkUser(t, "Ana", "4444")
		}
	}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if got := panelsStatus(a, key); got != 401 {
		t.Fatalf("after PIN change: %d", got)
	}
}

func TestSessionKeysStayWithTheirOwnUser(t *testing.T) {
	a := sessionKeyApp(t)
	ca, na := signedIn(t, a, "10.0.0.1", "1234")
	cb, nb := signedIn(t, a, "10.0.0.2", "567890")
	ka, kb := sessionKeyOf(t, a, ca, na), sessionKeyOf(t, a, cb, nb)
	ida := makePanel(t, a, ka, `{"name":"A"}`)
	idb := makePanel(t, a, kb, `{"name":"B"}`)
	pa, _ := a.panels.get(ida)
	pb, _ := a.panels.get(idb)
	if pa.Owner != "Ana" || pb.Owner != "Bo" {
		t.Fatalf("owners %q %q", pa.Owner, pb.Owner)
	}
	// Signing one browser out leaves the other's key alone.
	a.SignOut(ca)
	if panelsStatus(a, ka) != 401 || panelsStatus(a, kb) != 200 {
		t.Fatal("sign out reached the wrong key")
	}
}

func TestSessionKeyStaysOffTheLock(t *testing.T) {
	a := sessionKeyApp(t)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	a.setHeld(c, true)
	r := send(t, a, c, next, inMsg{T: "sessionKey", ID: 1}, "sessionKeyResult")
	if r["ok"] != false {
		t.Fatalf("a locked session got a key: %v", r)
	}
}

func TestSessionKeyForUserUnderPanelLock(t *testing.T) {
	a := sessionKeyApp(t)
	cfg := a.r().cfg
	cfg.Locks = config.Locks{Panel: true}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	a.Hi(c, "panel", "")
	key := sessionKeyOf(t, a, c, next)
	if panelsStatus(a, key) != 200 {
		t.Fatal("key refused")
	}
}
