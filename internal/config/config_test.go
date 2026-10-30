package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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
		"bad color":         func(c *Config) { c.LevelColors = []string{"#12345g"} },
		"bad numbering":     func(c *Config) { c.LevelNumbering = "2" },
		"bad router show":   func(c *Config) { c.RouterShow = "ip" },
		"long router name":  func(c *Config) { c.RouterName = strings.Repeat("x", 33) },
		"long brand text":   func(c *Config) { c.BrandText = strings.Repeat("x", 25) },
		"bad brand border":  func(c *Config) { c.BrandBorder = "red" },
		"pin tries < 0":     func(c *Config) { c.PinMaxTries = -1 },
		"pin tries > 1000":  func(c *Config) { c.PinMaxTries = 1001 },
		"long hidden dsts":  func(c *Config) { c.HiddenDsts = strings.Repeat("x", 501) },
		"bad theme":         func(c *Config) { c.DeviceDefaults.Theme = "blue" },
		"bad layout":        func(c *Config) { c.DeviceDefaults.Layout = "tablet" },
		"bad clock":         func(c *Config) { c.DeviceDefaults.Clock = "13" },
		"bad set stack":     func(c *Config) { c.DeviceDefaults.SetStack = "xor" },
		"bad next":          func(c *Config) { c.DeviceDefaults.NextMode = "random" },
		"unnamed set":       func(c *Config) { c.FilterSets = []FilterSet{{Sode: true}} },
		"same set names":    func(c *Config) { c.FilterSets = []FilterSet{{Name: "A"}, {Name: "a"}} },
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
	c.LevelNumbering, c.PinMaxTries = "va", 3
	c.HiddenDsts = "MV, !1-5"
	c.FilterSets = []FilterSet{{Name: "All except shader", Dst: "!MON", Sode: true, Ext: true}, {Name: "Panels", Ext: true, Hidden: true}}
	c.SettingsPin, _ = HashPin("1234")
	c.Normalize()
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(path)
	if err != nil || !exists || !reflect.DeepEqual(got, c) {
		t.Fatalf("round trip: %+v exists=%v err=%v", got, exists, err)
	}
	// Files from before levelTypedBase was dropped still load.
	os.WriteFile(path, []byte("router: 10.0.0.5:8910\nlevelTypedBase: 1\n"), 0o644)
	if _, _, err := Load(path); err != nil {
		t.Fatalf("legacy levelTypedBase: %v", err)
	}
	// So do files from the first hidden-destinations beta.
	os.WriteFile(path, []byte("router: 10.0.0.5:8910\nhideDsts: [3, 64]\n"), 0o644)
	if _, _, err := Load(path); err != nil {
		t.Fatalf("legacy hideDsts: %v", err)
	}
	os.WriteFile(path, []byte("router: 10.0.0.5:8910\nlevelz: 3\n"), 0o644)
	if _, _, err := Load(path); err == nil {
		t.Fatal("unknown key should fail")
	}
}

func TestDeviceDefaults(t *testing.T) {
	// Files from before the block load with today's behavior.
	path := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(path, []byte("router: 10.0.0.5:8910\n"), 0o644)
	c, _, err := Load(path)
	want := DeviceDefaults{Theme: "system", Layout: "auto", Clock: "24", ScrollTop: true, SetStack: "and", NextMode: "number"}
	if err != nil || c.DeviceDefaults != want {
		t.Fatalf("old file: %+v %v", c.DeviceDefaults, err)
	}
	// A partial block keeps the rest, and values are normalized.
	os.WriteFile(path, []byte("router: 10.0.0.5:8910\ndeviceDefaults:\n  theme: ' Dark '\n  flip: true\n"), 0o644)
	c, _, err = Load(path)
	want = DeviceDefaults{Theme: "dark", Layout: "auto", Clock: "24", Flip: true, ScrollTop: true, SetStack: "and", NextMode: "number"}
	if err != nil || c.DeviceDefaults != want {
		t.Fatalf("partial block: %+v %v", c.DeviceDefaults, err)
	}
	// Blank strings (a browser that doesn't know the block) become the defaults.
	c.DeviceDefaults = DeviceDefaults{}
	c.Normalize()
	if err := c.Validate(); err != nil || c.DeviceDefaults.Theme != "system" || c.DeviceDefaults.Clock != "24" {
		t.Fatalf("blank block: %+v %v", c.DeviceDefaults, err)
	}
	// Browsers get it as JSON, with booleans for the on/off ones.
	b, _ := json.Marshal(Defaults())
	if !strings.Contains(string(b), `"deviceDefaults":{"theme":"system","layout":"auto","clock":"24","flip":false,"scrollTop":true,"searchFilters":false,"setStack":"and","nextMode":"number"}`) {
		t.Fatalf("json: %s", b)
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
		t.Fatalf("level colors %q, want %q", got, want)
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

func TestLevelLabels(t *testing.T) {
	c := Defaults()
	c.Levels = 3
	for mode, want := range map[string][]string{"1": {"1", "2", "3"}, "0": {"0", "1", "2"}, "va": {"V", "1", "2"}} {
		c.LevelNumbering = mode
		got := []string{}
		for _, l := range c.LevelMap() {
			got = append(got, l.Label)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: labels %q, want %q", mode, got, want)
		}
	}
}

func TestPin(t *testing.T) {
	h, err := HashPin("4321")
	if err != nil {
		t.Fatal(err)
	}
	if !CheckPin(h, "4321") || CheckPin(h, "4322") || CheckPin("", "4321") || CheckPin("x$1$a$b", "4321") {
		t.Fatal("CheckPin")
	}
	if !ValidPinHash(h) || ValidPinHash("1234") || ValidPinHash("") {
		t.Fatal("ValidPinHash")
	}
	if CheckPinRules("123") == nil || CheckPinRules("1234") != nil {
		t.Fatal("CheckPinRules")
	}
	c := Defaults()
	c.SettingsPin = h
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "pbkdf2") {
		t.Fatal("PIN hash leaks into JSON")
	}
}
