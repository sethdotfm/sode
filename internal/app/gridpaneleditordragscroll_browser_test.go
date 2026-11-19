package app

import (
	"testing"
)

// A row drag held near the bottom edge of a Grid list scrolls the list to its end, and the big scroll bar moves with it. A
// drop there puts the row at the end. The same works up to the top, and the scroll stops with the drag.
func TestChromeGridPanelEditorDragScroll(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	asGrid(b, true)
	b.waitFor("the Sources list holds 200 rows and scrolls", `(() => { const l = document.getElementById('ed-sources-list'); return document.getElementById('ed-sources-state').textContent === '200' && l.scrollHeight > 3 * l.clientHeight; })()`)

	// Drag events as the browser sends them, with one DataTransfer for the whole drag. dragover goes every 50 ms, as
	// Chrome sends it while the pointer rests.
	b.eval(`
		window.list = document.getElementById('ed-sources-list');
		window.thumbTop = () => list.nextElementSibling.querySelector('.sode-bigscroll__thumb').getBoundingClientRect().top;
		window.rowOf = (n) => list.querySelector('li[data-id="' + n + '"]');
		window.fire = (node, type, y) => node.dispatchEvent(new DragEvent(type, { bubbles: true, cancelable: true, dataTransfer: dt, clientX: list.getBoundingClientRect().left + 60, clientY: y }));
		window.startDrag = (n) => { window.dt = new DataTransfer(); const g = rowOf(n).querySelector('.sode-grip'); const r = g.getBoundingClientRect(); fire(g, 'dragstart', r.top + r.height / 2); };
		window.hold = (edge) => { window.held = setInterval(() => {
			const r = list.getBoundingClientRect(), y = edge === 'bottom' ? r.bottom - 3 : r.top + 3;
			fire(document.elementFromPoint(r.left + 60, y), 'dragover', y);
		}, 50); };
		window.release = () => clearInterval(held);
		window.dropOn = (n, after) => { const li = rowOf(n), r = li.getBoundingClientRect(); fire(li, 'drop', after ? r.bottom - 2 : r.top + 2); };
		window.lastId = () => [...list.querySelectorAll('li[data-id]')].sort((x, y) => x.getBoundingClientRect().top - y.getBoundingClientRect().top).at(-1).dataset.id;
		window.firstId = () => [...list.querySelectorAll('li[data-id]')].sort((x, y) => x.getBoundingClientRect().top - y.getBoundingClientRect().top)[0].dataset.id;
		window.thumbWas = thumbTop();
		startDrag(0); hold('bottom'); true`)
	b.waitFor("a drag held at the bottom edge scrolls the list to its end", `list.scrollTop >= list.scrollHeight - list.clientHeight - 1`)
	b.waitFor("the big scroll bar's thumb moved down with it", `thumbTop() > thumbWas + 100`)
	b.eval(`release(); dropOn(199, true); true`)
	b.waitFor("the drop past the first view puts the row at the end", `lastId() === '0'`)

	b.eval(`list.scrollTop = 200; true`)
	b.waitFor("the drag ended, so the scroll stays where it is put", `new Promise((ok) => setTimeout(() => ok(list.scrollTop === 200), 600))`)

	b.eval(`list.scrollTop = list.scrollHeight; true`)
	b.waitFor("the row is in view at the end", `!!rowOf(0)`)
	b.eval(`window.thumbWas = thumbTop(); startDrag(0); hold('top'); true`)
	b.waitFor("a drag held at the top edge scrolls the list to its start", `list.scrollTop === 0`)
	b.waitFor("the thumb moved up with it", `thumbTop() < thumbWas - 100`)
	b.eval(`release(); dropOn(1, false); true`)
	b.waitFor("the drop puts the row back first", `firstId() === '0'`)
	b.check()
}
