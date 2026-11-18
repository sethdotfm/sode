package app

import (
	"fmt"
	"testing"
)

// A Grid panel in Instant mode never opens the take confirm: a press, a Diagonal and a Drawn line each send at once.
// A Diagonal across the protected Rec B (12) sends the rest, and its banner names Rec B. That banner is an error, so a
// later result that went well does not take its place. The corner counts it, and the X clears both. A screen reader
// gets each banner's words in one live region.
func TestChromeGridPanelInstant(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Instant grid","type":"grid","settings":{"mode":"instant","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(9, 2)+` !== null && document.querySelector('.sode-gridpanel__src[data-src="5"]')?.textContent !== ''`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)
	b.eval(`(() => {
		const c = document.getElementById('confirm');
		window.__asked = false;
		new MutationObserver(() => { if (!c.hidden) window.__asked = true; }).observe(c, { attributes: true });
		return true;
	})()`)
	asked := func(what string) {
		t.Helper()
		if b.eval(`window.__asked`) != false {
			t.Fatalf("%s opened the take confirm", what)
		}
	}

	b.eval(gridXp(9, 2) + `.click()`)
	tallyHas(t, a, ana, 2, 9)
	asked("a press")

	src := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__src[data-src="%d"]')`, n)
	}
	b.eval(src(3) + `.dispatchEvent(new MouseEvent('click', { bubbles: true })); true`)
	b.eval(src(5) + `.dispatchEvent(new MouseEvent('click', { bubbles: true, shiftKey: true })); true`)
	b.waitFor("three sources picked", src(5)+`.dataset.picked === ''`)
	b.eval(gridXp(3, 4) + `.click()`)
	tallyHas(t, a, ana, 4, 3)
	tallyHas(t, a, ana, 5, 4)
	tallyHas(t, a, ana, 6, 5)
	asked("a Diagonal")

	// Rec B (12) is protected in the simulator.
	gridShow(b, 3, 11)
	b.eval(gridXp(3, 11) + `.click()`)
	tallyHas(t, a, ana, 11, 3)
	tallyHas(t, a, ana, 13, 5)
	asked("a Diagonal across a protected row")
	b.waitFor("the banner names the skipped Rec B and is an error", gridMessage+`.startsWith('Taken: 2 routes') && `+gridMessage+`.includes('Rec B') && !!document.querySelector('.sode-gridpanel__msg .pb-alert--error')`)
	b.waitFor("a screen reader gets the banner's words", `document.querySelector('.sode-gridpanel__said[role="status"]').textContent === `+gridMessage)
	gridKey(b, "Escape")
	b.waitFor("Esc drops the picks", `!document.querySelector('.sode-gridpanel__src[data-picked]')`)

	b.eval(gridXp(6, 14) + `.click()`)
	tallyHas(t, a, ana, 14, 6)
	b.waitFor("the take that went well waits behind the error, counted in the corner", `(() => {
		const n = document.querySelector('.sode-gridpanel__held');
		return `+gridMessage+`.includes('Rec B') && !n.hidden && n.textContent === '1 message not shown';
	})()`)
	b.shot("instant-held")
	b.eval(`document.querySelector('.sode-gridpanel__msg .sode__banner:not(.sode__banner--out) .sode__banner-x').click(); true`)
	b.waitFor("the X clears the error and the count", gridMessage+` === '' && document.querySelector('.sode-gridpanel__held').hidden`)

	// A Drawn line from the keys: v, two rows down, then Space.
	gridShow(b, 7, 0)
	px := b.eval(`(() => { const r = ` + gridXp(7, 0) + `.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`).([]any)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": px[0], "y": px[1]})
	gridCursorOn(b, "the pointer places the cursor", 0, 7)
	realKey(b, "v", "KeyV", 86)
	realKey(b, "j", "KeyJ", 74)
	realKey(b, "j", "KeyJ", 74)
	realKey(b, " ", "Space", 32)
	tallyHas(t, a, ana, 0, 7)
	tallyHas(t, a, ana, 1, 7)
	tallyHas(t, a, ana, 2, 7)
	asked("a Drawn line")
	b.check()
}
