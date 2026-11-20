package config

import (
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

func TestProfileNormalizeKeepsUnknownPrefs(t *testing.T) {
	c := profileCfg(t)
	c.Users[0].Profile = Profile{Prefs: map[string]string{"theme": "dark", "bogus": "x"},
		FilterSets: []FilterSet{{Name: " Mine "}}}
	c.Normalize()
	p := c.Users[0].Profile
	if len(p.Prefs) != 2 || p.Prefs["bogus"] != "x" || p.FilterSets[0].Name != "Mine" {
		t.Fatalf("%+v", p)
	}
	if err := c.Validate(); err != nil {
		t.Fatalf("an unknown pref fails Validate: %v", err)
	}
	if b := p.Normalized().Prefs; len(b) != 1 || b["theme"] != "dark" {
		t.Fatalf("the browser's copy has %v", b)
	}
	c.Users[0].Profile = Profile{Prefs: map[string]string{"theme": "x"}}
	c.Normalize()
	if c.Validate() == nil {
		t.Fatal("a bad value of a known pref passed Validate")
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
		Presets: []Preset{{Name: "P", Items: []PresetItem{refItem(1, []int{1}, 2)}}}}
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
	d := back.Clone()
	d.Users[0].Profile.Prefs["theme"] = "light"
	d.Users[0].Profile.Presets[0].Items[0].Dsts[0].N = 99
	if back.Users[0].Profile.Prefs["theme"] != "dark" || back.Users[0].Profile.Presets[0].Items[0].Dsts[0].N != 2 {
		t.Fatal("Clone shares the profile")
	}
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
	// The Palette character's values are PALETTE_CHARS in paletterules.js, then off.
	if regexp.MustCompile(`(?m)^  paletteChar: \{ key: 'sode-palette-char', def: PALETTE_DEFAULT, values: PALETTE_VALUES \},$`).Match(b) {
		r, err := os.ReadFile("../../web/paletterules.js")
		if err != nil {
			t.Fatal(err)
		}
		m := regexp.MustCompile(`export const PALETTE_CHARS = '([^']*)';`).FindSubmatch(r)
		if m == nil {
			t.Fatal("no PALETTE_CHARS in paletterules.js")
		}
		for _, c := range string(m[1]) {
			found["paletteChar"] = append(found["paletteChar"], string(c))
		}
		found["paletteChar"] = append(found["paletteChar"], PaletteOff)
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

func TestProfileBookmarksNormalized(t *testing.T) {
	p := Profile{Bookmarks: []Bookmark{{Key: " page:log ", Label: " Log "}}}
	if got := p.Normalized().Bookmarks; len(got) != 1 || got[0].Key != "page:log" || got[0].Label != "Log" {
		t.Fatalf("bookmarks %+v", got)
	}
	if p.Empty() {
		t.Fatal("a profile with only bookmarks is not empty")
	}
	if n := (Profile{}).Normalized().Bookmarks; n == nil || len(n) != 0 {
		t.Fatalf("no bookmarks must be [] not %v", n)
	}
}
