package app

import (
	"fmt"
	"testing"
	"time"
)

// touchAt is the client centre of the element that expr gives, as "x,y".
func touchAt(b *tab, expr string) (float64, float64) {
	b.t.Helper()
	var x, y float64
	got := b.eval(`(() => { const r = (` + expr + `).getBoundingClientRect(); return (r.left + r.width / 2) + ',' + (r.top + r.height / 2); })()`)
	if _, err := fmt.Sscanf(got.(string), "%g,%g", &x, &y); err != nil {
		b.t.Fatalf("no place for %s: %v", expr, err)
	}
	return x, y
}

// touch puts one finger down on the element that expr gives and lifts it after hold. It uses the browser's own
// touch input, so the page gets Touch Events and the click that follows a tap.
func touch(b *tab, expr string, hold time.Duration) {
	b.t.Helper()
	x, y := touchAt(b, expr)
	b.call("Input.dispatchTouchEvent", map[string]any{"type": "touchStart", "touchPoints": []map[string]any{{"x": x, "y": y}}})
	time.Sleep(hold)
	b.call("Input.dispatchTouchEvent", map[string]any{"type": "touchEnd", "touchPoints": []map[string]any{}})
}

func tap(b *tab, expr string) { touch(b, expr, 30*time.Millisecond) }

