package config

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func presetOf(name string, src int, dsts ...int) Preset {
	return Preset{Name: name, Items: []PresetItem{refItem(src, nil, dsts...)}}
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
			c.Presets = []Preset{{Name: "A", Items: []PresetItem{refItem(1, []int{0}, 2)}}}
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
		{Name: " Show ", Items: []PresetItem{refItem(4, []int{3, 1, 3}, 1, 2), refItem(5, nil, 9)}},
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
	d := c.Clone()
	d.Presets[0].Items[0].Dsts[0].N = 77
	if c.Presets[0].Items[0].Dsts[0].N == 77 {
		t.Fatal("clone shares dsts")
	}
}

func TestPresetTotalAndConflict(t *testing.T) {
	item := refItem
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
	// The largest list MaxPresetsBytes allows must fit the 64 KiB WebSocket read limit (5-digit numbers, longest names).
	c := Defaults()
	c.Normalize()
	name := strings.Repeat("N", 16)
	ref := func(n int) Ref { return Ref{Router: c.UID, N: n, Name: name} }
	full := func(perPreset int) []Preset {
		var out []Preset
		for i := 0; i < MaxPresets; i++ {
			it := PresetItem{Src: ref(65000), Levels: []int{}}
			for j := 0; j < perPreset; j++ {
				it.Dsts = append(it.Dsts, ref(60000+j))
			}
			out = append(out, Preset{Name: strings.Repeat("n", 31) + string(rune('a'+i)), Items: []PresetItem{it}})
		}
		return out
	}
	fits := 0
	for per := 1; per <= MaxPresetDsts; per++ {
		c.Presets = full(per)
		c.Normalize()
		if c.Validate() != nil {
			break
		}
		fits = per
		b, _ := json.Marshal(map[string]any{"t": "presets", "id": 99999, "presets": c.Presets})
		if len(b) >= 64<<10 {
			t.Fatalf("%d destinations per preset: message is %d bytes", per, len(b))
		}
	}
	t.Logf("worst case: %d presets of %d destinations (%d references) fit", MaxPresets, fits, MaxPresets*(fits+1))
	if fits < 10 {
		t.Errorf("only %d destinations per preset fit", fits)
	}
	c.Presets = full(MaxPresetDsts)
	c.Normalize()
	if err := c.Validate(); err == nil || !strings.Contains(err.Error(), "too large to save") {
		t.Errorf("30 presets of 100 destinations: %v", err)
	}
	for i := range c.Presets {
		c.Presets[i].Items = nil
		for j := 0; j < MaxPresetItems; j++ {
			lv := make([]int, 64)
			for k := range lv {
				lv[k] = k + 1
			}
			c.Presets[i].Items = append(c.Presets[i].Items, refItem(j, lv, j))
		}
	}
	if err := c.Validate(); err == nil {
		t.Fatal("oversized presets accepted")
	}
}
