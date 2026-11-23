package app

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBlobDirStaysInsideItsFolder(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	b := blobDir{dir: filepath.Join(root, "blobs")}
	for _, name := range []string{"../outside", "a/../../outside", "/etc/passwd", ""} {
		if err := b.put(name, []byte("x")); err == nil {
			t.Errorf("put(%q) wrote outside the folder", name)
		}
		if _, ok := b.get(name); ok {
			t.Errorf("get(%q) read outside the folder", name)
		}
		b.del(name)
		b.delPrefix(name)
	}
	if got, err := os.ReadFile(outside); err != nil || string(got) != "keep" {
		t.Fatalf("file outside the folder changed: %q, %v", got, err)
	}
	if err := b.put("p.art/a-0000000f", []byte("ok")); err != nil {
		t.Fatal(err)
	}
	if got, ok := b.get("p.art/a-0000000f"); !ok || string(got) != "ok" {
		t.Fatalf("a nested name does not round-trip: %q, %v", got, ok)
	}
}
