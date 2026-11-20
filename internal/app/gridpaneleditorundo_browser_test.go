package app

import (
	"testing"
)

// Editor undo, redo, cut, copy and paste in a Grid panel's lists, in the drawer's Live Editor grid, and across Back to
// editing. The clipboard is stubbed: a synthetic key fires no paste event, so a paste goes through readText.
func TestChromeGridPanelEditorUndo(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	liveGridPanel(t, b, srv.URL)

	b.eval(`
		navigator.clipboard.writeText = async (t) => { window.copied = t };
		navigator.clipboard.readText = async () => window.copied;
		window.li = (k, n) => document.querySelector('#ed-' + k + '-list li[data-id="' + n + '"]');
		window.order = (k, m = 6) => [...document.querySelectorAll('#ed-' + k + '-list li[data-id]')].slice(0, m).map((x) => x.dataset.id).join();
		window.pick = (k, n, mods) => li(k, n).querySelector('.sode-pages__name').dispatchEvent(new MouseEvent('click', { bubbles: true, ...mods }));
		window.said = () => document.getElementById('ed-status').textContent;
		true`)
	ctrl := func(key, code string, vk int) { realKeyMods(b, key, code, vk, modCtrl) }
	undo := func() { ctrl("z", "KeyZ", 90) }
	redo := func() { realKeyMods(b, "z", "KeyZ", 90, modCtrl|modShift) }
	// A real Ctrl+V pastes the machine's empty clipboard; a synthetic one fires no paste event, so readText reads the stub.
	paste := func() {
		b.eval(`document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'v', code: 'KeyV', ctrlKey: true, bubbles: true, cancelable: true })); true`)
	}

	b.waitFor("the sources list is drawn in router order", `order('sources') === '0,1,2,3,4,5'`)
	b.eval(`pick('sources', 2); true`)
	realKeyMods(b, "ArrowDown", "ArrowDown", 40, modAlt)
	b.waitFor("Alt+Down moves the row", `order('sources') === '0,1,3,2,4,5'`)
	undo()
	b.waitFor("Ctrl+Z in the list undoes the move and says so", `order('sources') === '0,1,2,3,4,5' && said() === 'Undid move.'`)
	redo()
	b.waitFor("Ctrl+Shift+Z redoes it", `order('sources') === '0,1,3,2,4,5' && said() === 'Redid move.'`)
	b.eval(`li('sources', 4).querySelector('[data-act="pin"]').click(); true`)
	b.waitFor("a row's Pin pins it", `order('sources', 1) === '4'`)
	undo()
	b.waitFor("Ctrl+Z from the row's button undoes the pin", `order('sources') === '0,1,3,2,4,5' && said() === 'Undid pin.'`)

	b.eval(`pick('sources', 0); pick('sources', 1, { shiftKey: true }); true`)
	ctrl("c", "KeyC", 67)
	b.waitFor("Ctrl+C copies the two rows as a sources block", `said() === 'Copied 2 sources.' && JSON.parse(window.copied).sode === 'sode-panel-rows' && JSON.parse(window.copied).axis === 'sources'`)
	b.eval(`pick('destinations', 5); true`)
	paste()
	b.waitFor("a sources block pasted into Destinations is refused", `said().startsWith('Nothing pasted: these are sources') && order('destinations') === '0,1,2,3,4,5'`)
	b.eval(`pick('sources', 0); pick('sources', 1, { shiftKey: true }); true`)
	ctrl("x", "KeyX", 88)
	b.waitFor("Ctrl+X cuts them", `said() === 'Cut 2 sources.' && order('sources') === '3,2,4,5,6,7'`)
	b.eval(`pick('sources', 5); true`)
	paste()
	b.waitFor("Ctrl+V pastes them after the cursor row, selected", `said() === 'Pasted 2 sources.' && order('sources', 7) === '3,2,4,5,0,1,6'
		&& li('sources', 0).hasAttribute('data-selected') && li('sources', 1).hasAttribute('data-selected') && !li('sources', 5).hasAttribute('data-selected')`)
	paste()
	b.waitFor("a second paste skips the rows the list has", `said() === 'Nothing pasted. Skipped 2 sources already in the list.' && order('sources', 7) === '3,2,4,5,0,1,6'`)
	undo()
	b.waitFor("one undo takes the whole paste back", `said() === 'Undid paste.' && order('sources') === '3,2,4,5,6,7'`)
	undo()
	b.waitFor("the next undoes the cut", `said() === 'Undid cut.' && order('sources') === '0,1,3,2,4,5'`)

	b.eval(`document.getElementById('ed-destinations-all').click(); true`)
	b.waitFor("Follow the router is on for destinations", `document.getElementById('ed-destinations-all').checked && !document.getElementById('ed-destinations-following').hidden`)
	b.eval(`pick('destinations', 3); true`)
	ctrl("x", "KeyX", 88)
	b.waitFor("cut is refused while the list follows the router", `said().startsWith('Nothing cut: Follow the router is on') && !!li('destinations', 3)`)
	ctrl("c", "KeyC", 67)
	b.waitFor("copy still works there", `said() === 'Copied 1 destination.'`)
	paste()
	b.waitFor("paste is refused there", `said().startsWith('Nothing pasted: Follow the router is on')`)
	b.eval(`li('sources', 3).querySelector('.sode-pages__name').click(); true`)
	undo()
	b.waitFor("Follow the router made no step, and the undo leaves it on", `said() === 'Undid move.' && order('sources') === '0,1,2,3,4,5' && document.getElementById('ed-destinations-all').checked`)
	b.eval(`document.getElementById('ed-sources-find').focus(); true`)
	undo()
	b.eval(`new Promise((ok) => setTimeout(() => ok(true), 300))`)
	b.waitFor("a text field keeps its own Ctrl+Z", `said() === 'Undid move.'`)
	b.eval(`document.getElementById('ed-sources-list').focus(); true`)
	redo()
	b.waitFor("redo from the list", `said() === 'Redid move.' && order('sources') === '0,1,3,2,4,5'`)

	// The Live Editor saves first, and the stack stays.
	b.eval(`
		window.names = () => [...document.querySelectorAll('#ed-live-view .sode-gridpanel__src[data-src]')].map((x) => [x.dataset.src, x.getBoundingClientRect()]).filter(([, r]) => r.width > 0).sort((x, y) => x[1].left - y[1].left).map(([n]) => n).slice(0, 6).join();
		window.press = (n) => document.querySelector('#ed-live-view .sode-gridpanel__src[data-src="' + n + '"]:not([hidden])').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true }));
		window.toGrid = () => document.querySelector('#ed-live-view .sode-gridpanel__scroll').focus();
		window.undos = 0;
		const f = window.fetch;
		window.fetch = (u, o) => { if (String(u).includes('/undo')) window.undos++; return f(u, o); };
		document.getElementById('ed-live').click(); true`)
	b.waitFor("the live grid shows the moved row", `names() === '0,1,3,2,4,5'`)
	b.eval(`toGrid(); true`)
	undo()
	b.waitFor("Ctrl+Z in the live grid undoes the editor's move, and the bar says so", `names() === '0,1,2,3,4,5' && document.getElementById('ed-live-state').textContent === 'Undid move.'`)
	realKey(b, "u", "KeyU", 85)
	b.eval(`new Promise((ok) => setTimeout(() => ok(true), 500))`)
	b.waitFor("u in the Live Editor's grid asks for no route Undo and opens no confirm", `window.undos === 0 && document.getElementById('confirm').hidden`)

	b.eval(`press(4); toGrid(); true`)
	b.waitFor("the pick reaches the list", `document.getElementById('ed-sources-sel').textContent === '1 selected'`)
	ctrl("x", "KeyX", 88)
	b.waitFor("Ctrl+X in the live grid cuts the picked name", `names() === '0,1,2,3,5,6' && JSON.parse(window.copied).rows[0].n === 4`)
	b.eval(`press(1); toGrid(); true`)
	paste()
	b.waitFor("Ctrl+V pastes it after the picked name, picked", `names() === '0,1,4,2,3,5' && !!document.querySelector('#ed-live-view .sode-gridpanel__src[data-src="4"][data-picked]') && !document.querySelector('#ed-live-view .sode-gridpanel__src[data-src="1"][data-picked]')`)

	b.eval(`document.getElementById('ed-live-back').click(); true`)
	b.waitFor("back to editing", `!!document.getElementById('ed-sources-list')?.offsetParent`)
	b.eval(`document.getElementById('ed-sources-list').focus(); true`)
	undo()
	b.waitFor("the live paste undoes from the editor's list", `said() === 'Undid paste.' && order('sources') === '0,1,2,3,5,6'`)
	undo()
	b.waitFor("and then the live cut", `said() === 'Undid cut.' && order('sources') === '0,1,2,3,4,5'`)
	b.check()
}