// A Grid panel on the phone layout, by touch only, on a 2500 by 750 router. The page is one screen high: one band,
// then the grid, with at least 5 columns and 10 rows of crosspoints. The title always shows. More opens the rest of
// the band, with Find and Filter as two named choices and no field under 16 px. The band's levels follow the Levels
// box. No cursor shows. The corner gives the shown ranges. An edge scroll during a line is
// capped, even with the finger far past the edge. Taps add names to the picks, a long-press adds a range, and the
// numbered targets show with no hover. A tap on a crosspoint stages the Diagonal and the Take button opens the
// confirm. A double-tap on a name jumps and leaves the picks as they were. There is no key list.
func TestChromeGridPanelPhone(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t, "SIM_SOURCES=2500", "SIM_DESTS=750")
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Phone grid","type":"grid","settings":{"mode":"take","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	// A phone's scroll bars overlay the content; headless Chrome's would take 14 px.
	b.call("Emulation.setScrollbarsHidden", map[string]any{"hidden": true})
	b.open(srv.URL + "/panels/" + id + "?instance=Phone")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	signInOnPad(b)
	b.waitFor("the grid", gridXp(2, 2)+` !== null`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)

	xp := b.eval(`document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width`).(float64)
	if xp < 44 {
		t.Errorf("a crosspoint is %vpx on the phone, want a finger's size", xp)
	}
	if got := b.eval(`document.documentElement.scrollHeight - innerHeight`).(float64); got > 0 {
		t.Errorf("the phone page scrolls by %vpx, want one screen", got)
	}
	shown := b.eval(`(() => {
		const xp = document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width;
		const s = document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect();
		const c = document.querySelector('.sode-gridpanel__corner').getBoundingClientRect();
		return [Math.round(s.top), Math.floor((s.right - c.right) / xp), Math.floor((s.bottom - c.bottom) / xp)].join(',');
	})()`).(string)
	var top, cols, rows int
	fmt.Sscanf(shown, "%d,%d,%d", &top, &cols, &rows)
	if top > 104 || cols < 5 || rows < 10 {
		t.Errorf("grid at top %dpx shows %d by %d crosspoints, want the title and one band over it and at least 5 by 10", top, cols, rows)
	}
	b.waitFor("the title shows with More closed", `(() => { const t = document.getElementById('panel-title'); const r = t.getBoundingClientRect(); return t.textContent === 'Phone grid' && r.height > 0 && r.bottom <= document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect().top; })()`)
	cursorShown := `getComputedStyle(document.querySelector('.sode-gridpanel__cursor')).display !== 'none'`
	if b.eval(cursorShown) == true {
		t.Error("the phone shows a cursor at open")
	}
	b.waitFor("the corner gives the shown ranges", `/^1–\d+\/2500$/.test(document.querySelector('.sode-gridpanel__range--src').textContent) && /^1–\d+\/750$/.test(document.querySelector('.sode-gridpanel__range--dst').textContent)`)
	b.waitFor("the Find boxes are under More", `!document.querySelector('.sode-gridpanel__finds').offsetParent`)
	tap(b, `document.getElementById('panel-more')`)
	b.waitFor("More opens the Find boxes and the Levels box", `!!document.querySelector('.sode-gridpanel__finds').offsetParent && !!document.getElementById('panel-lvl').offsetParent`)
	b.waitFor("the open band does not widen the page", `document.documentElement.scrollWidth <= innerWidth`)
	// iOS zooms into a field under 16 px on focus and does not zoom back.
	b.waitFor("every field in the band is 16 px or more", `[...document.querySelectorAll('.sode-gridpanel__deck input:not([type=checkbox], [type=radio])')].every((i) => !i.offsetParent || parseFloat(getComputedStyle(i).fontSize) >= 16)`)
	b.waitFor("Find and Filter are two named choices, with no switch", `!!document.getElementById('panel-find-mode').offsetParent && !document.querySelector('.sode-gridpanel__find-mode').offsetParent && document.getElementById('panel-find-mode-find').checked`)
	b.shot("phone-more")
	tap(b, `document.querySelector('label[for="panel-find-mode-filter"]')`)
	b.waitFor("a tap on Filter sets Filter, and focus stays out of the boxes", `document.getElementById('panel-find-mode-filter').checked && document.querySelector('.sode-gridpanel__find-input').placeholder === 'Filter sources' && !document.activeElement.matches('input[type=text]')`)
	tap(b, `document.querySelector('label[for="panel-find-mode-find"]')`)
	b.waitFor("and back to Find", `document.querySelector('.sode-gridpanel__find-input').placeholder === 'Find sources'`)
	lvChip := `document.querySelector('.sode-gridpanel__lv').textContent`
	b.waitFor("the band shows the levels a press stages", lvChip+` === 'Levels1–2'`)
	tap(b, `document.querySelector('#panel-lvl-pick .sode__pick[data-ui="3"]')`)
	b.waitFor("a level pick changes the band's levels", lvChip+` === 'Levels1–3'`)
	tap(b, `document.querySelector('#panel-lvl-pick .sode__pick[data-ui="3"]')`)
	b.waitFor("and back", lvChip+` === 'Levels1–2'`)
	tap(b, `document.getElementById('panel-more')`)
	b.waitFor("More closes them", `!document.querySelector('.sode-gridpanel__finds').offsetParent`)

	// A line's edge scroll, with the finger dragged far past the body's top into the band.
	b.eval(`document.querySelector('.sode-gridpanel__scroll').scrollTo(0, 20000), true`)
	b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(true))))`)
	lx, ly := touchAt(b, `document.querySelector('.sode-gridpanel__scroll')`)
	b.call("Input.dispatchTouchEvent", map[string]any{"type": "touchStart", "touchPoints": []map[string]any{{"x": lx, "y": ly}}})
	time.Sleep(600 * time.Millisecond)
	b.call("Input.dispatchTouchEvent", map[string]any{"type": "touchMove", "touchPoints": []map[string]any{{"x": lx, "y": 5}}})
	scrollTop := `document.querySelector('.sode-gridpanel__scroll').scrollTop`
	// Timed from the first frame that scrolls, so a late start on a loaded runner does not read as no scroll.
	start := b.eval(scrollTop).(float64)
	b.waitFor("the edge scroll starts", fmt.Sprintf(`%s < %v`, scrollTop, start))
	from, at := b.eval(scrollTop).(float64), time.Now()
	time.Sleep(time.Second)
	moved, took := from-b.eval(scrollTop).(float64), time.Since(at).Seconds()
	b.call("Input.dispatchTouchEvent", map[string]any{"type": "touchMove", "touchPoints": []map[string]any{{"x": lx, "y": ly}}})
	b.call("Input.dispatchTouchEvent", map[string]any{"type": "touchCancel", "touchPoints": []map[string]any{}})
	// 10 crosspoints a second at full depth (gridpanelgesture.js EDGE_MAX), with room for timing.
	if v := moved / took / xp; v <= 0 || v > 13 {
		t.Errorf("edge scroll past the body ran at %.1f crosspoints a second, want above 0 and at most 13", v)
	}
	if b.eval(cursorShown) == true {
		t.Error("a line by touch showed the cursor")
	}
	b.eval(`document.querySelector('.sode-gridpanel__scroll').scrollTo(0, 0), true`)
	b.waitFor("the Take button shows, off with nothing staged", `(() => { const t = document.getElementById('panel-take'); return !!t?.offsetParent && t.disabled; })()`)

	// Router numbers start at 0, so source 11 is the twelfth column: it goes first.
	b.eval(`(() => {
		const sc = document.querySelector('.sode-gridpanel__scroll');
		sc.scrollLeft = 11 * document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width;
		return true;
	})()`)
	src := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__src[data-src="%d"]')`, n)
	}
	dst := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__dst[data-dst="%d"]')`, n)
	}
	picked := func(name string) string { return `('picked' in ` + name + `.dataset)` }
	b.waitFor("source 13 drawn", src(13)+` !== null && !`+src(13)+`.hidden && `+src(13)+`.getBoundingClientRect().right <= document.querySelector(".sode-gridpanel__scroll").getBoundingClientRect().right`)

	tap(b, src(11))
	b.waitFor("a tap picks", picked(src(11)))
	tap(b, src(12))
	b.waitFor("a second tap adds to the picks", picked(src(11))+` && `+picked(src(12)))
	time.Sleep(500 * time.Millisecond) // not a double-tap
	tap(b, src(12))
	b.waitFor("a tap on a picked name removes it", picked(src(11))+` && !`+picked(src(12)))
	cursor := b.eval(gridCursor)
	touch(b, src(13), 700*time.Millisecond)
	b.waitFor("a long-press adds the range from the last tap", picked(src(11))+` && `+picked(src(12))+` && `+picked(src(13)))
	if got := b.eval(gridCursor); got != cursor {
		t.Errorf("a long-press moved the cursor from %v to %v", cursor, got)
	}

	b.waitFor("the numbered targets show with no hover",
		gridXp(11, 0)+`?.dataset.n === '1' && `+gridXp(12, 1)+`?.dataset.n === '2' && `+gridXp(13, 2)+`?.dataset.n === '3'`)
	b.waitFor("the band fits the phone", `(() => { const d = document.querySelector('.sode-gridpanel__deck'); return d.scrollWidth <= d.clientWidth; })()`)
	b.shot("phone-preview")

	tap(b, gridXp(11, 0))
	staged := func(s, d int) string { return gridXp(s, d) + `.dataset.s === 'staged'` }
	b.waitFor("a tap on a crosspoint stages the Diagonal", staged(11, 0)+` && `+staged(12, 1)+` && `+staged(13, 2))
	b.waitFor("the Take button works and says only Take", `(() => { const t = document.getElementById('panel-take'); return !t.disabled && t.textContent === 'Take'; })()`)
	tap(b, `document.getElementById('panel-take')`)
	b.waitFor("the confirm lists all three", `document.getElementById('confirm-body').textContent.includes('3 destinations')`)
	b.shot("phone-confirm")
	// A phone has no keys: the confirm's own button, by touch.
	b.waitFor("the confirm lockout ends", `!document.getElementById('confirm-ok').disabled`)
	tap(b, `document.getElementById('confirm-ok')`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	tallyHas(t, a, ana, 0, 11)
	tallyHas(t, a, ana, 1, 12)
	tallyHas(t, a, ana, 2, 13)
	b.waitFor("the Take button is off again", `document.getElementById('panel-take').disabled`)

	// Destination 5 has source 5: a double-tap on its name goes along the row to it.
	b.eval(`document.querySelector('.sode-gridpanel__scroll').scrollTo(0, 0), true`)
	b.waitFor("destination 5 drawn", dst(5)+` !== null && !`+dst(5)+`.hidden`)
	b.eval(`new Promise((r) => setTimeout(() => requestAnimationFrame(() => r(true)), 300))`)
	// No pause between the taps: CDP round trips on a loaded runner already use some of the 350 ms window.
	tap(b, dst(5))
	tap(b, dst(5))
	gridCursorOn(b, "a double-tap on a name jumps", 5, 5)
	if b.eval(cursorShown) == true {
		t.Error("a jump by touch showed the cursor")
	}
	b.waitFor("the double-tap leaves the picks", `!`+picked(dst(5)))

	realKey(b, "?", "Slash", 191)
	time.Sleep(200 * time.Millisecond)
	if open := b.eval(`!document.getElementById('keys').hidden`); open == true {
		t.Error("the phone layout opened the key list")
	}
	b.check()
}
