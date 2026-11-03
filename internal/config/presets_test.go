package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func presetOf(name string, src int, dsts ...int) Preset {
	return Preset{Name: name, Items: []PresetItem{{Src: src, Dsts: dsts}}}
}

func TestPresetValidate(t *testing.T) {
	ok := Defaults()
	ok.Normalize()
	ok.Presets = []Preset{presetOf("Morning", 1, 2, 3)}
	ok.Normalize()
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	many := make([]Preset, 31)
	for i := range many {
		many[i] = presetOf(strings.Repeat("a", i+1), 0, 0)
	}
	big := make([]int, MaxPresetDsts+1)
	for name, mut := range map[string]func(c *Config){
		"unnamed":      func(c *Config) { c.Presets = []Preset{presetOf("", 1, 2)} },
		"same names":   func(c *Config) { c.Presets = []Preset{presetOf("A", 1, 2), presetOf("a", 1, 2)} },
		"long name":    func(c *Config) { c.Presets = []Preset{presetOf(strings.Repeat("x", 33), 1, 2)} },
		"too many":     func(c *Config) { c.Presets = many },
		"no takes":     func(c *Config) { c.Presets = []Preset{{Name: "A"}} },
		"no dsts":      func(c *Config) { c.Presets = []Preset{presetOf("A", 1)} },
		"too many dst": func(c *Config) { c.Presets = []Preset{presetOf("A", 1, big...)} },
		"bad src":      func(c *Config) { c.Presets = []Preset{presetOf("A", -1, 2)} },
		"bad dst":      func(c *Config) { c.Presets = []Preset{presetOf("A", 1, 70000)} },
		"bad level": func(c *Config) {
			c.Presets = []Preset{{Name: "A", Items: []PresetItem{{Src: 1, Dsts: []int{2}, Levels: []int{0}}}}}
		},
	} {
		c := ok.Clone()
		mut(&c)
		c.Normalize()
		if c.Validate() == nil {
			t.Errorf("%s: want error", name)
		}
	}
}

func TestPresetRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := Defaults()
	c.Presets = []Preset{
		{Name: " Show ", Items: []PresetItem{{Src: 4, Dsts: []int{1, 2}, Levels: []int{3, 1, 3}}, {Src: 5, Dsts: []int{9}}}},
	}
	c.Normalize()
	if got := c.Presets[0].Items[0].Levels; !reflect.DeepEqual(got, []int{1, 3}) || c.Presets[0].Name != "Show" {
		t.Fatalf("normalize: %+v", c.Presets)
	}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(path)
	if err != nil || !reflect.DeepEqual(got.Presets, c.Presets) {
		t.Fatalf("round trip: %+v err=%v want %+v", got.Presets, err, c.Presets)
	}
	// Clone shares nothing.
	d := c.Clone()
	d.Presets[0].Items[0].Dsts[0] = 77
	if c.Presets[0].Items[0].Dsts[0] == 77 {
		t.Fatal("clone shares dsts")
	}
}

func TestPresetTotalAndConflict(t *testing.T) {
	item := func(src int, levels []int, dsts ...int) PresetItem {
		return PresetItem{Src: src, Dsts: dsts, Levels: levels}
	}
	check := func(s Preset) error {
		c := Defaults()
		c.Presets = []Preset{s}
		c.Normalize()
		return c.Validate()
	}
	many := func(n int) []int {
		d := make([]int, n)
		for i := range d {
			d[i] = i
		}
		return d
	}
	// 200 in total across items is fine, 201 is not.
	if err := check(Preset{Name: "a", Items: []PresetItem{item(1, nil, many(100)...), item(1, nil, many(100)...)}}); err != nil {
		t.Errorf("200 total: %v", err)
	}
	if err := check(Preset{Name: "a", Items: []PresetItem{item(1, nil, many(100)...), item(1, nil, many(100)...), item(1, nil, 5)}}); err == nil || !strings.Contains(err.Error(), "200 destinations in total") {
		t.Errorf("201 total: %v", err)
	}
	// Different sources, same destination: overlapping (or default) levels conflict.
	if err := check(Preset{Name: "a", Items: []PresetItem{item(1, nil, 7), item(2, []int{1}, 7)}}); err == nil || !strings.Contains(err.Error(), "destination 7") {
		t.Errorf("default vs level: %v", err)
	}
	if err := check(Preset{Name: "a", Items: []PresetItem{item(1, []int{1, 2}, 7), item(2, []int{2, 3}, 7)}}); err == nil {
		t.Error("overlap on level 2 accepted")
	}
	// Disjoint levels, a different destination, or the same source are fine.
	for _, s := range []Preset{
		{Name: "a", Items: []PresetItem{item(1, []int{1, 2}, 7), item(2, []int{3, 4}, 7)}},
		{Name: "a", Items: []PresetItem{item(1, nil, 7), item(2, nil, 8)}},
		{Name: "a", Items: []PresetItem{item(1, nil, 7), item(1, nil, 7)}},
	} {
		if err := check(s); err != nil {
			t.Errorf("%+v: %v", s, err)
		}
	}
}

func TestPresetsMessageFits(t *testing.T) {
	// 30 presets of 200 five-digit destinations each is a "presets" message
	// that must still fit the 64 KiB WebSocket read limit.
	c := Defaults()
	for i := 0; i < MaxPresets; i++ {
		d := make([]int, MaxPresetTotal)
		for j := range d {
			d[j] = 60000 + j
		}
		c.Presets = append(c.Presets, Preset{Name: strings.Repeat("n", 31) + string(rune('a'+i)),
			Items: []PresetItem{{Src: 65000, Dsts: d[:100]}, {Src: 65001, Dsts: d[100:]}}})
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("max-size presets rejected: %v", err)
	}
	b, _ := json.Marshal(map[string]any{"t": "presets", "id": 99999, "presets": c.Presets})
	if len(b) >= 64<<10 {
		t.Fatalf("message is %d bytes", len(b))
	}
	// Heavy levels go past the byte cap: refused with a message, not a dropped socket.
	for i := range c.Presets {
		c.Presets[i].Items = nil
		for j := 0; j < MaxPresetItems; j++ {
			lv := make([]int, 64)
			for k := range lv {
				lv[k] = k + 1
			}
			c.Presets[i].Items = append(c.Presets[i].Items, PresetItem{Src: j, Dsts: []int{j}, Levels: lv})
		}
	}
	if err := c.Validate(); err == nil {
		t.Fatal("oversized presets accepted")
	}
}

// A file saved before presets were renamed has them under "salvos".
func TestLegacySalvosKeyLoadsAsPresets(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := Defaults()
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(path)
	old := string(b) + "salvos:\n  - name: Show\n    items:\n      - src: 4\n        dsts: [1, 2]\n"
	if err := os.WriteFile(path, []byte(old), 0o600); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	got.Normalize()
	if len(got.Presets) != 1 || got.Presets[0].Name != "Show" || got.LegacySalvos != nil {
		t.Fatalf("legacy key not moved: %+v legacy=%v", got.Presets, got.LegacySalvos)
	}
	if err := Save(path, got); err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(path)
	if strings.Contains(string(b), "salvos:") || !strings.Contains(string(b), "presets:") {
		t.Fatalf("saved file still uses the old key:\n%s", b)
	}
}
