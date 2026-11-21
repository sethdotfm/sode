package app

import (
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Push and checkpoints (docs/panels-design.md). A layout is what a Push copies: the grid with every
// Page and its id, its settings and fields this version does not know. Identity (id, name, owner, sharing), the API
// client, acts and artwork stay with the target.

const panelCheckpointMax = 10 // per panel, oldest dropped first

var (
	errNoPanel   = &panelClientFail{http.StatusNotFound, "not_found", "no such panel"}
	errBadLayout = &panelClientFail{http.StatusBadRequest, "bad_request", "the layout does not fit the panel"}
	// errLayoutType guards every way in, so a layout of the other type never blanks a panel's cells or lists.
	errLayoutType = &panelClientFail{http.StatusConflict, "conflict", "the layout is of the other panel type"}
)

type panelLayout struct {
	// Type is empty in a checkpoint or set entry saved before Grid panels; that is a Buttons panel.
	Type  string      `yaml:"type,omitempty" json:"type,omitempty"`
	Rows  int         `yaml:"rows" json:"rows"`
	Cols  int         `yaml:"cols" json:"cols"`
	Pages []PanelPage `yaml:"pages,omitempty" json:"pages,omitempty"`
	// LegacyCells is a checkpoint or set entry saved before Pages; withLayout hands it to normalize.
	LegacyCells  []map[string]any `yaml:"cells,omitempty" json:"cells,omitempty"`
	Settings     map[string]any   `yaml:"settings" json:"settings"`
	Extra        map[string]any   `yaml:"extra,omitempty" json:"extra,omitempty"`
	Sources      *GridEdge        `yaml:"sources,omitempty" json:"sources,omitempty"`
	Destinations *GridEdge        `yaml:"destinations,omitempty" json:"destinations,omitempty"`
}

func layoutOf(p Panel) panelLayout {
	return panelLayout{Type: p.Type, Rows: p.Rows, Cols: p.Cols, Pages: p.Pages, Settings: p.Settings, Extra: p.Extra,
		Sources: p.Sources, Destinations: p.Destinations}
}

func layoutType(t string) string {
	if t == "" {
		return panelTypeButtons
	}
	return t
}

// withLayout is p with the layout swapped in. Stored values are never mutated, so sharing
// the layout's slices and maps between panels is safe.
func (p Panel) withLayout(l panelLayout) Panel {
	p.Rows, p.Cols, p.Pages, p.LegacyCells, p.Settings, p.Extra = l.Rows, l.Cols, l.Pages, l.LegacyCells, l.Settings, l.Extra
	p.Sources, p.Destinations = l.Sources, l.Destinations
	return p
}

// panelCheckpoint is the target's layout as it was before a Push or a restore.
type panelCheckpoint struct {
	ID     string      `yaml:"id" json:"id"`
	At     time.Time   `yaml:"at" json:"at"`
	By     string      `yaml:"by" json:"by"`
	Why    string      `yaml:"why" json:"why"`
	Layout panelLayout `yaml:"layout" json:"-"`
}

func newShortID(prefix string) string {
	return prefix + config.RandomHexID(4)
}

// Checkpoints sit in panels/<id>.checkpoints, a name loadPanels does not read as a panel.
func (s *panelStore) cpPath(id string) string { return filepath.Join(s.dir, id+".checkpoints") }

// cpsLocked reads a panel's checkpoints from the file one time, then serves them from memory.
func (s *panelStore) cpsLocked(id string) []panelCheckpoint {
	if c, ok := s.cps[id]; ok {
		return c
	}
	var c []panelCheckpoint
	if s.dir != "" {
		if b, err := os.ReadFile(s.cpPath(id)); err == nil {
			if yaml.Unmarshal(b, &c) != nil {
				c = nil
			}
			c = slices.DeleteFunc(c, func(cp panelCheckpoint) bool {
				err := cp.Layout.noRouterProblem()
				if err != nil {
					log.Printf("panels: skipped checkpoint %s of %s: %v", cp.ID, id, err)
				}
				return err != nil
			})
		}
	}
	if s.cps == nil {
		s.cps = map[string][]panelCheckpoint{}
	}
	s.cps[id] = c
	return c
}

// Oldest first. The bool is false for an unknown panel.
func (s *panelStore) checkpoints(id string) ([]panelCheckpoint, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.by[id]; !ok {
		return nil, false
	}
	return slices.Clone(s.cpsLocked(id)), true
}

// applyLocked stores lay over panel id after keeping a checkpoint of what is there. A checkpoint
// that cannot be written stops the change, so a Push is never unrecoverable. refer, when given, makes the
// checkpoint's cells router references with the names the router has now.
func (s *panelStore) applyLocked(id string, lay panelLayout, by, why string, refer layoutRefer) (Panel, panelCheckpoint, error) {
	p, ok := s.by[id]
	if !ok {
		return Panel{}, panelCheckpoint{}, errNoPanel
	}
	if layoutType(lay.Type) != layoutType(p.Type) {
		return Panel{}, panelCheckpoint{}, errLayoutType
	}
	q := p.withLayout(lay)
	q.normalize()
	if err := validatePanel(&q); err != nil {
		return Panel{}, panelCheckpoint{}, fmt.Errorf("%w: %v", errBadLayout, err)
	}
	before := layoutOf(p)
	if refer != nil {
		before = refer(before, p.Owner)
	}
	cp := panelCheckpoint{ID: newShortID("c-"), At: time.Now().UTC().Truncate(time.Second), By: by, Why: why, Layout: before}
	cps := append(slices.Clone(s.cpsLocked(id)), cp)
	if len(cps) > panelCheckpointMax {
		cps = cps[len(cps)-panelCheckpointMax:]
	}
	if s.dir != "" {
		b, err := yaml.Marshal(cps)
		if err != nil {
			return Panel{}, panelCheckpoint{}, err
		}
		if err := os.MkdirAll(s.dir, 0o700); err != nil {
			return Panel{}, panelCheckpoint{}, err
		}
		if err := config.AtomicWrite(s.cpPath(id), b); err != nil {
			return Panel{}, panelCheckpoint{}, err
		}
	}
	if err := s.writeLocked(&q); err != nil {
		return Panel{}, panelCheckpoint{}, err
	}
	s.cps[id] = cps
	s.by[id] = q
	return q, cp, nil
}

// The target keeps its own fixed destinations.
func (s *panelStore) pushLayout(id string, lay panelLayout, by, why string, refer layoutRefer) (Panel, panelCheckpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyLocked(id, keepFixedDests(lay, s.by[id]), by, why, refer)
}

func (s *panelStore) applyLayout(id string, lay panelLayout, by, why string, refer layoutRefer) (Panel, panelCheckpoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.applyLocked(id, lay, by, why, refer)
}

// ok is false for an unknown checkpoint. The layout it replaces is kept as a checkpoint.
func (s *panelStore) restoreCheckpoint(id, cp, by string, refer layoutRefer) (p Panel, ok bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, found := s.by[id]; !found {
		return Panel{}, false, nil
	}
	i := slices.IndexFunc(s.cpsLocked(id), func(c panelCheckpoint) bool { return c.ID == cp })
	if i < 0 {
		return Panel{}, false, nil
	}
	p, _, err = s.applyLocked(id, s.cps[id][i].Layout, by, "before restoring "+cp, refer)
	return p, true, err
}

func (s *panelStore) dropCheckpoints(id string) {
	delete(s.cps, id)
	if s.dir != "" && panelIDRe.MatchString(id) { // the store's ids already match; CodeQL needs the check at the path
		os.Remove(s.cpPath(id))
	}
}

// Checkpoint sets: one saved layout per panel in a group. Files sit in panelsets/ beside the
// config, one for Everyone and one per user for Me, never inside a panel file.

const (
	panelSetsDir = "panelsets"
	panelSetMax  = 200
)

var errSetCap = &panelClientFail{http.StatusConflict, "conflict", "too many checkpoint sets"}

type PanelSetEntry struct {
	Panel       string `yaml:"panel" json:"panel"`
	Name        string `yaml:"name" json:"name"` // the panel's name when saved, for display
	panelLayout `yaml:",inline"`
}

// PanelSet: Scope is panelShareMe (Owner only) or panelShareAll (every user can push it). Owner is
// a user id; as sent, owner is the live name (setOut).
type PanelSet struct {
	ID      string          `yaml:"id" json:"id"`
	Name    string          `yaml:"name" json:"name"`
	Scope   string          `yaml:"scope" json:"scope"`
	Owner   string          `yaml:"owner" json:"owner"`
	Entries []PanelSetEntry `yaml:"entries" json:"entries"`
}

type panelSetFile struct {
	Sets []PanelSet `yaml:"sets"`
}

// Sets keep several records to a file, so only the shared plumbing of yamlDir applies.
type panelSetStore struct {
	yamlDir[PanelSet]
}

func loadPanelSets(cfgPath string) *panelSetStore {
	s := &panelSetStore{}
	s.by = map[string]PanelSet{}
	s.prefix = "s-"
	s.idOf = func(p *PanelSet) *string { return &p.ID }
	s.nameOf = func(p PanelSet) string { return p.Name }
	if cfgPath == "" {
		return s
	}
	s.dir = filepath.Join(filepath.Dir(cfgPath), panelSetsDir)
	ents, _ := os.ReadDir(s.dir)
	for _, e := range ents {
		if !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var f panelSetFile
		if yaml.Unmarshal(b, &f) != nil {
			continue
		}
	sets:
		for _, set := range f.Sets {
			for _, e := range set.Entries {
				if err := e.panelLayout.noRouterProblem(); err != nil {
					log.Printf("panels: skipped Checkpoint set %s: %v", set.ID, err)
					continue sets
				}
			}
			if set.ID != "" {
				s.by[set.ID] = set
			}
		}
	}
	return s
}

func setFileName(set PanelSet) string {
	if set.Scope == panelShareAll {
		return "everyone.yaml"
	}
	return "me-" + hex.EncodeToString([]byte(set.Owner)) + ".yaml"
}

func (s *panelSetStore) persistLocked(set PanelSet) error {
	if s.dir == "" {
		return nil
	}
	name := setFileName(set)
	var f panelSetFile
	for _, o := range s.by {
		if setFileName(o) == name {
			f.Sets = append(f.Sets, o)
		}
	}
	path := filepath.Join(s.dir, name)
	if len(f.Sets) == 0 {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return err
		}
		return nil
	}
	slices.SortFunc(f.Sets, func(a, b PanelSet) int { return strings.Compare(a.ID, b.ID) })
	b, err := yaml.Marshal(f)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	return config.AtomicWrite(path, b)
}

