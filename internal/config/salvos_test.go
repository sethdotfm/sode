package config

import (
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
