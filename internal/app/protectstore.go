package app

import (
	"encoding/json"
	"errors"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Sode's record of the Protects it set (protects.json beside the config); protect.go uses it.

// protectRec is Sode's record of one Protect it set. Levels are the UI levels Sode set and has not freed.
type protectRec struct {
	Router  string    `json:"router"`
	Dst     int       `json:"dst"`
	DstName string    `json:"dstName,omitempty"`
	Device  int       `json:"device"`
	Levels  []int     `json:"levels"`
	By      string    `json:"by"`               // the name when set, for a user that is gone
	User    string    `json:"user,omitempty"`   // the user's id, also for that user's key or own API client
	Client  string    `json:"client,omitempty"` // an API client's name
	At      time.Time `json:"at"`
}

// ownedBy: the caller (as recOwner gives it) set this Protect. A guest owns only a record with no user and no
// client. A nil record has no owner.
func (rec *protectRec) ownedBy(user, client string, act Actor) bool {
	if rec == nil {
		return false
	}
	return (user != "" && rec.User == user) || (client != "" && rec.Client == client) ||
		(user == "" && client == "" && rec.User == "" && rec.Client == "" && act.Kind == actorGuest)
}

type protectFile struct {
	Protects []protectRec `json:"protects"`
}

// protectStore keeps every router's records, so a router change does not drop them. Its own lock; never take mu
// while holding it.
type protectStore struct {
	path string
	mu   sync.Mutex
	recs []protectRec
}

func loadProtects(cfgPath string) *protectStore {
	s := &protectStore{}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), "protects.json")
	b, err := os.ReadFile(s.path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("protects: %v", err)
		}
		return s
	}
	var f protectFile
	if err := json.Unmarshal(b, &f); err != nil {
		log.Printf("protects: %s: %v", s.path, err)
		return s
	}
	for _, r := range f.Protects {
		if r.Router != "" && len(r.Levels) > 0 {
			s.recs = append(s.recs, r)
		}
	}
	return s
}

func (s *protectStore) get(router string, dst int) (protectRec, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, r := range s.recs {
		if r.Router == router && r.Dst == dst {
			r.Levels = slices.Clone(r.Levels)
			return r, true
		}
	}
	return protectRec{}, false
}

func (s *protectStore) list(router string) []protectRec {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []protectRec
	for _, r := range s.recs {
		if r.Router == router {
			r.Levels = slices.Clone(r.Levels)
			out = append(out, r)
		}
	}
	return out
}

// put replaces the record of rec's router and destination; with no levels it removes it. It writes the file before
// it returns.
func (s *protectStore) put(rec protectRec) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.recs = slices.DeleteFunc(s.recs, func(r protectRec) bool { return r.Router == rec.Router && r.Dst == rec.Dst })
	if len(rec.Levels) > 0 {
		sort.Ints(rec.Levels)
		s.recs = append(s.recs, rec)
	}
	if s.path == "" {
		return nil
	}
	b, err := json.MarshalIndent(protectFile{Protects: s.recs}, "", "  ")
	if err != nil {
		return err
	}
	return config.AtomicWrite(s.path, b)
}

// recOwner: what Sode stores as who set a Protect. A user's own API client counts as that user.
func recOwner(cfg config.Config, act Actor) (user, client string) {
	switch act.Kind {
	case actorUser:
		return act.User, ""
	case actorClient:
		if cl, ok := cfg.FindAPIClient(act.Name); ok && cl.Owner != "" {
			return cl.Owner, ""
		}
		return "", act.Name
	}
	return "", ""
}
