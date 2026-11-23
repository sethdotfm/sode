package app

import (
	"bytes"
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Default artwork (docs/panels-design.md): one shared set, artdefaults.yaml beside the config, images in
// artdefaults.art/. Cells resolve it when drawn and never store it, so a Push or a template carries a cell's own
// artwork only.

const (
	artDefaultsFile = "artdefaults.yaml"
	artDefaultsDir  = "artdefaults.art"
	artDefaultsMax  = 2000

	artKindSource = "source"
	artKindDst    = "destination"
	artKindPreset = "preset"
)

var (
	// A default's image id comes from its content, so the same image set twice is stored one time.
	artDefaultIDRe = regexp.MustCompile(`^d-[0-9a-f]{8}$`)
	artIconRe      = regexp.MustCompile(`^icon:[a-z0-9-]+$`)
	// Keep in step with ART_B_CHOICES in web/panellook.js.
	artBRe = regexp.MustCompile(`^(?:[0-9]|1[0-6]|[A-Z])$`)

	errArtDefaultsCap   = &panelClientFail{http.StatusConflict, "conflict", "Default artwork would pass the 64 MB total; clear some first"}
	errNoArtDefaultFile = &panelClientFail{http.StatusNotFound, "not_found", "no such Default artwork image"}
)

// A Preset default keys on the hidden ID, so a rename keeps it. A Source or Destination default keeps the router name
// from when it was set. Then the editor can warn when the number names a different item.
type ArtDefault struct {
	Kind     string `yaml:"kind" json:"kind"`
	Router   string `yaml:"router,omitempty" json:"router,omitempty"`
	N        *int   `yaml:"n,omitempty" json:"n,omitempty"`
	ID       string `yaml:"id,omitempty" json:"id,omitempty"`
	Scope    string `yaml:"scope,omitempty" json:"scope,omitempty"`
	Name     string `yaml:"name" json:"name"`
	Art      string `yaml:"art,omitempty" json:"art,omitempty"`
	ArtB     string `yaml:"artB,omitempty" json:"artB,omitempty"`
	HideText bool   `yaml:"hideText,omitempty" json:"hideText,omitempty"`
}

// A Source or Destination default keys by kind, router and number. Keep in step with itemKey in web/panellook.js.
func (d ArtDefault) key() string {
	if d.Kind == artKindPreset {
		if d.ID != "" {
			return d.Kind + "/" + d.ID
		}
		return d.Kind + "/" + d.Scope + "/" + d.Name
	}
	if d.Router == "" {
		return d.oldKey()
	}
	return fmt.Sprintf("%s/%s/%d", d.Kind, d.Router, d.num())
}

// oldKey is the key with no router, as a Sode with one router gave it. It names the first router's item.
func (d ArtDefault) oldKey() string {
	return fmt.Sprintf("%s/%d", d.Kind, d.num())
}

func (d ArtDefault) num() int {
	if d.N == nil {
		return -1
	}
	return *d.N
}

// keyed: key is d's key, or d's old key when d is on the first router.
func (d ArtDefault) keyed(key, first string) bool {
	return key == d.key() || d.Kind != artKindPreset && d.Router == first && key == d.oldKey()
}

// set checks the artwork fields.
func (d *ArtDefault) check() error {
	switch d.Kind {
	case artKindSource, artKindDst:
		if d.N == nil || *d.N < 0 || *d.N > 0xffff {
			return errors.New("a Source or Destination default needs its router number n")
		}
		d.Scope, d.ID = "", ""
		d.Router = cleanText(d.Router, 64)
		d.Name = cleanText(strings.TrimSpace(d.Name), panelNameMax)
	case artKindPreset:
		d.N, d.Router = nil, ""
		d.ID = cleanText(d.ID, 64)
		d.Name = cleanText(strings.TrimSpace(d.Name), panelNameMax)
		// No "me": the set is shared, so a Me default would reach every user's Me preset of that name.
		if d.Scope != "everyone" || d.Name == "" && d.ID == "" {
			return errors.New(`a Preset default needs scope "everyone" and the preset's id or name; Me presets have no Default artwork`)
		}
	default:
		return fmt.Errorf("kind is %q, %q or %q", artKindSource, artKindDst, artKindPreset)
	}
	return nil
}

type artDefaultsFileData struct {
	Rev      int          `yaml:"rev"`
	Defaults []ArtDefault `yaml:"defaults"`
	Images   []PanelArt   `yaml:"images,omitempty"`
}

// Values handed out are copies. Every change writes the file before memory moves.
type artDefaultStore struct {
	mu     sync.Mutex
	path   string // "" = memory only
	data   artDefaultsFileData
	images blobDir // named by image id
}

func loadArtDefaults(cfgPath string) *artDefaultStore {
	s := &artDefaultStore{}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), artDefaultsFile)
	s.images.dir = filepath.Join(filepath.Dir(cfgPath), artDefaultsDir)
	b, err := os.ReadFile(s.path)
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("default artwork: %v", err)
		}
		return s
	}
	var d artDefaultsFileData
	if err := yaml.Unmarshal(b, &d); err != nil {
		log.Printf("default artwork: %s is not valid YAML, so no defaults were read: %v", s.path, err)
		return s
	}
	seen := map[string]bool{}
	for _, x := range d.Defaults {
		// A stored Source or Destination default names its router. Only a request can leave it out.
		if x.check() != nil || x.Kind != artKindPreset && x.Router == "" || !storedArtOK(x) || seen[x.key()] {
			log.Printf("default artwork: skipped a default that is not valid or repeats an item: %+v", x)
			continue
		}
		seen[x.key()] = true
		s.data.Defaults = append(s.data.Defaults, x)
	}
	s.data.Rev = d.Rev
	for _, im := range d.Images {
		if !artDefaultIDRe.MatchString(im.ID) || !artTypeOK(im.Type) {
			log.Printf("default artwork: skipped an image with a bad id or type: %+v", im)
			continue
		}
		s.data.Images = append(s.data.Images, im)
	}
	return s
}

