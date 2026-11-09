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

func newKeySecret() string { return newSecret(32) }

// newShortSecret is 128 bits, for a panel's link key: the key is throttled, revocable and one panel's
// own, and every character of it is in the link.
// Its body never starts with the prefix, so the page can tell a link that left the prefix off from one that kept it.
func newShortSecret() string {
	for {
		if s := newSecret(16); !strings.HasPrefix(strings.TrimPrefix(s, keyPrefix), keyPrefix) {
			return s
		}
	}
}

// A session key's own prefix. '.' is outside base64url, so no other key can carry it, and a
// key of this shape that Sode does not hold reads as ended (isSessionShape).
const sessionKeyPrefix = keyPrefix + "ses."

const sessionSecretBytes = 32

func newSessionSecret() string {
	b := make([]byte, sessionSecretBytes)
	rand.Read(b)
	return sessionKeyPrefix + base64.RawURLEncoding.EncodeToString(b)
}

// isSessionShape: a secret only newSessionSecret makes. A memory-only Sode forgets its session
// keys on a restart, and the pages still holding one must hear that it ended. Answering such a
// key without counting it toward the throttle costs nothing: a 256-bit guess never lands.
func isSessionShape(secret string) bool {
	body, ok := strings.CutPrefix(secret, sessionKeyPrefix)
	if !ok || len(body) != base64.RawURLEncoding.EncodedLen(sessionSecretBytes) {
		return false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(body)
	return err == nil && len(b) == sessionSecretBytes
}

func newSecret(n int) string {
	b := make([]byte, n)
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
	LastUsed time.Time `json:"lastUsed,omitzero"`
	Count    int       `json:"count"`
	Expires  time.Time `json:"expires,omitzero"` // zero = never
}

// subject names this key in the rate table and the idempotency table.
func (k *apiKey) subject() string { return "key:" + k.ID }

type keyRec struct {
	apiKey
	Hash string `json:"hash"`
	// PinFP fingerprints the owner's PIN hash at creation, for user keys only, so a PIN
	// edited in config.yaml while Sode was down still ends the key. Empty in a keys.json
	// from before it existed; reconcile stamps those.
	PinFP string `json:"pinFp,omitempty"`
	// Sess is tokenKey of the session token a browser's key rides on (sessionkey.go); empty
	// for every other key. The key is live only while that token is.
	Sess string `json:"sess,omitempty"`
	// Setup marks a one-time setup link (panelsetup.go): Hash is its token's. It is never a
	// key, so find, list, count and expire pass over it.
	Setup bool `json:"setup,omitempty"`
}

// pinFP is a one-way stamp of a user's PIN hash: stored in place of it, never the PIN or hash.
func pinFP(pinHash string) string {
	h := sha256.Sum256([]byte("sode key owner pin v1\x00" + pinHash))
	return hex.EncodeToString(h[:16])
}

// endedKeyMax bounds the ended keys remembered; the oldest is forgotten first.
const endedKeyMax = 1024

// endedKey is a key that ended, by its hash, so a page still sending it hears `key_revoked`
// instead of counting as a guess. Session keys are left out: their shape says it.
type endedKey struct {
	Hash    string    `json:"hash"`
	At      time.Time `json:"at"`
	Expired bool      `json:"expired,omitempty"`
}

// keyStore holds API keys beside the config in keys.json, and ended keys in keys-ended.json:
// sha256 only, mode 0600, atomic write like tokens.json. With no config file it is memory
// only. Never call with App.mu held: changes write the file.
type keyStore struct {
	mu       sync.Mutex
	recs     []keyRec
	ended    map[string]bool // hash: ended by its expiry
	endedAt  []endedKey      // oldest first, at most endedKeyMax
	path     string          // "" = memory only
	fileMu   sync.Mutex
	seq      uint64
	wrote    uint64
	dirty    bool
	lastW    time.Time
	endSeq   uint64 // bumped under mu when endedAt changes
	endWrote uint64 // last endSeq written (under fileMu)
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
	var ended []endedKey
	if b, err := os.ReadFile(s.endedPath()); err == nil {
		if err := json.Unmarshal(b, &ended); err != nil {
			log.Printf("api: %s is damaged, ended keys forgotten: %v", s.endedPath(), err)
			ended = nil
		}
	}
	for _, e := range ended {
		s.noteEndedMu(e)
	}
	s.endWrote = s.endSeq
	return s
}

func (s *keyStore) endedPath() string { return filepath.Join(filepath.Dir(s.path), "keys-ended.json") }

// noteEndedMu remembers e, forgetting the oldest past endedKeyMax. Caller holds mu (or owns s).
func (s *keyStore) noteEndedMu(e endedKey) {
	if s.ended == nil {
		s.ended = map[string]bool{}
	}
	if _, ok := s.ended[e.Hash]; ok || e.Hash == "" {
		return
	}
	s.ended[e.Hash] = e.Expired
	s.endedAt = append(s.endedAt, e)
	if n := len(s.endedAt) - endedKeyMax; n > 0 {
		for _, old := range s.endedAt[:n] {
			delete(s.ended, old.Hash)
		}
		s.endedAt = slices.Delete(s.endedAt, 0, n)
	}
	s.endSeq++
}

// endRecMu remembers r as ended when it was a key, not a setup link or a session key. Caller holds mu.
func (s *keyStore) endRecMu(r keyRec, now time.Time) {
	if r.Setup || r.Sess != "" {
		return
	}
	s.noteEndedMu(endedKey{Hash: r.Hash, At: now.UTC(), Expired: !r.Expires.IsZero() && !now.Before(r.Expires)})
}

// endedAs says whether secret is a key that ended, and if so whether by its expiry.
func (s *keyStore) endedAs(secret string) (expired, ended bool) {
	if !strings.HasPrefix(secret, keyPrefix) {
		return false, false
	}
	h := keyHash(secret)
	s.mu.Lock()
	expired, ended = s.ended[h]
	s.mu.Unlock()
	if !ended && isSessionShape(secret) {
		return false, true
	}
	return expired, ended
}

// save writes the file after taking a snapshot under mu, so a slow disk blocks no lookup.
func (s *keyStore) save() {
	if s.path == "" {
		return
	}
	s.mu.Lock()
	b, _ := json.Marshal(s.recs)
	var eb []byte
	es := s.endSeq
	if es != s.endWrote {
		eb, _ = json.Marshal(s.endedAt)
	}
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
	if eb != nil && es > s.endWrote {
		s.endWrote = es
		if err := config.AtomicWrite(s.endedPath(), eb); err != nil {
			log.Printf("api: ended keys not saved to %s: %v", s.endedPath(), err)
		}
	}
}

// createCapped returns the secret, shown once and kept nowhere. It refuses once owner holds
// max keys (0 = no cap). fp is pinFP of a user owner's PIN hash ("" for a client).
func (s *keyStore) createCapped(o keyOwner, name string, expires time.Time, max int, fp string) (string, apiKey, error) {
	return s.create(o, name, expires, max, fp, "")
}

// sessionKeyMax bounds a session's keys. Tabs that share a session each hold one, so asking
// again must not end another tab's key; past the bound the oldest ends.
const sessionKeyMax = 16

// create is createCapped for a key that may ride a session (sess set), which gets the session shape.
func (s *keyStore) create(o keyOwner, name string, expires time.Time, max int, fp, sess string) (string, apiKey, error) {
	secret := newKeySecret()
	if sess != "" {
		secret = newSessionSecret()
	}
	return s.createWith(secret, o, name, expires, max, fp, sess)
}

func (s *keyStore) createWith(secret string, o keyOwner, name string, expires time.Time, max int, fp, sess string) (string, apiKey, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", apiKey{}, errors.New("a key needs a key name")
	}
	id := make([]byte, 6)
	rand.Read(id)
	s.mu.Lock()
	if sess != "" {
		n := 0
		for _, r := range s.recs {
			if r.Sess == sess {
				n++
			}
		}
		// recs is in creation order, so the session's first key is its oldest.
		s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
			if r.Sess != sess || n < sessionKeyMax {
				return false
			}
			n--
			return true
		})
	}
	if max > 0 && s.countMu(o) >= max {
		s.mu.Unlock()
		return "", apiKey{}, fmt.Errorf("%s already has %d keys", o.Name, max)
	}
	k := apiKey{ID: hex.EncodeToString(id), Owner: o, Name: name, Created: time.Now().UTC(), Expires: expires}
	s.recs = append(s.recs, keyRec{apiKey: k, Hash: keyHash(secret), PinFP: fp, Sess: sess})
	s.mu.Unlock()
	s.save()
	return secret, k, nil
}

