package app

import "testing"

// Toolbar and sidebar chrome: the state sits beside Save, Copy link shows for a user panel, the sidebar
// shows the link, the unassigned count says what it counts, and bare letters work inside the smart menu.
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
	b.waitFor("only Merge shows a shortcut, Ctrl or Cmd+M", `/^(Ctrl\+|⌘)M$/.test([...document.querySelectorAll('.sode-ed__menu .sode-ed__keys')].map(n => n.textContent).join()) && document.querySelector('.sode-ed__menu').textContent.includes('Type: Source')`)
	// Bare M does nothing in the menu. Ctrl+M merges, the item reads Unmerge, and Cmd+M on the grid undoes it.
	b.eval(`window.menuKey = (key, mods = {}) => document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true, ...mods }));
		document.querySelector('.sode-ed__menu .pb-menu__item').focus(); menuKey('m')`)
	b.waitFor("bare M did nothing", `document.querySelectorAll('.sode-ed__sq').length === 32`)
	b.eval(`menuKey('m', { ctrlKey: true })`)
	b.waitFor("merged by Ctrl+M", `document.querySelectorAll('.sode-ed__sq').length < 32 && document.querySelector('.sode-ed__menu .pb-menu__item').textContent.includes('Unmerge')`)
	b.eval(`press({ key: 'm', metaKey: true })`)
	b.waitFor("unmerged by Cmd+M on the grid", `document.querySelectorAll('.sode-ed__sq').length === 32`)
	b.eval(`document.body.dispatchEvent(new Event('pointerdown', { bubbles: true }))`)
	b.waitFor("a click outside closed the menu", `document.querySelector('.sode-ed__menu').hidden`)
	b.eval(`press({ key: 'ArrowDown' }); press({ key: 'ArrowRight', shiftKey: true }); document.querySelector('.sode-ed__menu .pb-menu__item').focus(); menuKey('f'); true`)
	b.waitFor("bare F in the menu did not open Find", `document.activeElement !== document.querySelector('.sode-find input')`)
	b.eval(`grid.focus(); press({ key: 'f' })`)
	b.waitFor("bare typing on the grid still opens Find", `document.activeElement === document.querySelector('.sode-find input')`)
	b.eval(`document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'm', bubbles: true, cancelable: true }))`)
	if n := b.eval(`document.querySelectorAll('.sode-ed__sq').length`); n != float64(32) {
		t.Errorf("typing M in the Find box changed the grid: %v squares", n)
	}
	b.shot("chrome")
	b.check()
}
