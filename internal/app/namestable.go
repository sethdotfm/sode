package app

import (
	"log"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// namesDir holds names/<router uid>.yaml beside the config. The export writes the same form, so
// keep namesTable in step with it.
const namesDir = "names"

// namesTable is one router's names table. N is Sode's number (0-based), as in config.Ref.
// Lists are sorted by N so the bytes, and a hash of them, depend only on the names.
type namesTable struct {
	Router       string      `yaml:"router"`
	At           time.Time   `yaml:"at"` // when Sode last saw these names on the router
	Sources      []namedItem `yaml:"sources"`
	Destinations []namedItem `yaml:"destinations"`
}

type namedItem struct {
	N    int    `yaml:"n" json:"n"`
	Name string `yaml:"name" json:"name"`
}

func tableList(m map[int]string) []namedItem {
	out := make([]namedItem, 0, len(m))
	for _, n := range slices.Sorted(maps.Keys(m)) {
		out = append(out, namedItem{n, m[n]})
	}
	return out
}

func tableMap(l []namedItem) map[int]string {
	m := make(map[int]string, len(l))
	for _, x := range l {
		if x.N >= 0 {
			m[x.N] = x.Name
		}
	}
	return m
}

// A hand-edited config can hold any UID; only these go into a file name.
var routerUIDRe = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

// namesStore writes each router's names table, one file for each router UID, when its names change. dir "" means
// memory only.
type namesStore struct {
	mu   sync.Mutex
	dir  string
	held map[string]namePoll // by router UID: what the file holds; a read that matches it writes nothing
}

func newNamesStore(cfgPath string) *namesStore {
	s := &namesStore{held: map[string]namePoll{}}
	if cfgPath != "" {
		s.dir = filepath.Join(filepath.Dir(cfgPath), namesDir)
	}
	return s
}

func (s *namesStore) path(uid string) string {
	if s.dir == "" || !routerUIDRe.MatchString(uid) {
		return ""
	}
	return filepath.Join(s.dir, uid+".yaml")
}

func (s *namesStore) load(uid string) (namesTable, bool) {
	path := s.path(uid)
	if path == "" {
		return namesTable{}, false
	}
	b, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("names table: %v", err)
		}
		return namesTable{}, false
	}
	var t namesTable
	if err := yaml.Unmarshal(b, &t); err != nil {
		log.Printf("names table: %s is not valid YAML, so it was not read: %v", path, err)
		return namesTable{}, false
	}
	if t.Router != uid {
		log.Printf("names table: %s names router %q, not this router, so it was not read", path, t.Router)
		return namesTable{}, false
	}
	s.mu.Lock()
	s.held[uid] = namePoll{tableMap(t.Sources), tableMap(t.Destinations)}
	s.mu.Unlock()
	return t, true
}

func (s *namesStore) save(uid string, at time.Time, src, dst map[int]string) {
	path := s.path(uid)
	if path == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if h, ok := s.held[uid]; ok && maps.Equal(h.src, src) && maps.Equal(h.dst, dst) {
		return
	}
	b, err := yaml.Marshal(namesTable{Router: uid, At: at.UTC(), Sources: tableList(src), Destinations: tableList(dst)})
	if err == nil {
		if err = os.MkdirAll(s.dir, 0o700); err == nil {
			err = config.AtomicWrite(path, b)
		}
	}
	if err != nil {
		log.Printf("names table: %v", err)
		return
	}
	s.held[uid] = namePoll{src, dst}
}
