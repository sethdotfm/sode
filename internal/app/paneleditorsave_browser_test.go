package app

import (
	"encoding/json"
	"testing"
)

// A save keeps edits made while it runs. A cell deleted on Page 2 during a held save stays deleted, and the editor
// says Unsaved changes. A save that fails tries once more by itself, then the Save button says Retry save until one works.
func TestChromeEditorSaveKeepsEdits(t *testing.T) {
	b, a, ana, id := editorTabPrep(t, `{"name":"Two","rows":1,"cols":2,"pages":[
		{"id":"p1","enabled":true,"cells":[{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":1}]},
		{"id":"p2","enabled":true,"cells":[{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":2}]}]}`, func(b *tab, _ *App, _, _ string) {
		// Before the page loads: sessionkey.js keeps the fetch it finds at import.
		b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": holdPuts})
	})

	b.eval(`
		window.press = (key) => { const g = document.getElementById('ed-grid'); g.focus(); g.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key, code: key })); };
		window.frame = (n) => [...document.querySelectorAll('#ed-open-pages .sode-ed__frame')].find((f) => !f.hidden && f.querySelector('.sode-ed__frame-title').textContent.startsWith('Page ' + n));
		window.cellsOn = (n) => frame(n)?.querySelectorAll('.sode-ed__sq.has-cell').length ?? -1;
		window.row = (n) => document.querySelectorAll('#ed-pages .sode-pages__row')[n - 1];
		window.delAt = (n) => {
			press('Escape');
			const sq = frame(n).querySelector('.sode-ed__sq[data-r="1"][data-c="1"]');
			const box = sq.getBoundingClientRect();
			const at = { bubbles: true, cancelable: true, button: 0, clientX: box.left + box.width / 2, clientY: box.top + box.height / 2 };
			sq.dispatchEvent(new PointerEvent('pointerdown', at));
			document.dispatchEvent(new PointerEvent('pointerup', at));
			press('Delete');
		};
		window.state = () => document.getElementById('ed-state').textContent;
		window.saveBtn = () => document.getElementById('ed-save');
		true`)
	b.waitFor("Page 1 open, and the header has the Panels logo link and Back to Sode", `cellsOn(1) === 1 && document.getElementById('ed-home').getBoundingClientRect().width > 0
		&& !!document.querySelector('#ed-home .sode-logo') && document.getElementById('ed-back-sode').getBoundingClientRect().width > 0 && document.querySelector('.sode-panels__title').textContent.endsWith('Panel Editor')`)
	b.eval(`delAt(1); true`)
	b.waitFor("Page 1 edited", `cellsOn(1) === 0 && state() === 'Unsaved changes'`)
	b.eval(`window.hold = new Promise((ok) => (window.release = ok)); saveBtn().click(); true`)
	b.waitFor("the save runs", `window.puts === 1 && state() === 'Saving' && saveBtn().disabled`)
	b.eval(`row(2).querySelector('[data-act="open"]').click(); true`)
	b.waitFor("Page 2 open", `cellsOn(2) === 1`)
	b.eval(`delAt(2); true`)
	b.waitFor("Page 2 edited during the save", `cellsOn(2) === 0`)
	b.eval(`window.hold = null; window.release(); true`)
	b.waitFor("the save answers; the Page 2 edit stays and is unsaved", `state() === 'Unsaved changes' && cellsOn(2) === 0 && !saveBtn().disabled && document.getElementById('ed-status').hidden`)
	stored := func() (n1, n2 int) {
		var p struct {
			Pages []struct {
				Cells []any `json:"cells"`
			} `json:"pages"`
		}
		raw, _ := json.Marshal(readPanel(t, a, ana, id))
		if err := json.Unmarshal(raw, &p); err != nil || len(p.Pages) != 2 {
			t.Fatalf("stored panel: %v %s", err, raw)
		}
		return len(p.Pages[0].Cells), len(p.Pages[1].Cells)
	}
	if n1, n2 := stored(); n1 != 0 || n2 != 1 {
		t.Errorf("after the first save the pages hold %d and %d cells, want 0 and 1", n1, n2)
	}

	b.eval(`window.puts = 0; window.fail = 2; saveBtn().click(); true`)
	b.waitFor("two failures (the save and its one retry): the button says Retry save", `window.puts === 2 && saveBtn().textContent === 'Retry save' && saveBtn().classList.contains('pb-btn--danger') && state() === 'Not saved' && !saveBtn().disabled && !document.getElementById('ed-status').hidden && cellsOn(2) === 0`)
	b.shot("save-failed")
	b.eval(`saveBtn().click(); true`)
	b.waitFor("Retry save works", `state() === 'Saved' && saveBtn().textContent === 'Save' && saveBtn().classList.contains('pb-btn--ok') && document.getElementById('ed-status').hidden`)
	if n1, n2 := stored(); n1 != 0 || n2 != 0 {
		t.Errorf("after the retry the pages hold %d and %d cells, want 0 and 0", n1, n2)
	}
	b.check()
}

// Holds a panel PUT while window.hold is a promise, and answers 503 while window.fail is above 0.
const holdPuts = `(() => {
	window.puts = 0; window.fail = 0; window.hold = null;
	const real = window.fetch;
	window.fetch = async (url, init) => {
		if (init?.method !== 'PUT' || !String(url).includes('api/v1/panels/')) return real(url, init);
		window.puts++;
		if (window.fail > 0) { window.fail--; return new Response('{}', { status: 503 }); }
		if (window.hold) await window.hold;
		return real(url, init);
	};
})();`
