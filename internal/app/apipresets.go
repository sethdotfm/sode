package app

import (
	"net/http"

	"github.com/sethdotfm/sode/internal/config"
)

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/presets", config.ScopePresets, (*App).apiPresets})
}

// apiPreset: Scope is "everyone" or "me". This browser presets live only in a browser, so
// no key sees them.
type apiPreset struct {
	Scope string `json:"scope"`
	config.Preset
}

func (a *App) apiPresets(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	cfg := a.r().cfg
	v := cl.view()
	out := []apiPreset{}
	add := func(scope string, ps []config.Preset) {
		for _, p := range ps {
			if v.Preset(p.Name) {
				out = append(out, apiPreset{scope, p})
			}
		}
	}
	add("everyone", cfg.Presets)
	if cl.Actor.Kind == actorUser {
		if u, ok := cfg.FindUser(cl.Actor.Name); ok {
			add("me", u.Profile.Presets)
		}
	}
	apiJSON(w, map[string]any{"presets": out})
}