// A failed write leaves memory as the file is.
func (s *panelSetStore) putLocked(set PanelSet) error {
	old, had := s.by[set.ID]
	s.by[set.ID] = set
	if err := s.persistLocked(set); err != nil {
		if had {
			s.by[set.ID] = old
		} else {
			delete(s.by, set.ID)
		}
		return err
	}
	return nil
}

func (s *panelSetStore) create(set PanelSet) (PanelSet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.by) >= panelSetMax {
		return PanelSet{}, errSetCap
	}
	set.ID = s.newIDLocked()
	return set, s.putLocked(set)
}

func (s *panelSetStore) replace(set PanelSet) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.by[set.ID]; !ok {
		return errors.New("no such panel") // not errNoPanel: a set lost to a race answers 500
	}
	return s.putLocked(set)
}

// put stores an imported set: over the one of its id, else as a new one that keeps a free id.
func (s *panelSetStore) put(set PanelSet) (PanelSet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old, had := s.by[set.ID]
	if !had {
		if len(s.by) >= panelSetMax {
			return set, errSetCap
		}
		if set.ID == "" {
			set.ID = s.newIDLocked()
		}
	}
	if err := s.putLocked(set); err != nil {
		return set, err
	}
	// A set that changed scope or owner leaves the file it was in.
	if had && setFileName(old) != setFileName(set) {
		return set, s.persistLocked(old)
	}
	return set, nil
}

