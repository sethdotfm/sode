package app

import (
	"encoding/json"
	"fmt"
	"testing"
)

// pressSquare is a pointerdown on a square of the editor grid, with Shift when extend is set.
func pressSquare(b *tab, r, c int, extend bool) {
	b.eval(fmt.Sprintf(`(() => {
		const q = document.querySelector('.sode-ed__sq[data-r="%d"][data-c="%d"]');
		const x = q.getBoundingClientRect();
		q.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, shiftKey: %t, clientX: x.x + 4, clientY: x.y + 4 }));
		return true;
	})()`, r, c, extend))
}

// A Buttons panel with four cells selected: the Cell tab edits them all. Icon A goes on every cell, and Icon B runs one
// glyph per cell in fill order. Hide text and levels go on every cell. Remove Icon clears them all.
func TestChromeEditorCellMany(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := makePanel(t, a, ana, `{"name":"Many","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":1},{"r":1,"c":2,"rs":1,"cs":1,"kind":"source","n":2},
		{"r":1,"c":3,"rs":1,"cs":1,"kind":"source","n":3},{"r":1,"c":4,"rs":1,"cs":1,"kind":"destination","n":1}]}`)
	b.open(srv.URL + "/panels/editor?panel=" + id)
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the panel loads", `document.getElementById('ed-name').value === 'Many' && !!document.querySelector('.sode-ed__sq[data-r="1"][data-c="4"]')`)

	pressSquare(b, 1, 1, false)
	pressSquare(b, 1, 4, true)
	cell := `document.getElementById('panel-ed-cell')`
	b.waitFor("the Cell tab edits the four cells", `document.getElementById('tab-ed-cell').getAttribute('aria-selected') === 'true' && document.getElementById('ed-cell-kind').textContent.startsWith('4 cells selected') && !document.getElementById('ed-cell-caption').closest('.sode-side__form').hidden`)
	b.eval(`[...` + cell + `.querySelectorAll('.pb-btn')].find((x) => x.textContent === 'Choose icon').click(); true`)
	b.waitFor("Icon A choices", `!!`+cell+`.querySelector('button[aria-label="tv"]')`)
	b.eval(cell + `.querySelector('button[aria-label="tv"]').click(); ` + cell + `.querySelector('[role="group"][aria-label="Icon B"] button[aria-label="1"]').click(); true`)
	b.waitFor("Icon A on every cell and Icon B 1 to 4 across", `[1, 2, 3, 4].every((c) => {
		const q = document.querySelector('.sode-ed__sq[data-r="1"][data-c="' + c + '"]');
		return q.querySelector('use')?.getAttribute('href') === '#pb-i-tv' && q.querySelector('.sode-panel__art-b')?.textContent === String(c);
	})`)
	b.eval(`[...` + cell + `.querySelectorAll('.pb-btn')].find((x) => x.textContent === 'Hide text').click(); true`)
	b.waitFor("Hide text on every cell", `[1, 2, 3, 4].every((c) => document.querySelector('.sode-ed__sq[data-r="1"][data-c="' + c + '"]').dataset.hideText === '1')`)
	b.eval(`const s = document.getElementById('ed-cell-size'); s.value = '150'; s.dispatchEvent(new Event('input', { bubbles: true })); true`)
	b.eval(`document.querySelector('#panel-ed-cell .sode__levels button[data-ui="2"]').click(); true`)
	b.eval(`document.getElementById('ed-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	var cells []map[string]any
	for _, x := range page1Cells(readPanel(t, a, ana, id)) {
		cells = append(cells, x.(map[string]any))
	}
	for _, c := range cells {
		if c["art"] != "icon:tv" || c["hideText"] != true || c["fontSize"] != 150.0 {
			t.Errorf("cell %v, want Icon A tv, Hide text and font size 150", c)
		}
		if lv, _ := json.Marshal(c["levels"]); string(lv) == "null" {
			t.Errorf("cell %v has no levels, want the change on every source and destination", c)
		}
	}
	if len(cells) != 4 || cells[3]["artB"] != "4" {
		t.Errorf("cells %v, want four, the last with Icon B 4", cells)
	}

	b.eval(`[...` + cell + `.querySelectorAll('.pb-btn')].find((x) => x.textContent === 'Remove Icon').click(); true`)
	b.waitFor("Remove Icon clears every cell", `![1, 2, 3, 4].some((c) => document.querySelector('.sode-ed__sq[data-r="1"][data-c="' + c + '"] .sode-panel__art'))`)
	b.check()
}

// A rename on the router reaches the open editor at once: a Buttons cell and a Grid panel's list row both show it.
func TestChromeEditorNamesLive(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := makePanel(t, a, ana, `{"name":"Names","rows":1,"cols":2,"cells":[{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":5}]}`)
	grid := makePanel(t, a, ana, `{"name":"Names grid","type":"grid","sources":{"members":"all"},"destinations":{"members":"all"}}`)
	b.open(srv.URL + "/panels/editor?panel=" + id)
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the cell shows the router name", `!!document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-ed__text')?.textContent`)
	rename := func(name string) {
		a.mu.Lock()
		a.router.srcNames[5] = name
		a.router.namesGen++
		a.broadcast(a.namesMsg())
		a.mu.Unlock()
	}
	rename("RENAMED 1")
	b.waitFor("the cell shows the new name", `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-ed__text').textContent === 'RENAMED 1'`)

	// The Grid panel opens from the Panel picker on the same page, so the page keeps its socket.
	b.eval(`const p = document.getElementById('ed-pick'); p.value = '` + grid + `'; p.dispatchEvent(new Event('change', { bubbles: true })); true`)
	row := `document.querySelector('#ed-sources-list li[data-id="5"] .sode-pages__name')`
	b.waitFor("the list row shows the name", row+`?.textContent === 'RENAMED 1'`)
	rename("RENAMED 2")
	b.waitFor("the list row shows the new name", row+`?.textContent === 'RENAMED 2'`)
	b.check()
}
