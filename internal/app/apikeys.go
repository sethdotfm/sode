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

// How often use counts reach the disk at most; flush writes the rest.
const keyWriteEvery = time.Minute

func newKeySecret() string { return newSecret(32) }

// 128 bits, for a panel's link key: every character is in the link, and the key is throttled and revocable.
// Its body never starts with the prefix, so the page can tell a link that left the prefix off.
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

func newSessionSecret() string { return newShapedSecret(sessionKeyPrefix) }

func newShapedSecret(prefix string) string {
	b := make([]byte, sessionSecretBytes)
	rand.Read(b)
	return prefix + base64.RawURLEncoding.EncodeToString(b)
}

func hasShape(secret, prefix string) bool {
	body, ok := strings.CutPrefix(secret, prefix)
	if !ok || len(body) != base64.RawURLEncoding.EncodedLen(sessionSecretBytes) {
		return false
	}
	b, err := base64.RawURLEncoding.Strict().DecodeString(body)
	return err == nil && len(b) == sessionSecretBytes
}

// A memory-only Sode forgets its session keys on a restart, and the pages that hold one must hear that it ended.
// The answer is not counted toward the throttle: a 256-bit guess never lands.
func isSessionShape(secret string) bool { return hasShape(secret, sessionKeyPrefix) }

func newSecret(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return keyPrefix + base64.RawURLEncoding.EncodeToString(b)
}

func keyHash(secret string) string {
	h := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(h[:])
}

// A user's Name is for display only: withLiveNames fills it in lists.
type keyOwner struct {
	Kind string `json:"kind"` // actorUser or actorClient
	User string `json:"user,omitempty"`
	Name string `json:"name"`
}

func (o keyOwner) is(p keyOwner) bool {
	if o.Kind == actorUser {
		return p.Kind == actorUser && o.User != "" && o.User == p.User
	}
	return o.Kind == p.Kind && strings.EqualFold(o.Name, p.Name)
}

func userOwner(u config.User) keyOwner { return keyOwner{Kind: actorUser, User: u.ID, Name: u.Name} }

// A user who is gone reads as the name the key was made under.
func (o keyOwner) label(cfg config.Config) string {
	if n := cfg.UserName(o.User); o.Kind == actorUser && n != "" {
		return n
	}
	return o.Name
}

func withLiveNames(cfg config.Config, keys []apiKey) []apiKey {
	for i := range keys {
		keys[i].Owner.Name = keys[i].Owner.label(cfg)
	}
	return keys
}

type apiKey struct {
	ID       string    `json:"id"`
	Owner    keyOwner  `json:"owner"`
	Name     string    `json:"name"`
	Created  time.Time `json:"created"`
	LastUsed time.Time `json:"lastUsed,omitzero"`
	Count    int       `json:"count"`
	Expires  time.Time `json:"expires,omitzero"` // zero = never
	public   bool      // a Public panel's page key (panelpublic.go): its name says nothing about the instance
}

func (k *apiKey) subject() string { return "key:" + k.ID }

type keyRec struct {
	apiKey
	Hash string `json:"hash"`
	// pinFP of the owner's PIN hash when the key was made (user keys only).
	// So a PIN edited in config.yaml while Sode was down still ends the key.
	PinFP string `json:"pinFp,omitempty"`
	// Sess is tokenKey of the session token a browser's key rides on (sessionkey.go). The key is live only while the
	// token is.
	Sess string `json:"sess,omitempty"`
	// Setup marks a one-time setup link (panelsetup.go), not a key: Hash is its token's, and find, list, count and
	// expire skip it.
	Setup bool `json:"setup,omitempty"`
}

// Stored in place of the PIN hash, and one-way.
func pinFP(pinHash string) string {
	h := sha256.Sum256([]byte("sode key owner pin v1\x00" + pinHash))
	return hex.EncodeToString(h[:16])
}

// The oldest ended key is forgotten first.
const endedKeyMax = 1024

// Kept by hash, so a page that still sends the key hears `key_revoked` and is not counted as a guess.
// Session keys are left out: their shape says it.
type endedKey struct {
	Hash    string    `json:"hash"`
	At      time.Time `json:"at"`
	Expired bool      `json:"expired,omitempty"`
}

// keys.json and keys-ended.json beside the config: sha256 only, mode 0600, atomic write as for tokens.json.
// Memory only with no config file. Never call with App.mu held: a change writes the file.
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
	endTaken uint64 // last endSeq marshalled for a write (under mu)
	endWrote uint64 // last endSeq written (under fileMu)
}

// So the next write cannot overwrite the file and an administrator can repair it. The time has no colons:
// some filesystems refuse them.
func setAsideDamaged(path, prefix, what string, perr error) {
	aside := path + ".damaged-" + time.Now().UTC().Format("20060102T150405Z")
	if err := os.Rename(path, aside); err != nil {
		log.Printf("%s: %s is damaged, %s forgotten, and setting it aside failed: %v (%v)", prefix, path, what, err, perr)
		return
	}
	log.Printf("%s: %s is damaged, %s forgotten; the file is kept as %s: %v", prefix, path, what, aside, perr)
}

