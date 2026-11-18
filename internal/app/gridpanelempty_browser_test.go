package app

import "testing"

// A saved Grid panel with no sources and no destinations, as a new one starts. The page opens, says the panel
// shows nothing and links to the editor. The grid keys, the search keys and a press on Take do nothing and throw
// nothing.
func TestChromeGridPanelEmpty(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"New grid","type":"grid","settings":{"mode":"take"},"sources":{"members":[]},"destinations":{"members":[]}}`)
	waitNamesAndTally(t, a, ana)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	b.waitFor("the grid says it shows nothing and links to the editor", `(() => {
		const e = document.querySelector('.sode-gridpanel__empty');
		const edit = e?.querySelector('a[href="panels/editor?panel=`+id+`"]');
		return !!e && !e.hidden && e.querySelector('.pb-empty__title').textContent === 'This panel shows no sources or destinations' &&
			!!edit && !edit.hidden && e.getBoundingClientRect().height > 0 && !document.querySelector('.sode-gridpanel__xp:not([hidden])');
	})()`)

	for _, k := range []struct {
		key, code string
		vk        int
	}{
		{"ArrowRight", "ArrowRight", 39}, {"ArrowDown", "ArrowDown", 40}, {"End", "End", 35}, {"Home", "Home", 36},
		{"PageDown", "PageDown", 34}, {" ", "Space", 32}, {"v", "KeyV", 86}, {"g", "KeyG", 71}, {"Delete", "Delete", 46},
		{"Escape", "Escape", 27}, {"Enter", "Enter", 13},
	} {
		realKey(b, k.key, k.code, k.vk)
	}
	realKeyMods(b, "/", "Slash", 191, modCtrl)
	realKey(b, "s", "KeyS", 83)
	b.waitFor("s goes to the Source bar box", `document.activeElement === document.querySelector('.sode-gridpanel__find[data-kind="srcs"] input')`)
	b.eval(`(() => { const i = document.activeElement; i.value = 'cam'; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	realKey(b, "Enter", "Enter", 13)
	realKey(b, "Escape", "Escape", 27)
	b.eval(`document.getElementById('panel-take').click(); true`)
	b.waitFor("nothing is staged and no confirm opened", `document.getElementById('confirm').hidden && document.getElementById('panel-take').disabled && !document.querySelector('.sode-gridpanel__xp[data-s]')`)
	b.waitFor("the note is still there", `!document.querySelector('.sode-gridpanel__empty').hidden`)
	b.check()
}
