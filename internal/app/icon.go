package app

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"slices"
	"strconv"
	"sync"
)

// The app icon (home screen, install dialog) is the Lucide "route" glyph on
// a solid tile. Default: accent glyph on the dark surface. With a brand
// colour (issue 126) the tile takes the colour and the glyph goes dark, so
// instances tell apart on a home screen. The colours are the dark values of
// the signal tokens that web/base.css maps the same names to.
var (
	iconDark   = color.NRGBA{0x08, 0x08, 0x08, 0xff}
	iconAccent = color.NRGBA{0x8b, 0x93, 0xff, 0xff}
	iconBrand  = map[string]color.NRGBA{
		"blue":   {0x5f, 0xb2, 0xe8, 0xff},
		"teal":   {0x5c, 0xcf, 0xc9, 0xff},
		"green":  {0x5f, 0xd0, 0x9a, 0xff},
		"amber":  {0xe8, 0xa3, 0x5e, 0xff},
		"violet": {0xa6, 0x7e, 0xf0, 0xff},
		"pink":   {0xe8, 0x7b, 0xb3, 0xff},
	}
	iconSizes = []int{180, 192, 512}
)

type iconKey struct {
	size  int
	brand string
}

var iconCache sync.Map // iconKey → []byte

// The glyph in its own 24-unit space, stroked 2 wide: three segments of the
// path and the two rings. Drawn at 56% of the tile, inside the maskable
// safe zone.
type seg struct{ x1, y1, x2, y2 float64 }

var iconLines = []seg{{9, 19, 17.5, 19}, {17.5, 12, 6.5, 12}, {6.5, 5, 15, 5}}

// inGlyph: is the point (24-unit space) within the stroke?
func inGlyph(x, y float64) bool {
	for _, l := range iconLines {
		if distSeg(x, y, l) <= 1 {
			return true
		}
	}
	// The two half circles of the path, joining the lines (centre, side).
	if distArc(x, y, 17.5, 15.5, 3.5, true) <= 1 || distArc(x, y, 6.5, 8.5, 3.5, false) <= 1 {
		return true
	}
	for _, c := range [][2]float64{{6, 19}, {18, 5}} {
		if math.Abs(math.Hypot(x-c[0], y-c[1])-3) <= 1 {
			return true
		}
	}
	return false
}

func distSeg(x, y float64, s seg) float64 {
	dx, dy := s.x2-s.x1, s.y2-s.y1
	t := math.Max(0, math.Min(1, ((x-s.x1)*dx+(y-s.y1)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(x-(s.x1+t*dx), y-(s.y1+t*dy))
}

// distArc: distance to the right (or left) half of a circle.
func distArc(x, y, cx, cy, r float64, right bool) float64 {
	if dx := x - cx; (dx >= 0) == right {
		return math.Abs(math.Hypot(dx, y-cy) - r)
	}
	return math.Min(math.Hypot(x-cx, y-(cy-r)), math.Hypot(x-cx, y-(cy+r)))
}

func renderIcon(size int, brand string) []byte {
	bg, fg := iconDark, iconAccent
	if c, ok := iconBrand[brand]; ok {
		bg, fg = c, iconDark
	}
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	scale := float64(size) * 12 / 512 // pixels per glyph unit
	off := (float64(size) - 24*scale) / 2
	const n = 3 // samples per axis, for smooth edges
	mix := func(a, b uint8, k float64) uint8 { return uint8(math.Round(float64(a)*(1-k) + float64(b)*k)) }
	for py := range size {
		for px := range size {
			hit := 0
			for sy := range n {
				for sx := range n {
					x := (float64(px) + (float64(sx)+0.5)/n - off) / scale
					y := (float64(py) + (float64(sy)+0.5)/n - off) / scale
					if inGlyph(x, y) {
						hit++
					}
				}
			}
			k := float64(hit) / (n * n)
			img.SetNRGBA(px, py, color.NRGBA{mix(bg.R, fg.R, k), mix(bg.G, fg.G, k), mix(bg.B, fg.B, k), 0xff})
		}
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}

// serveIcon: /icon.png?s=<180|192|512>&c=<brand colour>. Unknown sizes and
// colours fall back to 192 and the default look.
func serveIcon(w http.ResponseWriter, r *http.Request) {
	size, _ := strconv.Atoi(r.URL.Query().Get("s"))
	if !slices.Contains(iconSizes, size) {
		size = 192
	}
	brand := r.URL.Query().Get("c")
	if _, ok := iconBrand[brand]; !ok {
		brand = ""
	}
	k := iconKey{size, brand}
	b, ok := iconCache.Load(k)
	if !ok {
		b, _ = iconCache.LoadOrStore(k, renderIcon(size, brand))
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", "public, max-age=3600")
	w.Write(b.([]byte))
}

// serveManifest: the web app manifest follows the logo text and colour set
// in Settings, so each instance installs under its own name and icon.
func (a *App) serveManifest(w http.ResponseWriter, r *http.Request) {
	cfg := a.r().cfg
	name := cfg.BrandText
	if name == "" {
		name = "Sode"
	}
	q := ""
	if _, ok := iconBrand[cfg.BrandBorder]; ok {
		q = "&c=" + cfg.BrandBorder
	}
	icon := func(size int, purpose string) map[string]string {
		return map[string]string{
			"src": "icon.png?s=" + strconv.Itoa(size) + q, "type": "image/png",
			"sizes": strconv.Itoa(size) + "x" + strconv.Itoa(size), "purpose": purpose,
		}
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(map[string]any{
		"name": name, "short_name": name,
		"description": "Keyboard-first web patching for SW-P-08 routers",
		"start_url":   "./", "scope": "./", "display": "standalone",
		"background_color": "#080808", "theme_color": "#080808",
		"icons": []map[string]string{icon(192, "any"), icon(512, "any"), icon(512, "maskable")},
	})
}
