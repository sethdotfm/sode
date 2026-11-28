package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Env-only mode (no SODE_CONFIG): the Not saved banner on Settings at 1920x1080. It does not cover the Settings search
// box, and its link is a button. The Degraded note is in the same alert box.
func TestChromeEnvOnlySettingsNotSaved(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	a.cfgPath = ""
	srv := seamServer(t, a)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	const banner = `document.querySelector('[data-notsaved]')`
	b.waitFor("the Not saved banner shows", `!`+banner+`.hidden && !!document.getElementById('settings-find').offsetParent`)
	b.waitFor("the search box is below the banner, not under it", `(() => {
		const f = document.getElementById('settings-find').getBoundingClientRect();
		const hit = document.elementFromPoint(f.left + f.width / 2, f.top + f.height / 2);
		return hit === document.getElementById('settings-find') && f.top >= `+banner+`.getBoundingClientRect().bottom;
	})()`)
	b.shot("settings")
	b.waitFor("the banner's link is a button, not a chip", `[...document.querySelectorAll('[data-notsaved-fix]')].every((a) => a.matches('a.pb-btn.pb-btn--outline:not(.pb-chip):not(.pb-btn--sm)') && !!a.href)`)
	b.waitFor("the Degraded note is a warn alert box", `['conn-degraded', 'm-degraded'].every((id) => document.getElementById(id).matches('.pb-alert.pb-alert--warn'))`)
	b.check()
}