// A damaged file is set aside and the store starts empty, as for tokens.
func loadKeys(cfgPath string) *keyStore {
	s := &keyStore{}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), "keys.json")
	if b, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(b, &s.recs); err != nil {
			setAsideDamaged(s.path, "api", "keys", err)
			s.recs = nil
		}
	}
	var ended []endedKey
	if b, err := os.ReadFile(s.endedPath()); err == nil {
		if err := json.Unmarshal(b, &ended); err != nil {
			setAsideDamaged(s.endedPath(), "api", "ended keys", err)
			ended = nil
		}
	}
	for _, e := range ended {
		s.noteEndedMu(e)
	}
	s.endTaken, s.endWrote = s.endSeq, s.endSeq
	return s
}

func (s *keyStore) endedPath() string { return filepath.Join(filepath.Dir(s.path), "keys-ended.json") }

// Caller holds mu (or owns s).
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

// Setup links and session keys are not remembered. Caller holds mu.
func (s *keyStore) endRecMu(r keyRec, now time.Time) {
	if r.Setup || r.Sess != "" {
		return
	}
	s.noteEndedMu(endedKey{Hash: r.Hash, At: now.UTC(), Expired: !r.Expires.IsZero() && !now.Before(r.Expires)})
}

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

// The snapshot is made under mu and written outside it, so a slow disk blocks no lookup.
func (s *keyStore) save() {
	if s.path == "" {
		return
	}
	s.mu.Lock()
	b, _ := json.Marshal(s.recs)
	// Only one snapshot carries each change to the ended keys, so its write is never skipped below.
	var eb []byte
	es := s.endSeq
	if es != s.endTaken {
		eb, _ = json.Marshal(s.endedAt)
		s.endTaken = es
	}
	s.seq++
	n := s.seq
	s.dirty, s.lastW = false, time.Now()
	s.mu.Unlock()
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if eb != nil && es > s.endWrote {
		s.endWrote = es
		if err := config.AtomicWrite(s.endedPath(), eb); err != nil {
			log.Printf("api: ended keys not saved to %s: %v", s.endedPath(), err)
		}
	}
	if n < s.wrote {
		return
	}
	s.wrote = n
	if err := config.AtomicWrite(s.path, b); err != nil {
		log.Printf("api: keys not saved to %s: %v", s.path, err)
	}
}

// The secret is shown one time and kept nowhere. max is the owner's cap (0 = none). fp is pinFP of a user owner's PIN
// hash
// ("" for a client).
func (s *keyStore) createCapped(o keyOwner, name string, expires time.Time, max int, fp string) (string, apiKey, error) {
	return s.create(o, name, expires, max, fp, "")
}

// So a client in a loop cannot fill the key file. No setting or preset writes them.
const (
	userKeyMax   = 50
	clientKeyMax = 200
)

// Tabs that share a session each hold one key, so a new ask must not end another tab's key. Past the bound the oldest
// ends.
const sessionKeyMax = 16

// A key with sess set rides that session and gets the session shape.
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
		who := "a user"
		if o.Kind == actorClient {
			who = "an API client"
		}
		return "", apiKey{}, fmt.Errorf("%s already has %d keys, the most %s can have. Revoke a key first", o.Name, max, who)
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
		// Session keys have their own bound (sessionKeyMax) and end with the session, so they do not count toward the
		// owner's cap.
		if r.Owner.is(o) && !r.Setup && r.Sess == "" {
			n++
		}
	}
	return n
}

// expired: the secret matched a key past its expiry, so the API can name that reason. Such a key is never returned as
// live.
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

// The file follows within keyWriteEvery or at flush.
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

// "" for a key that rides no session or an unknown id.
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

// Runs one time at startup. It returns the keys it ended: owner gone, or a user key whose
// stamp does not match the PIN hash (an unstamped one included).
func (s *keyStore) reconcile(cfg config.Config) []apiKey {
	var out []apiKey
	s.mu.Lock()
	kept := s.recs[:0]
	for _, r := range s.recs {
		ok := keyOwnerExists(cfg, r.Owner)
		if ok && r.Owner.Kind == actorUser {
			u, _ := cfg.FindUserByID(r.Owner.User)
			ok = r.PinFP == pinFP(u.Pin)
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
	if len(out) > 0 {
		s.save()
	}
	return out
}

// Returns the removed keys, so the caller can log each.
func (s *keyStore) expire(now time.Time) []apiKey {
	return s.dropWhere(func(r keyRec) bool { return !r.Setup && !r.Expires.IsZero() && !now.Before(r.Expires) })
}
