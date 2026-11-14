package config

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func unknownCopies(t *testing.T, path string) []string {
	t.Helper()
	m, err := filepath.Glob(path + ".unknown-*")
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestBackupUnknownKeepsDatedCopies(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	first := []byte("router: 10.0.0.1:8910\nfutureKey: 1\n")
	if err := os.WriteFile(p, first, 0o600); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 11, 14, 9, 30, 0, 0, time.UTC)
	if err := BackupUnknown(p, at); err != nil {
		t.Fatal(err)
	}
	c := unknownCopies(t, p)
	if len(c) != 1 || c[0] != p+".unknown-20261114T093000Z" {
		t.Fatalf("copies = %q", c)
	}
	if b, _ := os.ReadFile(c[0]); !bytes.Equal(b, first) {
		t.Fatalf("copy holds %q", b)
	}
	if st, _ := os.Stat(c[0]); st.Mode().Perm() != 0o600 {
		t.Errorf("copy mode %v", st.Mode().Perm())
	}

	// A restart loop adds nothing.
	if err := BackupUnknown(p, at.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if c := unknownCopies(t, p); len(c) != 1 {
		t.Fatalf("same file, copies = %q", c)
	}

	if err := os.WriteFile(p, []byte("router: 10.0.0.2:8910\nfutureKey: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := BackupUnknown(p, at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if c := unknownCopies(t, p); len(c) != 2 {
		t.Fatalf("changed file, copies = %q", c)
	}
}

func TestBackupUnknownNeedsUnknownKeys(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	for _, text := range []string{"", "router: 10.0.0.1:8910\n"} {
		if text != "" {
			if err := os.WriteFile(p, []byte(text), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := BackupUnknown(p, time.Now()); err != nil {
			t.Fatal(err)
		}
		if c := unknownCopies(t, p); len(c) != 0 {
			t.Fatalf("copies = %q", c)
		}
	}
}
