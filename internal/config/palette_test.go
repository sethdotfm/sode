package config

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestPalettePinCases(t *testing.T) {
	raw, err := os.ReadFile("../../web/test/pin_cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Cases []struct {
			Pins    []string `json:"pins"`
			Problem string   `json:"problem"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	for _, c := range doc.Cases {
		if got := PalettePinsProblem(c.Pins); got != c.Problem {
			t.Errorf("%v: got %q, want %q", c.Pins, got, c.Problem)
		}
	}
}

func TestProfilePalettePins(t *testing.T) {
	p := Profile{PalettePins: []string{" page:log ", "preset:global:p1"}}.Normalized()
	if len(p.PalettePins) != 2 || p.PalettePins[0] != "page:log" {
		t.Fatalf("normalized pins: %q", p.PalettePins)
	}
	if errs := validateProfile("Ana", p); len(errs) != 0 {
		t.Fatalf("good pins refused: %v", errs)
	}
	if (Profile{PalettePins: []string{"page:log"}}).Empty() {
		t.Fatal("a profile with a pin is empty")
	}
	full := make([]string, MaxPalettePins+1)
	for i := range full {
		full[i] = "preset:global:p" + string(rune('a'+i))
	}
	if errs := validateProfile("Ana", Profile{PalettePins: full}); len(errs) != 1 || !strings.Contains(errs[0].Error(), "at most 24 Palette pins") {
		t.Fatalf("25th pin: %v", errs)
	}
}
