package app

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sethdotfm/sode/internal/config"
)

// v1Names: what v1.8 wrote beside the config, with the image's log files. It is v1's list, not
// this build's: never rename or drop an entry when a v2 store or env name changes.
var v1Names = []string{
	"tokens.json", "keys.json", "keys-ended.json",
	"panels", "panelsets", "templates", "panelwatch.yaml",
	"icons.yaml", "icons", "artdefaults.yaml", "artdefaults.art",
	"history.jsonl", "sent.jsonl", "uptime.jsonl",
}

// MoveV1Files moves a v1 install's files into a v1 directory beside the config, so v2 starts as a
// fresh install. It does nothing unless cfgPath holds a config with no format marker. logs are
// the History, Communication Log and uptime paths (empty ones are skipped). It returns the
// directory, or "" when nothing moved. Only names Sode writes move: the config's directory can hold
// other files when SODE_CONFIG is not /data/config.yaml.
func MoveV1Files(cfgPath string, logs ...string) (string, error) {
	if cfgPath == "" {
		return "", nil
	}
	cfgPath = filepath.Clean(cfgPath)
	v1, err := config.IsV1(cfgPath)
	if err != nil || !v1 {
		return "", err
	}
	dir := filepath.Dir(cfgPath)
	dst := filepath.Join(dir, "v1")
	for n := 2; ; n++ {
		if _, err := os.Lstat(dst); os.IsNotExist(err) {
			break
		}
		dst = filepath.Join(dir, fmt.Sprintf("v1-%d", n))
	}
	byDir := map[string][]string{dir: append(slices.Clone(v1Names), filepath.Base(cfgPath))}
	for _, p := range logs {
		if p != "" {
			byDir[filepath.Dir(p)] = append(byDir[filepath.Dir(p)], filepath.Base(p))
		}
	}
	var moves []string
	for d, names := range byDir {
		ents, err := os.ReadDir(d)
		if err != nil && !os.IsNotExist(err) {
			return "", err
		}
		for _, e := range ents {
			if p := filepath.Join(d, e.Name()); p != cfgPath && sodeFile(e.Name(), names) {
				moves = append(moves, p)
			}
		}
	}
	// The config goes last: a move cut short runs again on the next start.
	moves = append(moves, cfgPath)
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return "", err
	}
	for _, p := range moves {
		if err := os.Rename(p, filepath.Join(dst, filepath.Base(p))); err != nil {
			return "", fmt.Errorf("moving the v1 files to %s: %w", dst, err)
		}
	}
	log.Printf("config: %s is a Sode v1 configuration; moved %d v1 files to %s, so Sode starts as a fresh install", cfgPath, len(moves), dst)
	return dst, nil
}

// sodeFile matches a name and its side files: name.prev, name.tmp, sent.1.jsonl.
func sodeFile(entry string, names []string) bool {
	for _, n := range names {
		stem := strings.TrimSuffix(n, filepath.Ext(n))
		if entry == n || strings.HasPrefix(entry, n+".") || strings.HasPrefix(entry, stem+".") {
			return true
		}
	}
	return false
}
