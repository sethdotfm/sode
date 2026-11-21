package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/sethdotfm/sode/internal/config"
	"gopkg.in/yaml.v3"
)

// blobDir holds named bytes as files under dir, or in memory when dir is "". Names use "/" and can nest. Ids,
// indexes, caps and lifetimes are each store's own; this is only the bytes.
type blobDir struct {
	dir string
	mu  sync.Mutex // guards mem only: the inbox writes art outside its store lock
	mem map[string][]byte
}

var errBlobName = errors.New("blob name leaves its folder")

// Callers check their ids already; this keeps a missed check from reaching a file outside dir.
func (b *blobDir) path(name string) (string, bool) {
	name = filepath.FromSlash(name)
	if !filepath.IsLocal(name) {
		return "", false
	}
	return filepath.Join(b.dir, name), true
}

func (b *blobDir) put(name string, data []byte) error {
	if b.dir == "" {
		b.mu.Lock()
		defer b.mu.Unlock()
		if b.mem == nil {
			b.mem = map[string][]byte{}
		}
		b.mem[name] = data
		return nil
	}
	p, ok := b.path(name)
	if !ok {
		return errBlobName
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	return config.AtomicWrite(p, data)
}

func (b *blobDir) get(name string) ([]byte, bool) {
	if b.dir == "" {
		b.mu.Lock()
		defer b.mu.Unlock()
		d, ok := b.mem[name]
		return d, ok
	}
	p, ok := b.path(name)
	if !ok {
		return nil, false
	}
	d, err := os.ReadFile(p)
	return d, err == nil
}

// A name that is not there counts as deleted.
func (b *blobDir) del(name string) error {
	if b.dir == "" {
		b.mu.Lock()
		defer b.mu.Unlock()
		delete(b.mem, name)
		return nil
	}
	p, ok := b.path(name)
	if !ok {
		return errBlobName
	}
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// delPrefix removes the folder prefix and every name under it.
func (b *blobDir) delPrefix(prefix string) {
	if b.dir == "" {
		b.mu.Lock()
		defer b.mu.Unlock()
		for k := range b.mem {
			if strings.HasPrefix(k, prefix+"/") {
				delete(b.mem, k)
			}
		}
		return
	}
	if p, ok := b.path(prefix); ok {
		os.RemoveAll(p)
	}
}

// "" is memory only and writes nothing.
func writeYAML(path string, v any) error {
	if path == "" {
		return nil
	}
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	return config.AtomicWrite(path, b)
}
