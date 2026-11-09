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

// A click in the sidebar puts the smart menu away without dropping the selection, so the Cell tab keeps its cell and
// the menu no longer sits over the fields; the next grid click brings it back.
func TestChromeEditorMenuLeavesSidebar(t *testing.T) {
	b, _, _, _ := editorTab(t, `{"name":"Wall","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"take"}]}`)
	b.waitFor("sidebar built", `!!document.querySelector('#ed-side .pb-tab')`)
	b.eval(`window.down = (el) => { const r = el.getBoundingClientRect(); el.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); };
		window.sq = () => document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]');
		down(sq()); true`)
	b.waitFor("the menu is beside the selection", `!document.querySelector('.sode-ed__menu').hidden && !!document.querySelector('.sode-ed__sq.is-sel')`)
	b.waitFor("the Cell tab opened", `document.querySelector('#ed-side .pb-tab[aria-selected="true"]')?.textContent === 'Cell'`)
	b.eval(`down(document.getElementById('ed-cell-caption')); document.getElementById('ed-cell-caption').focus(); true`)
	b.waitFor("a click in the sidebar hid the menu", `document.querySelector('.sode-ed__menu').hidden`)
	b.waitFor("and kept the selection", `!!document.querySelector('.sode-ed__sq.is-sel') && document.querySelector('#ed-side .pb-tab[aria-selected="true"]')?.textContent === 'Cell'`)
	b.eval(`const c = document.getElementById('ed-cell-caption'); c.value = 'Go'; c.dispatchEvent(new Event('input', { bubbles: true })); true`)
	b.waitFor("editing the cell kept the menu away", `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-ed__text')?.textContent === 'Go' && document.querySelector('.sode-ed__menu').hidden`)
	b.eval(`down(sq()); true`)
	b.waitFor("a grid click brings the menu back", `!document.querySelector('.sode-ed__menu').hidden`)
	b.eval(`down(document.getElementById('ed-side')); true`)
	b.waitFor("the menu goes away again", `document.querySelector('.sode-ed__menu').hidden`)
	b.eval(`document.getElementById('ed-grid').focus(); document.getElementById('ed-grid').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true, cancelable: true })); true`)
	b.waitFor("an arrow key brings it back too", `!document.querySelector('.sode-ed__menu').hidden && !!document.querySelector('.sode-ed__sq.is-sel[data-c="2"]')`)
	b.check()
}

// The Find flyout opened from a selection at the bottom of a tall grid stays inside the viewport.
func TestChromeEditorFindFitsViewport(t *testing.T) {
	b, _, _, _ := editorTab(t, `{"name":"Tall","rows":12,"cols":8,"cells":[]}`)
	b.waitFor("grid drawn", `document.querySelectorAll('#ed-grid .sode-ed__sq').length === 96`)
	// Bootstrap's reboot scrolls smoothly, so the square is pressed only once it has arrived in view.
	b.eval(`window.sq = document.querySelector('.sode-ed__sq[data-r="12"][data-c="8"]'); window.scrollTo(0, 1e6); true`)
	b.waitFor("bottom right square in view", `(() => { const r = sq.getBoundingClientRect(); return r.top >= 0 && r.bottom <= document.documentElement.clientHeight + 1; })()`)
	b.eval(`const r = sq.getBoundingClientRect(); sq.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); true`)
	b.waitFor("bottom right square selected", `!!document.querySelector('.sode-ed__sq.is-sel[data-r="12"][data-c="8"]')`)
	b.eval(`document.getElementById('ed-grid').focus(); document.getElementById('ed-grid').dispatchEvent(new KeyboardEvent('keydown', { key: 'z', bubbles: true, cancelable: true })); true`)
	b.waitFor("Find open", `document.activeElement === document.querySelector('.sode-find input')`)
	b.waitFor("the flyout sits inside the viewport", `(() => { const r = document.querySelector('.sode-find').getBoundingClientRect(); return r.height > 100 && r.top >= 0 && r.left >= 0 && r.bottom <= document.documentElement.clientHeight && r.right <= document.documentElement.clientWidth; })()`)
	b.shot("find-fit")
	b.check()
}
