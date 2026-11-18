package app

import (
	"slices"

	"github.com/sethdotfm/sode/internal/config"
)

// apiSettingsReq: a nil field keeps the current value. The key caps are fixed (userKeyMax,
// clientKeyMax in apikeys.go).
type apiSettingsReq struct {
	RateLimit    *int  `json:"rateLimit"`
	TakeCap      *int  `json:"takeCap"`
	GuestEnabled *bool `json:"guestEnabled"`
	LogReads     *bool `json:"logReads"`
	// GuestScopes: the keyless scopes (config.GuestAPIScopes).
	GuestScopes *[]string `json:"guestScopes"`
}

// SaveAPISettings needs an Administrator. API is an owned field, so the ordinary Settings
// save never touches it.
func (a *App) SaveAPISettings(c *client, req apiSettingsReq) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if err := a.guardSave(c, config.RoleAdministrator); err != nil {
		return err
	}
	before := a.r().cfg
	err := a.applyConfig(a.critical(c, func(cur *config.Config) error {
		set := func(dst *bool, v *bool) {
			if v != nil {
				*dst = *v
			}
		}
		set(&cur.API.GuestEnabled, req.GuestEnabled)
		set(&cur.API.LogReads, req.LogReads)
		if req.RateLimit != nil {
			cur.API.RateLimit = *req.RateLimit
		}
		if req.TakeCap != nil {
			cur.API.TakeCap = *req.TakeCap
		}
		if req.GuestScopes != nil {
			cur.GuestAPIScopes = slices.Clone(*req.GuestScopes)
		}
		return nil
	}))
	if err == nil {
		a.auditAPISettings(connActor(c), before, a.r().cfg)
	}
	return err
}
