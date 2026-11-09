package app

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// tokenStore: persistent users' session tokens, in memory and in a file beside the config.
// A token is bound to user name and PIN hash and lookups re-read the live user, so a PIN
// change, removal or unticking Persistent kills it whatever the file holds. Only sha256 of
// a token is stored.
type tokenStore struct {
	mu     sync.Mutex
	toks   map[string]tokenRec // sha256 of the token, hex
	path   string              // "" = memory only
	fileMu sync.Mutex          // one writer of the file at a time
	seq    uint64              // bumped under mu per save; fileMu holders skip stale ones
	wrote  uint64              // last seq written (under fileMu)
}

type tokenRec struct {
	User string    `json:"user"` // the user it was issued to
	Pin  string    `json:"pin"`  // that user's PIN hash then
	At   time.Time `json:"at"`
	Tab  bool      `json:"tab,omitempty"` // a sign-in with no page lock: for a user who is not persistent, kept by one browser tab
	// Locked: the session's Lock (ADR 0001); a reload with this token comes back locked.
	Locked bool `json:"locked,omitempty"`
}

// perUserTokens: one per browser; the oldest goes.
const perUserTokens = 8

// tokenMaxAge: unused tokens die so a forgotten browser isn't signed in forever. Use is a
// hi or an open connection (touch). Rewritten only past tokenTouchAfter: about one write a
// day.
const (
	tokenMaxAge     = 90 * 24 * time.Hour
	tokenTouchAfter = 24 * time.Hour
	// Tab tokens live in sessionStorage, forgotten on close; an open tab renews it.
	tabTokenMaxAge = 48 * time.Hour
)

func tokenKey(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// loadTokens: once, from New (callers hold App.mu: no other file I/O). A damaged file is
// forgotten.
func loadTokens(cfgPath string) *tokenStore {
	s := &tokenStore{toks: map[string]tokenRec{}}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), "tokens.json")
	if b, err := os.ReadFile(s.path); err == nil {
		if err := json.Unmarshal(b, &s.toks); err != nil {
			log.Printf("users: %s is damaged, tokens forgotten: %v", s.path, err)
			s.toks = map[string]tokenRec{}
		}
		// Old admin tokens ({pin, at}, no user) are dropped.
		for k, r := range s.toks {
			if r.User == "" {
				delete(s.toks, k)
			}
		}
	}
	return s
}

// save writes the file. Never with mu held: it takes mu to snapshot and releases it before
// the write, so a slow disk doesn't block lookups.
func (s *tokenStore) save() {
	if s.path == "" {
		return
	}
	s.mu.Lock()
	b, _ := json.Marshal(s.toks)
	s.seq++
	n := s.seq
	s.mu.Unlock()
	s.fileMu.Lock()
	defer s.fileMu.Unlock()
	if n < s.wrote {
		return // a newer state has already been written
	}
	s.wrote = n
	if err := config.AtomicWrite(s.path, b); err != nil {
		log.Printf("users: tokens not saved to %s: %v", s.path, err)
	}
}

// valid: prune deletes invalid tokens so one can't revive when the user is made persistent
// again, renamed back or given the old PIN. A tab token is valid only while its user is
// NOT persistent, and for tabTokenMaxAge, so ticking Never sign out can't turn it into a
// persistent one that skips a lock.
func valid(r tokenRec, cfg config.Config) (config.User, bool) {
	u, ok := cfg.FindUser(r.User)
	if !ok || u.Pin != r.Pin {
		return u, false
	}
	max := tokenMaxAge
	if r.Tab {
		if u.Persistent {
			return u, false
		}
		max = tabTokenMaxAge
	} else if !u.Persistent {
		return u, false
	}
	return u, time.Since(r.At) < max
}

// prune: after every config change, at startup, before issuing. Never with mu held (writes
// the file).
func (s *tokenStore) prune(cfg config.Config) {
	s.dropWhere(func(r tokenRec) bool {
		_, ok := valid(r, cfg)
		return !ok
	})
}

// dropWhere deletes every record gone reports and writes the file only if any went. Never
// with mu held.
func (s *tokenStore) dropWhere(gone func(tokenRec) bool) {
	s.mu.Lock()
	n := len(s.toks)
	for k, r := range s.toks {
		if gone(r) {
			delete(s.toks, k)
		}
	}
	changed := len(s.toks) != n
	s.mu.Unlock()
	if changed {
		s.save()
	}
}

// heldOnly: under a PIN lock a non-persistent user's token only carries a held Lock, never
// a sign-in. Hi refuses it once the Lock has ended, issueToken gives none for a sign-in, a resume drops it.
func heldOnly(u config.User, page string, locks config.Locks) bool {
	return !u.Persistent && pageGates[page].on(locks)
}

func (s *tokenStore) dropUser(name string) {
	s.dropWhere(func(r tokenRec) bool { return strings.EqualFold(r.User, name) })
}

// Never with App.mu held (writes the file).
func (s *tokenStore) touch(cfg config.Config, tokens ...string) {
	s.mu.Lock()
	now, moved := time.Now(), false
	for _, t := range tokens {
		k := tokenKey(t)
		if r, ok := s.toks[k]; ok && time.Since(r.At) > tokenTouchAfter {
			if _, good := valid(r, cfg); good {
				r.At = now
				s.toks[k] = r
				moved = true
			}
		}
	}
	s.mu.Unlock()
	if moved {
		s.save()
	}
}

func (s *tokenStore) user(t string, cfg config.Config) (config.User, bool) {
	if t == "" {
		return config.User{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.toks[tokenKey(t)]
	if !ok {
		return config.User{}, false
	}
	return valid(r, cfg)
}

// live: the token with this sha256 key still exists, is valid and was issued to owner.
func (s *tokenStore) live(hash, owner string, cfg config.Config) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.toks[hash]
	if !ok || !strings.EqualFold(r.User, owner) {
		return false
	}
	_, ok = valid(r, cfg)
	return ok
}

// issue: locked is the session's Lock now, so the one write carries it.
func (s *tokenStore) issue(u config.User, cfg config.Config, tab, locked bool) string {
	b := make([]byte, 32)
	rand.Read(b)
	t := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	for k, r := range s.toks {
		if _, ok := valid(r, cfg); !ok {
			delete(s.toks, k)
		}
	}
	for {
		n, oldest := 0, ""
		for k, r := range s.toks {
			if !strings.EqualFold(r.User, u.Name) || r.Tab != tab {
				continue // the cap is per user and per kind
			}
			n++
			if oldest == "" || r.At.Before(s.toks[oldest].At) {
				oldest = k
			}
		}
		if n < perUserTokens {
			break
		}
		delete(s.toks, oldest)
	}
	s.toks[tokenKey(t)] = tokenRec{User: u.Name, Pin: u.Pin, At: time.Now(), Tab: tab, Locked: locked}
	s.mu.Unlock()
	s.save()
	return t
}

func (s *tokenStore) locked(t string) bool {
	if t == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.toks[tokenKey(t)].Locked
}

// setLocked changes the flag in memory only, cheap enough under App.mu; save writes it. A token the store does not know is ignored.
func (s *tokenStore) setLocked(t string, on bool) {
	if t == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	k := tokenKey(t)
	if r, ok := s.toks[k]; ok {
		r.Locked = on
		s.toks[k] = r
	}
}

func (s *tokenStore) forget(t string) {
	if t == "" {
		return
	}
	s.mu.Lock()
	_, ok := s.toks[tokenKey(t)]
	delete(s.toks, tokenKey(t))
	s.mu.Unlock()
	if ok {
		s.save()
	}
}
