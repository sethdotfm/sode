package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The phone layout keeps the panel list, Launch and the Preferences topic. On /panels the editor, Assign to, keys and
// sharing give way to "Open Sode on a computer to ...", and so does /panels/editor. Settings lists only This browser
// and My user, with "Other settings need a computer". Its Use desktop layout switches the layout, and the panel's
// actions come back.
func TestChromePhoneDesktopOnly(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	id := makePanel(t, a, userKey(t, a, "Ana"), `{"name":"Phone list"}`)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/panels")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	signInOnPad(b)
	row := `document.querySelector('[data-id="` + id + `"].sode-ov__item')`
	b.waitFor("the panel in the list", `!!`+row)
	b.eval(row + `.click(); true`)
	b.waitFor("Launch stays", `!!document.querySelector('#panels-detail a.pb-btn--ok')?.offsetParent`)
	b.waitFor("the desktop-only message in place of the editor, Assign to, keys and sharing", `(() => {
		const m = document.querySelector('#panels-detail .sode__desktop-only');
		return !!m?.offsetParent && m.textContent.includes('Open Sode on a computer to edit, share, assign or delete panels and manage their keys.')
			&& !!m.querySelector('[data-desktop-layout]')
			&& !document.querySelector('#panels-detail [data-act]') && !document.querySelector('#panels-detail a[href*="editor"]')
			&& !document.getElementById('panels-new').offsetParent;
	})()`)
	b.shot("phone-panels")

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the editor page says to use a computer", `(() => {
		const m = document.querySelector('.sode-ed > .sode__desktop-only');
		return !!m?.offsetParent && m.textContent.includes('Open Sode on a computer to edit panels.') && !document.getElementById('ed-grid').offsetParent;
	})()`)

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	b.waitFor("only Preferences, with the message", `(() => {
		const toc = document.getElementById('settings-toc');
		const shown = (sel) => [...toc.querySelectorAll(sel)].filter((x) => x.offsetParent).map((x) => x.textContent.trim());
		const m = document.getElementById('settings-desktop-only');
		return shown('.sode__toc-topic').join() === 'Preferences' && shown('.sode__toc-sec').join() === 'This browser,My user'
			&& !!m?.offsetParent && m.textContent.includes('Other settings need a computer.');
	})()`)
	b.waitFor("search finds nothing outside Preferences", `(() => {
		const f = document.getElementById('settings-find');
		f.value = 'confirm lockout'; f.dispatchEvent(new Event('input'));
		const none = !document.querySelector('#settings-found li');
		f.value = ''; f.dispatchEvent(new Event('input'));
		return none;
	})()`)
	b.shot("phone-settings")
	b.eval(`document.querySelector('#settings-desktop-only [data-desktop-layout]').click(); true`)
	b.waitFor("Use desktop layout switches the layout for this browser", `document.documentElement.dataset.layout === 'desktop' && localStorage.getItem('sode-layout') === 'desktop'`)
	b.waitFor("the Administrator's topics come back", `(() => {
		const bands = [...document.querySelectorAll('#settings-view > .sode__topic')].filter((h) => h.offsetParent).map((h) => h.textContent.trim());
		return bands.join('|') === 'Preferences|Look and feel|Routers|Access|Server' && !document.getElementById('settings-desktop-only').offsetParent;
	})()`)

	b.open(srv.URL + "/panels")
	b.waitFor("the panel in the list", `!!`+row)
	b.eval(row + `.click(); true`)
	b.waitFor("the panel's actions are back", `!!document.querySelector('#panels-detail [data-act="assign"]') && !!document.querySelector('#panels-detail [data-act="share"]') && !document.querySelector('#panels-detail .sode__desktop-only')`)
	b.check()
}
