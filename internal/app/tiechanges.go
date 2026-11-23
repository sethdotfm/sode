package app

import (
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// The last change of each tie line (docs/tielines-design.md, "Last change of a tie line"). Any crosspoint change on
// the upstream destination of a tie line, on any level, by Sode or by an External, stamps it. The stamps are in
// names/tielines.changes.yaml beside the names tables, so they survive a restart. The file name has a dot, which no
// router UID has (routerUIDRe), so it never clashes with a names table.
const tieChangesFile = "tielines.changes.yaml"

// tieStamp: At is when Sode saw the change. Found: Sode learned of it at a re-read, so At is when Sode found it and
// the real change was earlier.
type tieStamp struct {
	At    time.Time
	Found bool
}

type tieChangeRow struct {
	ID    string    `yaml:"id"`
	At    time.Time `yaml:"at"`
	Found bool      `yaml:"found,omitempty"`
}

type tieChangeDoc struct {
	TieLines []tieChangeRow `yaml:"tieLines"`
}

type tieChanges struct {
	mu    sync.Mutex
	path  string // "" is memory only
	stamp map[string]tieStamp
	dirty bool
}

func loadTieChanges(cfgPath string) *tieChanges {
	s := &tieChanges{stamp: map[string]tieStamp{}}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), namesDir, tieChangesFile)
	b, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("tie line changes: %v", err)
		}
		return s
	}
	var d tieChangeDoc
	if err := yaml.Unmarshal(b, &d); err != nil {
		log.Printf("tie line changes: %s is not valid YAML, so it was not read: %v", s.path, err)
		return s
	}
	for _, r := range d.TieLines {
		if r.ID != "" && !r.At.IsZero() {
			s.stamp[r.ID] = tieStamp{r.At, r.Found}
		}
	}
	return s
}

func (s *tieChanges) set(id string, at time.Time, found bool) {
	s.mu.Lock()
	s.stamp[id] = tieStamp{at, found}
	s.dirty = true
	s.mu.Unlock()
}

// forget drops the stamps of ids. The next write takes them out of the file.
func (s *tieChanges) forget(ids []string) {
	if len(ids) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, id := range ids {
		if _, ok := s.stamp[id]; ok {
			delete(s.stamp, id)
			s.dirty = true
		}
	}
}

func (s *tieChanges) dirtyNow() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dirty
}

// snapshot gives the stamps of the tie lines that still exist.
func (s *tieChanges) snapshot(ties []config.TieLine) map[string]tieStamp {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]tieStamp, len(s.stamp))
	for _, t := range ties {
		if st, ok := s.stamp[t.ID]; ok {
			out[t.ID] = st
		}
	}
	return out
}

// write saves the stamps when one changed since the last write. A stamp of a deleted tie line goes out here.
func (s *tieChanges) write(ties []config.TieLine) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.path == "" || !s.dirty {
		return
	}
	var d tieChangeDoc
	for _, t := range ties {
		if st, ok := s.stamp[t.ID]; ok {
			d.TieLines = append(d.TieLines, tieChangeRow{t.ID, st.At.UTC(), st.Found})
		}
	}
	for id := range s.stamp {
		if !slices.ContainsFunc(ties, func(t config.TieLine) bool { return t.ID == id }) {
			delete(s.stamp, id)
		}
	}
	b, err := yaml.Marshal(d)
	if err == nil {
		if err = os.MkdirAll(filepath.Dir(s.path), 0o700); err == nil {
			err = config.AtomicWrite(s.path, b)
		}
	}
	if err != nil {
		log.Printf("tie line changes: %v", err)
		return
	}
	s.dirty = false
}

// stampTieLines marks each tie line that starts at destination dst of unit u as changed now. found is true for a
// change that came as a tally, not a Connected. Caller holds mu.
func (a *App) stampTieLines(u *routerUnit, r *runtime, dst int, found bool) {
	for _, t := range r.cfg.TieLines {
		if t.From.N != dst {
			continue
		}
		if uid, ok := a.unitUID(r, t.From.Router); ok && uid == u.uid {
			a.tieChanges.set(t.ID, a.now(), found)
		}
	}
}

// forgetMovedTieLines drops the stamp of each tie line whose ends a config change moved: Settings, an import or any
// other commit. The stamp is the last change of the old upstream destination, and Sode never claims more than it
// knows. A rename keeps it. Caller holds mu.
func (a *App) forgetMovedTieLines(old, nr *runtime) {
	type at struct {
		router string
		n      int
	}
	end := func(rt *runtime, r config.Ref) at {
		if uid, ok := a.unitUID(rt, r.Router); ok {
			return at{uid, r.N}
		}
		return at{r.Router, r.N}
	}
	was := make(map[string]config.TieLine, len(old.cfg.TieLines))
	for _, t := range old.cfg.TieLines {
		was[t.ID] = t
	}
	var moved []string
	for _, t := range nr.cfg.TieLines {
		w, ok := was[t.ID]
		if ok && (end(old, w.From) != end(nr, t.From) || end(old, w.To) != end(nr, t.To)) {
			moved = append(moved, t.ID)
		}
	}
	a.tieChanges.forget(moved)
}
