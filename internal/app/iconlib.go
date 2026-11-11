package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"unicode/utf8"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Icon library: one install-wide set of SVG and PNG icons, icons.yaml beside the config and the files in icons/
// (memory only with no config path). A cell's Icon A names one as `lib:<id>`. Docs: docs/panels-design.md.

const (
	iconLibFile  = "icons.yaml"
	iconLibDir   = "icons"
	iconNameMax  = 40 // runes, as libNameProblem in web/iconlibrules.js
	iconLibTotal = 64 << 20
)

var (
	// Random, not by content: two names for the same file stay two icons. Keep in step with LIB in web/panellook.js.
	iconIDRe = regexp.MustCompile(`^[0-9a-f]{8}$`)

	errIconLibCap  = errors.New("the Icon library would pass its 64 MB total; delete some icons first")
	errIconBadName = fmt.Errorf("an icon needs a name of 1 to %d characters", iconNameMax)
)

// LibIcon is one library icon as the index stores it and the API sends it.
type LibIcon struct {
	ID   string `yaml:"id" json:"id"`
	Name string `yaml:"name" json:"name"`
	Type string `yaml:"type" json:"type"`
	Size int    `yaml:"size" json:"size"`
}

func (ic LibIcon) fileName() string {
	if ic.Type == "image/svg+xml" {
		return ic.ID + ".svg"
	}
	return ic.ID + ".png"
}

type iconLibData struct {
	Rev   int       `yaml:"rev"`
	Icons []LibIcon `yaml:"icons"`
}

// iconLib: values handed out are copies; every change writes the index before memory moves.
type iconLib struct {
	mu     sync.Mutex
	scanMu sync.Mutex // one rescan at a time, so two never add the same file
	path   string     // "" = memory only
	data   iconLibData
	mem    map[string][]byte
}

func loadIconLib(cfgPath string) *iconLib {
	s := &iconLib{mem: map[string][]byte{}}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), iconLibFile)
	b, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("icon library: %v", err)
		}
		return s
	}
	var d iconLibData
	if err := yaml.Unmarshal(b, &d); err != nil {
		log.Printf("icon library: %s is not valid YAML, so no icons were read: %v", s.path, err)
		return s
	}
	s.data.Rev = d.Rev
	for _, ic := range d.Icons {
		if !iconIDRe.MatchString(ic.ID) || slices.ContainsFunc(s.data.Icons, func(x LibIcon) bool { return x.ID == ic.ID }) {
			log.Printf("icon library: skipped an entry with a bad or repeated id: %+v", ic)
			continue
		}
		// The type becomes the Content-Type and the name reaches every page, so a hand edit gets the upload's rules.
		if ic.Type != "image/png" && ic.Type != "image/svg+xml" || !iconNameOK(ic.Name) {
			log.Printf("icon library: skipped an entry with a bad type or name: %+v", ic)
			continue
		}
		s.data.Icons = append(s.data.Icons, ic)
	}
	return s
}

func (s *iconLib) dir() string { return filepath.Join(filepath.Dir(s.path), iconLibDir) }

func (s *iconLib) list() []LibIcon {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.Icons)
}

func iconNameOK(name string) bool {
	return name != "" && utf8.RuneCountInString(name) <= iconNameMax && cleanText(name, iconNameMax) == name
}

// freeName is name, or name with " 2", " 3" and so on when another icon has it ignoring case. The base is cut so
// the result still fits iconNameMax. "" with errIconBadName when name is empty or too long.
func freeName(name string, icons []LibIcon) (string, error) {
	name = strings.TrimSpace(name)
	if !iconNameOK(name) {
		return "", errIconBadName
	}
	taken := func(n string) bool {
		return slices.ContainsFunc(icons, func(ic LibIcon) bool { return strings.EqualFold(ic.Name, n) })
	}
	if !taken(name) {
		return name, nil
	}
	for i := 2; ; i++ {
		tail := fmt.Sprintf(" %d", i)
		base := []rune(name)
		if keep := iconNameMax - len(tail); len(base) > keep {
			base = base[:keep]
		}
		if n := strings.TrimSpace(string(base)) + tail; !taken(n) {
			return n, nil
		}
	}
}

