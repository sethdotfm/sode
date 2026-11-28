package app

import "testing"

func TestChromeGridPanelEmpty(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"New grid","type":"grid","settings":{"mode":"take"},"sources":{"members":[]},"destinations":{"members":[]}}`)
	waitNamesAndTally(t, a, ana)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	b.waitFor("the grid says it shows nothing and links to the editor", `(() => {
		const e = document.querySelector('.sode-gridpanel__empty');
		const edit = e?.querySelector('a[href="panels/editor?panel=`+id+`"]');
		return !!e && !e.hidden && e.querySelector('.pb-empty__title').textContent === 'This panel has no sources or destinations.' &&
			!!edit && !edit.hidden && e.getBoundingClientRect().height > 0 && !document.querySelector('.sode-gridpanel__xp:not([hidden])');
	})()`)

	// The corner's words name nothing, and the deck has nothing to work on, so its controls are disabled. The Edit
	// panel link stays.
	b.shot("empty")
	b.waitFor("the corner's words go and the deck's controls are disabled", `(() => {
		const head = document.querySelector('.sode-gridpanel__head');
		const live = [...document.querySelectorAll('#panel-levels :is(button, input), .sode-gridpanel__finds :is(button, input)')].filter((n) => !n.disabled);
		const edit = document.querySelector('.sode-gridpanel__empty a');
		return getComputedStyle(head).visibility === 'hidden' && live.length === 0 && !!document.querySelector('#panel-take:disabled') &&
			!!document.querySelector('.sode-gridpanel__find-input:disabled') && edit.textContent.trim() === 'Edit panel' &&
			getComputedStyle(document.querySelector('#panel-lvl-pick .sode__pick')).cursor === 'not-allowed';
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
	b.waitFor("s does not go to the disabled Source bar box", `document.activeElement !== document.querySelector('.sode-gridpanel__find[data-kind="srcs"] input')`)
	realKey(b, "Enter", "Enter", 13)
	realKey(b, "Escape", "Escape", 27)
	b.eval(`document.getElementById('panel-take').click(); true`)
	b.waitFor("nothing is staged and no confirm opened", `document.getElementById('confirm').hidden && document.getElementById('panel-take').disabled && !document.querySelector('.sode-gridpanel__xp[data-s]')`)
	b.waitFor("the note is still there", `!document.querySelector('.sode-gridpanel__empty').hidden`)
	b.check()
}
