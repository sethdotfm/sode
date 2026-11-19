package config

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Format is config.yaml's top-level `format` key. Raise it only when an older loader would
// misread the file; a file with settings but without the key is from v1 (docs/agents/server.md, config load).
const Format = 2

var ErrV1 = errors.New("the file is from Sode v1, and Sode v2 does not read v1 files")

// formatOf reads only the marker, so Defaults cannot fill it in. 0 means no key.
func formatOf(b []byte) (int, error) {
	var head struct {
		Format *int `yaml:"format"`
	}
	if err := yaml.Unmarshal(b, &head); err != nil {
		return 0, err
	}
	if head.Format == nil {
		return 0, nil
	}
	return *head.Format, nil
}

// noKeys: empty, comments only, null or {}. v1 always wrote its settings, so such a file is not v1.
func noKeys(b []byte) bool {
	var m map[string]any
	return yaml.Unmarshal(b, &m) == nil && len(m) == 0
}

func checkFormat(b []byte) error {
	f, err := formatOf(b)
	if err != nil {
		return err
	}
	switch {
	case f == 0 && noKeys(b):
		return errors.New("the file has no settings")
	case f == 0:
		return ErrV1
	case f > Format:
		return fmt.Errorf("made by a newer Sode (format %d; this release reads format %d)", f, Format)
	case f != Format:
		return fmt.Errorf("unknown format %d", f)
	}
	return nil
}

// A missing or unparsable file is not v1.
func IsV1(path string) (bool, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	f, err := formatOf(b)
	return err == nil && f == 0 && !noKeys(b), nil
}
