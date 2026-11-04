package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/sethdotfm/sode/internal/config"
)

// yamlMaxBody caps a converter request. The route is stateless and open, so
// the cap is its only guard.
const yamlMaxBody = 1 << 20

// exportVersion is the only export document version understood.
const exportVersion = 1

// exportDoc is the export/import document (Beta 1.6). Names ride beside the
// numbers so an import can match by name when numbers differ between sites.
type exportDoc struct {
	Sode       int                `yaml:"sode" json:"sode"`
	Site       string             `yaml:"site,omitempty" json:"site,omitempty"`
	User       string             `yaml:"user,omitempty" json:"user,omitempty"`
	Exported   string             `yaml:"exported,omitempty" json:"exported,omitempty"`
	Prefs      map[string]string  `yaml:"prefs,omitempty" json:"prefs,omitempty"`
	FilterSets []config.FilterSet `yaml:"filterSets,omitempty" json:"filterSets,omitempty"`
	Presets    []exportPreset     `yaml:"presets,omitempty" json:"presets,omitempty"`
}

type exportPreset struct {
	Name  string       `yaml:"name" json:"name"`
	Items []exportItem `yaml:"items" json:"items"`
}

type exportItem struct {
	Src     int         `yaml:"src" json:"src"`
	SrcName string      `yaml:"srcName,omitempty" json:"srcName,omitempty"`
	Dsts    []exportDst `yaml:"dsts" json:"dsts"`
	Levels  []int       `yaml:"levels,omitempty" json:"levels,omitempty"`
}

type exportDst struct {
	N    int    `yaml:"n" json:"n"`
	Name string `yaml:"name,omitempty" json:"name,omitempty"`
}

// check rejects other versions and anything the config's own caps and name
// rules would refuse.
func (d *exportDoc) check() error {
	if d.Sode != exportVersion {
		if d.Sode == 0 {
			return errors.New("not a Sode export: missing \"sode\" version")
		}
		return fmt.Errorf("unsupported export version %d (this Sode reads version %d)", d.Sode, exportVersion)
	}
	if d.Exported != "" {
		if _, err := time.Parse(time.RFC3339, d.Exported); err != nil {
			return errors.New("\"exported\" must be an RFC 3339 time")
		}
	}
	var errs []error
	names := make([]string, len(d.FilterSets))
	for i, f := range d.FilterSets {
		names[i] = f.Name
	}
	errs = append(errs, config.ValidateNames("filter set", names, 1000)...)
	names = make([]string, len(d.Presets))
	for i, p := range d.Presets {
		names[i] = p.Name
	}
	errs = append(errs, config.ValidateNames("preset", names, config.MaxPresets)...)
	for _, p := range d.Presets {
		total := 0
		if len(p.Items) > config.MaxPresetItems {
			errs = append(errs, fmt.Errorf("preset %q: at most %d sources", p.Name, config.MaxPresetItems))
		}
		for _, it := range p.Items {
			total += len(it.Dsts)
			if len(it.Dsts) > config.MaxPresetDsts {
				errs = append(errs, fmt.Errorf("preset %q: at most %d destinations per source", p.Name, config.MaxPresetDsts))
			}
			if it.Src < 0 {
				errs = append(errs, fmt.Errorf("preset %q: negative source number", p.Name))
			}
			for _, x := range it.Dsts {
				if x.N < 0 {
					errs = append(errs, fmt.Errorf("preset %q: negative destination number", p.Name))
				}
			}
			for _, l := range it.Levels {
				if l < 1 || l > config.MaxPresetLevelNum {
					errs = append(errs, fmt.Errorf("preset %q: level %d out of range", p.Name, l))
				}
			}
		}
		if total > config.MaxPresetTotal {
			errs = append(errs, fmt.Errorf("preset %q: at most %d destinations in all", p.Name, config.MaxPresetTotal))
		}
	}
	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// serveYAML converts an export document: JSON body to YAML, or YAML body to
// JSON with ?from=yaml. Stateless and open on purpose: it reads no config,
// holds nothing and changes nothing.
func (a *App) serveYAML(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, yamlMaxBody))
	if err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			http.Error(w, "body too large (1 MiB at most)", http.StatusRequestEntityTooLarge)
			return
		}
		http.Error(w, "could not read body", http.StatusBadRequest)
		return
	}
	var doc exportDoc
	fromYAML := r.URL.Query().Get("from") == "yaml"
	if fromYAML {
		dec := yaml.NewDecoder(bytes.NewReader(body))
		dec.KnownFields(true)
		err = dec.Decode(&doc)
	} else {
		dec := json.NewDecoder(bytes.NewReader(body))
		dec.DisallowUnknownFields()
		err = dec.Decode(&doc)
	}
	if err != nil {
		kind := "JSON"
		if fromYAML {
			kind = "YAML"
		}
		if errors.Is(err, io.EOF) {
			err = errors.New("empty document")
		}
		http.Error(w, fmt.Sprintf("bad %s: %s", kind, strings.TrimSpace(err.Error())), http.StatusBadRequest)
		return
	}
	if err := doc.check(); err != nil {
		http.Error(w, err.Error(), http.StatusUnprocessableEntity)
		return
	}
	if fromYAML {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		json.NewEncoder(w).Encode(doc)
		return
	}
	out, err := yaml.Marshal(doc)
	if err != nil {
		http.Error(w, "could not write YAML", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Write(out)
}