// putCheckpoints adds imported checkpoints to panel id's, by id, keeping the newest panelCheckpointMax.
func (s *panelStore) putCheckpoints(id string, add []panelCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.by[id]; !ok {
		return errNoPanel
	}
	cps := slices.Clone(s.cpsLocked(id))
	for _, cp := range add {
		if !slices.ContainsFunc(cps, func(x panelCheckpoint) bool { return x.ID == cp.ID }) {
			cps = append(cps, cp)
		}
	}
	slices.SortStableFunc(cps, func(a, b panelCheckpoint) int { return a.At.Compare(b.At) })
	if len(cps) > panelCheckpointMax {
		cps = cps[len(cps)-panelCheckpointMax:]
	}
	if s.dir != "" {
		b, err := yaml.Marshal(cps)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(s.dir, 0o700); err != nil {
			return err
		}
		if err := config.AtomicWrite(s.cpPath(id), b); err != nil {
			return err
		}
	}
	s.cps[id] = cps
	return nil
}

func (s *panelSetStore) delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	set, ok := s.by[id]
	if !ok {
		return nil
	}
	delete(s.by, id)
	if err := s.persistLocked(set); err != nil {
		s.by[id] = set
		return err
	}
	return nil
}

// visible is every Everyone set and user's (an id) own Me sets, by name.
func (s *panelSetStore) visible(user string) []PanelSet {
	return s.sorted(func(set PanelSet) bool { return set.Scope == panelShareAll || user != "" && set.Owner == user })
}
