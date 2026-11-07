package app

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlMaxBody: the route is stateless and open, so the cap is its only guard.
const yamlMaxBody = 1 << 20

// serveYAML: JSON to YAML, or YAML to JSON with ?from=yaml. Knows no schema, reads no
// config; top level must be a mapping.
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
	fromYAML := r.URL.Query().Get("from") == "yaml"
	var out []byte
	if fromYAML {
		out, err = yamlToJSON(body)
	} else {
		out, err = jsonToYAML(body)
	}
	if err != nil {
		kind := "JSON"
		if fromYAML {
			kind = "YAML"
		}
		http.Error(w, fmt.Sprintf("bad %s: %s", kind, strings.TrimSpace(err.Error())), http.StatusBadRequest)
		return
	}
	if fromYAML {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
	} else {
		w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	}
	w.Write(out)
}

var errNotMapping = errors.New("the top level must be a mapping")

// Keeps key order via a node tree; scalars keep their tags ("24" stays quoted, 24
// doesn't).
func jsonToYAML(body []byte) ([]byte, error) {
	if !json.Valid(body) {
		return nil, errors.New("not valid JSON")
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, errNotMapping
	}
	var plain func(n *yaml.Node, key bool)
	plain = func(n *yaml.Node, key bool) {
		n.Style = 0
		if !key && n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
			switch strings.ToLower(n.Value) { // booleans in YAML 1.1: other readers would change the type
			case "y", "yes", "n", "no", "on", "off":
				n.Style = yaml.DoubleQuotedStyle
			}
		}
		for i, c := range n.Content {
			plain(c, n.Kind == yaml.MappingNode && i%2 == 0)
		}
	}
	plain(&doc, false)
	return yaml.Marshal(&doc)
}

func yamlToJSON(body []byte) ([]byte, error) {
	var v any
	if err := yaml.NewDecoder(bytes.NewReader(body)).Decode(&v); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, errors.New("empty document")
		}
		return nil, err
	}
	if _, ok := v.(map[string]any); !ok {
		return nil, errNotMapping
	}
	out, err := json.Marshal(v) // a non-string key fails here
	if err != nil {
		return nil, errors.New("keys must be text")
	}
	return append(out, '\n'), nil
}
