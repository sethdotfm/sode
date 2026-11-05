package config

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func profileCfg(t *testing.T) Config {
	t.Helper()
	h, err := HashPin("1234")
	if err != nil {
		t.Fatal(err)
	}
	c := Defaults()
	c.Users = []User{{Name: "Ana", Pin: h, Len: 4}}
	return c
}

func TestProfileNormalizeDropsUnknownPrefs(t *testing.T) {
	c := profileCfg(t)
	c.Users[0].Profile = Profile{Prefs: map[string]string{"theme": "dark", "bogus": "x"},
		FilterSets: []FilterSet{{Name: " Mine "}}}
	c.Normalize()
	p := c.Users[0].Profile
	if len(p.Prefs) != 1 || p.Prefs["theme"] != "dark" || p.FilterSets[0].Name != "Mine" {
		t.Fatalf("%+v", p)
	}
	// An empty profile is stored as nothing.
	c.Users[0].Profile = Profile{Prefs: map[string]string{"bogus": "x"}}
	c.Normalize()
	if c.Users[0].Profile.Prefs != nil || !c.Users[0].Profile.Empty() {
		t.Fatalf("%+v", c.Users[0].Profile)
	}
}

func TestProfileValidate(t *testing.T) {
	check := func(p Profile) error {
		c := profileCfg(t)
		c.Users[0].Profile = p
		c.Normalize()
		return c.Validate()
	}
	if err := check(Profile{Prefs: map[string]string{"theme": "dark", "clock": "locale", "flip": "off"}}); err != nil {
		t.Fatal(err)
	}
	for name, p := range map[string]Profile{
		"bad value":     {Prefs: map[string]string{"theme": "purple"}},
		"bool as word":  {Prefs: map[string]string{"flip": "true"}},
		"dup set names": {FilterSets: []FilterSet{{Name: "A"}, {Name: "a"}}},
		"long expr":     {FilterSets: []FilterSet{{Name: "A", Src: strings.Repeat("x", 501)}}},
		"empty preset":  {Presets: []Preset{{Name: "P"}}},
	} {
		if check(p) == nil {
			t.Errorf("%s accepted", name)
		}
	}
}

func TestProfileYAMLRoundTripAndClone(t *testing.T) {
	c := profileCfg(t)
	c.Users[0].Profile = Profile{Prefs: map[string]string{"theme": "dark"},
		Presets: []Preset{{Name: "P", Items: []PresetItem{{Src: 1, Dsts: []int{2}, Levels: []int{1}}}}}}
	c.Normalize()
	path := filepath.Join(t.TempDir(), "c.yaml")
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if !strings.Contains(string(raw), "profile:") {
		t.Fatalf("not saved:\n%s", raw)
	}
	back, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := back.Users[0].Profile; got.Prefs["theme"] != "dark" || len(got.Presets) != 1 {
		t.Fatalf("%+v", got)
	}
	// Clone shares nothing.
	d := back.Clone()
	d.Users[0].Profile.Prefs["theme"] = "light"
	d.Users[0].Profile.Presets[0].Items[0].Dsts[0] = 99
	if back.Users[0].Profile.Prefs["theme"] != "dark" || back.Users[0].Profile.Presets[0].Items[0].Dsts[0] != 2 {
		t.Fatal("Clone shares the profile")
	}
	// A user with no profile writes no key.
	e := profileCfg(t)
	e.Normalize()
	if err := Save(path, e); err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if strings.Contains(string(raw), "profile") {
		t.Fatalf("empty profile written:\n%s", raw)
	}
}

// The Go pref table lists exactly what web/prefs.js PREFS does.
func TestPrefTable(t *testing.T) {
	b, err := os.ReadFile("../../web/prefs.js")
	if err != nil {
		t.Skip(err)
	}
	line := regexp.MustCompile(`(?m)^  (\w+): \{ key: [^\n]*?values: \{([^}]*)\} \},$`)
	key := regexp.MustCompile(`(\w+):`)
	found := map[string][]string{}
	for _, m := range line.FindAllStringSubmatch(string(b), -1) {
		for _, k := range key.FindAllStringSubmatch(m[2], -1) {
			found[m[1]] = append(found[m[1]], k[1])
		}
	}
	if len(found) != len(PrefValues) {
		t.Fatalf("prefs.js has %v, Go has %v", found, PrefValues)
	}
	for name, vals := range found {
		if strings.Join(PrefValues[name], ",") != strings.Join(vals, ",") {
			t.Errorf("%s: prefs.js %v, Go %v", name, vals, PrefValues[name])
		}
	}
}

func TestProfilePinOrderNormalized(t *testing.T) {
	p := Profile{PinOrder: []string{" Global:Cam ", "global:cam", "", "user:Mon 1", strings.Repeat("x", MaxPinKey+1)}}
	got := p.Normalized().PinOrder
	if len(got) != 2 || got[0] != "global:cam" || got[1] != "user:mon 1" {
		t.Fatalf("pin order %q", got)
	}
	if (Profile{PinOrder: []string{"a"}}).Empty() {
		t.Fatal("a profile with only a pin order is not empty")
	}
	if n := (Profile{}).Normalized().PinOrder; n == nil || len(n) != 0 {
		t.Fatalf("empty pin order must be [] not %v", n)
	}
	long := make([]string, MaxPinOrder+50)
	for i := range long {
		long[i] = fmt.Sprintf("local:p%d", i)
	}
	if n := len((Profile{PinOrder: long}).Normalized().PinOrder); n != MaxPinOrder {
		t.Fatalf("cut at %d, got %d", MaxPinOrder, n)
	}
}
