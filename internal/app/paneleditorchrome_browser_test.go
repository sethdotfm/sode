package app

import "testing"

// Toolbar and sidebar chrome: the state sits beside Save, Copy link shows for a user panel, the sidebar
// shows the link, the unassigned count says what it counts, and Alt+M toggles merge.
func TestChromeEditorChrome(t *testing.T) {
	b, _, _, id := editorTab(t, `{"name":"Wall","rows":4,"cols":8,"cells":[{"r":1,"c":1,"kind":"source","n":1},{"r":1,"c":2,"kind":"source","n":2},{"r":2,"c":1,"kind":"destination"}]}`)
	b.waitFor("count says what it counts", `document.getElementById('ed-todo').textContent === '30 unassigned squares'`)
	b.waitFor("state beside Save", `document.getElementById('ed-state').parentNode === document.getElementById('ed-save').parentNode`)
	b.waitFor("Copy link beside Open panel", `!document.getElementById('ed-copylink').hidden && document.getElementById('ed-copylink').parentNode === document.getElementById('ed-open').parentNode`)
	b.waitFor("link in Identity", `document.querySelector('#panel-ed-panel .sode-linkrow__text')?.value.endsWith('/panels/`+id+`')`)
	b.waitFor("picker at least 14rem", `document.getElementById('ed-pick').getBoundingClientRect().width >= 14 * parseFloat(getComputedStyle(document.documentElement).fontSize)`)
	b.eval(`window.copied = ''; navigator.clipboard.writeText = async (t) => { window.copied = t }; document.getElementById('ed-copylink').click()`)
	b.waitFor("link copied", `window.copied.endsWith('/panels/`+id+`') && document.getElementById('ed-status').textContent === 'Link copied.'`)

	b.eval(`window.grid = document.getElementById('ed-grid');
		window.press = (init) => grid.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));
		press({ key: 'ArrowDown' }); press({ key: 'ArrowRight', shiftKey: true }); true`)
	b.waitFor("menu shows its shortcuts", `[...document.querySelectorAll('.sode-ed__menu .sode-ed__keys')].map(n => n.textContent).join() === 'Alt+M,Alt+M,Alt+T,Alt+F'`)
	b.eval(`press({ key: 'µ', code: 'KeyM', altKey: true })`)
	b.waitFor("merged by the key", `document.querySelectorAll('.sode-ed__sq').length < 32`)
	b.eval(`press({ key: 'µ', code: 'KeyM', altKey: true })`)
	b.waitFor("unmerged by the same key", `document.querySelectorAll('.sode-ed__sq').length === 32`)
	b.eval(`press({ key: '†', code: 'KeyT', altKey: true })`)
	b.waitFor("Type menu focused", `document.activeElement?.textContent.includes('Type')`)
	b.shot("chrome")
	b.check()
}
