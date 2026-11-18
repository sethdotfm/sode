package app

import (
	"encoding/json"
	"fmt"
	"regexp"
	"testing"
)

// A Grid panel's Sources list at 2500 sources. Add from the router draws only the rows in view. It lists only what
// the panel does not have. A typed range uses the numbers as shown (offset 1), and Shift+Enter or Add all appends
// every match. A click selects (Shift-click a range, Ctrl-click one more) and + adds the selection. Enter adds the
// highlighted match, the arrows move the highlight, and the next match slides into it. Save keeps the list.
func TestChromeGridPanelEditorChunk(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t, "SIM_SOURCES=2500", "SIM_DESTS=750")
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	asGrid(b, false)

	rows := `document.querySelectorAll('#ed-sources-router [data-n]:not([hidden])')`
	b.waitFor("the router list shows its first rows", `!!document.querySelector('#ed-sources-router [data-n="0"]:not([hidden])')`)
	if n := int(b.eval(`document.querySelectorAll('#ed-sources-router .sode-rlist__row').length`).(float64)); n > 200 {
		t.Fatalf("the router list draws %d rows for 2500 sources", n)
	}
	b.eval(`const r = document.getElementById('ed-sources-router'); r.scrollTop = r.scrollHeight; true`)
	b.waitFor("the last source shows at the end", `!!document.querySelector('#ed-sources-router [data-n="2499"]:not([hidden])')`)

	// Follow the router: the panel's own list has 2500 rows, and nothing is left to add. Only the rows in view have nodes, and an
	// edit draws only those again. Every row drawn cost 66,000 elements, 33,000 listeners and 800 ms an edit.
	b.eval(`document.getElementById('ed-sources-all').click(); true`)
	b.waitFor("nothing is left to add", `!document.querySelector('#ed-sources-router [data-n]:not([hidden])') && document.getElementById('ed-sources-shown').textContent === '0'`)
	b.waitFor("the panel's list has every source", `document.getElementById('ed-sources-state').textContent === '2500' && document.getElementById('ed-sources-all').checked`)
	b.waitFor("the panel's list shows its first rows", `!!document.querySelector('#ed-sources-list li[data-id="0"]')`)
	if n := int(b.eval(`document.querySelectorAll('#ed-sources-list li').length`).(float64)); n > 60 {
		t.Errorf("the panel's list draws %d rows for 2500 sources, want at most 60", n)
	}
	if n := int(b.eval(`document.getElementsByTagName('*').length`).(float64)); n > 8000 {
		t.Errorf("the editor holds %d elements, want at most 8000", n)
	}
	// The best of three (pin, unpin, pin), so one slow frame on a loaded runner does not fail it.
	pin := b.eval(`(async () => {
		let best = Infinity;
		for (let i = 0; i < 3; i++) {
			const t0 = performance.now();
			document.querySelector('#ed-sources-list li[data-id="3"] [data-act="pin"]').click();
			await new Promise((r) => requestAnimationFrame(() => setTimeout(r, 0)));
			best = Math.min(best, performance.now() - t0);
		}
		return best;
	})()`).(float64)
	t.Logf("a Pin with 2500 rows: %.0f ms to the next frame, best of three", pin)
	// About 20 ms windowed and about 900 ms with every row drawn; the margin is for a loaded CI runner.
	if pin > 300 {
		t.Errorf("a Pin took %.0f ms to the next frame, want under 300", pin)
	}
	b.waitFor("the pin shows", `document.querySelector('#ed-sources-list li[data-id="3"] [data-act="pin"]').getAttribute('aria-pressed') === 'true'`)
	b.eval(`const l = document.getElementById('ed-sources-list'); l.scrollTop = l.scrollHeight; true`)
	b.waitFor("the last source shows at the end of the panel's list", `!!document.querySelector('#ed-sources-list li[data-id="2499"]')`)

	b.waitFor("Clear is off while the list follows the router", `document.getElementById('ed-sources-clear').disabled`)
	b.eval(`document.getElementById('ed-sources-all').click(); true`)
	b.waitFor("Follow the router off keeps every source as the panel's own list", `!document.getElementById('ed-sources-all').checked && document.getElementById('ed-sources-state').textContent === '2500' && !document.getElementById('ed-sources-clear').disabled && !document.getElementById('ed-sources-find').disabled`)
	b.eval(`document.getElementById('ed-sources-clear').click(); true`)
	b.waitFor("Clear empties the panel's list and shows the empty state", `document.querySelectorAll('#ed-sources-list li').length === 0 && !document.getElementById('ed-sources-empty').hidden && document.getElementById('ed-sources-acts').hidden && !document.getElementById('ed-sources-all').checked`)

	find := func(text string) {
		b.eval(fmt.Sprintf(`(() => { const f = document.getElementById('ed-sources-find'); f.focus(); f.value = %q; f.dispatchEvent(new Event('input', { bubbles: true })); return true })()`, text))
	}
	key := func(k, mods string) {
		b.eval(fmt.Sprintf(`document.getElementById('ed-sources-find').dispatchEvent(new KeyboardEvent('keydown', { key: %q, bubbles: true, cancelable: true, %s })); true`, k, mods))
	}
	// The panel's list draws only the rows in view, so it is read a screen at a time.
	members := `(async () => {
		const l = document.getElementById('ed-sources-list');
		const ids = new Map();
		const frame = () => new Promise((r) => requestAnimationFrame(() => setTimeout(r, 0)));
		for (let top = 0; ; top += Math.max(1, l.clientHeight / 2)) {
			l.scrollTop = top;
			await frame();
			for (const li of l.querySelectorAll('li')) ids.set(li.querySelector('[data-i]').dataset.i, li.dataset.id);
			if (l.scrollTop + l.clientHeight >= l.scrollHeight - 1) break;
		}
		l.scrollTop = 0;
		return [...ids].sort((a, b) => a[0] - b[0]).map((x) => x[1]).join();
	})()`
	span := func(lo, hi int) (s string) {
		for n := lo; n <= hi; n++ {
			if s != "" {
				s += ","
			}
			s += fmt.Sprint(n)
		}
		return s
	}

	find("101-140")
	b.waitFor("the range shows forty rows, from shown number 101", `document.getElementById('ed-sources-shown').textContent.startsWith('40 ') && !!document.querySelector('#ed-sources-router [data-n="100"]:not([hidden])') && !document.querySelector('#ed-sources-router [data-n="99"]:not([hidden])')`)
	b.waitFor("Add all names the matches", `document.getElementById('ed-sources-add').textContent === 'Add all 40' && !document.getElementById('ed-sources-add').hidden`)
	b.eval(`document.getElementById('ed-sources-add').click(); true`)
	b.waitFor("Add all appends the range in router order", `(async () => (await `+members+`) === '`+span(100, 139)+`')()`)

	find("131-150")
	key("Enter", "shiftKey: true")
	b.waitFor("Shift+Enter adds every match and skips what the panel has", `(async () => (await `+members+`) === '`+span(100, 149)+`')()`)

	// Router 12..23 are CAM 1..12; "cam 1" shows CAM 1, 10, 11 and 12.
	click := func(n int, mods string) {
		b.eval(fmt.Sprintf(`document.querySelector('#ed-sources-router [data-n="%d"]:not([hidden])').dispatchEvent(new MouseEvent('click', { bubbles: true, %s })); true`, n, mods))
	}
	find("bars")
	b.waitFor("Find shows Bars", rows+`.length === 1`)
	click(1, "")
	b.waitFor("a click selects Bars and adds nothing", `document.getElementById('ed-sources-rsel').textContent === '1 selected' && document.querySelector('#ed-sources-router [data-n="1"]').getAttribute('aria-selected') === 'true' && document.getElementById('ed-sources-state').textContent === '50'`)
	b.eval(`document.getElementById('ed-sources-add-sel').click(); true`)
	b.waitFor("+ adds Bars at the end", `(async () => (await `+members+`) === '`+span(100, 149)+`,1')()`)
	find("cam 1")
	b.waitFor("Find shows four cameras, the first highlighted", rows+`.length === 4 && document.getElementById('ed-sources-find').getAttribute('aria-activedescendant') === 'ed-sources-opt-12'`)
	key("Enter", "")
	b.waitFor("Enter adds the highlighted camera, and the next one slides into the highlight", `(async () => (await `+members+`) === '`+span(100, 149)+`,1,12' && document.getElementById('ed-sources-find').getAttribute('aria-activedescendant') === 'ed-sources-opt-21')()`)
	key("ArrowDown", "")
	b.waitFor("Down moves the highlight, and one row is marked", `document.getElementById('ed-sources-find').getAttribute('aria-activedescendant') === 'ed-sources-opt-22' && document.querySelectorAll('#ed-sources-router .is-cur:not([hidden])').length === 1`)
	b.shot("highlight")
	key("Enter", "")
	b.waitFor("Enter adds it, and Find keeps its text and the focus", `(async () => (await `+members+`) === '`+span(100, 149)+`,1,12,22' && document.activeElement.id === 'ed-sources-find')()`)
	b.waitFor("what the panel has leaves Add from the router", `!document.querySelector('#ed-sources-router [data-n="12"]:not([hidden])') && `+rows+`.length === 2`)

	// CAM 2, Shift-click CAM 4, Ctrl-click CAM 6, then + adds the four in router order.
	find("14-21")
	b.waitFor("Find shows the cameras left", `!!document.querySelector('#ed-sources-router [data-n="17"]:not([hidden])')`)
	click(13, "")
	click(15, "shiftKey: true")
	click(17, "ctrlKey: true")
	b.waitFor("four are selected", `document.getElementById('ed-sources-rsel').textContent === '4 selected'`)
	b.eval(`document.getElementById('ed-sources-add-sel').click(); true`)
	b.waitFor("+ adds them as a group, and the selection empties", `(async () => (await `+members+`) === '`+span(100, 149)+`,1,12,22,13,14,15,17' && document.getElementById('ed-sources-rsel').textContent === '0 selected')()`)
	b.shot("chunk")

	b.eval(`document.getElementById('ed-name').value = 'Chunk grid'; document.getElementById('ed-name').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('ed-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved' && location.search.startsWith('?panel=')`)
	id := regexp.MustCompile(`panel=([\w-]+)`).FindStringSubmatch(b.eval(`location.search`).(string))[1]
	srcs, _ := readPanel(t, a, ana, id)["sources"].(map[string]any)
	got, _ := json.Marshal(srcs["members"])
	if want := "[" + span(100, 149) + ",1,12,22,13,14,15,17]"; string(got) != want {
		t.Errorf("members %s, want %s", got, want)
	}
	b.check()
}
