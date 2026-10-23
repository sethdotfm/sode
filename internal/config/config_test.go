package config

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestValidate(t *testing.T) {
	ok := Defaults()
	ok.Router = "10.0.0.5:8910"
	ok.Normalize()
	if err := ok.Validate(); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	for name, mut := range map[string]func(*Config){
		"zero levels":       func(c *Config) { c.Levels = 0 },
		"negative levels":   func(c *Config) { c.Levels = -1 },
		"std matrix 16":     func(c *Config) { c.Matrix = 16 },
		"offset below wire": func(c *Config) { c.LevelWireOffset = -2 },
		"bad router":        func(c *Config) { c.Router = "10.0.0.5" },
		"bad default":       func(c *Config) { c.DefaultLevels = "1-99" },
		"bad name len":      func(c *Config) { c.NameLength = 10 },
		"bad proxy":         func(c *Config) { c.TrustedProxies = []string{"caddy"} },
		"refresh too fast":  func(c *Config) { c.NameRefreshSeconds = 1 },
		"bad colour":        func(c *Config) { c.LevelColors = []string{"#12345g"} },
	} {
		c := ok
		mut(&c)
		if c.Validate() == nil {
			t.Errorf("%s: want error", name)
		}
	}
	ext := ok
	ext.NameRequests, ext.Matrix = "ext", 200
	if err := ext.Validate(); err != nil {
		t.Errorf("ext matrix 200: %v", err)
	}
}

func TestSaveLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if _, exists, err := Load(path); exists || err != nil {
		t.Fatalf("missing file: exists=%v err=%v", exists, err)
	}
	c := Defaults()
	c.Router = "10.0.0.5:8910"
	c.TrustedProxies = []string{"10.0.0.2", "172.18.0.0/16"}
	c.Normalize()
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(path)
	if err != nil || !exists || !reflect.DeepEqual(got, c) {
		t.Fatalf("round trip: %+v exists=%v err=%v", got, exists, err)
	}
	os.WriteFile(path, []byte("router: 10.0.0.5:8910\nlevelz: 3\n"), 0o644)
	if _, _, err := Load(path); err == nil {
		t.Fatal("unknown key should fail")
	}
}

func TestLevelColors(t *testing.T) {
	c := Defaults()
	c.Levels = 3
	c.LevelColors = []string{" #ABC", "", "3A7BD5", "", ""}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if want := []string{"#aabbcc", "", "#3a7bd5"}; !reflect.DeepEqual(c.LevelColors, want) {
		t.Fatalf("normalized %q, want %q", c.LevelColors, want)
	}
	got := []string{}
	for _, l := range c.LevelMap() {
		got = append(got, l.Color)
	}
	if want := []string{"#aabbcc", DefaultLevelColors[1], "#3a7bd5"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("level colours %q, want %q", got, want)
	}
}

func TestParseLevels(t *testing.T) {
	got, err := ParseLevels("3-1, 5,5", 17)
	if err != nil || !reflect.DeepEqual(got, []int{1, 2, 3, 5}) {
		t.Fatalf("got %v %v", got, err)
	}
	for _, bad := range []string{"", "0", "18", "a", "1-"} {
		if _, err := ParseLevels(bad, 17); err == nil {
			t.Errorf("%q: want error", bad)
		}
	}
}
