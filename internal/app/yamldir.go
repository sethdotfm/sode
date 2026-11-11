package app

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// yamlDir is what the panel, template and Checkpoint set stores share: a mutex, a directory
// ("" means memory only), the records by id, and the file work. File names, id formats and
// which files are skipped stay with each store. Methods ending in Locked need mu held.
type yamlDir[T any] struct {
	mu  sync.Mutex
	dir string
	by  map[string]T

	prefix string           // id prefix, such as "p-"
	idOf   func(*T) *string // the record's id field
	nameOf func(T) string
	pathOf func(dir string, v T) string // the file that holds v
}

func (d *yamlDir[T]) get(id string) (T, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.by[id]
	return v, ok
}

// sorted is the records keep accepts, by lowercase name and then id.
func (d *yamlDir[T]) sorted(keep func(T) bool) []T {
	d.mu.Lock()
	out := make([]T, 0, len(d.by))
	for _, v := range d.by {
		if keep == nil || keep(v) {
			out = append(out, v)
		}
	}
	d.mu.Unlock()
	slices.SortFunc(out, func(a, b T) int {
		if c := strings.Compare(strings.ToLower(d.nameOf(a)), strings.ToLower(d.nameOf(b))); c != 0 {
			return c
		}
		return strings.Compare(*d.idOf(&a), *d.idOf(&b))
	})
	return out
}

// loadDir reads every <id>.yaml in dir and hands accept the path, the id, the bytes and the read
// error. accept parses and decides whether to keep the record. It returns the ReadDir error.
func (d *yamlDir[T]) loadDir(dir string, accept func(path, id string, raw []byte, readErr error) (T, bool)) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		id, ok := strings.CutSuffix(e.Name(), ".yaml")
		if !ok || e.IsDir() {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, rerr := os.ReadFile(path)
		if v, ok := accept(path, id, b, rerr); ok {
			d.by[id] = v
		}
	}
	return nil
}

// newIDLocked picks an id nobody holds.
func (d *yamlDir[T]) newIDLocked() string {
	for {
		id := newShortID(d.prefix)
		if _, taken := d.by[id]; !taken {
			return id
		}
	}
}

// insert checks the cap, then vet, gives v a new id and calls save (which must write the file
// and may change v) before keeping it.
func (d *yamlDir[T]) insert(v T, limit int, full error, vet func(T) error, save func(*T) error) (T, error) {
	var zero T
	d.mu.Lock()
	defer d.mu.Unlock()
	if len(d.by) >= limit {
		return zero, full
	}
	if vet != nil {
		if err := vet(v); err != nil {
			return zero, err
		}
	}
	*d.idOf(&v) = d.newIDLocked()
	if err := save(&v); err != nil {
		return zero, err
	}
	d.by[*d.idOf(&v)] = v
	return v, nil
}

// writeLocked marshals v into its file; memory-only stores write nothing.
func (d *yamlDir[T]) writeLocked(v T) error {
	if d.dir == "" {
		return nil
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	path := d.pathOf(d.dir, v)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return config.AtomicWrite(path, b)
}

// remove deletes the file, then runs also (still under mu) and forgets the record. The bool is
// false when there was no such record.
func (d *yamlDir[T]) remove(id string, also func(T)) (bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	v, ok := d.by[id]
	if !ok {
		return false, nil
	}
	if d.dir != "" {
		if err := os.Remove(d.pathOf(d.dir, v)); err != nil && !os.IsNotExist(err) {
			return true, err
		}
	}
	if also != nil {
		also(v)
	}
	delete(d.by, id)
	return true, nil
}
