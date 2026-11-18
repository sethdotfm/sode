package app

import (
	"fmt"
	"testing"
)

// CDP Input.dispatchKeyEvent modifier bits.
const (
	modAlt   = 1
	modCtrl  = 2
	modMeta  = 4
	modShift = 8
)

// realKeyMods is realKey with modifiers held (modShift and the others).
func realKeyMods(b *tab, key, code string, vk, mods int) {
	b.t.Helper()
	down := map[string]any{"type": "keyDown", "key": key, "code": code, "windowsVirtualKeyCode": vk, "modifiers": mods}
	if len(key) == 1 && mods&(modCtrl|modMeta|modAlt) == 0 {
		down["text"] = key
	}
	// Without the text, Chrome runs no default action for Enter, such as a focused button's click.
	if key == "Enter" {
		down["text"] = "\r"
	}
	b.call("Input.dispatchKeyEvent", down)
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": key, "code": code, "windowsVirtualKeyCode": vk, "modifiers": mods})
}

// Picks by keys: Space picks a name on the Source bar and Shift+l extends the range. j into the grid goes to the
// range's start, and Space on a crosspoint stages the Diagonal. A focused header button keeps its own Space. ? opens the key list and Esc closes it.
func TestChromeGridPanelKeys(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Keys grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(3, 2)+` !== null`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)
	gridCursorOn(b, "the cursor starts at the corner", -1, -1)

	src := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__src[data-src="%d"]')`, n)
	}
	// Name nodes are pooled, so their order in the page is not display order.
	picked := `[...document.querySelectorAll('.sode-gridpanel__src[data-picked]')].map((n) => +n.dataset.src).sort((a, b) => a - b).join(',')`

	realKey(b, "l", "KeyL", 76)
	realKey(b, "l", "KeyL", 76)
	gridCursorOn(b, "l moves along the Source bar", -1, 1)
	realKey(b, " ", "Space", 32)
	b.waitFor("Space on the Source bar picks the name", picked+` === '1'`)
	realKeyMods(b, "L", "KeyL", 76, modShift)
	realKeyMods(b, "ArrowRight", "ArrowRight", 39, modShift)
	gridCursorOn(b, "Shift moves go along the bar", -1, 3)
	b.waitFor("Shift moves extend the pick as a range", picked+` === '1,2,3'`)
	realKey(b, "x", "KeyX", 88)
	b.waitFor("x takes one name out of the picks", picked+` === '1,2'`)
	realKey(b, "x", "KeyX", 88)
	b.waitFor("x adds it again", src(3)+`.dataset.picked === ''`)
	b.shot("picked")

	// The cursor is on the range's end (column 3). Into the grid it goes to the range's start.
	realKey(b, "j", "KeyJ", 74)
	gridCursorOn(b, "j from the picked range goes to its first column", 0, 1)
	realKey(b, " ", "Space", 32)
	staged := func(s, d int) string { return gridXp(s, d) + `.dataset.s === 'staged'` }
	b.waitFor("Space stages the Diagonal", staged(1, 0)+` && `+staged(2, 1)+` && `+staged(3, 2))
	b.waitFor("three routes staged", `document.getElementById('panel-staged').textContent === '3 staged'`)

	// A focused button outside the grid (the Levels box's Select All) takes Space and Enter, not the grid.
	b.waitFor("Select All", `document.getElementById('panel-lvl-all').textContent.includes('All')`)
	b.eval(`document.getElementById('panel-lvl-all').focus(); true`)
	realKey(b, " ", "Space", 32)
	b.waitFor("Space works the focused button", `document.getElementById('panel-lvl-all').textContent.includes('None')`)
	realKeyMods(b, "Enter", "Enter", 13, 0)
	b.waitFor("Enter works the focused button", `document.getElementById('panel-lvl-all').textContent.includes('All')`)
	if open := b.eval(`!document.getElementById('confirm').hidden`); open != false {
		t.Errorf("Enter on a button outside the grid opened the take confirm")
	}
	b.waitFor("the staged routes stay", staged(1, 0)+` && `+staged(3, 2))
	b.eval(`document.activeElement.blur(); true`)

	realKeyMods(b, "?", "Slash", 191, modShift)
	b.waitFor("? opens the key list", `!document.getElementById('keys').hidden`)
	b.shot("keys")
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc closes the key list", `document.getElementById('keys').hidden`)
	b.waitFor("and keeps the picks", picked+` === '1,2,3'`)
	realKeyMods(b, "?", "Slash", 191, modShift|modCtrl)
	b.waitFor("Ctrl+? opens the key list", `!document.getElementById('keys').hidden`)
	realKeyMods(b, "?", "Slash", 191, modShift|modCtrl)
	b.waitFor("a second Ctrl+? closes it", `document.getElementById('keys').hidden`)

	realKey(b, "Enter", "Enter", 13)
	b.waitFor("the confirm lists all three", `document.getElementById('confirm-body').textContent.includes('3 destinations')`)
	gridConfirm(b)
	tallyHas(t, a, ana, 0, 1)
	tallyHas(t, a, ana, 1, 2)
	tallyHas(t, a, ana, 2, 3)

	// The same on the Destination bar: rows 1 to 3 picked down the bar, and l goes back to row 1.
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc drops the picks", `!document.querySelector('[data-picked]')`)
	realKey(b, "Home", "Home", 36)
	realKey(b, "j", "KeyJ", 74)
	realKey(b, "j", "KeyJ", 74)
	gridCursorOn(b, "on the Destination bar", 1, -1)
	realKey(b, " ", "Space", 32)
	realKeyMods(b, "J", "KeyJ", 74, modShift)
	realKeyMods(b, "J", "KeyJ", 74, modShift)
	gridCursorOn(b, "Shift+j goes down the bar", 3, -1)
	b.waitFor("three destinations picked", `document.querySelectorAll('.sode-gridpanel__dst[data-picked]:not([hidden])').length === 3`)
	realKey(b, "l", "KeyL", 76)
	gridCursorOn(b, "l from the picked range goes to its first row", 1, 0)

	// On a crosspoint, Shift moves pick the names they cross: sources across, destinations down.
	gridKey(b, "Escape")
	b.waitFor("Esc drops the picks again", `!document.querySelector('[data-picked]')`)
	realKeyMods(b, "L", "KeyL", 76, modShift)
	realKeyMods(b, "ArrowRight", "ArrowRight", 39, modShift)
	realKeyMods(b, "J", "KeyJ", 74, modShift)
	gridCursorOn(b, "Shift moves on a crosspoint move the cursor", 2, 2)
	b.waitFor("and pick three sources across and two destinations down", `document.querySelectorAll('.sode-gridpanel__src[data-picked]:not([hidden])').length === 3 && `+
		`document.querySelectorAll('.sode-gridpanel__dst[data-picked]:not([hidden])').length === 2`)
	b.check()
}
