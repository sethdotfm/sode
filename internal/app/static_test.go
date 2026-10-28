package app

import (
	"encoding/json"
	"image/png"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

func TestFaviconServed(t *testing.T) {
	a := newTestApp(t)
	rec := httptest.NewRecorder()
	a.Handler(web.FS).ServeHTTP(rec, httptest.NewRequest("GET", "/favicon.svg", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "image/svg+xml") {
		t.Fatalf("content type %q", ct)
	}
}

func get(t *testing.T, a *App, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	a.Handler(web.FS).ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestManifestFollowsBranding(t *testing.T) {
	a := newTestApp(t)
	rec := get(t, a, "/manifest.webmanifest")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/manifest+json") {
		t.Fatalf("%d %q", rec.Code, rec.Header().Get("Content-Type"))
	}
	var m struct {
		ShortName string `json:"short_name"`
		Icons     []struct{ Src string }
	}
	json.Unmarshal(rec.Body.Bytes(), &m)
	if m.ShortName != "Sode" || len(m.Icons) != 3 || m.Icons[0].Src != "icon.png?s=192" {
		t.Fatalf("default manifest %+v", m)
	}

	cfg := a.r().cfg
	cfg.BrandText, cfg.BrandBorder = "Studio B", "teal"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(get(t, a, "/manifest.webmanifest").Body.Bytes(), &m)
	if m.ShortName != "Studio B" || m.Icons[1].Src != "icon.png?s=512&c=teal" {
		t.Fatalf("branded manifest %+v", m)
	}
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
