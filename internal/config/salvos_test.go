package config

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func salvoOf(name string, src int, dsts ...int) Salvo {
	return Salvo{Name: name, Items: []SalvoItem{{Src: src, Dsts: dsts}}}
}

func TestSalvoValidate(t *testing.T) {
	ok := Defaults()
	ok.Normalize()
	ok.Salvos = []Salvo{salvoOf("Morning", 1, 2, 3)}
	ok.Normalize()
	if err := ok.Validate(); err != nil {
		t.Fatal(err)
	}
	many := make([]Salvo, 31)
	for i := range many {
		many[i] = salvoOf(strings.Repeat("a", i+1), 0, 0)
	}
	big := make([]int, MaxSalvoDsts+1)
	for name, mut := range map[string]func(c *Config){
		"unnamed":      func(c *Config) { c.Salvos = []Salvo{salvoOf("", 1, 2)} },
		"same names":   func(c *Config) { c.Salvos = []Salvo{salvoOf("A", 1, 2), salvoOf("a", 1, 2)} },
		"long name":    func(c *Config) { c.Salvos = []Salvo{salvoOf(strings.Repeat("x", 33), 1, 2)} },
		"too many":     func(c *Config) { c.Salvos = many },
		"no takes":     func(c *Config) { c.Salvos = []Salvo{{Name: "A"}} },
		"no dsts":      func(c *Config) { c.Salvos = []Salvo{salvoOf("A", 1)} },
		"too many dst": func(c *Config) { c.Salvos = []Salvo{salvoOf("A", 1, big...)} },
		"bad src":      func(c *Config) { c.Salvos = []Salvo{salvoOf("A", -1, 2)} },
		"bad dst":      func(c *Config) { c.Salvos = []Salvo{salvoOf("A", 1, 70000)} },
		"bad level": func(c *Config) {
			c.Salvos = []Salvo{{Name: "A", Items: []SalvoItem{{Src: 1, Dsts: []int{2}, Levels: []int{0}}}}}
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

func TestSalvoRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	c := Defaults()
	c.Salvos = []Salvo{
		{Name: " Show ", Items: []SalvoItem{{Src: 4, Dsts: []int{1, 2}, Levels: []int{3, 1, 3}}, {Src: 5, Dsts: []int{9}}}},
	}
	c.Normalize()
	if got := c.Salvos[0].Items[0].Levels; !reflect.DeepEqual(got, []int{1, 3}) || c.Salvos[0].Name != "Show" {
		t.Fatalf("normalize: %+v", c.Salvos)
	}
	if err := Save(path, c); err != nil {
		t.Fatal(err)
	}
	got, _, err := Load(path)
	if err != nil || !reflect.DeepEqual(got.Salvos, c.Salvos) {
		t.Fatalf("round trip: %+v err=%v want %+v", got.Salvos, err, c.Salvos)
	}
	// Clone shares nothing.
	d := c.Clone()
	d.Salvos[0].Items[0].Dsts[0] = 77
	if c.Salvos[0].Items[0].Dsts[0] == 77 {
		t.Fatal("clone shares dsts")
	}
}

func TestSalvoTotalAndConflict(t *testing.T) {
	item := func(src int, levels []int, dsts ...int) SalvoItem {
		return SalvoItem{Src: src, Dsts: dsts, Levels: levels}
	}
	check := func(s Salvo) error {
		c := Defaults()
		c.Salvos = []Salvo{s}
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
	if err := check(Salvo{Name: "a", Items: []SalvoItem{item(1, nil, many(100)...), item(1, nil, many(100)...)}}); err != nil {
		t.Errorf("200 total: %v", err)
	}
	if err := check(Salvo{Name: "a", Items: []SalvoItem{item(1, nil, many(100)...), item(1, nil, many(100)...), item(1, nil, 5)}}); err == nil || !strings.Contains(err.Error(), "200 destinations in total") {
		t.Errorf("201 total: %v", err)
	}
	// Different sources, same destination: overlapping (or default) levels conflict.
	if err := check(Salvo{Name: "a", Items: []SalvoItem{item(1, nil, 7), item(2, []int{1}, 7)}}); err == nil || !strings.Contains(err.Error(), "destination 7") {
		t.Errorf("default vs level: %v", err)
	}
	if err := check(Salvo{Name: "a", Items: []SalvoItem{item(1, []int{1, 2}, 7), item(2, []int{2, 3}, 7)}}); err == nil {
		t.Error("overlap on level 2 accepted")
	}
	// Disjoint levels, a different destination, or the same source are fine.
	for _, s := range []Salvo{
		{Name: "a", Items: []SalvoItem{item(1, []int{1, 2}, 7), item(2, []int{3, 4}, 7)}},
		{Name: "a", Items: []SalvoItem{item(1, nil, 7), item(2, nil, 8)}},
		{Name: "a", Items: []SalvoItem{item(1, nil, 7), item(1, nil, 7)}},
	} {
		if err := check(s); err != nil {
			t.Errorf("%+v: %v", s, err)
		}
	}
}

func TestSalvosMessageFits(t *testing.T) {
	// 30 salvos of 200 five-digit destinations each is a "salvos" message
	// that must still fit the 64 KiB WebSocket read limit.
	c := Defaults()
	for i := 0; i < MaxSalvos; i++ {
		d := make([]int, MaxSalvoTotal)
		for j := range d {
			d[j] = 60000 + j
		}
		c.Salvos = append(c.Salvos, Salvo{Name: strings.Repeat("n", 31) + string(rune('a'+i)),
			Items: []SalvoItem{{Src: 65000, Dsts: d[:100]}, {Src: 65001, Dsts: d[100:]}}})
	}
	c.Normalize()
	if err := c.Validate(); err != nil {
		t.Fatalf("max-size salvos rejected: %v", err)
	}
	b, _ := json.Marshal(map[string]any{"t": "salvos", "id": 99999, "salvos": c.Salvos})
	if len(b) >= 64<<10 {
		t.Fatalf("message is %d bytes", len(b))
	}
	// Heavy levels go past the byte cap: refused with a message, not a dropped socket.
	for i := range c.Salvos {
		c.Salvos[i].Items = nil
		for j := 0; j < MaxSalvoItems; j++ {
			lv := make([]int, 64)
			for k := range lv {
				lv[k] = k + 1
			}
			c.Salvos[i].Items = append(c.Salvos[i].Items, SalvoItem{Src: j, Dsts: []int{j}, Levels: lv})
		}
	}
	if err := c.Validate(); err == nil {
		t.Fatal("oversized salvos accepted")
	}
}
