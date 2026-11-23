package config

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestV2ConfigLoadsUnchanged(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(p, []byte("format: 2\nrouters: [{address: 10.0.0.5:8910, levels: 4}]\nbrandText: Studio\n"), 0o600)
	c, exists, err := Load(p)
	if err != nil || !exists {
		t.Fatalf("load: exists=%v err=%v", exists, err)
	}
	if c.Format != Format || c.Address != "10.0.0.5:8910" || c.Levels != 4 || c.BrandText != "Studio" {
		t.Fatalf("got %+v", c)
	}
	if k := c.UnknownKeys(); len(k) != 0 {
		t.Fatalf("format read as unknown: %v", k)
	}
}

// A file with no keys (touch config.yaml) is a new install, not v1.
func TestEmptyConfigLoadsAsNew(t *testing.T) {
	for _, body := range []string{"", "# x\n", "{}\n", "~\n"} {
		p := filepath.Join(t.TempDir(), "config.yaml")
		os.WriteFile(p, []byte(body), 0o600)
		if _, exists, err := Load(p); err != nil || exists {
			t.Errorf("%q: exists=%v err=%v", body, exists, err)
		}
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
		"v2":       {"format: 2\nrouters: [{address: a:1}]\n", false},
		"newer":    {"format: 9\nrouters: [{address: a:1}]\n", false},
		"v1":       {"router: a:1\n", true},
		"comments": {"# Sode settings\n", false},
		"empty":    {"\n", false},
		"braces":   {"{}\n", false},
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
	_, err := Parse([]byte("format: 3\nrouters: [{address: 10.0.0.5:8910}]\n"))
	if err == nil || !strings.Contains(err.Error(), "newer Sode") {
		t.Fatalf("Parse(format 3) = %v", err)
	}
}

func TestSaveWritesFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config.yaml")
	c := Defaults()
	c.Address = "10.0.0.5:8910"
	c.Format = 0
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	if !strings.Contains(string(b), "\nformat: 2\n") {
		t.Fatalf("saved file has no format line:\n%s", b)
	}
	if strings.Index(string(b), "format:") > strings.Index(string(b), "routers:") {
		t.Fatalf("format is not the first key:\n%s", b)
	}
	if _, _, err := Load(p); err != nil {
		t.Fatalf("reload: %v", err)
	}
}

// v1's read-only keys load as unknown keys; `salvos` is not a preset list.
func TestV1LegacyKeysAreUnknownInV2(t *testing.T) {
	c, err := Parse([]byte(`format: 2
routers: [{address: 10.0.0.5:8910}]
salvos:
  - name: old
    items: [{src: 1, dsts: [2]}]
grids: [1]
gridConfirm: true
hideDsts: [3]
levelTypedBase: 1
locks:
  grid: true
`))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"gridConfirm", "grids", "hideDsts", "levelTypedBase", "locks.grid", "salvos"}
	if got := c.UnknownKeys(); !slices.Equal(got, want) {
		t.Fatalf("UnknownKeys = %q, want %q", got, want)
	}
	if len(c.Presets) != 0 {
		t.Fatalf("salvos loaded as presets: %+v", c.Presets)
	}
}

// The key loads as nothing and no later write carries it.
func TestSettingsPinIsDropped(t *testing.T) {
	c, err := Parse([]byte("format: 2\nrouters: [{address: 10.0.0.5:8910}]\nsettingsPin: pbkdf2$1$x$y\nfuture: 1\n"))
	if err != nil {
		t.Fatal(err)
	}
	if got := c.UnknownKeys(); !slices.Equal(got, []string{"future"}) {
		t.Fatalf("UnknownKeys = %q", got)
	}
	p := filepath.Join(t.TempDir(), "config.yaml")
	if err := Save(p, c); err != nil {
		t.Fatal(err)
	}
	if b, _ := os.ReadFile(p); strings.Contains(string(b), "settingsPin") {
		t.Fatalf("a write carries settingsPin:\n%s", b)
	}
}

// Bookmarks replaced preset pins with no migration (v2 is a clean install). A file from before still loads: the
// preset and the profile keep everything else, and the pin keys are only unknown keys.
func TestPresetPinsFromAnOldFileLoad(t *testing.T) {
	c, err := Parse([]byte(`format: 2
routers: [{uid: r1, address: 127.0.0.1:8910}]
users:
  - name: a
    profile:
      pinOrder: [a1]
presets:
  - id: a1
    name: Morning
    pinned: true
    items:
      - src: {router: r1, n: 1}
        dsts: [{router: r1, n: 2}]
`))
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Presets) != 1 || c.Presets[0].Name != "Morning" || len(c.Presets[0].Items) != 1 {
		t.Fatalf("presets %+v", c.Presets)
	}
	want := []string{"presets[0].pinned", "users[0].profile.pinOrder"}
	if got := c.UnknownKeys(); !slices.Equal(got, want) {
		t.Fatalf("UnknownKeys = %q, want %q", got, want)
	}
}
