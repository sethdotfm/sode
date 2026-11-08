package app

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

const keyPrefix = "sode_"

// keyWriteEvery bounds how often use counts reach the disk; flush writes the rest.
const keyWriteEvery = time.Minute

// newKeySecret is the one-time key: prefix plus 32 random bytes, URL-safe.
func newKeySecret() string {
	b := make([]byte, 32)
	rand.Read(b)
	return keyPrefix + base64.RawURLEncoding.EncodeToString(b)
}

func keyHash(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// keyOwner is who a key acts as: a user (Name is the user's name) or an API client.
type keyOwner struct {
	Kind string `json:"kind"` // actorUser or actorClient
	Name string `json:"name"`
}

func (o keyOwner) is(p keyOwner) bool { return o.Kind == p.Kind && strings.EqualFold(o.Name, p.Name) }

// apiKey is a key's record without its hash; what lists and lookups return.
type apiKey struct {
	ID       string    `json:"id"`
	Owner    keyOwner  `json:"owner"`
	Name     string    `json:"name"` // the key name, e.g. "Panel 21"
	Created  time.Time `json:"created"`
	LastUsed time.Time `json:"lastUsed,omitempty"`
	Count    int       `json:"count"`
	Expires  time.Time `json:"expires,omitempty"` // zero = never
}

type keyRec struct {
	apiKey
	Hash string `json:"hash"`
}

// keyStore holds API keys beside the config in keys.json: sha256 only, mode 0600, atomic
// write like tokens.json. With no config file it is memory only. Never call with App.mu
// held: changes write the file.
type keyStore struct {
	mu     sync.Mutex
	recs   []keyRec
	path   string // "" = memory only
	fileMu sync.Mutex
	seq    uint64
	wrote  uint64
	dirty  bool
	lastW  time.Time
}

// loadKeys: once, from New. A damaged file is forgotten, as tokens are.
func loadKeys(cfgPath string) *keyStore {
	s := &keyStore{}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), "keys.json")
	if b, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(b, &s.recs); err != nil {
			log.Printf("api: %s is damaged, keys forgotten: %v", s.path, err)
			s.recs = nil
		}
	}
	return s
}

// save writes the file after taking a snapshot under mu, so a slow disk blocks no lookup.
func (s *keyStore) save() {
	if s.path == "" {
		return
	}
	s.mu.Lock()
	b, _ := json.Marshal(s.recs)
	s.seq++
	n := s.seq
	s.dirty, s.lastW = false, time.Now()
	s.mu.Unlock()
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if n < s.wrote {
		return
	}
	s.wrote = n
	if err := config.AtomicWrite(s.path, b); err != nil {
		log.Printf("api: keys not saved to %s: %v", s.path, err)
	}
}

// create stores a new key and returns its secret, shown once and kept nowhere.
func (s *keyStore) create(o keyOwner, name string, expires time.Time) (string, apiKey, error) {
	return s.createCapped(o, name, expires, 0)
}

// createCapped refuses once owner holds max keys (0 = no cap).
func (s *keyStore) createCapped(o keyOwner, name string, expires time.Time, max int) (string, apiKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", apiKey{}, errors.New("a key needs a key name")
	}
	secret := newKeySecret()
	id := make([]byte, 6)
	rand.Read(id)
	s.mu.Lock()
	if max > 0 && s.countMu(o) >= max {
		s.mu.Unlock()
		return "", apiKey{}, fmt.Errorf("%s already has %d keys", o.Name, max)
	}
	k := apiKey{ID: hex.EncodeToString(id), Owner: o, Name: name, Created: time.Now().UTC(), Expires: expires}
	s.recs = append(s.recs, keyRec{apiKey: k, Hash: keyHash(secret)})
	s.mu.Unlock()
	s.save()
	return secret, k, nil
}

func (s *keyStore) countMu(o keyOwner) int {
	n := 0
	for _, r := range s.recs {
		if r.Owner.is(o) {
			n++
		}
	}
	return n
}

func (s *keyStore) count(o keyOwner) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.countMu(o)
}

// lookup finds the live key for a presented secret; an expired key is not found.
func (s *keyStore) lookup(secret string, now time.Time) (apiKey, bool) {
	k, _, ok := s.find(secret, now)
	return k, ok
}

// find is lookup that also says a key was found but has expired, so the API can name that
// reason. An expired key still in the store is not returned as live.
func (s *keyStore) find(secret string, now time.Time) (k apiKey, expired, ok bool) {
	if !strings.HasPrefix(secret, keyPrefix) {
		return apiKey{}, false, false
	}
	h := keyHash(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.recs {
		if subtle.ConstantTimeCompare([]byte(r.Hash), []byte(h)) == 1 {
			if !r.Expires.IsZero() && !now.Before(r.Expires) {
				return apiKey{}, true, false
			}
			return r.apiKey, false, true
		}
	}
	return apiKey{}, false, false
}

// used counts one use. The file follows within keyWriteEvery or at flush.
func (s *keyStore) used(id string, now time.Time) {
	s.mu.Lock()
	for i := range s.recs {
		if s.recs[i].ID == id {
			s.recs[i].LastUsed, s.recs[i].Count = now.UTC(), s.recs[i].Count+1
			s.dirty = true
		}
	}
	due := s.dirty && time.Since(s.lastW) > keyWriteEvery
	s.mu.Unlock()
	if due {
		s.save()
	}
}

// flush writes counts not yet on disk.
func (s *keyStore) flush() {
	s.mu.Lock()
	d := s.dirty
	s.mu.Unlock()
	if d {
		s.save()
	}
}

// list is every key, oldest first.
func (s *keyStore) list() []apiKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]apiKey, len(s.recs))
	for i, r := range s.recs {
		out[i] = r.apiKey
	}
	slices.SortStableFunc(out, func(a, b apiKey) int { return a.Created.Compare(b.Created) })
	return out
}

func (s *keyStore) dropWhere(gone func(keyRec) bool) []apiKey {
	s.mu.Lock()
	var out []apiKey
	s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
		if gone(r) {
			out = append(out, r.apiKey)
			return true
		}
		return false
	})
	s.mu.Unlock()
	if len(out) > 0 {
		s.save()
	}
	return out
}

// revoke ends one key; false if there was none.
func (s *keyStore) revoke(id string) bool {
	return len(s.dropWhere(func(r keyRec) bool { return r.ID == id })) > 0
}

// dropOwner ends every key of owner: a user's on PIN change, rename or removal (ADR 0003), a
// client's on removal. Returns how many went.
func (s *keyStore) dropOwner(o keyOwner) int {
	return len(s.dropWhere(func(r keyRec) bool { return r.Owner.is(o) }))
}

// expire removes keys past their expiry and returns them, so the caller can log each.
func (s *keyStore) expire(now time.Time) []apiKey {
	return s.dropWhere(func(r keyRec) bool { return !r.Expires.IsZero() && !now.Before(r.Expires) })
}
