package app

import (
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func get(t *testing.T, a *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	a.Handler(web.FS).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestIconColours(t *testing.T) {
	a := newTestApp(t)
	for _, tc := range []struct {
		path     string
		size     int
		cornerBG [3]uint8 // top-left pixel is always the tile
	}{
		{"/icon.png?s=180", 180, [3]uint8{0x08, 0x08, 0x08}},
		{"/icon.png?s=512&c=teal", 512, [3]uint8{0x5c, 0xcf, 0xc9}},
		{"/icon.png?s=999&c=red", 192, [3]uint8{0x08, 0x08, 0x08}}, // unknown size and colour fall back
	} {
		rec := get(t, a, tc.path)
		if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/png" {
			t.Fatalf("%s: %d %q", tc.path, rec.Code, rec.Header().Get("Content-Type"))
		}
		img, err := png.Decode(rec.Body)
		if err != nil {
			t.Fatalf("%s: %v", tc.path, err)
		}
		if b := img.Bounds(); b.Dx() != tc.size || b.Dy() != tc.size {
			t.Fatalf("%s: %v", tc.path, b)
		}
		r, g, bl, al := img.At(0, 0).RGBA()
		if got := [3]uint8{uint8(r >> 8), uint8(g >> 8), uint8(bl >> 8)}; got != tc.cornerBG || al != 0xffff {
			t.Fatalf("%s: corner %v alpha %x", tc.path, got, al)
		}
		// The ring of the lower-left circle is glyph-coloured, the tile is not.
		mid := tc.size / 2
		if img.At(mid, mid) == img.At(0, 0) && img.At(mid/2, mid+mid/3) == img.At(0, 0) {
			t.Fatalf("%s: no glyph drawn", tc.path)
		}
	}
}

func TestIconURL(t *testing.T) {
	if iconURL(180, "") != "icon.png?s=180" || iconURL(512, "teal") != "icon.png?s=512&c=teal" {
		t.Fatal(iconURL(180, ""), iconURL(512, "teal"))
	}
}

// Every colour in the table needs its CSS rule, and names stay plain.
func TestBrandColorsMatchCSS(t *testing.T) {
	css, err := web.FS.ReadFile("base.css")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, c := range config.BrandColors {
		if c.Name == "" || c.Name != strings.ToLower(c.Name) || seen[c.Name] || c.Label == "" {
			t.Errorf("bad or duplicate colour %+v", c)
		}
		seen[c.Name] = true
		if !strings.Contains(string(css), `[data-brand="`+c.Name+`"]`) {
			t.Errorf("base.css has no rule for %q", c.Name)
		}
		if got := hexColor(c.Hex); got == (color.NRGBA{0, 0, 0, 0xff}) {
			t.Errorf("%s: hex %q", c.Name, c.Hex)
		}
	}
}
