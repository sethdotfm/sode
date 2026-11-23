package config

import (
	"fmt"
	"strings"
)

// The Palette character (GLOSSARY.md) is a pref like theme, so it is one of a fixed list. Go twin of PALETTE_CHARS in
// web/paletterules.js (TestPrefTable). Each one passes paletteCharProblem there: no letter, digit, mark, space or
// control character, and none of + , - . that the patch boxes give a meaning to.
const (
	PaletteChars   = ">~!/;:=#@$%^&*|?<_()[]{}§°¬£€¤"
	PaletteDefault = ">"
	// PaletteOff is the value for no Palette character. It is not one character, so it never clashes with one.
	PaletteOff = "off"
)

func paletteValues() []string {
	out := []string{}
	for _, r := range PaletteChars {
		out = append(out, string(r))
	}
	return append(out, PaletteOff)
}

// PaletteHomeKey is Home's row key; Home is first in the empty Palette, so it is never a pin (HOME_KEY in
// web/pinrules.js).
const PaletteHomeKey = "page:home"

// MaxPalettePicks caps Profile.PalettePicks; MaxPalettePins caps Profile.PalettePins (PINS_MAX in web/pinrules.js);
// MaxPaletteKey caps one key, in runes.
const (
	MaxPalettePicks = 5
	MaxPalettePins  = 24
	MaxPaletteKey   = 120
)

// Keys are opaque, as pin order keys are: the browser builds them (palettecatalogue.js) and compares bytes.
func normalizePalettePicks(in []string) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, k := range in {
		k = strings.TrimSpace(k)
		if k == "" || seen[k] || len([]rune(k)) > MaxPaletteKey || len(out) >= MaxPalettePicks {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	return out
}

// Palette pins (GLOSSARY.md, Palette pin) are Palette row keys in the person's own order. Only the spaces around a key
// go; a bad key or a repeat is refused (PalettePinsProblem), not dropped.
func normalizePalettePins(in []string) []string {
	out := make([]string, len(in))
	for i, k := range in {
		out[i] = strings.TrimSpace(k)
	}
	return out
}

// PalettePinsProblem says why Sode refuses a pin list, or "". The page shows the same words (web/pinrules.js
// pinsProblem); web/test/pin_cases.json holds the cases that both read.
func PalettePinsProblem(list []string) string {
	if len(list) > MaxPalettePins {
		return fmt.Sprintf("a list holds at most %d Palette pins", MaxPalettePins)
	}
	seen := map[string]bool{}
	for _, k := range list {
		if k == PaletteHomeKey {
			return "Home is always first in the Palette, so it is not pinned"
		}
		if !RowKeyOK(k) {
			return "a Palette pin names a Palette row that Sode does not know"
		}
		if seen[k] {
			return "a Palette row is pinned only one time"
		}
		seen[k] = true
	}
	return ""
}
