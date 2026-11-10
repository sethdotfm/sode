package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// The editor's Pages bar on a 3-Page panel: Add page twice, Cmd-click and a Mac Ctrl-click (contextmenu) open all
// three side by side, a copy on Page 1 pastes onto Page 2 and Page 3, Copy to over the used Page 3 asks before it
// replaces, and an undo pressed on Page 1 takes back the Copy to on Page 3. The save stores every Page, and the copied
// Go to Page cell keeps its target.
func TestChromeEditorPages(t *testing.T) {
	b, a, ana, id := editorTab(t, `{"name":"Paged","rows":2,"cols":4,"pages":[{"id":"p1","enabled":true,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":1},
		{"r":1,"c":2,"rs":1,"cs":1,"kind":"source","n":2},
		{"r":2,"c":1,"rs":1,"cs":1,"kind":"goToPage","page":"p1"}]}]}`)

	b.eval(`window.copied = '';
		navigator.clipboard.writeText = async (t) => { window.copied = t };
		navigator.clipboard.readText = async () => window.copied;
		window.press = (key, code, init = {}) => { const g = document.getElementById('ed-grid'); g.focus(); const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key, code, ...init }); g.dispatchEvent(e); return e.defaultPrevented; };
		window.frame = (n) => [...document.querySelectorAll('#ed-open-pages .sode-ed__frame')].find((f) => !f.hidden && f.querySelector('.sode-ed__frame-title').textContent.startsWith('Page ' + n));
		window.cellsOn = (n) => frame(n)?.querySelectorAll('.sode-ed__sq.has-cell').length ?? -1;
		// Escape first: the smart menu sits beside the selection and can cover the next Page's squares.
		window.pressSq = (n, r, c) => {
			press('Escape', 'Escape');
			const sq = frame(n).querySelector('.sode-ed__sq[data-r="' + r + '"][data-c="' + c + '"]');
			sq.scrollIntoView({ block: 'center', behavior: 'instant' });
			const box = sq.getBoundingClientRect();
			const at = { bubbles: true, cancelable: true, button: 0, clientX: box.left + box.width / 2, clientY: box.top + box.height / 2 };
			sq.dispatchEvent(new PointerEvent('pointerdown', at));
			document.dispatchEvent(new PointerEvent('pointerup', at));
		};
		window.row = (n) => document.querySelectorAll('#ed-pages .sode-pages__row')[n - 1];
		window.stat = () => document.getElementById('ed-status').textContent;
		true`)

	b.eval(`document.getElementById('ed-page-add').click(); document.getElementById('ed-page-add').click()`)
	b.waitFor("three Pages in the bar, the new one open alone", `document.querySelectorAll('#ed-pages .sode-pages__row').length === 3 && stat() === 'Added Page 3.' && cellsOn(3) === 0 && !frame(1)`)

	b.eval(`row(1).querySelector('[data-act="open"]').click()`)
	b.eval(`row(2).querySelector('[data-act="open"]').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, metaKey: true }))`)
	b.eval(`row(3).querySelector('[data-act="open"]').dispatchEvent(new MouseEvent('contextmenu', { bubbles: true, cancelable: true, ctrlKey: true }))`)
	b.waitFor("three Pages side by side", `cellsOn(1) === 3 && cellsOn(2) === 0 && cellsOn(3) === 0`)
	b.shot("pages-side-by-side")

	b.eval(`pressSq(1, 1, 1); press('ArrowRight', 'ArrowRight', { shiftKey: true }); press('c', 'KeyC', { metaKey: true })`)
	b.waitFor("copied on Page 1", `stat() === 'Copied 2 cells.' && row(1).classList.contains('is-focus')`)
	b.eval(`pressSq(2, 2, 1); press('v', 'KeyV', { ctrlKey: true })`)
	b.waitFor("pasted on Page 2", `stat() === 'Pasted 2 cells.' && cellsOn(2) === 2 && row(2).classList.contains('is-focus')`)
	b.eval(`pressSq(3, 1, 1); press('v', 'KeyV', { metaKey: true })`)
	b.waitFor("pasted on Page 3", `stat() === 'Pasted 2 cells.' && cellsOn(3) === 2`)

	b.eval(`row(1).querySelector('[data-act="copy"]').click()`)
	b.waitFor("Copy to asks for a Page", `!!document.getElementById('ed-copyto-page')`)
	b.eval(`const p = document.getElementById('ed-copyto-page'); p.value = '2'; p.dispatchEvent(new Event('change')); document.getElementById('ed-copyto-go').click()`)
	b.waitFor("a used Page asks first", `!document.getElementById('ed-copyto-warn').hidden && document.getElementById('ed-copyto-go').textContent === 'Replace' && cellsOn(3) === 2`)
	b.eval(`document.getElementById('ed-copyto-go').click()`)
	b.waitFor("copied onto Page 3", `!document.getElementById('ed-copyto') && stat() === 'Copied Page 1 to Page 3.' && cellsOn(3) === 3`)

	// One stack for the panel: an undo pressed on Page 1 takes back the Copy to on Page 3.
	b.eval(`pressSq(1, 2, 4)`)
	b.waitFor("Page 1 focused", `row(1).classList.contains('is-focus')`)
	if b.eval(`press('z', 'KeyZ', { metaKey: true })`) != true {
		t.Fatal("Cmd+Z on the grid was not taken by the editor")
	}
	b.waitFor("Copy to undone from Page 1", `stat() === 'Undid Copy to.' && cellsOn(3) === 2 && cellsOn(1) === 3`)
	b.eval(`press('Z', 'KeyZ', { ctrlKey: true, shiftKey: true })`)
	b.waitFor("Copy to redone", `stat() === 'Redid Copy to.' && cellsOn(3) === 3`)

	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	var p struct {
		Pages []struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
			Cells   []struct {
				R, C       int
				Kind, Page string
				N          int
			} `json:"cells"`
		} `json:"pages"`
	}
	if err := json.Unmarshal(apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if len(p.Pages) != 3 || p.Pages[0].ID != "p1" || p.Pages[1].ID == "" || p.Pages[2].ID == "" || p.Pages[1].ID == p.Pages[2].ID {
		t.Fatalf("stored Pages: %+v", p.Pages)
	}
	var got []string
	for i, pg := range p.Pages {
		var cells []string
		for _, c := range pg.Cells {
			cells = append(cells, fmt.Sprintf("%d,%d %s%d%s", c.R, c.C, c.Kind, c.N, c.Page))
		}
		got = append(got, fmt.Sprintf("%d:%v:%s", i+1, pg.Enabled, strings.Join(cells, " ")))
	}
	want := []string{
		"1:true:1,1 source1 1,2 source2 2,1 goToPage0p1",
		"2:true:2,1 source1 2,2 source2",
		"3:true:1,1 source1 1,2 source2 2,1 goToPage0p1",
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("stored Pages:\n got %q\nwant %q", got, want)
	}
	b.check()
}
