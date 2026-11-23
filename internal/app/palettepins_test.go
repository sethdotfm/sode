package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

// Me pins go in profileSave `palettePins`, and a save of other parts keeps them. The server refuses a 25th pin or a
// key that names no Palette row, as the page does (web/test/pin_cases.json).
func TestProfileSavePalettePins(t *testing.T) {
	a := userApp(t, false)
	ana := watched(t, a, "10.0.0.1", "1234")
	pins := []string{"page:log", "preset:global:p1"}
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", PalettePins: pins}); err != nil {
		t.Fatal(err)
	}
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", Prefs: map[string]string{"theme": "dark"}}); err != nil {
		t.Fatal(err)
	}
	if got := anasProfile(t, a).PalettePins; !slices.Equal(got, pins) {
		t.Fatalf("pins after a save of prefs: %q", got)
	}
	full := make([]string, 25)
	for i := range full {
		full[i] = fmt.Sprintf("preset:global:p%d", i)
	}
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", PalettePins: full[:24]}); err != nil {
		t.Fatalf("24 pins refused: %v", err)
	}
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", PalettePins: full}); err == nil || !strings.Contains(err.Error(), "at most 24 Palette pins") {
		t.Fatalf("25th pin: %v", err)
	}
	if _, err := a.ProfileSave(ana, profileReq{Name: "Ana", PalettePins: []string{"https://example.com/"}}); err == nil {
		t.Fatal("a web address kept as a pin")
	}
	if got := anasProfile(t, a).PalettePins; len(got) != 24 {
		t.Fatalf("a refused save wrote %d pins", len(got))
	}
}
