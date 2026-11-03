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
)

// tokenStore is the admin tokens (#151): in memory, and in a file next to the
// config when there is one. A token is only good against the settings PIN it
// was issued under, so changing or removing the PIN makes every old one fail
// in ok, whatever the file still holds.
type tokenStore struct {
	mu     sync.Mutex
	toks   map[string]tokenRec // sha256 of the token, hex
	path   string              // "" = memory only
	fileMu sync.Mutex          // one writer of the file at a time, in order
}

type tokenRec struct {
	Pin string    `json:"pin"` // the settings PIN hash it was issued against
	At  time.Time `json:"at"`
}

const maxTokens = 100

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
	}
	return s
}

// save writes the file from the state as it is now. Caller holds mu, which
// save releases before the write, so a slow disk never holds up tokenOK.
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

// ok reports whether t was issued against the settings PIN in force.
func (s *tokenStore) ok(t, settingsPin string) bool {
	if t == "" || settingsPin == "" {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	r, ok := s.toks[tokenKey(t)]
	return ok && r.Pin == settingsPin
}

// issue makes a token for the settings PIN pin. Tokens of any other PIN are
// dead already and dropped here; if the store is still full the oldest goes.
func (s *tokenStore) issue(pin string) string {
	b := make([]byte, 32)
	rand.Read(b)
	t := base64.RawURLEncoding.EncodeToString(b)
	s.mu.Lock()
	for k, r := range s.toks {
		if r.Pin != pin {
			delete(s.toks, k)
		}
	}
	if len(s.toks) >= maxTokens {
		var oldest string
		for k, r := range s.toks {
			if oldest == "" || r.At.Before(s.toks[oldest].At) {
				oldest = k
			}
		}
		delete(s.toks, oldest)
	}
	s.toks[tokenKey(t)] = tokenRec{Pin: pin, At: time.Now()}
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
