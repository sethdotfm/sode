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
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// tokenStore is the session tokens of persistent users: in memory, and in a
// file next to the config when there is one. A token is bound to a user's name
// and PIN hash, and lookups re-read the live user, so changing the PIN,
// removing the user or unticking Persistent kills every token of theirs at
// once, whatever the file still holds. Only the sha256 of a token is stored.
type tokenStore struct {
	mu     sync.Mutex
	toks   map[string]tokenRec // sha256 of the token, hex
	path   string              // "" = memory only
	fileMu sync.Mutex          // one writer of the file at a time, in order
}

type tokenRec struct {
	User string    `json:"user"` // the user it was issued to
	Pin  string    `json:"pin"`  // that user's PIN hash then
	At   time.Time `json:"at"`
}

// perUserTokens caps tokens per user (one per browser), so the file can't
// grow without bound; the oldest goes.
const perUserTokens = 8

func tokenKey(t string) string {
	h := sha256.Sum256([]byte(t))
	return hex.EncodeToString(h[:])
}

// loadTokens reads the token file (once, from New: callers hold App.mu, so
// nothing else may do file I/O here). A damaged file is forgotten.
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
		// The first version held admin tokens ({pin, at}, no user): they
		// are no longer a thing, so drop them.
		for k, r := range s.toks {
			if r.User == "" {
				delete(s.toks, k)
			}
		}
	}
	return s
}

// save writes the file from the state as it is now. Caller holds mu, which
// save releases before the write, so a slow disk never holds up a lookup.
func (s *tokenStore) save() {
	if s.path == "" {
		s.mu.Unlock()
		return
	}
	b, _ := json.Marshal(s.toks)
	s.fileMu.Lock() // taken before mu goes, so writes land in order
	s.mu.Unlock()
	defer s.fileMu.Unlock()
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err == nil {
		if err = os.Rename(tmp, s.path); err == nil {
			return
		}
	}
	log.Printf("users: tokens not saved to %s", s.path)
}

// valid: r still names a persistent user with the PIN it was issued under.
func valid(r tokenRec, cfg config.Config) (config.User, bool) {
	u, ok := cfg.FindUser(r.User)
	return u, ok && u.Persistent && u.Pin == r.Pin
}

// user returns the user t was issued to, if it is still good against cfg.
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

// issue makes a token for u. Tokens that are dead against cfg are dropped
// here, and u's oldest if it would have more than perUserTokens.
func (s *tokenStore) issue(u config.User, cfg config.Config) string {
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
			if r.User != u.Name {
				continue
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
	s.toks[tokenKey(t)] = tokenRec{User: u.Name, Pin: u.Pin, At: time.Now()}
	s.save()
	return t
}

func (s *tokenStore) forget(t string) {
	if t == "" {
		return
	}
	s.mu.Lock()
	if _, ok := s.toks[tokenKey(t)]; !ok {
		s.mu.Unlock()
		return
	}
	delete(s.toks, tokenKey(t))
	s.save()
}
