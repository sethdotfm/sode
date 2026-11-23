package app

import (
	"testing"
	"time"
)

// homeKey presses Alt+Shift+h through the browser's input pipeline.
func homeKey(b *tab) {
	b.t.Helper()
	realKeyMods(b, "H", "KeyH", 72, modAlt|modShift)
}

// The Home key (homekey.js, homeview.js). Settings with an unsaved change warns on the first press and closes on the
// second, with the focus in Source. On Home with the Grid popup open, it closes the popup and keeps the Source box's
// text and the staged route. From /log it opens Home. From the editor with an unsaved edit it warns, then a second
// press leaves. On a Kiosk panel it does nothing.
func TestChromeHomeShortcut(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := makePanel(t, a, ana, `{"name":"Studio grid","type":"grid","sources":{"members":"all"},"destinations":{"members":"all"}}`)
	kiosk := makePanel(t, a, ana, `{"name":"Kiosk wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}],"settings":{"kiosk":true}}`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.eval(`{ const n = document.getElementById('cfg-brandtext'); n.focus(); n.value = 'Studio'; n.dispatchEvent(new Event('input', { bubbles: true })); } true`)
	homeKey(b)
	b.waitFor("an unsaved change warns and Settings stays", `!document.getElementById('settings-unsaved').hidden && document.documentElement.dataset.view === 'settings'`)
	homeKey(b)
	b.waitFor("a second press closes Settings and focuses Source", `document.documentElement.dataset.view !== 'settings' && document.activeElement.id === 'src'`)

	popTyped(b, "src", "CAM 1")
	b.waitFor("Source holds CAM 1", `document.getElementById('src').value.endsWith('CAM 1')`)
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the Grid popup opens", `!document.getElementById('grid-pop').hidden`)
	gridShow(b, 1, 5)
	b.eval(gridXp(1, 5) + `.click()`)
	b.waitFor("a staged route", gridXp(1, 5)+`?.dataset.s === 'staged'`)
	b.eval(`window.srcText = document.getElementById('src').value; true`)
	homeKey(b)
	b.waitFor("the popup closes, the text and the staged route stay, and Source has the focus", `document.getElementById('grid-pop').hidden && document.getElementById('src').value === window.srcText
		&& document.activeElement.id === 'src' && document.querySelectorAll('#grid-staged-list li').length === 1`)

	b.open(srv.URL + "/log")
	b.waitFor("the Log's Home link", `!!document.querySelector('.sode-bar__home')`)
	homeKey(b)
	b.waitFor("the key opens Home from /log", `location.pathname === '/' && !!document.getElementById('src')`)

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the lists show", `!!document.getElementById('ed-sources-col')?.offsetParent`)
	b.eval(`{ const n = document.getElementById('ed-name'); n.focus(); n.value = 'Studio grid 2'; n.dispatchEvent(new Event('input', { bubbles: true })); } true`)
	b.waitFor("an unsaved edit", `!document.getElementById('ed-save').disabled`)
	b.eval(`window.stays = 1; true`)
	homeKey(b)
	b.waitFor("the key warns and the editor stays", `window.stays === 1 && location.pathname.endsWith('/editor') && document.getElementById('ed-status').textContent === 'Unsaved changes. Choose again to discard.'`)
	homeKey(b)
	b.waitFor("a second press discards and opens Home", `location.pathname === '/'`)

	b.open(srv.URL + "/panels/" + kiosk)
	b.waitFor("Kiosk mode", `document.documentElement.hasAttribute('data-sode-kiosk')`)
	b.eval(`window.stays = 1; true`)
	homeKey(b)
	time.Sleep(500 * time.Millisecond)
	if b.eval(`window.stays === 1 && location.pathname === '/panels/`+kiosk+`'`) != true {
		t.Error("the Home key left a Kiosk panel")
	}
	b.check()
}
