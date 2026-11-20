package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Home's Destination box after an add: the box empties, so new digits start a new search. Directly after an add, + and
// Ctrl+Enter on the empty box add the next destination, and the box shows which one. They use the Next match behavior
// setting. With no add directly before, they do nothing. A take after + does not refuse the same text later. A typed list shows
// its destinations all selected and adds them all, a range with a tail too; the Source box gives a note instead. Esc with nothing to clear does nothing, and
// Alt+Shift+k starts and ends Keyboard mode.
func TestChromeMainMultiAdd(t *testing.T) {
	b := startBrowser(t)
	// Names for the tail list and the Next match of the name: numbers 41 to 48.
	names := filepath.Join(t.TempDir(), "names")
	if err := os.WriteFile(names, []byte("dst 40 Mon 1 PIP 1\ndst 41 Mon 1 PIP 2\ndst 42 Mon2 PIP1\ndst 43 Mon 2 PIP 2\ndst 44 Mon 3 PIP 1\ndst 45 Mon 9\ndst 46 Bars X\ndst 47 Mon 10\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	sim := startSim(t, "SIM_NAMES_FILE="+names)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("17 level picks", `document.querySelectorAll('#lvl-pick > button').length === 17`)

	chips := `[...document.querySelectorAll('#dst-chips .pb-chip')].map((c) => c.firstChild.textContent).join('|')`
	hint := `document.getElementById('dst').placeholder`
	typed := func(id, text string) {
		b.eval(`document.getElementById('` + id + `').focus(); true`)
		b.call("Input.insertText", map[string]any{"text": text})
	}
	listed := func(id string) {
		b.waitFor(id+" lists matches", `!document.getElementById('`+id+`-menu').hidden`)
	}
	plus := func() { realKey(b, "+", "Equal", 187) }
	ctrlEnter := func() { realKeyMods(b, "Enter", "Enter", 13, modCtrl) }
	esc := func(id string) {
		b.eval(`document.getElementById('` + id + `').focus(); true`)
		realKey(b, "Escape", "Escape", 27)
	}
	// One Esc closes an open list first, then a second Esc clears.
	clear := func() {
		for _, id := range []string{"src", "dst"} {
			if !b.eval(`document.getElementById('` + id + `-menu').hidden`).(bool) {
				esc(id)
			}
		}
		esc("src")
		b.waitFor("Esc clears everything", `!document.getElementById('src').value && !document.getElementById('dst').value && document.getElementById('dst-chips').hidden && `+hint+` === ' '`)
	}
	confirmBody := `document.getElementById('confirm-body').textContent`

	// The fast path: CAM 1, Tab, MV 1, Ctrl+Enter four times, Enter takes CAM 1 to MV 1 to MV 4.
	popTyped(b, "src", "CAM 1")
	typed("dst", "MV 1")
	listed("dst")
	ctrlEnter()
	b.waitFor("the box empties and shows the next add", `document.getElementById('dst').value === '' && `+hint+` === 'Ctrl+Enter adds 15 MV 2'`)
	b.shot("armed")
	ctrlEnter()
	ctrlEnter()
	ctrlEnter()
	b.waitFor("four destinations in number order", chips+` === '14 MV 1|15 MV 2|16 MV 3|17 MV 4' && `+hint+` === 'Ctrl+Enter adds 18 MV 5'`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter opens the confirm", `!document.getElementById('confirm').hidden`)
	if body := b.eval(confirmBody).(string); !strings.Contains(body, "4 destinations") || !strings.Contains(body, "MV 4") || strings.Contains(body, "MV 5") {
		t.Fatalf("the confirm says %q, want CAM 1 to MV 1 to MV 4", body)
	}
	gridConfirm(b)
	lastGroup(t, a, ana, 4)
	tallyHas(t, a, ana, 16, 12)
	b.waitFor("the take ends the next add", `document.getElementById('dst-chips').hidden && `+hint+` === ' '`)

	// With no add directly before, + and Ctrl+Enter on the empty box add nothing.
	b.eval(`document.getElementById('dst').focus(); true`)
	ctrlEnter()
	plus()
	b.eval(`document.getElementById('dst-add').click(); true`)
	if got := b.eval(chips).(string); got != "" {
		t.Fatalf("an add on the empty box with no add before it added %q", got)
	}

	// New digits start a new search: 10, +, 11 adds number 11, not a match of "1011".
	typed("dst", "10")
	listed("dst")
	plus()
	b.waitFor("10 is added", chips+` === '10 TX 1' && !document.getElementById('dst').value`)
	typed("dst", "11")
	listed("dst")
	plus()
	b.waitFor("11 is added", chips+` === '10 TX 1|11 TX 2' && `+hint+` === 'Ctrl+Enter adds 12 Rec A'`)
	// Typing replaces the hint, and a box emptied by hand does not arm again.
	typed("dst", "x")
	b.waitFor("typing ends the next add", hint+` === ' '`)
	b.eval(`(() => { const i = document.getElementById('dst'); i.value = ''; i.dispatchEvent(new Event('input')); })()`)
	b.waitFor("the box is empty", `!document.getElementById('dst').value && `+hint+` === ' '`)
	ctrlEnter()
	if got := b.eval(chips).(string); got != "10 TX 1|11 TX 2" {
		t.Fatalf("Ctrl+Enter after typing and Backspace changed the chips to %q", got)
	}
	clear()

	// A take after + does not refuse the same text: 13, Tab, 14, +, Enter takes; then 15, Tab, 14, Enter opens the
	// Confirm.
	popTyped(b, "src", "13")
	typed("dst", "14")
	listed("dst")
	plus()
	b.waitFor("14 is added", chips+` === '14 MV 1'`)
	realKey(b, "Enter", "Enter", 13)
	gridConfirm(b)
	tallyHas(t, a, ana, 13, 12)
	b.waitFor("the take clears the boxes", `!document.getElementById('src').value && document.getElementById('dst-chips').hidden`)
	popTyped(b, "src", "15")
	typed("dst", "14")
	listed("dst")
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("the same text opens the confirm", `!document.getElementById('confirm').hidden && `+confirmBody+`.includes('MV 1')`)
	if b.eval(`document.getElementById('dst').classList.contains('pb-input--invalid')`).(bool) {
		t.Fatal("the Destination box is red after the second route")
	}
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	clear()

	// A list in the Destination box: all its destinations show selected, and Ctrl+Enter adds them all.
	rows := `document.querySelectorAll('#dst-menu [role="option"]')`
	selected := `[...document.querySelectorAll('#dst-menu [role="option"]')].every((r) => r.classList.contains('active') && r.getAttribute('aria-selected') === 'true')`
	note := func(id string) string {
		return `(document.querySelector('#` + id + `-menu .sode__menu-note')?.textContent || '')`
	}
	typed("dst", "MV 1-4")
	listed("dst")
	b.waitFor("four rows, all selected", rows+`.length === 4 && `+selected+` && document.getElementById('dst-menu').getAttribute('aria-multiselectable') === 'true'`)
	b.shot("list")
	realKey(b, "ArrowDown", "ArrowDown", 40)
	if !b.eval(selected).(bool) {
		t.Fatal("an arrow changed the multiselection")
	}
	ctrlEnter()
	b.waitFor("Ctrl+Enter adds the list", chips+` === '14 MV 1|15 MV 2|16 MV 3|17 MV 4' && !document.getElementById('dst').value && `+hint+` === 'Ctrl+Enter adds 18 MV 5'`)
	clear()

	// A range with a tail: Mon 1-4 PIP 1 finds three destinations. The note names the one that is not there.
	typed("dst", "Mon 1-4 PIP 1")
	listed("dst")
	b.waitFor("three rows and a note", rows+`.length === 3 && `+selected+` && `+note("dst")+` === 'Not found: Mon 4 PIP 1'`)
	b.shot("tail-list")
	ctrlEnter()
	b.waitFor("Ctrl+Enter adds the three", chips+` === '41 Mon 1 PIP 1|43 Mon2 PIP1|45 Mon 3 PIP 1'`)
	clear()

	// The next add uses the Next match behavior setting: after Mon 9, by number Bars X, by name Mon 10.
	typed("dst", "Mon 9")
	listed("dst")
	plus()
	b.waitFor("by number", chips+` === '46 Mon 9' && `+hint+` === 'Ctrl+Enter adds 47 Bars X'`)
	b.eval(`import('./device.js').then((m) => { m.setPref('nextMode', 'fuzzy'); return true })`)
	b.waitFor("by name", hint+` === 'Ctrl+Enter adds 48 Mon 10'`)
	ctrlEnter()
	b.waitFor("Ctrl+Enter adds Mon 10", chips+` === '46 Mon 9|48 Mon 10'`)
	b.eval(`import('./device.js').then((m) => { m.setPref('nextMode', ''); return true })`)
	clear()

	// Numbers the router does not have are left out, and the note says so.
	typed("dst", "20-22, 9999")
	listed("dst")
	b.waitFor("three rows and a note", rows+`.length === 3 && `+selected+` && `+note("dst")+` === 'Not found: 9999'`)
	esc("dst")
	b.waitFor("the first Esc closes the list", `document.getElementById('dst-menu').hidden && document.getElementById('dst').value === '20-22, 9999'`)
	clear()

	// Enter adds a list of names and takes them with the source.
	popTyped(b, "src", "CAM 2")
	typed("dst", "DSM 1, DSM 3-4")
	listed("dst")
	b.waitFor("three DSM rows", rows+`.length === 3 && `+selected)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter opens the confirm", `!document.getElementById('confirm').hidden`)
	if body := b.eval(confirmBody).(string); !strings.Contains(body, "3 destinations") || !strings.Contains(body, "DSM1") || !strings.Contains(body, "DSM4") || strings.Contains(body, "DSM2") {
		t.Fatalf("the confirm says %q, want CAM 2 to DSM1, DSM3 and DSM4", body)
	}
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	clear()

	// The Source box takes one source: a list there shows a note that points to the Grid, and no rows.
	typed("src", "CAM 1-3")
	listed("src")
	b.waitFor("the Source note", note("src")+`.includes('Grid (Ctrl+e)') && document.querySelectorAll('#src-menu [role="option"]').length === 0`)
	b.shot("source-note")
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter takes nothing", `document.getElementById('confirm').hidden && document.getElementById('src').classList.contains('pb-input--invalid')`)
	clear()

	// Esc with nothing to clear does nothing. Alt+Shift+k starts Keyboard mode, and a second press goes back to Source.
	esc("src")
	if id := b.eval(`document.activeElement.id`).(string); id != "src" {
		t.Fatalf("Esc with nothing to clear moved focus to %q", id)
	}
	realKeyMods(b, "K", "KeyK", 75, modAlt|modShift)
	b.waitFor("Keyboard mode", `document.activeElement === document.getElementById('history') && document.documentElement.hasAttribute('data-keys-on')`)
	realKeyMods(b, "K", "KeyK", 75, modAlt|modShift)
	b.waitFor("back to Source", `document.activeElement === document.getElementById('src')`)
	if v := b.eval(`document.getElementById('src').value`).(string); v != "" {
		t.Fatalf("Alt+Shift+k typed %q", v)
	}
	b.check()
}
