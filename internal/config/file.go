package config

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"slices"
	"time"

	"gopkg.in/yaml.v3"
)

// Made ids are written back so the file keeps them; callers back the file up before Load.
func Load(path string) (c Config, exists bool, err error) {
	c = Defaults()
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return c, false, nil
	}
	if err != nil {
		return c, false, err
	}
	if noKeys(b) {
		return c, false, nil
	}
	c, err = Parse(b)
	if err != nil {
		return c, true, fmt.Errorf("%s: %w", path, err)
	}
	if err := c.Validate(); err != nil {
		return c, true, fmt.Errorf("%s: %w", path, err)
	}
	if !bytes.Contains(b, []byte(c.UID)) || !bytes.Contains(b, []byte(c.InstallUID)) || c.idsFilled || !hasPresetIDs(b, c) {
		if err := Save(path, c); err != nil {
			return c, true, fmt.Errorf("%s: keeping the new install UID, router UID, user ids, preset ids, filter set ids or tie line ids: %w", path, err)
		}
	}
	return c, true, nil
}

// Pins, bookmarks and other items name a preset, a filter set or a tie line by its id. So Load writes back each id that
// it made.
func hasPresetIDs(b []byte, c Config) bool {
	var ids []string
	add := func(ps []Preset, fs []FilterSet) {
		for _, p := range ps {
			ids = append(ids, p.ID)
		}
		for _, f := range fs {
			ids = append(ids, f.ID)
		}
	}
	add(c.Presets, c.FilterSets)
	for _, t := range c.TieLines {
		ids = append(ids, t.ID)
	}
	for _, u := range c.Users {
		add(u.Profile.Presets, u.Profile.FilterSets)
	}
	return !slices.ContainsFunc(ids, func(id string) bool { return !bytes.Contains(b, []byte(id)) })
}

// Parse is Load's decoder (also config import). Unknown keys are kept (see UnknownKeys) and legacy
// keys load. It does not Validate, so a caller can merge PIN hashes first.
func Parse(b []byte) (Config, error) {
	c := Defaults()
	if err := checkFormat(b); err != nil {
		return c, err
	}
	dec := yaml.NewDecoder(bytes.NewReader(b))
	dec.KnownFields(true)
	if err := dec.Decode(&c); err != nil && !errors.Is(err, io.EOF) {
		return c, err
	}
	if err := c.noRouterProblem(); err != nil {
		return c, err
	}
	c.Normalize()
	return c, nil
}

const header = "# Sode settings. Edited by the settings dialog; hand edits apply on restart.\n"

func Save(path string, c Config) error {
	c.Format = Format
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	if err := copyFile(path, path+".prev"); err != nil {
		log.Printf("config: keeping %s.prev: %v", path, err)
	}
	return AtomicWrite(path, []byte(header), b)
}

// BackupOnce keeps the file as it was before this release touched it, so a rollback can restore it.
// The backup holds PIN hashes, so AtomicWrite's private mode matters.
func BackupOnce(path, label string) error {
	dst := path + ".before-" + label
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	return copyFile(path, dst)
}

// After a rollback .before-<version> often exists already and .prev lasts
// one save. A copy equal to the newest one is skipped, so a restart loop does not fill the disk.
func BackupUnknown(path string, now time.Time) error {
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	c, err := Parse(b)
	if err != nil || len(c.UnknownKeys()) == 0 {
		return err
	}
	older, err := filepath.Glob(path + ".unknown-*")
	if err != nil {
		return err
	}
	if len(older) > 0 {
		if last, err := os.ReadFile(slices.Max(older)); err == nil && bytes.Equal(last, b) {
			return nil
		}
	}
	return AtomicWrite(path+".unknown-"+now.UTC().Format("20060102T150405Z"), b)
}

// copyFile treats a missing source as nothing to copy.
func copyFile(src, dst string) error {
	b, err := os.ReadFile(src)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return AtomicWrite(dst, b)
}

// AtomicWrite: private temp file, fsync, rename, dir sync; used for config and the session token file.
// syncWrites is false only under UseTestCosts.
var syncWrites = true

// History's rewrite follows it too.
func SyncWrites() bool { return syncWrites }

func AtomicWrite(path string, parts ...[]byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".sode-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	for _, p := range parts {
		if _, err = tmp.Write(p); err != nil {
			break
		}
	}
	if err == nil && syncWrites {
		err = tmp.Sync()
	}
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	if !syncWrites {
		return nil
	}
	if d, err := os.Open(filepath.Dir(path)); err == nil {
		d.Sync()
		d.Close()
	}
	return nil
}
