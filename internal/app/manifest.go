package app

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sethdotfm/sode/internal/config"
)

type manifestIcon struct {
	Src     string `json:"src"`
	Type    string `json:"type"`
	Sizes   string `json:"sizes"`
	Purpose string `json:"purpose"`
}

type manifest struct {
	Name            string         `json:"name"`
	ShortName       string         `json:"short_name"`
	Description     string         `json:"description"`
	ID              string         `json:"id"`
	StartURL        string         `json:"start_url"`
	Scope           string         `json:"scope"`
	Display         string         `json:"display"`
	BackgroundColor string         `json:"background_color"`
	ThemeColor      string         `json:"theme_color"`
	Icons           []manifestIcon `json:"icons"`
}

// serveManifest: the web app manifest follows the logo text and colour set
// in Settings, so each instance installs under its own name and icon.
func (a *App) serveManifest(w http.ResponseWriter, r *http.Request) {
	cfg := a.r().cfg
	name := cfg.BrandText
	if name == "" {
		name = "Sode"
	}
	brand := cfg.BrandBorder
	if _, ok := config.BrandColorByName(brand); !ok {
		brand = ""
	}
	icon := func(size int, purpose string) manifestIcon {
		s := strconv.Itoa(size)
		return manifestIcon{Src: iconURL(size, brand), Type: "image/png", Sizes: s + "x" + s, Purpose: purpose}
	}
	w.Header().Set("Content-Type", "application/manifest+json")
	w.Header().Set("Cache-Control", "no-cache")
	json.NewEncoder(w).Encode(manifest{
		Name: name, ShortName: name,
		Description: "Keyboard-first web patching for SW-P-08 routers",
		ID:          "./", StartURL: "./", Scope: "./", Display: "standalone",
		BackgroundColor: "#080808", ThemeColor: "#080808",
		Icons: []manifestIcon{icon(192, "any"), icon(512, "any"), icon(512, "maskable")},
	})
}
