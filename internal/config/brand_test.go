package config

import (
	"strings"
	"testing"
)

func TestBrandNormalize(t *testing.T) {
	c := Defaults()
	c.Router = "10.0.0.5:8910"
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
