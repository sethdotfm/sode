package config

import (
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

// MaxPalettePicks caps Profile.PalettePicks; MaxPaletteKey caps one key, in runes.
const (
	MaxPalettePicks = 5
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
