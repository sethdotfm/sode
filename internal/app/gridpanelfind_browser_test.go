package app

import (
	"fmt"
	"testing"
)

// gridFindBox is the search box input of a bar, kind "srcs" or "dsts".
func gridFindBox(kind string) string {
	return fmt.Sprintf(`document.querySelector('.sode-gridpanel__find[data-kind="%s"] input')`, kind)
}

// gridFound is the router numbers of the drawn names that Find marks on a bar, as "n,n".
func gridFound(kind string) string {
	attr := map[string]string{"srcs": "src", "dsts": "dst"}[kind]
	return fmt.Sprintf(`[...document.querySelectorAll('.sode-gridpanel__%s[data-found]:not([hidden])')].map((n) => n.dataset.%s).sort((a, b) => a - b).join(',')`, attr, attr)
}

// s and d go to the bars' search boxes in the corner. Find marks matches in place and moves the cursor to the first;
// Enter and Shift+Enter step. Ctrl+/ switches to Filter, which shows only matches. Tab goes to the other box, which
// has the same mode. n and N on the grid step the box last typed in. Esc clears the box, keeps the mode and gives the
// keys back to the grid with the cursor on the last match.
func TestChromeGridPanelFind(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Find grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(3, 2)+` !== null`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)
	cols := `getComputedStyle(document.querySelector('.sode-gridpanel__field')).getPropertyValue('--sode-cols').trim()`
	b.waitFor("200 sources", cols+` === '200'`)

	realKey(b, "s", "KeyS", 83)
	b.waitFor("s goes to the Source bar box and types nothing", `document.activeElement === `+gridFindBox("srcs")+` && document.activeElement.value === ''`)
	b.waitFor("no step buttons on an empty box", `[...document.querySelectorAll('.sode-gridpanel__find-step')].every((n) => n.hidden)`)
	// SRC 031 (source 30) is the first match in panel order, on letters in order only. SRC 100 to SRC 199 start with
	// the text, so Find goes to the first of them.
	b.call("Input.insertText", map[string]any{"text": "src 1"})
	gridCursorOn(b, "Find goes to the best match, not the first in panel order", -1, 99)
	// The glide ends with the match in the middle of what shows past the Destination bar.
	b.waitFor("the best match glides to the middle and pulses", `(() => {
		const n = document.querySelector('.sode-gridpanel__src[data-src="99"]:not([hidden])');
		const sc = document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect();
		const bar = document.querySelector('.sode-gridpanel__head').getBoundingClientRect();
		if (!n || n.dataset.hit === undefined) return false;
		const r = n.getBoundingClientRect();
		return Math.abs((r.left + r.right) / 2 - (bar.right + sc.right) / 2) <= r.width / 2;
	})()`)
	b.waitFor("the pulse ends", `!document.querySelector('.sode-gridpanel__src[data-hit]')`)
	realKeyMods(b, "n", "KeyN", 78, modCtrl)
	gridCursorOn(b, "Ctrl+N in the box steps to the next match", -1, 100)
	realKeyMods(b, "p", "KeyP", 80, modCtrl)
	gridCursorOn(b, "Ctrl+P steps back", -1, 99)
	b.waitFor("Ctrl+N and Ctrl+P type nothing", gridFindBox("srcs")+`.value === 'src 1'`)
	b.eval(`document.querySelector('.sode-gridpanel__find[data-kind="srcs"] .sode-gridpanel__find-step[data-dir="1"]').click(); true`)
	gridCursorOn(b, "the next match button steps", -1, 100)
	b.eval(`document.querySelector('.sode-gridpanel__find[data-kind="srcs"] .sode-gridpanel__find-step[data-dir="-1"]').click(); true`)
	gridCursorOn(b, "the previous match button steps back", -1, 99)
	b.waitFor("a step button gives the keys back to the box", `document.activeElement === `+gridFindBox("srcs"))
	b.waitFor("the Destination bar box shows no step buttons", `[...document.querySelectorAll('.sode-gridpanel__find[data-kind="dsts"] .sode-gridpanel__find-step')].every((n) => n.hidden)`)
	b.shot("find-best")
	realKey(b, "Escape", "Escape", 27)
	realKey(b, "s", "KeyS", 83)
	b.waitFor("s goes back to the empty box", `document.activeElement === `+gridFindBox("srcs")+` && document.activeElement.value === ''`)
	b.call("Input.insertText", map[string]any{"text": "cam 1"})
	// CAM 1 and CAM 10 to CAM 12 are sources 12 and 21 to 23; only about 13 columns show, so the grid scrolls.
	gridCursorOn(b, "Find moves the cursor to the first match", -1, 12)
	b.waitFor("the first match is marked", gridFound("srcs")+`.split(',').includes('12')`)
	b.waitFor("the box counts the matches", `document.querySelector('.sode-gridpanel__find[data-kind="srcs"]').textContent.includes('1 of 4')`)
	b.shot("find")

	realKey(b, "Enter", "Enter", 13)
	gridCursorOn(b, "Enter steps to the next match", -1, 21)
	b.waitFor("the grid scrolls to show it", `(() => {
		const n = document.querySelector('.sode-gridpanel__src[data-src="21"]:not([hidden])');
		const r = document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect();
		return !!n && n.dataset.found !== undefined && n.getBoundingClientRect().right <= r.right;
	})()`)
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13, "modifiers": 8})
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "Enter", "code": "Enter", "windowsVirtualKeyCode": 13, "modifiers": 8})
	gridCursorOn(b, "Shift+Enter steps back", -1, 12)
	b.waitFor("Find hides nothing", cols+` === '200'`)

	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "/", "code": "Slash", "windowsVirtualKeyCode": 191, "modifiers": 2})
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "/", "code": "Slash", "windowsVirtualKeyCode": 191, "modifiers": 2})
	shownSrcs := `[...document.querySelectorAll('.sode-gridpanel__src:not([hidden])')].map((n) => n.dataset.src).sort((a, b) => a - b).join(',')`
	b.waitFor("Ctrl+/ switches to Filter, which shows only the matches, unmarked", cols+` === '4' && `+
		shownSrcs+` === '12,21,22,23' && `+gridFound("srcs")+` === ''`)
	b.waitFor("both boxes name the mode", `[...document.querySelectorAll('.sode-gridpanel__find-input')].map((n) => n.placeholder).join('|') === 'Filter sources|Filter destinations'`)
	b.waitFor("the placeholder fits its box", `(() => {
		const i = `+gridFindBox("dsts")+`;
		const cs = getComputedStyle(i);
		const x = document.createElement('canvas').getContext('2d');
		x.font = cs.font;
		return x.measureText(i.placeholder).width <= i.clientWidth - parseFloat(cs.paddingLeft) - parseFloat(cs.paddingRight);
	})()`)
	b.waitFor("one Filter switch, on", `[...document.querySelectorAll('.sode-gridpanel__find-mode input[role="switch"]:checked')].map((n) => n.labels[0].querySelector('.sode-gridpanel__find-word').textContent).join() === 'Filter'`)
	b.waitFor("the box stays focused and keeps its text", `document.activeElement === `+gridFindBox("srcs")+` && document.activeElement.value === 'cam 1'`)
	gridCursorOn(b, "the cursor stays on its source", -1, 0)
	b.shot("filter")

	realKey(b, "Tab", "Tab", 9)
	b.waitFor("Tab goes to the Destination bar box", `document.activeElement === `+gridFindBox("dsts"))
	b.waitFor("the Destination bar box has the same mode", `document.querySelector('.sode-gridpanel__find[data-kind="dsts"]').dataset.mode === 'filter'`)
	b.call("Input.insertText", map[string]any{"text": "dsm"})
	// DSM1 to DSM5 are destinations 2 to 6.
	rows := `getComputedStyle(document.querySelector('.sode-gridpanel__field')).getPropertyValue('--sode-rows').trim()`
	b.waitFor("the Destination bar box filters too", rows+` === '5'`)
	gridCursorOn(b, "the cursor goes to the first match", 0, 0)
	b.eval(`document.activeElement.blur(); true`)
	realKey(b, "n", "KeyN", 78)
	gridCursorOn(b, "n on the grid steps the box last typed in", 1, 0)
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "N", "code": "KeyN", "windowsVirtualKeyCode": 78, "modifiers": 8})
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "N", "code": "KeyN", "windowsVirtualKeyCode": 78, "modifiers": 8})
	gridCursorOn(b, "N steps back", 0, 0)

	realKey(b, "s", "KeyS", 83)
	// A blank makes this box the one typed in last.
	b.call("Input.insertText", map[string]any{"text": " "})
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc clears the box and leaves it", ``+gridFindBox("srcs")+`.value === '' && document.activeElement !== `+gridFindBox("srcs"))
	b.waitFor("every source shows again", cols+` === '200'`)
	gridCursorOn(b, "the cursor stays on the source it was on", 0, 12)
	b.waitFor("the box keeps Filter for the visit", `document.querySelector('.sode-gridpanel__find[data-kind="srcs"]').dataset.mode === 'filter'`)
	b.waitFor("the Destination bar keeps its Filter", rows+` === '5'`)
	realKey(b, "n", "KeyN", 78)
	gridCursorOn(b, "with the Source bar box empty, n steps the other box", 1, 12)
	realKey(b, "j", "KeyJ", 74)
	gridCursorOn(b, "the keys move the cursor again", 2, 12)

	realKey(b, "s", "KeyS", 83)
	b.call("Input.insertText", map[string]any{"text": "zz"})
	b.waitFor("a Filter with no match shows no sources and says so", cols+` === '0' && `+
		`document.querySelector('.sode-gridpanel__find[data-kind="srcs"]').textContent.includes('No match')`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc shows every source again", cols+` === '200'`)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	b.waitFor("the grid after a load", gridXp(3, 2)+` !== null && `+cols+` === '200'`)
	b.waitFor("a load starts with empty boxes in Find mode", gridFindBox("srcs")+`.value === '' && `+gridFindBox("dsts")+`.value === '' && `+
		`document.querySelector('.sode-gridpanel__find[data-kind="srcs"]').dataset.mode === 'find'`)
	b.check()
}
