package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestV2ConfigLoadsUnchanged(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(p, []byte("format: 2\nrouter: 10.0.0.5:8910\nlevels: 4\nbrandText: Studio\n"), 0o600)
	c, exists, err := Load(p)
	if err != nil || !exists {
		t.Fatalf("load: exists=%v err=%v", exists, err)
	}
	if c.Format != Format || c.Router != "10.0.0.5:8910" || c.Levels != 4 || c.BrandText != "Studio" {
		t.Fatalf("got %+v", c)
	}
	if k := c.UnknownKeys(); len(k) != 0 {
		t.Fatalf("format read as unknown: %v", k)
	}
}

func TestV1ConfigIsNeverReadAsV2(t *testing.T) {
	v1 := []byte("router: 10.0.0.5:8910\nlevels: 4\n")
	if _, err := Parse(v1); !errors.Is(err, ErrV1) {
		t.Fatalf("Parse(v1) = %v, want ErrV1", err)
	}
	p := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(p, v1, 0o600)
	if _, _, err := Load(p); !errors.Is(err, ErrV1) {
		t.Fatalf("Load(v1) = %v, want ErrV1", err)
	}
	if v, err := IsV1(p); err != nil || !v {
		t.Fatalf("IsV1 = %v, %v", v, err)
	}
}

func TestIsV1(t *testing.T) {
	dir := t.TempDir()
	for name, tc := range map[string]struct {
		body string
		want bool
	}{
		"missing":  {"", false},
		"v2":       {"format: 2\nrouter: a:1\n", false},
		"newer":    {"format: 9\nrouter: a:1\n", false},
		"v1":       {"router: a:1\n", true},
		"v1 empty": {"# Sode settings\n", true},
	} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-")+".yaml")
		if name != "missing" {
			os.WriteFile(p, []byte(tc.body), 0o600)
		}
		if got, err := IsV1(p); err != nil || got != tc.want {
			t.Errorf("%s: IsV1 = %v, %v; want %v", name, got, err, tc.want)
		}
	}
}

// A file from a later format must not load, and must not count as v1 (that would move it away).
func TestNewerFormatRefused(t *testing.T) {
	_, err := Parse([]byte("format: 3\nrouter: 10.0.0.5:8910\n"))
	if err == nil || !strings.Contains(err.Error(), "newer Sode") {
		t.Fatalf("Parse(format 3) = %v", err)
	}
}

func TestSaveWritesFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	c := Defaults()
	c.Router = "10.0.0.5:8910"
	c.Format = 0
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "\nformat: 2\n") {
		t.Fatalf("saved file has no format line:\n%s", b)
	}
	if _, _, err := Load(p); err != nil {
		t.Fatalf("reload: %v", err)
	}
}
