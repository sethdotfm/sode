package app

import (
	"net/http"

	"github.com/sethdotfm/sode/internal/config"
)

func init() {
	registerAPI(apiRoute{"GET", apiBase + "/presets", config.ScopePresets, (*App).apiPresets})
}

// Scope is "everyone" or "me". A This browser preset is only in a browser, so no key sees it.
type apiPreset struct {
	Scope string `json:"scope"`
	config.Preset
}

func (a *App) apiPresets(w http.ResponseWriter, _ *http.Request, cl *apiCtx) {
	cfg := a.r().cfg
	v := a.view(cl)
	out := []apiPreset{}
	add := func(scope string, ps []config.Preset) {
		for _, p := range ps {
			if v.WholePreset(p) {
				out = append(out, apiPreset{scope, p})
			}
		}
	}
	add("everyone", cfg.Presets)
	if cl.Actor.Kind == actorUser {
		if u, ok := cfg.FindUserByID(cl.Actor.User); ok {
			add("me", u.Profile.Presets)
		}
	}
	apiJSON(w, map[string]any{"presets": out})
}
