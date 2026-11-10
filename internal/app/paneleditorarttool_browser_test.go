package app

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"testing"
)

// The smart menu's Artwork tool over a 2x3 block with one empty square: Esc after a preview changes nothing, Icon A
// goes on every cell, Icon B runs on in the fill order, and a save stores both, read back over REST.
func TestChromeEditorArtTool(t *testing.T) {
	b, a, ana, id := editorTab(t, `{"name":"Art tool","rows":2,"cols":3,"cells":[
		{"r":1,"c":1,"kind":"take"},{"r":1,"c":2,"kind":"clear"},{"r":1,"c":3,"kind":"source"},
		{"r":2,"c":1,"kind":"current"},{"r":2,"c":3,"kind":"take"}]}`)
	stored := func() string {
		var p Panel
		if err := json.Unmarshal(apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.Bytes(), &p); err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, c := range p.Cells {
			out = append(out, fmt.Sprintf("%v,%v=%v/%v", c["r"], c["c"], c["art"], c["artB"]))
		}
		sort.Strings(out)
		return strings.Join(out, " ")
	}
	before := stored()
	// A headless page never has focus, so focusin would not fire; the flyout's preview follows focus as in a real window.
	b.call("Emulation.setFocusEmulationEnabled", map[string]any{"enabled": true})

	b.waitFor("grid drawn", `document.querySelectorAll('#ed-grid .sode-ed__sq').length === 6`)
	b.eval(`window.press = (r, c, shift) => { const sq = document.querySelector('.sode-ed__sq[data-r="' + r + '"][data-c="' + c + '"]'); const b = sq.getBoundingClientRect(); sq.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, shiftKey: shift, clientX: b.x + 4, clientY: b.y + 4 })); };
		window.key = (n, k) => n.dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true, cancelable: true }));
		window.tool = () => document.querySelector('.sode-arttool');
		window.toolItem = () => tool()?.parentElement.querySelector(':scope > .pb-menu__item');
		press(1, 1); press(2, 3, true); true`)
	b.waitFor("block selected with the menu", `document.querySelectorAll('.sode-ed__sq.is-sel').length === 6 && !document.querySelector('.sode-ed__menu').hidden`)
	b.waitFor("Artwork sits below Find", `(() => { const items = [...document.querySelectorAll('.sode-ed__menu > li > .pb-menu__item')].map((n) => n.textContent); return items.indexOf('Artwork') === items.findIndex((t) => t.startsWith('Find')) + 1; })()`)

	// Keyboard: focus the item, the arrow toward the flyout enters it at the search box, Down enters the icons.
	b.eval(`toolItem().focus(); key(toolItem(), toolItem().parentElement.dataset.subSide === 'left' ? 'ArrowLeft' : 'ArrowRight'); true`)
	b.waitFor("the arrow enters the flyout at its search box", `document.activeElement === tool().querySelector('input[type=search]')`)
	b.waitFor("the flyout is open", `getComputedStyle(tool()).display !== 'none'`)
	b.eval(`key(document.activeElement, 'ArrowDown'); true`)
	b.waitFor("Down enters the icons", `document.activeElement?.dataset.choice?.startsWith('a:icon:')`)
	b.waitFor("a focused icon previews on the five cells", `document.querySelectorAll('.sode-ed__sq.is-preview').length === 5 && /Sets Icon A on 5 cells\. 1 empty square skipped\./.test(tool().querySelector('.sode-find__note').textContent)`)
	b.eval(`key(document.activeElement, 'Escape'); true`)
	b.waitFor("Esc backs out to the grid and drops the preview", `document.activeElement === document.getElementById('ed-grid') && !document.querySelector('.sode-ed__sq.is-preview')`)
	b.waitFor("Esc changed nothing", `document.getElementById('ed-save').disabled`)

	// Icon A by Enter on a chosen icon, then Icon B from 15 across: 15, 16 and the rest left without.
	b.eval(`toolItem().click(); true`)
	b.waitFor("the item opens the flyout", `document.activeElement === tool().querySelector('input[type=search]')`)
	b.eval(`const s = tool().querySelector('input[type=search]'); s.value = 'camera'; s.dispatchEvent(new Event('input')); true`)
	b.eval(`tool().querySelector('[data-choice="a:icon:camera"]').focus(); tool().querySelector('[data-choice="a:icon:camera"]').click(); true`)
	b.waitFor("Icon A on every cell", `document.querySelectorAll('.sode-ed__sq.has-cell .sode-panel__art-a').length === 5 && /Set Icon A on 5 cells/.test(document.getElementById('ed-status').textContent)`)
	b.waitFor("focus stays on the choice", `document.activeElement?.dataset.choice === 'a:icon:camera'`)
	b.eval(`tool().querySelector('[data-choice="b:15"]').focus(); true`)
	b.waitFor("the Icon B preview names what is left", `/Sets Icon B 15 to 16 on 2 cells\. 3 cells left without Icon B\./.test(tool().querySelector('.sode-find__note').textContent)`)
	b.eval(`tool().querySelector('[data-choice="b:15"]').click(); true`)
	b.waitFor("Icon B applied", `/Set Icon B 15 to 16/.test(document.getElementById('ed-status').textContent)`)
	b.shot("arttool")

	b.eval(`document.getElementById('ed-save').click()`)
	// Save disables the button at once, before the PUT; only the state slot says Saved once the reply is in.
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	got := stored()
	want := "1,1=icon:camera/15 1,2=icon:camera/16 1,3=icon:camera/<nil> 2,1=icon:camera/<nil> 2,3=icon:camera/<nil>"
	if got != want {
		t.Errorf("stored after the Artwork tool:\n got %s\nwant %s\n(before %s)", got, want, before)
	}
	b.check()
}

// The Artwork flyout opened from the bottom right square of a tall grid stays inside the viewport.
func TestChromeEditorArtToolFitsViewport(t *testing.T) {
	b, _, _, _ := editorTab(t, `{"name":"Tall","rows":12,"cols":8,"cells":[]}`)
	b.waitFor("grid drawn", `document.querySelectorAll('#ed-grid .sode-ed__sq').length === 96`)
	b.eval(`window.sq = document.querySelector('.sode-ed__sq[data-r="12"][data-c="8"]'); window.scrollTo(0, 1e6); true`)
	b.waitFor("bottom right square in view", `(() => { const r = sq.getBoundingClientRect(); return r.top >= 0 && r.bottom <= document.documentElement.clientHeight + 1; })()`)
	b.eval(`const r = sq.getBoundingClientRect(); sq.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); true`)
	b.waitFor("bottom right square selected", `!!document.querySelector('.sode-ed__sq.is-sel[data-r="12"][data-c="8"]')`)
	b.eval(`document.querySelector('.sode-arttool').parentElement.querySelector(':scope > .pb-menu__item').click(); true`)
	b.waitFor("Artwork open", `document.activeElement === document.querySelector('.sode-arttool input[type=search]')`)
	b.waitFor("the flyout sits inside the viewport", `(() => { const r = document.querySelector('.sode-arttool').getBoundingClientRect(); return r.height > 100 && r.top >= 0 && r.left >= 0 && r.bottom <= document.documentElement.clientHeight && r.right <= document.documentElement.clientWidth; })()`)
	b.shot("arttool-fit")
	b.check()
}
