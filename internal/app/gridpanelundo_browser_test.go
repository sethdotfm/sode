package app

import (
	"fmt"
	"strings"
	"testing"
)

// gridMenu is a right-click on a drawn Grid panel node.
func gridMenu(b *tab, node string) {
	b.t.Helper()
	b.eval(node + `.dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true })); true`)
}

// Jumps and Undo on a Grid panel in Take mode. Each destination starts on the source of its own number.
// g and a right-click on a destination go to its current source. G and a right-click on a source step through
// the destinations that have it. A right-click on a staged crosspoint unstages it. With a route staged, u asks first. Then it stages this instance's Undo in place
// of the route. Enter and the confirm send the Undo, and the old source comes back.
func TestChromeGridPanelUndo(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Undo grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(5, 2)+` !== null`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)
	status := gridMessage

	b.eval(gridXp(5, 2) + `.click()`)
	realKey(b, "Enter", "Enter", 13)
	gridConfirm(b)
	tallyHas(t, a, ana, 2, 5)
	b.waitFor("the pie shows the take", gridXp(5, 2)+`.dataset.on === '1'`)

	gridMenu(b, `document.querySelector('.sode-gridpanel__dst[data-dst="2"]')`)
	gridCursorOn(b, "a right-click on a destination goes to its current source", 2, 5)
	b.waitFor("the jump pulses the crosspoint and both its names in one phase", `(() => {
		const xp = document.querySelector('.sode-gridpanel__xp[data-hit][data-src="5"][data-dst="2"]');
		const s = document.querySelector('.sode-gridpanel__src[data-hit][data-src="5"]'), d = document.querySelector('.sode-gridpanel__dst[data-hit][data-dst="2"]');
		if (!xp || !s || !d || !(xp.dataset.hit === s.dataset.hit && s.dataset.hit === d.dataset.hit)) return false;
		const t = [xp.firstElementChild.nextElementSibling, s, d].flatMap((n) => n.getAnimations()).filter((a) => a.animationName.startsWith('sode-gridpanel-hit'));
		return t.length === 3 && Math.max(...t.map((a) => a.startTime)) - Math.min(...t.map((a) => a.startTime)) < 20;
	})()`)
	realKey(b, "j", "KeyJ", 74)
	realKey(b, "g", "KeyG", 71)
	gridCursorOn(b, "g goes along the row to the current source", 3, 3)

	// Real mouse clicks with a small move between them, which puts the cursor back on the name.
	src5 := `document.querySelector('.sode-gridpanel__src[data-src="5"]')`
	at := b.eval(`(() => { const r = ` + src5 + `.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`).([]any)
	y := at[1].(float64)
	for k, i := range []int{2, 5, 2} {
		x := at[0].(float64) + float64(k)
		b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y})
		for _, typ := range []string{"mousePressed", "mouseReleased"} {
			b.call("Input.dispatchMouseEvent", map[string]any{"type": typ, "x": x, "y": y, "button": "right", "clickCount": 1})
		}
		gridCursorOn(b, fmt.Sprintf("a right-click on a source steps to row %d", i), i, 5)
	}
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "G", "code": "KeyG", "windowsVirtualKeyCode": 71, "modifiers": 8, "text": "G"})
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "G", "code": "KeyG", "windowsVirtualKeyCode": 71, "modifiers": 8})
	gridCursorOn(b, "G steps on", 5, 5)

	staged := func(src, dst int) string { return gridXp(src, dst) + `.dataset.s === 'staged'` }
	b.eval(gridXp(9, 4) + `.click()`)
	b.waitFor("a route staged", staged(9, 4))
	gridMenu(b, gridXp(9, 4))
	b.waitFor("a right-click on a staged crosspoint unstages it", `!(`+staged(9, 4)+`)`)
	gridCursorOn(b, "and does not jump", 5, 5)
	b.eval(gridXp(9, 4) + `.click()`)
	b.waitFor("the route staged again", staged(9, 4))
	realKey(b, "u", "KeyU", 85)
	b.waitFor("u asks before it drops the staged route", `!document.getElementById('confirm').hidden && document.getElementById('confirm-body').textContent.includes('1 staged route')`)
	b.shot("replace")
	gridConfirm(b)
	b.waitFor("the Undo is staged in place of the route", staged(2, 2)+` && !(`+staged(9, 4)+`)`)
	if !strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"dst":2,"level":1,"src":5`) {
		t.Fatal("u sent the Undo before its confirm")
	}
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("the Undo confirm", `document.getElementById('confirm-title').textContent === 'Confirm undo'`)
	b.shot("undo")
	gridConfirm(b)
	tallyHas(t, a, ana, 2, 2)
	b.waitFor("the status says it is undone", status+`.startsWith('Undone')`)
	b.waitFor("nothing staged", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)

	realKey(b, "u", "KeyU", 85)
	b.waitFor("nothing left to undo", status+` === 'Nothing to undo'`)
}
