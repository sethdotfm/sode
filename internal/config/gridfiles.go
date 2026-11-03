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
	"strings"

	"gopkg.in/yaml.v3"
)

// Each panel lives in its own file, grids/<link>.yml in a directory beside
// the config file (/data/grids in the image), holding one Grid. They are read
// at startup and written by the grid editor; hand edits apply on restart.
// Without a config file there is no directory and panels live in memory.

const gridExt = ".yml"

// GridsDir is the directory of the panel files for the config file at
// cfgPath; "" (no config file) means panels aren't stored.
func GridsDir(cfgPath string) string {
	if cfgPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfgPath), "grids")
}

// gridPath is the file of link in dir. The link is checked, so nothing can
// point outside dir.
func gridPath(dir, link string) (string, error) {
	if !ValidLink(link) {
		return "", fmt.Errorf("%q is not a grid link", link)
	}
	return filepath.Join(dir, link+gridExt), nil
}

// SaveGrid writes g to its file in dir (created if missing), atomically.
func SaveGrid(dir string, g Grid) error {
	path, err := gridPath(dir, g.Link)
	if err != nil {
		return err
	}
	b, err := yaml.Marshal(g)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return atomicWrite(path, b)
}

// DeleteGrid removes link's file from dir; a file already gone is fine.
func DeleteGrid(dir, link string) error {
	path, err := gridPath(dir, link)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

// SyncGrids makes dir match next, given the panels it held before: it writes
// the panels that are new or changed and removes the ones that are gone.
func SyncGrids(dir string, prev, next []Grid) error {
	old := map[string]Grid{}
	for _, g := range prev {
		old[g.Link] = g
	}
	for _, g := range next {
		if o, ok := old[g.Link]; ok && gridEqual(o, g) {
			continue
		}
		if err := SaveGrid(dir, g); err != nil {
			return fmt.Errorf("saving grid %q: %w", g.Link, err)
		}
	}
	for _, o := range prev {
		if !slices.ContainsFunc(next, func(g Grid) bool { return g.Link == o.Link }) {
			if err := DeleteGrid(dir, o.Link); err != nil {
				return fmt.Errorf("removing grid %q: %w", o.Link, err)
			}
		}
	}
	return nil
}

func gridEqual(a, b Grid) bool {
	ya, _ := yaml.Marshal(a)
	yb, _ := yaml.Marshal(b)
	return bytes.Equal(ya, yb)
}

// LoadGridFiles reads the *.yml files of dir, sorted by link. A missing
// directory is no panels. A file that is damaged, invalid or not named for
// its link is an error naming the file; files of other kinds are ignored.
func LoadGridFiles(dir string) ([]Grid, error) {
	ents, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []Grid{}, nil
	}
	if err != nil {
		return nil, err
	}
	grids := []Grid{}
	for _, e := range ents { // ReadDir sorts by file name, so by link
		name := e.Name()
		stem, ok := strings.CutSuffix(name, gridExt)
		if !ok || e.IsDir() || strings.HasPrefix(name, ".") {
			continue
		}
		path := filepath.Join(dir, name)
		fail := func(err error) ([]Grid, error) { return nil, fmt.Errorf("grid file %s: %w", path, err) }
		if !ValidLink(stem) {
			return fail(errors.New("the file name must be the grid's link: lowercase letters, digits and dashes, then " + gridExt))
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fail(err)
		}
		var g Grid
		dec := yaml.NewDecoder(bytes.NewReader(b))
		dec.KnownFields(true)
		if err := dec.Decode(&g); err != nil && !errors.Is(err, io.EOF) {
			return fail(err)
		}
		normalizeGrid(&g)
		if g.Link != stem {
			return fail(fmt.Errorf("link is %q but the file is named %q", g.Link, stem))
		}
		if err := errors.Join(ValidateGrid(g)...); err != nil {
			return fail(err)
		}
		grids = append(grids, g)
	}
	return grids, nil
}

// LoadGrids fills c.Grids for the config file at cfgPath, as startup does.
// First it moves any `grids:` list still in the config file (from before
// panels had files) into the directory, once: a panel whose file already
// exists keeps that file, and the config file is rewritten without the list.
// Then it reads the directory and validates the result. With no cfgPath c's
// panels stay as they are.
func (c *Config) LoadGrids(cfgPath string) error {
	dir := GridsDir(cfgPath)
	if dir == "" {
		return nil
	}
	if len(c.LegacyGrids) > 0 {
		old := cloneGrids(c.LegacyGrids)
		for i := range old {
			normalizeGrid(&old[i])
			if err := errors.Join(ValidateGrid(old[i])...); err != nil {
				return fmt.Errorf("%s: grids: %w", cfgPath, err)
			}
		}
		for _, g := range old {
			path, _ := gridPath(dir, g.Link)
			if _, err := os.Stat(path); err == nil {
				log.Printf("config: grid %q is in %s and in %s; keeping the file", g.Link, cfgPath, path)
				continue
			}
			if err := SaveGrid(dir, g); err != nil {
				return fmt.Errorf("moving grid %q to %s: %w", g.Link, dir, err)
			}
		}
		moved := c.Clone()
		moved.LegacyGrids = nil
		moved.Grids = nil
		if err := Save(cfgPath, moved); err != nil {
			return fmt.Errorf("rewriting %s: %w", cfgPath, err)
		}
		log.Printf("config: moved %d grids from %s to %s", len(old), cfgPath, dir)
		c.LegacyGrids = nil
	}
	grids, err := LoadGridFiles(dir)
	if err != nil {
		return err
	}
	c.Grids = grids
	return errors.Join(c.validateGrids()...)
}
