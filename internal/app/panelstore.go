package app

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Panels: one YAML file per panel in panels/ beside the config (memory only with no config
// path, like the key store). Artwork sits in panels/<id>.art/. Docs: docs/panels-design.md.

const (
	panelsDir     = "panels"
	panelMax      = 500
	panelArtMax   = 40      // artwork files per panel
	panelArtBytes = 1 << 20 // 1 MB
	panelRowsMax  = 24
	panelColsMax  = 24
	panelNameMax  = 60
	panelBodyMax  = 256 << 10

	panelActsSession = "session"
	panelActsAPI     = "api"
	panelActsBoth    = "both"   // its own client when the page has a key, else Session mode
	panelActsPublic  = "public" // its own client, with a short-lived key for any page that asks (panelpublic.go)
	panelShareMe     = "me"
	panelShareNamed  = "named"
	panelShareAll    = "everyone"
)

var (
	panelIDRe = regexp.MustCompile(`^p-[0-9a-f]{8}$`)
	// A slug is the panel's short name in /panels/<slug>. It never looks like an id or the editor's path.
	panelSlugRe = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,30}[a-z0-9])?$`)
	artIDRe     = regexp.MustCompile(`^a-[0-9a-f]{8}$`)

	errPanelCap  = errors.New("too many panels")
	errSlugTaken = errors.New("another panel already has that short name")
	errArtCap    = errors.New("too many artwork files on this panel")
)

// PanelShare: Mode is panelShareMe, panelShareNamed (Users) or panelShareAll. Everyone it
// reaches may use the panel, never change it.
type PanelShare struct {
	Mode  string   `yaml:"mode" json:"mode"`
	Users []string `yaml:"users,omitempty" json:"users,omitempty"`
}

// PanelArt is one stored artwork file. Type comes from the content when it was uploaded.
type PanelArt struct {
	ID   string `yaml:"id" json:"id"`
	Type string `yaml:"type" json:"type"`
	Size int    `yaml:"size" json:"size"`
}

// Panel is a file's content. Cells and Settings stay free-form here: their shape belongs to
// the panel page and editor, and Extra keeps any field this version does not know, so an
// older Sode never drops what a newer one wrote.
type Panel struct {
	ID     string `yaml:"id" json:"id"`
	Name   string `yaml:"name" json:"name"`
	Owner  string `yaml:"owner" json:"owner"`
	Acts   string `yaml:"acts" json:"acts"`
	Client string `yaml:"client,omitempty" json:"client,omitempty"` // the API client of an api panel
	// Slug is the optional short name that opens the panel at /panels/<slug>. Unique across panels.
	Slug string `yaml:"slug,omitempty" json:"slug,omitempty"`
	// ClientRights rides the API only: sent by an editor, read from the client in the config,
	// never stored in the panel file.
	ClientRights *config.Rights   `yaml:"-" json:"clientRights,omitempty"`
	Share        PanelShare       `yaml:"share" json:"share"`
	Rows         int              `yaml:"rows" json:"rows"`
	Cols         int              `yaml:"cols" json:"cols"`
	Cells        []map[string]any `yaml:"cells" json:"cells"`
	Settings     map[string]any   `yaml:"settings" json:"settings"`
	Art          []PanelArt       `yaml:"art,omitempty" json:"art,omitempty"`
	// Rev is the server's: it counts stored changes, and an open panel updates when it moves.
	Rev int `yaml:"rev,omitempty" json:"rev"`

	Extra map[string]any `yaml:",inline" json:"-"`
}

// panelKnown are the JSON names the struct owns. canEdit, hasKey and clientNetworks are output only and never stored,
// so a client that echoes a read back does not plant it in Extra.
var panelKnown = []string{"id", "name", "owner", "acts", "client", "slug", "share", "rows", "cols", "cells", "settings", "art", "rev", "canEdit", "clientRights", "hasKey", "clientNetworks", "online", "watching", "instances"}

type panelPlain Panel // no methods, so the JSON hooks below do not recurse

func (p Panel) MarshalJSON() ([]byte, error) {
	b, err := json.Marshal(panelPlain(p))
	if err != nil || len(p.Extra) == 0 {
		return b, err
	}
	var m map[string]json.RawMessage
	json.Unmarshal(b, &m)
	for k, v := range p.Extra {
		if _, own := m[k]; !own {
			m[k], _ = json.Marshal(v)
		}
	}
	return json.Marshal(m)
}

func (p *Panel) UnmarshalJSON(b []byte) error {
	var plain panelPlain
	if err := json.Unmarshal(b, &plain); err != nil {
		return err
	}
	var m map[string]any
	json.Unmarshal(b, &m)
	for _, k := range panelKnown {
		delete(m, k)
	}
	*p = Panel(plain)
	p.Extra = nil
	if len(m) > 0 {
		p.Extra = m
	}
	return nil
}

// normalize fills defaults; it never fails, validatePanel judges the result.
func (p *Panel) normalize() {
	if p.Rows == 0 {
		p.Rows = 4
	}
	if p.Cols == 0 {
		p.Cols = 8
	}
	if p.Acts == "" {
		p.Acts = panelActsSession
	}
	if p.Share.Mode == "" {
		p.Share.Mode = panelShareMe
	}
	if p.Cells == nil {
		p.Cells = []map[string]any{}
	}
	if p.Settings == nil {
		p.Settings = map[string]any{}
	}
}

func validatePanel(p *Panel) error {
	p.Name = cleanText(strings.TrimSpace(p.Name), panelNameMax)
	switch {
	case p.Name == "":
		return errors.New("a panel needs a name")
	case p.Slug != "" && !validPanelSlug(p.Slug):
		return errors.New("a short name is 1 to 32 lowercase letters, digits and dashes, and starts and ends with a letter or digit")
	case p.Rows < 1 || p.Rows > panelRowsMax || p.Cols < 1 || p.Cols > panelColsMax:
		return fmt.Errorf("rows and columns are 1 to %d and 1 to %d", panelRowsMax, panelColsMax)
	case p.Acts != panelActsSession && !actsHasClient(p.Acts):
		return fmt.Errorf("acts is %q, %q, %q or %q", panelActsSession, panelActsAPI, panelActsBoth, panelActsPublic)
	case !slices.Contains([]string{panelShareMe, panelShareNamed, panelShareAll}, p.Share.Mode):
		return fmt.Errorf("share mode is %q, %q or %q", panelShareMe, panelShareNamed, panelShareAll)
	case len(p.Cells) > panelRowsMax*panelColsMax:
		return errors.New("too many cells for the grid")
	case len(p.Share.Users) > config.MaxUsers:
		return errors.New("too many named users")
	}
	settings, err := cleanFixedDests(p.Settings)
	if err != nil {
		return err
	}
	p.Settings = settings
	if p.Share.Mode != panelShareNamed {
		p.Share.Users = nil
	}
	return nil
}

func validPanelID(id string) bool { return panelIDRe.MatchString(id) }

// actsHasClient: the panel owns an API client and holds keys. Keep in step with hasClient in web/panelsrules.js.
func actsHasClient(acts string) bool {
	return acts == panelActsAPI || acts == panelActsBoth || acts == panelActsPublic
}

// validPanelSlug leaves out "editor" (its own page) and anything shaped like an id, so a slug never shadows either.
func validPanelSlug(s string) bool {
	return len(s) <= 32 && panelSlugRe.MatchString(s) && s != "editor" && !validPanelID(s)
}

// slugTakenLocked says another panel than id holds slug.
func (s *panelStore) slugTakenLocked(slug, id string) bool {
	if slug == "" {
		return false
	}
	for _, q := range s.by {
		if q.Slug == slug && q.ID != id {
			return true
		}
	}
	return false
}

// bySlug is the panel with that short name.
func (s *panelStore) bySlug(slug string) (Panel, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if slug != "" {
		for _, q := range s.by {
			if q.Slug == slug {
				return q, true
			}
		}
	}
	return Panel{}, false
}

// panelStore: Panel values held here are never mutated in place; every change stores a new
// one, so a copy returned by get stays valid. Never call with App.mu held: changes write files.
type panelStore struct {
	yamlDir[Panel]
	art   map[string][]byte // memory only: "<id>/<artID>"
	probs []string
	cps   map[string][]panelCheckpoint // checkpoints read so far, see panelpush.go
	sets  *panelSetStore
}

// loadPanels runs once from New. A missing folder is empty. A file that does not parse, or
// whose id differs from its name, is logged, kept in problems and skipped.
func loadPanels(cfgPath string) *panelStore {
	s := &panelStore{art: map[string][]byte{}, sets: loadPanelSets(cfgPath)}
	s.by = map[string]Panel{}
	s.prefix = "p-"
	s.idOf = func(p *Panel) *string { return &p.ID }
	s.nameOf = func(p Panel) string { return p.Name }
	s.pathOf = func(dir string, p Panel) string { return filepath.Join(dir, p.ID+".yaml") }
	if cfgPath == "" {
		return s
	}
	s.dir = filepath.Join(filepath.Dir(cfgPath), panelsDir)
	slugSeen := map[string]bool{}
	err := s.loadDir(s.dir, func(path, id string, b []byte, err error) (Panel, bool) {
		var p Panel
		if err != nil {
			s.problem("%s: %v", path, err)
			return p, false
		}
		if err := yaml.Unmarshal(b, &p); err != nil {
			s.problem("%s: not valid YAML: %v", path, err)
			return p, false
		}
		if p.ID != id || !validPanelID(id) {
			s.problem("%s: the file name and its id %q differ", path, p.ID)
			return p, false
		}
		p.normalize()
		if p.Slug != "" && (!validPanelSlug(p.Slug) || slugSeen[p.Slug]) {
			s.problem("%s: short name %q is not valid or is used by an earlier panel, so it was dropped", path, p.Slug)
			p.Slug = ""
		}
		slugSeen[p.Slug] = true
		return p, true
	})
	if err != nil && !os.IsNotExist(err) {
		s.problem("%s: %v", s.dir, err)
	}
	return s
}

func (s *panelStore) problem(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	log.Printf("panels: skipped %s", msg)
	s.probs = append(s.probs, msg)
}

func (s *panelStore) problems() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.probs)
}

func (s *panelStore) list() []Panel { return s.sorted(nil) }

// writeLocked saves p before the map changes, so a failed write leaves memory as the file is.
// Every stored change moves Rev, which open panels poll to know they are out of date.
func (s *panelStore) writeLocked(p *Panel) error {
	p.Rev++
	return s.yamlDir.writeLocked(*p)
}

// create stores p under a new id and returns it. Callers validate first.
func (s *panelStore) create(p Panel) (Panel, error) {
	vet := func(p Panel) error {
		if s.slugTakenLocked(p.Slug, "") {
			return errSlugTaken
		}
		return nil
	}
	return s.insert(p, panelMax, errPanelCap, vet, func(p *Panel) error {
		p.normalize()
		return s.writeLocked(p)
	})
}

// replace stores p over the panel of the same id; it reports false if there is none.
func (s *panelStore) replace(p Panel) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.by[p.ID]; !ok {
		return false, nil
	}
	if s.slugTakenLocked(p.Slug, p.ID) {
		return true, errSlugTaken
	}
	if err := s.writeLocked(&p); err != nil {
		return true, err
	}
	s.by[p.ID] = p
	return true, nil
}

// followUsers applies a Users save: renames maps a lowercased old name to the new one (owner and
// sharing), removed holds lowercased names that leave sharing. An owner who is removed stays on
// their panels so a Trusted user can still reach them. Each panel is written on its own; the
// count is of panels changed. A failed write skips that panel and returns the first error.
func (s *panelStore) followUsers(renames map[string]string, removed map[string]bool) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	var first error
	for id, p := range s.by {
		q := p
		changed := false
		if to, ok := renames[strings.ToLower(p.Owner)]; ok && to != p.Owner {
			q.Owner, changed = to, true
		}
		if len(p.Share.Users) > 0 {
			users := make([]string, 0, len(p.Share.Users))
			for _, u := range p.Share.Users {
				k := strings.ToLower(u)
				if removed[k] {
					changed = true
					continue
				}
				if to, ok := renames[k]; ok && to != u {
					u, changed = to, true
				}
				users = append(users, u)
			}
			q.Share.Users = users
		}
		if !changed {
			continue
		}
		if err := s.writeLocked(&q); err != nil {
			if first == nil {
				first = err
			}
			continue
		}
		s.by[id] = q
		n++
	}
	return n, first
}

func (s *panelStore) delete(id string) (bool, error) {
	return s.remove(id, func(p Panel) {
		if s.dir != "" {
			os.RemoveAll(filepath.Join(s.dir, id+".art"))
		}
		for _, a := range p.Art {
			delete(s.art, id+"/"+a.ID)
		}
		s.dropCheckpoints(id)
	})
}

func (s *panelStore) artPath(id, art string) string {
	return filepath.Join(s.dir, id+".art", art)
}

// addArt stores data (already type-checked) and lists it on the panel.
func (s *panelStore) addArt(id string, data []byte, typ string) (PanelArt, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return PanelArt{}, false, nil
	}
	if len(p.Art) >= panelArtMax {
		return PanelArt{}, true, errArtCap
	}
	b := make([]byte, 4)
	rand.Read(b)
	art := PanelArt{ID: "a-" + hex.EncodeToString(b), Type: typ, Size: len(data)}
	if s.dir != "" {
		if err := os.MkdirAll(filepath.Join(s.dir, id+".art"), 0o700); err != nil {
			return PanelArt{}, true, err
		}
		if err := config.AtomicWrite(s.artPath(id, art.ID), data); err != nil {
			return PanelArt{}, true, err
		}
	} else {
		s.art[id+"/"+art.ID] = data
	}
	p.Art = append(slices.Clone(p.Art), art)
	if err := s.writeLocked(&p); err != nil {
		return PanelArt{}, true, err
	}
	s.by[id] = p
	return art, true, nil
}

func (s *panelStore) artData(id, art string) ([]byte, PanelArt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return nil, PanelArt{}, false
	}
	i := slices.IndexFunc(p.Art, func(a PanelArt) bool { return a.ID == art })
	if i < 0 {
		return nil, PanelArt{}, false
	}
	if s.dir == "" {
		b, ok := s.art[id+"/"+art]
		return b, p.Art[i], ok
	}
	b, err := os.ReadFile(s.artPath(id, art))
	return b, p.Art[i], err == nil
}

func (s *panelStore) delArt(id, art string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return false, nil
	}
	i := slices.IndexFunc(p.Art, func(a PanelArt) bool { return a.ID == art })
	if i < 0 {
		return false, nil
	}
	p.Art = slices.Delete(slices.Clone(p.Art), i, i+1)
	if err := s.writeLocked(&p); err != nil {
		return true, err
	}
	s.by[id] = p
	if s.dir != "" {
		os.Remove(s.artPath(id, art))
	}
	delete(s.art, id+"/"+art)
	return true, nil
}

// copyArt gives dst a copy of every artwork file of src, under the same ids, so cells that
// name an artwork id keep working in the copy. Caller holds no lock.
func (s *panelStore) copyArt(src Panel, dst string) {
	for _, a := range src.Art {
		if b, _, ok := s.artData(src.ID, a.ID); ok {
			s.putArtAs(dst, a, b)
		}
	}
}

func (s *panelStore) putArtAs(id string, art PanelArt, data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, ok := s.by[id]
	if !ok {
		return
	}
	if s.dir != "" {
		if os.MkdirAll(filepath.Join(s.dir, id+".art"), 0o700) != nil || config.AtomicWrite(s.artPath(id, art.ID), data) != nil {
			return
		}
	} else {
		s.art[id+"/"+art.ID] = data
	}
	p.Art = append(slices.Clone(p.Art), art)
	if s.writeLocked(&p) == nil {
		s.by[id] = p
	}
}

const svgNS = "http://www.w3.org/2000/svg"

// sniffArt decides the type from the bytes, never from a name or header. "" = not allowed.
func sniffArt(data []byte) string {
	switch t := http.DetectContentType(data); t {
	case "image/png", "image/jpeg", "image/gif":
		return t
	}
	if isSVG(data) {
		return "image/svg+xml"
	}
	return ""
}

// isSVG: the first element is svg, after only a declaration, comments, a doctype and blanks.
func isSVG(data []byte) bool {
	dec := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, []byte("\xef\xbb\xbf"))))
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return t.Name.Local == "svg" && (t.Name.Space == "" || t.Name.Space == svgNS)
		case xml.CharData:
			if len(bytes.TrimSpace(t)) > 0 {
				return false
			}
		}
	}
}
