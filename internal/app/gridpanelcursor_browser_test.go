package app

import (
	"fmt"
	"testing"
)

// realKey sends one key press through the browser's input pipeline. So default actions run as for a person, such
// as a button's Space click. vk is the Windows virtual key code.
func realKey(b *tab, key, code string, vk int) {
	b.t.Helper()
	down := map[string]any{"type": "keyDown", "key": key, "code": code, "windowsVirtualKeyCode": vk}
	if len(key) == 1 {
		down["text"] = key
	}
	b.call("Input.dispatchKeyEvent", down)
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": key, "code": code, "windowsVirtualKeyCode": vk})
}

// gridCursor is the cursor's display position as "i,j" (-1 is a bar).
const gridCursor = `(() => { const c = document.querySelector('.sode-gridpanel__cursor'); return c && !c.hidden ? c.dataset.i + ',' + c.dataset.j : ''; })()`

// gridCued is the router numbers of the names outlined for the cursor, as "src,dst" (” for none).
const gridCued = `(() => {
	const n = (k) => [...document.querySelectorAll('.sode-gridpanel__' + k + '[data-cue]:not([hidden])')].map((x) => x.dataset[k]).join('+');
	return n('src') + ',' + n('dst');
})()`

// gridCursorOn waits for the cursor to be at display row i and column j.
func gridCursorOn(b *tab, what string, i, j int) {
	b.t.Helper()
	b.waitFor(what, fmt.Sprintf(`%s === '%d,%d'`, gridCursor, i, j))
}

