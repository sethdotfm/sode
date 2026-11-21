package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// With no page locks and Guest role Trusted user, a guest has the Protect right (GuestActs). The Palette's Protect
// opens the picker, and the Grid popup's shield offers Protect.
func TestChromeProtectTrustedGuest(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Locks = config.Locks{}
		c.GuestRole = config.RoleTrusted
		c.ProtectDevice = 127
	})
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('dst'); t.value = 'DSM'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('dst-opt-0'); })()`)
	b.eval(`(() => { const t = document.getElementById('dst'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)

	b.eval(`document.activeElement.blur(); true`)
	b.eval(ctrlShiftP)
	b.waitFor("the Palette", paletteShown)
	paletteFind(b, "Protect", "action:protect")
	paletteKey(b, "Enter", false)
	b.waitFor("the picker", `!document.getElementById('protpick').hidden`)
	b.eval(`[...document.querySelectorAll('#protpick button')].find((n) => n.textContent === 'Cancel').click(); true`)
	b.waitFor("the picker closes", `!document.getElementById('protpick')`)

	b.eval(`document.getElementById('grid-open').click(); true`)
	name := `document.querySelector('#grid-pop .sode-gridpanel__dst[data-dst="1"]:not([hidden])')`
	b.waitFor("the popup", `!document.getElementById('grid-pop').hidden && !!`+name)
	r := b.eval(`(() => { const r = ` + name + `.getBoundingClientRect(); return [r.left + 10, r.top + r.height / 2]; })()`).([]any)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": r[0], "y": r[1]})
	b.waitFor("the shield offers Protect", `document.querySelector('.sode-gridpop__shield')?.dataset.protect === 'set' && !document.querySelector('.sode-gridpop__shield').hidden`)
	b.check()
}
