package app

import (
	"math"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Protect state under Destination at 1920x1080. A Router lock and a Sode Protect have one layout: the badge
// beside its level strip, its top on the strip's top. Protect and Free are text buttons, and Free sits under the badge
// of the Protect it frees, on its left edge.
func TestChromeProtectUnderDestination(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].ProtectDevice = 126
	})
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("Settings button", `!!document.getElementById('settings-open')`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	dst := func(text string) {
		b.eval(`(() => { const c = document.getElementById('clear-all'); if (!c.disabled) c.click(); return true; })()`)
		b.waitFor(text+" typed", `(() => { const t = document.getElementById('dst'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('dst-opt-0'); })()`)
		b.eval(`document.getElementById('dst').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
	}
	oneLine := func(what string) {
		t.Helper()
		xs := b.boxes(`#dst-under .sode__hold > .sode__bar`, `#dst-under .sode__hold .pb-badge[data-owner]`)
		if len(xs) != 2 || xs[1].Left <= xs[0].Right || math.Abs(xs[1].Top-xs[0].Top) > layoutTol {
			t.Errorf("%s: strip and badge at %v, want the badge beside the strip, tops on one line", what, xs)
		}
	}

	// The simulator holds a Router lock on Rec B (0-based 12).
	dst("Rec B")
	b.waitFor("the Router lock", `[...document.querySelectorAll('#dst-under .pb-badge[data-owner]')].some((n) => n.textContent.startsWith('Router lock'))`)
	b.shot("router-lock")
	oneLine("Router lock")

	dst("DSM1")
	b.waitFor("Protect offered as text", `document.querySelector('#dst-under [data-protect="set"]')?.textContent.trim() === 'Protect'`)
	b.eval(`document.querySelector('#dst-under [data-protect="set"]').click(); true`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("Free offered as text", `document.querySelector('#dst-under [data-protect="clear"]')?.textContent.trim() === 'Free'`)
	b.shot("sode-protect")
	oneLine("Sode Protect")
	b.sameLeft("Free under its badge", `#dst-under .pb-badge[data-owner="you"]`, `#dst-under [data-protect="clear"]`)
	b.gapY("Free under its badge", "--pb-space-2", `#dst-under .pb-badge[data-owner="you"]`, `#dst-under [data-protect="clear"]`)
	b.noClip("Free", `#dst-under [data-protect="clear"]`)
	b.check()
}
