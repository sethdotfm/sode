package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// fixRestart starts a second App on a's config path after mut edits the config, as an
// edit of config.yaml while Sode was down would.
func fixRestart(t *testing.T, a *App, mut func(*config.Config)) *App {
	t.Helper()
	cfg := a.r().cfg.Clone()
	mut(&cfg)
	hist, _ := OpenHistory("")
	sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	b, err := New(cfg, a.cfgPath, hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func fixNewPin(t *testing.T, name, pin string) func(*config.Config) {
	return func(c *config.Config) {
		for i := range c.Users {
			if c.Users[i].Name == name {
				c.Users[i] = mkUser(t, name, pin)
			}
		}
	}
}

func fixOwnKey(t *testing.T, a *App) string {
	t.Helper()
	c := &client{ip: "10.0.0.5"}
	if err := a.UserUnlockAs(c, "Ana", "1234"); err != nil {
		t.Fatal(err)
	}
	secret, _, err := a.CreateOwnKey(c, "1234", "Desk", 0)
	if err != nil {
		t.Fatal(err)
	}
	return secret
}

func TestStartupEndsUserKeyWhosePinChangedWhileDown(t *testing.T) {
	a := userApp(t, false)
	secret := fixOwnKey(t, a)

	same := fixRestart(t, a, func(*config.Config) {})
	if _, _, ok := same.keys.find(secret, time.Now()); !ok {
		t.Fatal("key ended though the PIN did not change")
	}
	changed := fixRestart(t, a, fixNewPin(t, "Ana", "9999"))
	if _, _, ok := changed.keys.find(secret, time.Now()); ok {
		t.Fatal("key survived a PIN change made while Sode was down")
	}
	if n := len(changed.keys.list()); n != 0 {
		t.Fatalf("%d keys left", n)
	}
}

func TestStartupKeepsClientKeysWhenUserPinsChange(t *testing.T) {
	a, _ := apiTestApp(t, nil)
	secret := botKey(t, a)
	b := fixRestart(t, a, fixNewPin(t, "Ana", "9999"))
	if _, _, ok := b.keys.find(secret, time.Now()); !ok {
		t.Fatal("an API client's key ended with a user's PIN change")
	}
}

// A keys.json from before the fingerprint has none: those keys are kept and stamped at the
// first startup, so a PIN change after that ends them.
func TestStartupStampsLegacyUserKeys(t *testing.T) {
	a := userApp(t, false)
	secret := apiTestKey(t, a, keyOwner{Kind: actorUser, Name: "Ana"}, "Old", time.Time{})

	kept := fixRestart(t, a, func(*config.Config) {})
	if _, _, ok := kept.keys.find(secret, time.Now()); !ok {
		t.Fatal("legacy key not kept at first startup")
	}
	b, err := os.ReadFile(filepath.Join(filepath.Dir(a.cfgPath), "keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), `"pinFp"`) {
		t.Fatalf("fingerprint not stored: %s", b)
	}
	gone := fixRestart(t, a, fixNewPin(t, "Ana", "9999"))
	if _, _, ok := gone.keys.find(secret, time.Now()); ok {
		t.Fatal("stamped key survived a PIN change while down")
	}
}

func TestKeyFileNeverHoldsPinOrHash(t *testing.T) {
	a := userApp(t, false)
	fixOwnKey(t, a)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(a.cfgPath), "keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	ana, _ := a.r().cfg.FindUser("Ana")
	// "1234" as a substring turns up in random hex now and then, so only a whole value counts.
	var recs []map[string]any
	if err := json.Unmarshal(b, &recs); err != nil {
		t.Fatal(err)
	}
	for _, r := range recs {
		for k, v := range r {
			if str, ok := v.(string); ok && (str == "1234" || strings.Contains(str, ana.Pin)) {
				t.Fatalf("keys.json field %q holds the PIN or its hash: %s", k, b)
			}
		}
	}
}
