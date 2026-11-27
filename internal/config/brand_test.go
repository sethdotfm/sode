package config

import (
	"strings"
	"testing"
)

func TestBrandNormalize(t *testing.T) {
	c := Defaults()
	c.Routers[0].Address = "10.0.0.5:8910"
	c.BrandText = "  Studio B "
	c.BrandBorder = " Teal "
	c.Normalize()
	if c.BrandText != "Studio B" || c.BrandBorder != "teal" {
		t.Fatalf("got %q %q", c.BrandText, c.BrandBorder)
	}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	c.BrandText = strings.Repeat("é", 24)
	if err := c.Validate(); err != nil {
		t.Fatalf("24 runes: %v", err)
	}
}

func TestBrandCustomHex(t *testing.T) {
	for in, want := range map[string]string{"teal": "#5ccfc9", "#A1B2C3": "#a1b2c3"} {
		c := Defaults()
		c.Routers[0].Address = "10.0.0.5:8910"
		c.BrandBorder = in
		c.Normalize()
		if err := c.Validate(); err != nil {
			t.Fatalf("%q: %v", in, err)
		}
		if got, ok := BrandHex(c.BrandBorder); !ok || got != want {
			t.Fatalf("%q: got %q %v", in, got, ok)
		}
	}
	for _, bad := range []string{"#abc", "#12345g", "a1b2c3", "red", "#A1B2C3"} {
		if _, ok := BrandHex(bad); ok {
			t.Fatalf("%q accepted", bad)
		}
	}
	c := Defaults()
	c.Routers[0].Address = "10.0.0.5:8910"
	c.BrandBorder = "#12345g"
	if c.Validate() == nil {
		t.Fatal("bad hex validated")
	}
}