func (s *keyStore) countMu(o keyOwner) int {
	n := 0
	for _, r := range s.recs {
		if r.Owner.is(o) && !r.Setup {
			n++
		}
	}
	return n
}

// find returns the live key for a presented secret. expired says the secret matched a key
// past its expiry, so the API can name that reason; such a key is never returned as live.
func (s *keyStore) find(secret string, now time.Time) (k apiKey, expired, ok bool) {
	if !strings.HasPrefix(secret, keyPrefix) {
		return apiKey{}, false, false
	}
	h := keyHash(secret)
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.recs {
		if !r.Setup && subtle.ConstantTimeCompare([]byte(r.Hash), []byte(h)) == 1 {
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

func (s *keyStore) flush() {
	s.mu.Lock()
	d := s.dirty
	s.mu.Unlock()
	if d {
		s.save()
	}
}

func (s *keyStore) alive(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.ContainsFunc(s.recs, func(r keyRec) bool { return r.ID == id })
}

// sessionOf is the session a key rides on, "" for any other key or an unknown id.
func (s *keyStore) sessionOf(id string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.recs {
		if r.ID == id {
			return r.Sess
		}
	}
	return ""
}

func (s *keyStore) list() []apiKey {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]apiKey, 0, len(s.recs))
	for _, r := range s.recs {
		if !r.Setup {
			out = append(out, r.apiKey)
		}
	}
	slices.SortStableFunc(out, func(a, b apiKey) int { return a.Created.Compare(b.Created) })
	return out
}

func (s *keyStore) dropWhere(gone func(keyRec) bool) []apiKey {
	now := time.Now()
	s.mu.Lock()
	var out []apiKey
	s.recs = slices.DeleteFunc(s.recs, func(r keyRec) bool {
		if gone(r) {
			out = append(out, r.apiKey)
			s.endRecMu(r, now)
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

// reconcile runs once at startup against the loaded config and returns the keys it ended:
// owner gone, or a user whose PIN hash no longer matches the stamp. A key with no stamp
// (older keys.json) is kept and stamped now; a PIN edited while down before this version
// ran cannot be told.
func (s *keyStore) reconcile(cfg config.Config) []apiKey {
	changed := false
	var out []apiKey
	s.mu.Lock()
	kept := s.recs[:0]
	for _, r := range s.recs {
		ok := keyOwnerExists(cfg, r.Owner)
		if ok && r.Owner.Kind == actorUser {
			u, _ := cfg.FindUser(r.Owner.Name)
			switch fp := pinFP(u.Pin); {
			case r.PinFP == "":
				r.PinFP, changed = fp, true
			case r.PinFP != fp:
				ok = false
			}
		}
		if ok {
			kept = append(kept, r)
		} else {
			out = append(out, r.apiKey)
			s.endRecMu(r, time.Now())
		}
	}
	s.recs = kept
	s.mu.Unlock()
	if changed || len(out) > 0 {
		s.save()
	}
	return out
}

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
	return s.dropWhere(func(r keyRec) bool { return !r.Setup && !r.Expires.IsZero() && !now.Before(r.Expires) })
}
