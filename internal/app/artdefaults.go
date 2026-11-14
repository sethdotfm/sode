package app

import (
	"crypto/sha256"
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

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// Default artwork: one shared set, artdefaults.yaml beside the config, its images in artdefaults.art/
// (memory only with no config path). Cells resolve it when drawn and never store it, so a Push or a
// template carries a cell's own artwork only. Docs: docs/panels-design.md.

const (
	artDefaultsFile = "artdefaults.yaml"
	artDefaultsDir  = "artdefaults.art"
	artDefaultsMax  = 2000

	artKindSource = "source"
	artKindDst    = "destination"
	artKindPreset = "preset"
)

var (
	// A default's image id comes from its content, so the same image set twice is stored once.
	artDefaultIDRe = regexp.MustCompile(`^d-[0-9a-f]{8}$`)
	artIconRe      = regexp.MustCompile(`^icon:[a-z0-9-]+$`)
	// Icon B choices, as ART_B_CHOICES in web/panellook.js.
	artBRe = regexp.MustCompile(`^(?:[0-9]|1[0-6]|[A-Z])$`)

	//lint:ignore ST1005 the UI shows this text as is
	errArtDefaultsCap   = errors.New("Default artwork would pass the 64 MB total; clear some first")
	errNoArtDefaultFile = errors.New("no such Default artwork image")
)

// ArtDefault is one item's Default artwork. N is the router number of a Source or Destination; a
// Preset is keyed by Scope and Name. For a Source or Destination Name is the router name when the
// default was set, so the editor can warn when the number now names something else.
type ArtDefault struct {
	Kind     string `yaml:"kind" json:"kind"`
	N        *int   `yaml:"n,omitempty" json:"n,omitempty"`
	Scope    string `yaml:"scope,omitempty" json:"scope,omitempty"`
	Name     string `yaml:"name" json:"name"`
	Art      string `yaml:"art,omitempty" json:"art,omitempty"`
	ArtB     string `yaml:"artB,omitempty" json:"artB,omitempty"`
	HideText bool   `yaml:"hideText,omitempty" json:"hideText,omitempty"`
}

func (d ArtDefault) key() string {
	if d.Kind == artKindPreset {
		return d.Kind + "/" + d.Scope + "/" + d.Name
	}
	n := -1
	if d.N != nil {
		n = *d.N
	}
	return fmt.Sprintf("%s/%d", d.Kind, n)
}

// check fits the key fields to what the store keeps; the artwork fields are checked by set.
func (d *ArtDefault) check() error {
	switch d.Kind {
	case artKindSource, artKindDst:
		if d.N == nil || *d.N < 0 || *d.N > 0xffff {
			return errors.New("a Source or Destination default needs its router number n")
		}
		d.Scope = ""
		d.Name = cleanText(strings.TrimSpace(d.Name), panelNameMax)
	case artKindPreset:
		d.N = nil
		d.Name = cleanText(strings.TrimSpace(d.Name), panelNameMax)
		// No "me": the set is shared, so a Me default would reach every user's Me preset of that name.
		if d.Scope != "everyone" || d.Name == "" {
			return errors.New(`a Preset default needs scope "everyone" and the preset's name; Me presets have no Default artwork`)
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

// artDefaultStore: values handed out are copies; every change writes the file before memory moves.
type artDefaultStore struct {
	mu   sync.Mutex
	path string // "" = memory only
	data artDefaultsFileData
	mem  map[string][]byte // image bytes when memory only
}

func loadArtDefaults(cfgPath string) *artDefaultStore {
	s := &artDefaultStore{mem: map[string][]byte{}}
	if cfgPath == "" {
		return s
	}
	s.path = filepath.Join(filepath.Dir(cfgPath), artDefaultsFile)
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
		if x.check() != nil || !storedArtOK(x) || seen[x.key()] {
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
	art := d.Art == "" || artIconRe.MatchString(d.Art) || artDefaultIDRe.MatchString(d.Art) ||
		strings.HasPrefix(d.Art, "lib:") && iconIDRe.MatchString(d.Art[len("lib:"):])
	return art && (d.ArtB == "" || artBRe.MatchString(d.ArtB))
}

func (s *artDefaultStore) list() []ArtDefault {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.data.Defaults)
}

func (s *artDefaultStore) imgDir() string { return filepath.Join(filepath.Dir(s.path), artDefaultsDir) }

// writeLocked saves next, then drops images no default names any more.
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
	if s.path != "" {
		b, err := yaml.Marshal(next)
		if err != nil {
			return err
		}
		if err := config.AtomicWrite(s.path, b); err != nil {
			return err
		}
	}
	s.data = next
	for _, id := range gone {
		if s.path != "" {
			os.Remove(filepath.Join(s.imgDir(), id))
		}
		delete(s.mem, id)
	}
	return nil
}

// putImageLocked stores an image by content and returns its id. Callers check the bytes first.
func (s *artDefaultStore) putImageLocked(next *artDefaultsFileData, data []byte, typ string) (string, error) {
	sum := sha256.Sum256(data)
	id := "d-" + hex.EncodeToString(sum[:4])
	if slices.ContainsFunc(next.Images, func(im PanelArt) bool { return im.ID == id }) {
		return id, nil
	}
	if artBytes(Panel{Art: next.Images})+len(data) > panelArtTotal {
		return "", errArtDefaultsCap
	}
	if s.path != "" {
		if err := os.MkdirAll(s.imgDir(), 0o700); err != nil {
			return "", err
		}
		if err := config.AtomicWrite(filepath.Join(s.imgDir(), id), data); err != nil {
			return "", err
		}
	} else {
		s.mem[id] = data
	}
	next.Images = append(next.Images, PanelArt{ID: id, Type: typ, Size: len(data)})
	return id, nil
}

// artImage is an image to store with a default: bytes already checked by checkArtBytes.
type artImage struct {
	data []byte
	typ  string
}

// set stores d over any default for the same item. img, when given, becomes d.Art.
func (s *artDefaultStore) set(d ArtDefault, img *artImage) (ArtDefault, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	next := s.data
	next.Images = slices.Clone(s.data.Images)
	if img != nil {
		id, err := s.putImageLocked(&next, img.data, img.typ)
		if err != nil {
			return d, err
		}
		d.Art = id
	} else if artDefaultIDRe.MatchString(d.Art) && !slices.ContainsFunc(next.Images, func(im PanelArt) bool { return im.ID == d.Art }) {
		return d, errNoArtDefaultFile
	}
	i := slices.IndexFunc(s.data.Defaults, func(x ArtDefault) bool { return x.key() == d.key() })
	next.Defaults = slices.Clone(s.data.Defaults)
	if i >= 0 {
		next.Defaults[i] = d
	} else {
		if len(next.Defaults) >= artDefaultsMax {
			return d, fmt.Errorf("at most %d defaults", artDefaultsMax)
		}
		next.Defaults = append(next.Defaults, d)
	}
	return d, s.writeLocked(next)
}

// false: d's item had no default.
func (s *artDefaultStore) clear(d ArtDefault) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.data.Defaults, func(x ArtDefault) bool { return x.key() == d.key() })
	if i < 0 {
		return false, nil
	}
	next := s.data
	next.Defaults = slices.Delete(slices.Clone(s.data.Defaults), i, i+1)
	return true, s.writeLocked(next)
}

func (s *artDefaultStore) image(id string) ([]byte, PanelArt, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := slices.IndexFunc(s.data.Images, func(im PanelArt) bool { return im.ID == id })
	if i < 0 || !artDefaultIDRe.MatchString(id) {
		return nil, PanelArt{}, false
	}
	if s.path == "" {
		b, ok := s.mem[id]
		return b, s.data.Images[i], ok
	}
	b, err := os.ReadFile(filepath.Join(s.imgDir(), id))
	return b, s.data.Images[i], err == nil
}