// One cursor for keys and the pointer. It starts at the corner. Physical h/j/k/l and arrows move it, Space stages at
// it and a second Space opens the take confirm, PageDown scrolls one screen, Home goes back. A pointer move places the cursor and
// changes nothing else on the page but the outline on its names. After a pointer move a scroll moves the cursor with the content under the
// pointer; after a key a scroll leaves it. The pointer-to-cursor delay is reported, not asserted.
func TestChromeGridPanelCursor(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Cursor grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(3, 2)+` !== null`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)
	gridCursorOn(b, "the cursor starts at the corner", -1, -1)
	b.shot("corner")

	realKey(b, "j", "KeyJ", 74)
	realKey(b, "j", "KeyJ", 74)
	realKey(b, "ArrowDown", "ArrowDown", 40)
	gridCursorOn(b, "j and the down arrow move down onto the Destination bar", 2, -1)
	b.waitFor("on a bar no names are outlined for the cursor", gridCued+` === ','`)
	b.shot("bar")
	for range 4 {
		realKey(b, "l", "KeyL", 76)
	}
	realKey(b, "ArrowRight", "ArrowRight", 39)
	realKey(b, "h", "KeyH", 72)
	gridCursorOn(b, "l, the right arrow and h move along the row", 2, 3)
	b.waitFor("the cursor sits on its crosspoint", `(() => {
		const c = document.querySelector('.sode-gridpanel__cursor').getBoundingClientRect();
		const x = `+gridXp(3, 2)+`.getBoundingClientRect();
		return Math.abs(c.left - x.left) < 1 && Math.abs(c.top - x.top) < 1 && Math.abs(c.width - x.width) < 1;
	})()`)
	b.waitFor("the cursor's source and destination names are outlined in the strong text colour", gridCued+` === '3,2' && (() => {
		const n = document.querySelector('.sode-gridpanel__src[data-cue]');
		const p = document.createElement('i'); p.style.color = 'var(--pb-text-strong)'; document.body.append(p);
		const want = getComputedStyle(p).color; p.remove();
		const s = getComputedStyle(n);
		return s.outlineStyle === 'solid' && s.outlineColor === want && parseFloat(s.outlineWidth) >= 2;
	})()`)
	b.shot("cursor")

	// A crosspoint button with focus would also click on a real Space; that would stage and unstage at once.
	b.eval(gridXp(3, 2) + `.focus(); true`)
	staged := gridXp(3, 2) + `.dataset.s === 'staged'`
	realKey(b, " ", "Space", 32)
	b.waitFor("Space stages at the cursor", staged)
	b.waitFor("one route staged", `document.getElementById('panel-staged').textContent === '1 staged'`)
	realKey(b, " ", "Space", 32)
	b.waitFor("a second Space opens the take confirm", `!document.getElementById('confirm').hidden`)
	b.eval(`document.getElementById('confirm-cancel').click(); document.getElementById('panel-clear').click(); true`)
	b.waitFor("Cancel and Clear drop it", `document.getElementById('confirm').hidden && !(`+staged+`)`)

	realKey(b, "PageDown", "PageDown", 34)
	b.waitFor("PageDown moves one screen and scrolls to show it", `(() => {
		const [i] = `+gridCursor+`.split(',').map(Number);
		const sc = document.querySelector('.sode-gridpanel__scroll');
		const c = document.querySelector('.sode-gridpanel__cursor').getBoundingClientRect();
		const r = sc.getBoundingClientRect();
		return i >= 6 && sc.scrollTop > 0 && c.bottom <= r.bottom && c.top >= r.top;
	})()`)
	realKey(b, "Home", "Home", 36)
	gridCursorOn(b, "Home goes to the corner", -1, -1)
	b.waitFor("Home scrolls to the start", `document.querySelector('.sode-gridpanel__scroll').scrollTop === 0`)

	// The pointer: moves change only the cursor, and the delay to the next frame is reported.
	at := func(src, dst int) (float64, float64) {
		r := b.eval(`(() => { const r = ` + gridXp(src, dst) + `.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`).([]any)
		return r[0].(float64), r[1].(float64)
	}
	// Home's scroll draws on a later frame; it must not count as a pointer move's work.
	b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(true))))`)
	b.eval(`(() => {
		const grid = document.getElementById('panel-grid');
		const cur = grid.querySelector('.sode-gridpanel__cursor');
		window.__muts = 0;
		// The cursor's names take or drop their outline; anything else is work a pointer move must not do.
		const cue = (r) => r.attributeName === 'data-cue' && r.target.matches('.sode-gridpanel__src, .sode-gridpanel__dst');
		window.__mo = new MutationObserver((recs) => { for (const r of recs) if (r.target !== cur && !cue(r)) { window.__muts++; (window.__what ||= []).push(r.type + ' ' + r.attributeName + ' ' + r.target.className); } });
		window.__mo.observe(grid, { childList: true, subtree: true, attributes: true, characterData: true });
		window.__lag = [];
		document.addEventListener('pointermove', (e) => {
			const t0 = e.timeStamp;
			requestAnimationFrame(() => window.__lag.push(performance.now() - t0));
		});
		return true;
	})()`)
	x0, y0 := at(1, 1)
	x1, y1 := at(4, 3)
	const moves = 20
	for k := 0; k <= moves; k++ {
		f := float64(k) / moves
		b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x0 + (x1-x0)*f, "y": y0 + (y1-y0)*f})
	}
	gridCursorOn(b, "the cursor follows the pointer", 3, 4)
	b.waitFor("its names are outlined", gridCued+` === '4,3'`)
	if same := b.eval(`getComputedStyle(` + gridXp(4, 3) + `).backgroundColor === getComputedStyle(` + gridXp(5, 3) + `).backgroundColor`); same != true {
		t.Errorf("the crosspoint under the pointer has a hover background")
	}
	if n := b.eval(`(() => { window.__mo.disconnect(); return window.__muts; })()`); n != float64(0) {
		t.Errorf("a pointer move changed %v nodes other than the cursor: %v", n, b.eval(`window.__what.slice(0, 6)`))
	}
	lag := b.eval(`(() => { const l = window.__lag.slice().sort((a, b) => a - b); return [l.length, l[l.length >> 1], l[l.length - 1]]; })()`).([]any)
	t.Logf("pointer to cursor: %v moves, median %.1f ms, most %.1f ms to the next frame", lag[0], lag[1], lag[2])

	// After a pointer move, a scroll moves the cursor to what is now under the pointer.
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": x1, "y": y1, "deltaX": 0, "deltaY": 400})
	b.waitFor("a scroll after the pointer moves the cursor with it", `(() => {
		const [i, j] = `+gridCursor+`.split(',').map(Number);
		const sc = document.querySelector('.sode-gridpanel__scroll');
		return sc.scrollTop > 0 && i > 3 && j === 4 && document.elementsFromPoint(`+fmt.Sprint(x1)+`, `+fmt.Sprint(y1)+`)
			.some((n) => n.matches?.('.sode-gridpanel__xp') && n.dataset.dst === String(i) && n.dataset.src === String(j));
	})()`)

	// After a key, a scroll leaves the cursor where the key put it.
	realKey(b, "k", "KeyK", 75)
	want := b.eval(gridCursor).(string)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": x1, "y": y1, "deltaX": 0, "deltaY": 200})
	b.waitFor("the scroll ran", `document.querySelector('.sode-gridpanel__scroll').scrollTop > 400`)
	b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(true))))`)
	if got := b.eval(gridCursor).(string); got != want {
		t.Errorf("a scroll after a key moved the cursor from %s to %s", want, got)
	}

	// With two sources picked, a press uses them and not the cursor's column, so only the targets are outlined.
	cuedSrc := `getComputedStyle(document.querySelector('.sode-gridpanel__src[data-src="4"]')).outlineStyle`
	b.waitFor("the cursor's source is outlined", cuedSrc+` === 'solid'`)
	b.eval(`(() => {
		const src = (n) => document.querySelector('.sode-gridpanel__src[data-src="' + n + '"]');
		src(1).click();
		src(2).dispatchEvent(new MouseEvent('click', { bubbles: true, ctrlKey: true, metaKey: true }));
		return true;
	})()`)
	b.waitFor("a press preview hides the cursor's outline", `!!document.querySelector('.sode-gridpanel__src[data-target]') && `+cuedSrc+` === 'none'`)
	b.check()
}
