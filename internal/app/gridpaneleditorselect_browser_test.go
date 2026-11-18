package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
)

// asGrid makes the new panel open in the editor a Grid panel, by the Panel type switch. A new Grid panel starts with
// no items. all fills both lists with every router item: Follow the router on, then off, so the rows can move.
func asGrid(b *tab, all bool) {
	b.waitFor("the type switch is built", `!!document.getElementById('ed-set-type-grid')`)
	b.eval(`document.getElementById('ed-set-type-grid').click(); true`)
	b.waitFor("the lists show", `!!document.getElementById('ed-sources-col')?.offsetParent`)
	if all {
		b.eval(`for (const k of ['sources', 'destinations']) { const s = document.getElementById('ed-' + k + '-all'); s.click(); s.click(); } true`)
		b.waitFor("both lists hold every router item, not following", `!document.getElementById('ed-sources-all').checked && !document.getElementById('ed-destinations-all').checked && document.getElementById('ed-sources-following').hidden`)
	}
}

// The Grid editor at 2500 sources. Sources and Destinations are side by side on the left. The Cell, Panel and Access
// sidebar is on the right, at most 85% of the window high, and scrolls inside itself. In the panel's
// list, click, Shift-click and Ctrl-click select rows. The Cell tab sets Icon A on every selected row. It runs Icon B
// down them in the order the panel page draws them, pinned rows first. A drag from a grip moves the selection as a
// block, and so do Alt+arrows, Ctrl+arrows and Cmd+Shift+arrows. A row's Pin and Remove act on the whole selection.
func TestChromeGridPanelEditorSelect(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t, "SIM_SOURCES=2500", "SIM_DESTS=750")
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	asGrid(b, true)
	b.waitFor("the lists fill", `!!document.querySelector('#ed-sources-list li[data-id="0"]') && !!document.querySelector('#ed-destinations-list li[data-id="0"]')`)
	box := func(id string) string { return `document.getElementById('` + id + `').getBoundingClientRect()` }
	b.waitFor("Sources, Destinations and the sidebar side by side, the sidebar a third or less", `(() => {
		const s = `+box("ed-sources-col")+`, d = `+box("ed-destinations-col")+`, side = `+box("ed-side")+`;
		return s.width > 0 && s.right <= d.left && d.right <= side.left && Math.abs(s.top - d.top) < 4 && side.width <= innerWidth / 3 + 1;
	})()`)
	b.waitFor("the sidebar keeps Cell, Panel and Access", `[...document.querySelectorAll('#ed-side .pb-tab')].map((x) => x.textContent).join() === 'Cell,Panel,Access'`)
	b.waitFor("the sidebar is at most 85% of the window and scrolls inside itself", `(() => {
		const s = document.getElementById('ed-side');
		return s.getBoundingClientRect().height <= innerHeight * 0.85 + 1 && s.scrollHeight > s.clientHeight && getComputedStyle(s).overflowY === 'auto';
	})()`)
	b.shot("lists-wide")
	if n := int(b.eval(`document.getElementsByTagName('*').length`).(float64)); n > 8000 {
		t.Errorf("the editor holds %d elements with both lists, want at most 8000", n)
	}

	b.eval(`
		window.li = (n) => document.querySelector('#ed-sources-list li[data-id="' + n + '"]');
		window.pick = (n, mods) => li(n).querySelector('.sode-pages__name').dispatchEvent(new MouseEvent('click', { bubbles: true, ...mods }));
		window.picked = () => [...document.querySelectorAll('#ed-sources-list li[data-selected]')].map((x) => Number(x.dataset.id)).sort((a, b) => a - b).join();
		window.cellTab = () => document.getElementById('panel-ed-cell');
		true`)
	b.waitFor("row 8 drawn", `!!li(8)`)
	b.eval(`pick(2); pick(5, { shiftKey: true }); pick(8, { ctrlKey: true }); true`)
	b.waitFor("click, Shift-click and Ctrl-click select five rows, and the Cell tab opens on them", `picked() === '2,3,4,5,8' && document.getElementById('ed-sources-sel').textContent === '5 selected' && document.getElementById('tab-ed-cell').getAttribute('aria-selected') === 'true' && document.getElementById('ed-cell-kind').textContent === '5 sources selected'`)

	// The Cell tab: Icon A on all five, then Icon B from 7 down the rows in list order. The bar's Artwork opens its picker.
	b.eval(`document.getElementById('tab-ed-panel').click(); document.getElementById('ed-sources-sel-art').click(); true`)
	b.waitFor("Artwork selects the Cell tab with Icon A choices open", `document.getElementById('tab-ed-cell').getAttribute('aria-selected') === 'true' && !!cellTab().querySelector('button[aria-label="camera"]')?.offsetParent`)
	b.eval(`cellTab().querySelector('button[aria-label="camera"]').click(); cellTab().querySelector('[role="group"][aria-label="Icon B"] button[aria-label="7"]').click(); true`)
	b.waitFor("every selected row shows its artwork, and only those", `[2, 3, 4, 5, 8].every((n) => !!li(n).querySelector('.sode-edge__art')) && !li(6).querySelector('.sode-edge__art')`)
	b.eval(`[...cellTab().querySelectorAll('.pb-btn')].find((x) => x.textContent === 'Hide text' && !x.hidden).click(); true`)
	b.waitFor("Hide text on the rows sets it on all five, and the button turns to Show text", `[...cellTab().querySelectorAll('.pb-btn')].some((x) => x.textContent === 'Show text' && !x.hidden)`)
	b.shot("selected")

	// A selection in Destinations ends the one in Sources, so the Cell tab has one target.
	b.eval(`document.querySelector('#ed-destinations-list li[data-id="1"] .sode-pages__name').click(); true`)
	b.waitFor("the Cell tab moves to the destination", `picked() === '' && document.getElementById('ed-cell-kind').textContent.startsWith('Destination 2')`)
	b.eval(`pick(2); pick(5, { shiftKey: true }); pick(8, { ctrlKey: true }); true`)
	b.waitFor("the five are selected again", `picked() === '2,3,4,5,8'`)

	// A drag from the grip of a selected row moves all five, in list order, before row 0 (the top half of row 0).
	b.eval(`(() => {
		const dt = new DataTransfer();
		const to = li(0);
		const r = to.getBoundingClientRect();
		const at = { bubbles: true, cancelable: true, dataTransfer: dt, clientX: r.left + 10, clientY: r.top + 2 };
		li(8).querySelector('.sode-grip').dispatchEvent(new DragEvent('dragstart', { bubbles: true, dataTransfer: dt }));
		window.draggedDuring = [...document.querySelectorAll('#ed-sources-list li[data-dragged]')].length;
		to.dispatchEvent(new DragEvent('dragover', at));
		to.dispatchEvent(new DragEvent('drop', at));
		return true;
	})()`)
	order := `[...document.querySelectorAll('#ed-sources-list li')].slice(0, 9).map((x) => x.dataset.id).join()`
	b.waitFor("the drag moves the block to the top", order+` === '2,3,4,5,8,0,1,6,7' && picked() === '2,3,4,5,8'`)
	if n := b.eval(`window.draggedDuring`); n != 5.0 {
		t.Errorf("%v rows marked as dragged, want the five selected", n)
	}
	if b.eval(`li(2).draggable || !li(2).querySelector('.sode-grip').draggable || !li(2).querySelector('.sode-pages__head').draggable || li(2).querySelector('[data-act="pin"]').draggable`) != false {
		t.Errorf("a drag starts on the row itself or a button, want only on its grip and its text")
	}

	// From the keyboard: Alt+Down and Cmd+Shift+Down move the block down a row each; Ctrl+Shift+Up moves it back.
	key := func(k string, mods string) {
		b.eval(fmt.Sprintf(`document.getElementById('ed-sources-list').focus(); document.getElementById('ed-sources-list').dispatchEvent(new KeyboardEvent('keydown', { key: %q, bubbles: true, cancelable: true, %s })); true`, k, mods))
	}
	key("ArrowDown", "altKey: true")
	key("ArrowDown", "metaKey: true, shiftKey: true")
	b.waitFor("Alt+Down and Cmd+Shift+Down move the block past two rows", order+` === '0,1,2,3,4,5,8,6,7'`)
	key("ArrowUp", "ctrlKey: true, shiftKey: true")
	b.waitFor("Ctrl+Shift+Up moves it back a row", order+` === '0,2,3,4,5,8,1,6,7'`)
	key("ArrowDown", "ctrlKey: true")
	b.waitFor("Ctrl+Down moves it down a row", order+` === '0,1,2,3,4,5,8,6,7'`)
	key("ArrowUp", "ctrlKey: true")
	b.waitFor("Ctrl+Up moves it back", order+` === '0,2,3,4,5,8,1,6,7'`)
	key("Home", "altKey: true")
	b.waitFor("Alt+Home moves it to the top", order+` === '2,3,4,5,8,0,1,6,7'`)

	// A row's Pin and Remove act on the whole selection when the row is in it.
	b.eval(`li(3).querySelector('[data-act="pin"]').click(); true`)
	b.waitFor("a row's Pin pins the five", `[2, 3, 4, 5, 8].every((n) => li(n).querySelector('[data-act="pin"]').getAttribute('aria-pressed') === 'true') && li(0).querySelector('[data-act="pin"]').getAttribute('aria-pressed') === 'false'`)
	b.eval(`li(3).querySelector('[data-act="pin"]').click(); true`)
	b.waitFor("and Unpin unpins them", `[2, 3, 4, 5, 8].every((n) => li(n).querySelector('[data-act="pin"]').getAttribute('aria-pressed') === 'false')`)

	// Icon B runs in the order the panel page draws the rows: a pinned row first, though it is later in the list.
	key("Escape", "")
	b.eval(`li(1).querySelector('[data-act="pin"]').click(); true`)
	b.waitFor("row 1 alone is pinned", `li(1).querySelector('[data-act="pin"]').getAttribute('aria-pressed') === 'true' && picked() === ''`)
	b.eval(`pick(0); pick(1, { ctrlKey: true }); true`)
	b.waitFor("rows 0 and 1 are selected", `picked() === '0,1'`)
	b.eval(`cellTab().querySelector('[role="group"][aria-label="Icon B"] button[aria-label="1"]').click(); true`)
	b.waitFor("the pinned row gets the first glyph", `(() => {
		const b = (n) => li(n).querySelector('.sode-edge__art .sode-panel__art-b')?.textContent;
		return b(1) === '1' && b(0) === '2';
	})()`)
	b.eval(`li(1).querySelector('[data-act="pin"]').click(); true`)
	b.waitFor("and both unpin", `li(1).querySelector('[data-act="pin"]').getAttribute('aria-pressed') === 'false'`)

	key("ArrowDown", "")
	b.waitFor("an arrow moves the cursor and selects that row alone", `picked().split(',').length === 1`)
	key("ArrowDown", "shiftKey: true")
	key("ArrowDown", "shiftKey: true")
	b.waitFor("Shift+Down selects a range from the anchor", `picked().split(',').length === 3`)
	key("Escape", "")
	b.waitFor("Esc selects none", `picked() === '' && document.getElementById('ed-sources-sel-pin').disabled`)

	// A selection of every row: Ctrl+A and an Icon A for 2500 rows stay quick.
	// The best of three, so one slow frame on a loaded runner does not fail it. An odd count leaves the camera on.
	ms := b.eval(`(async () => {
		let best = Infinity;
		for (let i = 0; i < 3; i++) {
			const t0 = performance.now();
			const l = document.getElementById('ed-sources-list');
			l.focus();
			l.dispatchEvent(new KeyboardEvent('keydown', { key: 'a', code: 'KeyA', ctrlKey: true, bubbles: true, cancelable: true }));
			cellTab().querySelector('button[aria-label="camera"]').click();
			await new Promise((r) => requestAnimationFrame(() => setTimeout(r, 0)));
			best = Math.min(best, performance.now() - t0);
		}
		return best;
	})()`).(float64)
	t.Logf("Ctrl+A and Icon A on 2500 rows: %.0f ms to the next frame, best of three", ms)
	if ms > 300 {
		t.Errorf("Ctrl+A and Icon A on 2500 rows took %.0f ms, want under 300", ms)
	}
	b.waitFor("Ctrl+A selects all 2500", `document.getElementById('ed-sources-sel').textContent === '2500 selected'`)
	key("Escape", "")

	// Remove on a row in a selection of two takes both.
	b.eval(`pick(4); pick(5, { shiftKey: true }); li(5).querySelector('[data-act="delete"]').click(); true`)
	b.waitFor("a row's Remove takes the selection", `!li(4) && !li(5) && !!li(3) && picked() === ''`)

	b.eval(`document.getElementById('ed-name').value = 'Selected grid'; document.getElementById('ed-name').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('ed-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved' && location.search.startsWith('?panel=')`)
	id := regexp.MustCompile(`panel=([\w-]+)`).FindStringSubmatch(b.eval(`location.search`).(string))[1]
	srcs, _ := readPanel(t, a, ana, id)["sources"].(map[string]any)
	members, _ := srcs["members"].([]any)
	head, _ := json.Marshal(members[:6])
	// Row 1 went to the top when pinned and stayed there when unpinned.
	if len(members) != 2498 || string(head) != `[1,2,3,8,0,6]` {
		t.Errorf("members start %s (%d), want [1,2,3,8,0,6] of 2498", head, len(members))
	}
	items, _ := srcs["items"].(map[string]any)
	if it, _ := items["8"].(map[string]any); it["art"] != "icon:camera" || it["artB"] != "11" || it["hideText"] != true {
		t.Errorf("item 8 %v, want Icon A camera, Icon B 11 (the fifth from 7) and Hide text", it)
	}
	if it, _ := items["100"].(map[string]any); it["art"] != "icon:camera" || it["artB"] != nil {
		t.Errorf("item 100 %v, want Icon A camera alone", it)
	}
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 800, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.waitFor("Sources above Destinations on a narrow window", `(() => { const s = `+box("ed-sources-col")+`, d = `+box("ed-destinations-col")+`; return s.width > 0 && s.bottom <= d.top; })()`)
	b.check()
}
