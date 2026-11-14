package app

import (
	"bytes"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func fixDamagedCopies(t *testing.T, dir, name string) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(dir, name+".damaged-*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestStartupSetsDamagedKeyAndTokenFilesAside(t *testing.T) {
	a := userApp(t, false)
	dir := filepath.Dir(a.cfgPath)
	garbage := map[string][]byte{
		"keys.json":       []byte(`[{"hash": "abc", broken`),
		"keys-ended.json": []byte(`not json at all`),
		"tokens.json":     []byte(`{"x": {`),
	}
	for name, b := range garbage {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)

	b := fixRestart(t, a, func(*config.Config) {})
	if n := len(b.keys.list()); n != 0 {
		t.Fatalf("%d keys loaded from a damaged file", n)
	}
	if n := len(b.tokens.toks); n != 0 {
		t.Fatalf("%d tokens loaded from a damaged file", n)
	}
	for name, want := range garbage {
		copies := fixDamagedCopies(t, dir, name)
		if len(copies) != 1 {
			t.Fatalf("%s: copies %v, want one", name, copies)
		}
		got, err := os.ReadFile(copies[0])
		if err != nil || !bytes.Equal(got, want) {
			t.Fatalf("%s: the copy holds %q (%v), want the original bytes", name, got, err)
		}
		if strings.Contains(filepath.Base(copies[0]), ":") {
			t.Fatalf("%s: a colon in the copy's name", copies[0])
		}
		if !strings.Contains(buf.String(), filepath.Base(copies[0])) {
			t.Fatalf("the log does not name %s: %s", copies[0], buf.String())
		}
	}
}

func TestStartupLeavesValidKeyAndTokenFilesInPlace(t *testing.T) {
	a := userApp(t, false)
	secret := fixOwnKey(t, a)
	a.keys.flush()
	dir := filepath.Dir(a.cfgPath)
	if err := os.WriteFile(filepath.Join(dir, "tokens.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b := fixRestart(t, a, func(*config.Config) {})
	if _, _, ok := b.keys.find(secret, time.Now()); !ok {
		t.Fatal("a key from a valid keys.json was lost")
	}
	for _, name := range []string{"keys.json", "tokens.json"} {
		if c := fixDamagedCopies(t, dir, name); len(c) != 0 {
			t.Fatalf("a valid %s was set aside: %v", name, c)
		}
	}
}
