package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	ok := Defaults()
	ok.Routers[0].Address = "10.0.0.5:8910"
	ok.Normalize()
	if err := ok.Validate(); err != nil {
		t.Fatalf("defaults: %v", err)
	}
	for name, mut := range map[string]func(*Config){
		"zero levels":       func(c *Config) { c.Routers[0].Levels = 0 },
		"negative levels":   func(c *Config) { c.Routers[0].Levels = -1 },
		"std matrix 16":     func(c *Config) { c.Routers[0].Matrix = 16 },
		"offset below wire": func(c *Config) { c.Routers[0].LevelWireOffset = -2 },
		"bad router":        func(c *Config) { c.Routers[0].Address = "10.0.0.5" },
		"bad default":       func(c *Config) { c.Routers[0].DefaultLevels = "1-99" },
		"bad name len":      func(c *Config) { c.Routers[0].NameLength = 10 },
		"bad proxy":         func(c *Config) { c.TrustedProxies = []string{"caddy"} },
		"refresh too fast":  func(c *Config) { c.Routers[0].NameRefreshSeconds = 1 },
		"bad color":         func(c *Config) { c.LevelColors = []string{"#12345g"} },
		"bad numbering":     func(c *Config) { c.Routers[0].LevelNumbering = "2" },
		"bad router show":   func(c *Config) { c.Routers[0].RouterShow = "ip" },
		"long router name":  func(c *Config) { c.Routers[0].Title = strings.Repeat("x", 33) },
		"long brand text":   func(c *Config) { c.BrandText = strings.Repeat("x", 25) },
		"bad brand border":  func(c *Config) { c.BrandBorder = "red" },
		"long hidden dsts":  func(c *Config) { c.HiddenDsts = strings.Repeat("x", 501) },
		"bad theme":         func(c *Config) { c.DeviceDefaults.Theme = "blue" },
		"bad layout":        func(c *Config) { c.DeviceDefaults.Layout = "tablet" },
		"bad clock":         func(c *Config) { c.DeviceDefaults.Clock = "13" },
		"bad set stack":     func(c *Config) { c.DeviceDefaults.SetStack = "xor" },
		"bad next":          func(c *Config) { c.DeviceDefaults.NextMode = "random" },
		"bad stack gap":     func(c *Config) { c.DeviceDefaults.StackGap = "45" },
		"letter palette":    func(c *Config) { c.DeviceDefaults.PaletteChar = "p" },
		"plus palette":      func(c *Config) { c.DeviceDefaults.PaletteChar = "+" },
		"unnamed set":       func(c *Config) { c.FilterSets = []FilterSet{{Sode: true}} },
		"same set names":    func(c *Config) { c.FilterSets = []FilterSet{{Name: "A"}, {Name: "a"}} },
		"bad fire mode":     func(c *Config) { c.Routers[0].FireMode = "parallel" },
		"salvo group < 0":   func(c *Config) { c.Routers[0].SalvoGroup = -1 },
		"salvo group > 127": func(c *Config) { c.Routers[0].SalvoGroup = 128 },
		"native matrix 16": func(c *Config) {
			c.Routers[0].FireMode, c.Routers[0].NameRequests, c.Routers[0].Matrix = "native", "ext", 16
		},
	} {
		c := ok.Clone()
		mut(&c)
		if c.Validate() == nil {
			t.Errorf("%s: want error", name)
		}
	}
	native := ok.Clone()
	native.Routers[0].FireMode, native.Routers[0].SalvoGroup = "native", 0
	if err := native.Validate(); err != nil {
		t.Errorf("native, group 0: %v", err)
	}
	if ok.Routers[0].FireMode != "sequential" || ok.Routers[0].SalvoGroup != 127 {
		t.Errorf("fire defaults %q %d", ok.Routers[0].FireMode, ok.Routers[0].SalvoGroup)
	}
	blank := ok.Clone()
	blank.Routers[0].FireMode = " Native "
	blank.Normalize()
	if blank.Routers[0].FireMode != "native" {
		t.Errorf("normalized fire mode %q", blank.Routers[0].FireMode)
	}
	blank.Routers[0].FireMode = ""
	blank.Normalize()
	if blank.Routers[0].FireMode != "sequential" {
		t.Errorf("empty fire mode %q", blank.Routers[0].FireMode)
	}
	ext := ok.Clone()
	ext.Routers[0].NameRequests, ext.Routers[0].Matrix = "ext", 200
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
	c.Routers[0].Address = "10.0.0.5:8910"
	c.TrustedProxies = []string{"10.0.0.2", "172.18.0.0/16"}
	c.Routers[0].LevelNumbering, c.PinWaits = "va", false
	c.HiddenDsts = "MV, !1-5"
	c.FilterSets = []FilterSet{{Name: "All except shader", Dst: "!MON", Sode: true, Ext: true}, {Name: "Panels", Ext: true, Hidden: true}}
	c.Normalize()
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, exists, err := Load(path)
	if err != nil || !exists || !reflect.DeepEqual(got, c) {
		t.Fatalf("round trip: %+v exists=%v err=%v", got, exists, err)
	}
	os.WriteFile(path, []byte("format: 2\nrouters: [{address: 10.0.0.5:8910}]\nlevelz: 3\n"), 0o644)
	if got, _, err := Load(path); err != nil || !slices.Equal(got.UnknownKeys(), []string{"levelz"}) || got.Extra["levelz"] != 3 || !got.PinWaits {
		t.Fatalf("unknown key: %v %v", err, got.Extra)
	}
}

func TestDeviceDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	os.WriteFile(path, []byte("format: 2\nrouters: [{address: 10.0.0.5:8910}]\n"), 0o644)
	c, _, err := Load(path)
	want := DeviceDefaults{Theme: "system", Layout: "auto", Clock: "24", ScrollTop: true, SetStack: "and", NextMode: "number", StackGap: "30", PaletteChar: ">"}
	if err != nil || !reflect.DeepEqual(c.DeviceDefaults, want) {
		t.Fatalf("old file: %+v %v", c.DeviceDefaults, err)
	}
	os.WriteFile(path, []byte("format: 2\nrouters: [{address: 10.0.0.5:8910}]\nbrowserDefaults:\n  theme: ' Dark '\n  flip: true\n  compact: true\n"), 0o644)
	c, _, err = Load(path)
	want = DeviceDefaults{Theme: "dark", Layout: "auto", Clock: "24", Flip: true, ScrollTop: true, SetStack: "and", NextMode: "number", StackGap: "30", Compact: true, PaletteChar: ">"}
	if err != nil || !reflect.DeepEqual(c.DeviceDefaults, want) {
		t.Fatalf("partial block: %+v %v", c.DeviceDefaults, err)
	}
	c.DeviceDefaults = DeviceDefaults{}
	c.Normalize()
	if err := c.Validate(); err != nil || c.DeviceDefaults.Theme != "system" || c.DeviceDefaults.Clock != "24" {
		t.Fatalf("blank block: %+v %v", c.DeviceDefaults, err)
	}
	b, _ := json.Marshal(Defaults())
	if !strings.Contains(string(b), `"browserDefaults":{"theme":"system","layout":"auto","clock":"24","flip":false,"scrollTop":true,"searchFilters":false,"setStack":"and","nextMode":"number","stackGap":"30","compact":false,"paletteChar":"\u003e"}`) {
		t.Fatalf("json: %s", b)
	}
}

func TestLevelColors(t *testing.T) {
	c := Defaults()
	c.Routers[0].Levels = 3
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
	c.Routers[0].Levels = 3
	for mode, want := range map[string][]string{"1": {"1", "2", "3"}, "0": {"0", "1", "2"}, "va": {"V", "1", "2"}} {
		c.Routers[0].LevelNumbering = mode
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
	c := Defaults()
	c.Users = []User{{ID: "u-1", Name: "Ana", Pin: h, Len: 4}}
	b, _ := json.Marshal(c)
	if strings.Contains(string(b), "pbkdf2") {
		t.Fatal("PIN hash leaks into JSON")
	}
}

func TestFireModeFromEnv(t *testing.T) {
	t.Setenv("SODE_FIRE_MODE", "native")
	t.Setenv("SODE_SALVO_GROUP", "12")
	c, err := FromEnv(Defaults())
	if err != nil || c.Routers[0].FireMode != "native" || c.Routers[0].SalvoGroup != 12 {
		t.Fatalf("%q %d %v", c.Routers[0].FireMode, c.Routers[0].SalvoGroup, err)
	}
}

func TestNameRereadFromEnv(t *testing.T) {
	t.Setenv("SODE_NAME_REREAD_S", "90")
	c, err := FromEnv(Defaults())
	if err != nil || c.Routers[0].NameRefreshSeconds != 90 {
		t.Fatalf("%d %v", c.Routers[0].NameRefreshSeconds, err)
	}
	if b, _ := json.Marshal(c); !strings.Contains(string(b), `"nameRereadSeconds":90`) {
		t.Fatalf("json: %s", b)
	}
}

// Retired page border colors still load and draw, but are not offered.
func TestRetiredBrandColorsStillLoad(t *testing.T) {
	for _, name := range []string{"green", "amber", "pink"} {
		c := Defaults()
		c.Routers[0].Address = "10.0.0.5:8910"
		c.BrandBorder = name
		c.Normalize()
		if err := c.Validate(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		if _, ok := BrandHex(name); !ok {
			t.Errorf("%s: no hex", name)
		}
		if slices.Contains(BrandNames(), name) {
			t.Errorf("%s is offered", name)
		}
	}
	if got := strings.Join(BrandNames(), " "); got != "blue teal sky slate violet" {
		t.Errorf("offered %q", got)
	}
}
