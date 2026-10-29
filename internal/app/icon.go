package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/sethdotfm/sode/internal/config"
)

// The app icon (home screen, install dialog) is the Lucide "route" glyph on
// a solid tile. Default: accent glyph on the dark surface. With a brand
// colour (issue 126) the tile takes the colour and the glyph goes dark, so
// instances tell apart on a home screen. The colours are config.BrandColors,
// the dark values of the signal tokens web/base.css maps the names to.
var (
	iconDark   = color.NRGBA{0x08, 0x08, 0x08, 0xff}
	iconAccent = color.NRGBA{0x8b, 0x93, 0xff, 0xff}
	iconSizes  = []int{180, 192, 512}
)

type iconKey struct {
	size  int
	brand string
}

var iconCache sync.Map // iconKey → []byte

// The glyph in its own 24-unit space, stroked 2 wide. It mirrors the Lucide
// "route" icon: path `M9 19h8.5a3.5 3.5 0 0 0 0-7h-11a3.5 3.5 0 0 1 0-7H15`
// (three segments and two half circles) plus circles at (6,19) and (18,5),
// r=3. Drawn at 56% of the tile, inside the maskable
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

// hexColor parses the table's "#rrggbb".
func hexColor(h string) color.NRGBA {
	v, _ := strconv.ParseUint(strings.TrimPrefix(h, "#"), 16, 32)
	return color.NRGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

// iconURL is the path of the icon the server draws (web/brandicon.js builds
// the same); brand "" is the default look.
func iconURL(size int, brand string) string {
	u := "icon.png?s=" + strconv.Itoa(size)
	if brand != "" {
		u += "&c=" + url.QueryEscape(brand)
	}
	return u
}

func renderIcon(size int, brand string) []byte {
	bg, fg := iconDark, iconAccent
	if c, ok := config.BrandColorByName(brand); ok {
		bg, fg = hexColor(c.Hex), iconDark
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
	if _, ok := config.BrandColorByName(brand); !ok {
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
