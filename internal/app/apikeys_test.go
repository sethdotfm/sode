package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewKeyFormat(t *testing.T) {
	k := newKeySecret()
	if !strings.HasPrefix(k, "sode_") || len(k) != len("sode_")+43 {
		t.Fatalf("key %q", k)
	}
	if k == newKeySecret() {
		t.Fatal("two keys equal")
	}
}

func TestKeyStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "config.yaml")
	s := loadKeys(cfgPath)
	secret, rec, err := s.createCapped(keyOwner{Kind: actorClient, Name: "PanelsBot"}, "Panel 43", time.Time{}, 0, "")
	if err != nil {
		t.Fatal(err)
	}
	if rec.ID == "" || rec.Name != "Panel 43" || rec.Created.IsZero() {
		t.Fatalf("rec %+v", rec)
	}

	// Disk holds the hash and never the secret.
	raw, err := os.ReadFile(filepath.Join(dir, "keys.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), secret) || strings.Contains(string(raw), strings.TrimPrefix(secret, "sode_")) {
		t.Fatal("keys.json holds the key")
	}
	if fi, _ := os.Stat(filepath.Join(dir, "keys.json")); fi.Mode().Perm() != 0o600 {
		t.Errorf("mode %v", fi.Mode().Perm())
	}

	got, _, ok := s.find(secret, time.Now())
	if !ok || got.ID != rec.ID || got.Owner.Name != "PanelsBot" {
		t.Fatalf("lookup %+v %v", got, ok)
	}
	if _, _, ok := s.find("sode_nope", time.Now()); ok {
		t.Fatal("unknown key accepted")
	}

	s.used(rec.ID, time.Now())
	s.used(rec.ID, time.Now())
	s.flush()
	again := loadKeys(cfgPath)
	l := again.list()
	if len(l) != 1 || l[0].Count != 2 || l[0].LastUsed.IsZero() {
		t.Fatalf("after reload %+v", l)
	}
	if _, _, ok := again.find(secret, time.Now()); !ok {
		t.Fatal("key lost on reload")
	}

	if !s.revoke(rec.ID) || s.revoke(rec.ID) {
		t.Fatal("revoke result")
	}
	if _, _, ok := s.find(secret, time.Now()); ok {
		t.Fatal("revoked key still works")
	}
}

func TestKeyExpiry(t *testing.T) {
	s := loadKeys("")
	now := time.Now()
	secret, rec, _ := s.createCapped(keyOwner{Kind: actorUser, Name: "Ana"}, "Desk", now.Add(time.Hour), 0, "")
	if _, _, ok := s.find(secret, now); !ok {
		t.Fatal("live key refused")
	}
	if _, _, ok := s.find(secret, now.Add(2*time.Hour)); ok {
		t.Fatal("expired key accepted")
	}
	gone := s.expire(now.Add(2 * time.Hour))
	if len(gone) != 1 || gone[0].ID != rec.ID || len(s.list()) != 0 {
		t.Fatalf("expire %+v", gone)
	}
}

func TestKeyDropOwnerAndCaps(t *testing.T) {
	s := loadKeys("")
	for i := range 3 {
		s.createCapped(keyOwner{Kind: actorUser, Name: "Ana"}, string(rune('a'+i)), time.Time{}, 0, "")
	}
	s.createCapped(keyOwner{Kind: actorClient, Name: "Ana"}, "x", time.Time{}, 0, "")
	if n := keyCount(s, keyOwner{Kind: actorUser, Name: "ana"}); n != 3 {
		t.Fatalf("count %d (owner names compare ignoring case, kinds apart)", n)
	}
	if _, _, err := s.createCapped(keyOwner{Kind: actorUser, Name: "Ana"}, "z", time.Time{}, 3, ""); err == nil {
		t.Fatal("cap not enforced")
	}
	if n := s.dropOwner(keyOwner{Kind: actorUser, Name: "ANA"}); n != 3 {
		t.Fatalf("dropped %d", n)
	}
	if len(s.list()) != 1 {
		t.Fatal("client key dropped with the user's")
	}
}

func TestDamagedKeyFileIsForgotten(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "keys.json"), []byte("{nope"), 0o600)
	if n := len(loadKeys(filepath.Join(dir, "config.yaml")).list()); n != 0 {
		t.Fatal(n)
	}
}

func keyCount(s *keyStore, o keyOwner) int {
	n := 0
	for _, k := range s.list() {
		if k.Owner.is(o) {
			n++
		}
	}
	return n
}