// add stores data (already checked by checkArtBytes, PNG or SVG) under a free name.
func (s *iconLib) add(name string, data []byte, typ string) (LibIcon, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	name, err := freeName(name, s.data.Icons)
	if err != nil {
		return LibIcon{}, err
	}
	total := len(data)
	for _, ic := range s.data.Icons {
		total += ic.Size
	}
	if total > iconLibTotal {
		return LibIcon{}, errIconLibCap
	}
	b := make([]byte, 4)
	rand.Read(b)
	ic := LibIcon{ID: hex.EncodeToString(b), Name: name, Type: typ, Size: len(data)}
	for slices.ContainsFunc(s.data.Icons, func(x LibIcon) bool { return x.ID == ic.ID }) {
		rand.Read(b)
		ic.ID = hex.EncodeToString(b)
	}
	next := s.data
	next.Icons = append(slices.Clone(s.data.Icons), ic)
	if s.path == "" {
		s.mem[ic.ID] = data
		return ic, s.saveLocked(next)
	}
	if err := os.MkdirAll(s.dir(), 0o700); err != nil {
		return LibIcon{}, err
	}
	if err := config.AtomicWrite(filepath.Join(s.dir(), ic.fileName()), data); err != nil {
		return LibIcon{}, err
	}
	if err := s.saveLocked(next); err != nil {
		os.Remove(filepath.Join(s.dir(), ic.fileName()))
		return LibIcon{}, err
	}
	return ic, nil
}

// saveLocked writes next as the index; only then does memory take it.
func (s *iconLib) saveLocked(next iconLibData) error {
	next.Rev = s.data.Rev + 1
	if s.path != "" {
		out, err := yaml.Marshal(next)
		if err == nil {
			err = config.AtomicWrite(s.path, out)
		}
		if err != nil {
			return err
		}
	}
	s.data = next
	return nil
}

// rename keeps the id, so every `lib:<id>` still names it. The name is settled by freeName against the others.
// found is false for an id the library does not hold.
func (s *iconLib) rename(id, name string) (ic LibIcon, found bool, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.data.Icons, func(x LibIcon) bool { return x.ID == id })
	if i < 0 {
		return LibIcon{}, false, nil
	}
	others := slices.Delete(slices.Clone(s.data.Icons), i, i+1)
	if name, err = freeName(name, others); err != nil {
		return LibIcon{}, true, err
	}
	next := s.data
	next.Icons = slices.Clone(s.data.Icons)
	next.Icons[i].Name = name
	if err := s.saveLocked(next); err != nil {
		return LibIcon{}, true, err
	}
	return next.Icons[i], true, nil
}

// remove leaves cells that name the icon alone: they draw no Icon A, as for any id the library does not hold.
func (s *iconLib) remove(id string) (LibIcon, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.data.Icons, func(x LibIcon) bool { return x.ID == id })
	if i < 0 {
		return LibIcon{}, false, nil
	}
	ic := s.data.Icons[i]
	next := s.data
	next.Icons = slices.Delete(slices.Clone(s.data.Icons), i, i+1)
	if err := s.saveLocked(next); err != nil {
		return LibIcon{}, true, err
	}
	if s.path == "" {
		delete(s.mem, id)
	} else if err := os.Remove(filepath.Join(s.dir(), ic.fileName())); err != nil && !os.IsNotExist(err) {
		log.Printf("icon library: %v", err) // the index no longer names it, so it is only a stray file
	}
	return ic, true, nil
}

func (s *iconLib) file(id string) ([]byte, LibIcon, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.data.Icons, func(ic LibIcon) bool { return ic.ID == id })
	if i < 0 {
		return nil, LibIcon{}, false
	}
	ic := s.data.Icons[i]
	if s.path == "" {
		b, ok := s.mem[id]
		return b, ic, ok
	}
	b, err := os.ReadFile(filepath.Join(s.dir(), ic.fileName()))
	return b, ic, err == nil
}