// storedArtOK: the forms set stores, so a hand edit cannot name a URL or another panel's artwork.
func storedArtOK(d ArtDefault) bool {
	r, ok := parseArtRef(d.Art)
	return ok && r.kind != artRefPanel && (d.ArtB == "" || artBRe.MatchString(d.ArtB))
}

func (s *artDefaultStore) list() []ArtDefault {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.Defaults)
}

func (s *artDefaultStore) writeLocked(next artDefaultsFileData) error {
	next.Rev = s.data.Rev + 1
	used := map[string]bool{}
	for _, d := range next.Defaults {
		used[d.Art] = true
	}
	var gone []string
	next.Images = slices.DeleteFunc(slices.Clone(next.Images), func(im PanelArt) bool {
		if used[im.ID] {
			return false
		}
		gone = append(gone, im.ID)
		return true
	})
	if err := writeYAML(s.path, next); err != nil {
		return err
	}
	s.data = next
	for _, id := range gone {
		s.images.del(id)
	}
	return nil
}

// Callers check the bytes first. added is false when the same bytes are stored already.
func (s *artDefaultStore) putImageLocked(next *artDefaultsFileData, data []byte, typ string) (id string, added bool, err error) {
	sum := sha256.Sum256(data)
	// The id keeps 4 bytes of the hash (exports name the file by it), so a clash with other bytes takes the next 4.
	for i := 0; ; i += 4 {
		if i+4 > len(sum) {
			return "", false, errors.New("no free image id")
		}
		id = "d-" + hex.EncodeToString(sum[i:i+4])
		if !slices.ContainsFunc(next.Images, func(im PanelArt) bool { return im.ID == id }) {
			break
		}
		if b, ok := s.images.get(id); ok && bytes.Equal(b, data) {
			return id, false, nil
		}
	}
	if artBytes(Panel{Art: next.Images})+len(data) > panelArtTotal {
		return "", false, errArtDefaultsCap
	}
	if err := s.images.put(id, data); err != nil {
		return "", false, err
	}
	next.Images = append(next.Images, PanelArt{ID: id, Type: typ, Size: len(data)})
	return id, true, nil
}

// The bytes already passed checkArtBytes.
type artImage struct {
	data []byte
	typ  string
}

func (s *artDefaultStore) set(d ArtDefault, img *artImage) (ArtDefault, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.data
	next.Images = slices.Clone(s.data.Images)
	next.Defaults = slices.DeleteFunc(slices.Clone(s.data.Defaults), func(x ArtDefault) bool { return d.replaces(x) })
	i := slices.IndexFunc(next.Defaults, func(x ArtDefault) bool { return x.key() == d.key() })
	if i < 0 && len(next.Defaults) >= artDefaultsMax {
		return d, fmt.Errorf("at most %d defaults", artDefaultsMax)
	}
	added := false
	if img != nil {
		id, add, err := s.putImageLocked(&next, img.data, img.typ)
		if err != nil {
			return d, err
		}
		d.Art, added = id, add
	} else if r, _ := parseArtRef(d.Art); r.kind == artRefDefault && !slices.ContainsFunc(next.Images, func(im PanelArt) bool { return im.ID == d.Art }) {
		return d, errNoArtDefaultFile
	}
	if i >= 0 {
		next.Defaults[i] = d
	} else {
		next.Defaults = append(next.Defaults, d)
	}
	err := s.writeLocked(next)
	if err != nil && added {
		s.images.del(d.Art)
	}
	return d, err
}

// false: d's item had no default.
func (s *artDefaultStore) clear(d ArtDefault) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := slices.DeleteFunc(slices.Clone(s.data.Defaults), func(x ArtDefault) bool { return x.key() == d.key() || d.replaces(x) })
	if len(kept) == len(s.data.Defaults) {
		return false, nil
	}
	next := s.data
	next.Defaults = kept
	return true, s.writeLocked(next)
}

// x is a preset default stored by name with no id, for the preset that d names by id.
func (d ArtDefault) replaces(x ArtDefault) bool {
	return d.Kind == artKindPreset && d.ID != "" && x.Kind == artKindPreset && x.ID == "" && strings.EqualFold(x.Name, d.Name)
}

func (s *artDefaultStore) image(id string) ([]byte, PanelArt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.data.Images, func(im PanelArt) bool { return im.ID == id })
	if i < 0 || !artDefaultIDRe.MatchString(id) {
		return nil, PanelArt{}, false
	}
	b, ok := s.images.get(id)
	return b, s.data.Images[i], ok
}

// A default whose number is gone stays.
func (s *artDefaultStore) rename(k *pollCheck) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.data
	next.Defaults = slices.Clone(s.data.Defaults)
	changed := false
	for i, d := range next.Defaults {
		if d.Kind == artKindPreset || d.N == nil {
			continue
		}
		r := config.Ref{Router: cmp.Or(d.Router, k.uid), N: *d.N, Name: d.Name}
		if k.quiet(d.Kind == artKindDst, &r) {
			next.Defaults[i].Name, changed = r.Name, true
		}
	}
	if !changed {
		return nil
	}
	return s.writeLocked(next)
}
