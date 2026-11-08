package app

import (
	"fmt"

	"github.com/sethdotfm/sode/internal/config"
)

// apiSettingsReq: a nil field keeps the current value. The key caps (MaxUserKeys,
// MaxClientKeys) are fixed v1 limits and not editable here.
type apiSettingsReq struct {
	Enabled      *bool `json:"enabled"`
	RateLimit    *int  `json:"rateLimit"`
	TakeCap      *int  `json:"takeCap"`
	GuestEnabled *bool `json:"guestEnabled"`
	LogReads     *bool `json:"logReads"`
}

// SaveAPISettings needs the Advanced Settings PIN, even for a trusted user. API is an
// owned field, so the ordinary Settings save never touches it.
func (a *App) SaveAPISettings(c *client, req apiSettingsReq) error {
	a.cfgMu.Lock()
	defer a.cfgMu.Unlock()
	if a.r().cfg.SettingsPin == "" {
		return errNeedAdminPin
	}
	if err := a.guardSave(c, false); err != nil {
		return err
	}
	before := a.r().cfg.API
	err := a.applyConfig(func(cur *config.Config) error {
		set := func(dst *bool, v *bool) {
			if v != nil {
				*dst = *v
			}
		}
		set(&cur.API.Enabled, req.Enabled)
		set(&cur.API.GuestEnabled, req.GuestEnabled)
		set(&cur.API.LogReads, req.LogReads)
		if req.RateLimit != nil {
			cur.API.RateLimit = *req.RateLimit
		}
		if req.TakeCap != nil {
			cur.API.TakeCap = *req.TakeCap
		}
		return nil
	})
	if err == nil {
		a.auditAPISettings(connActor(c), before, a.r().cfg.API)
	}
	return err
}

func (a *App) auditAPISettings(act Actor, was, now config.APISettings) {
	onOff := func(b bool) string { return map[bool]string{true: "on", false: "off"}[b] }
	if was.Enabled != now.Enabled {
		a.apiEvent(act, "apiEnabled", onOff(now.Enabled), "", nil)
	}
	if was.GuestEnabled != now.GuestEnabled {
		a.apiEvent(act, "guestApi", onOff(now.GuestEnabled), "", nil)
	}
	if was.LogReads != now.LogReads {
		a.apiEvent(act, "logApiReads", onOff(now.LogReads), "", nil)
	}
	if was.RateLimit != now.RateLimit || was.TakeCap != now.TakeCap {
		a.apiEvent(act, "apiLimits", "ok", fmt.Sprintf("rate limit %d a minute, take cap %d", now.RateLimit, now.TakeCap), nil)
	}
}
