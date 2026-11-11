package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Cmd or Ctrl+A on the editor grid selects every square, and the same chord in the name field is left to the field.
func TestChromeEditorSelectAll(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Chord panel","rows":4,"cols":8,"cells":[{"r":1,"c":1,"rs":2,"cs":2,"kind":"take"}]}`)
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Chord panel'`)

	// Synthetic keys never run the browser's own select all, so the name field check is that the editor left the key alone.
	b.eval(`window.press = (el, init) => { const e = new KeyboardEvent('keydown', { bubbles: true, cancelable: true, key: 'a', code: 'KeyA', ...init }); el.dispatchEvent(e); return e.defaultPrevented; };
		window.grid = document.getElementById('ed-grid');
		window.picked = () => document.querySelectorAll('#ed-grid .is-sel').length;
		window.squares = () => document.querySelectorAll('#ed-grid .sode-ed__sq').length;
		true`)
	for _, mod := range []string{"ctrlKey", "metaKey"} {
		b.eval(`press(grid, { key: 'Escape', code: 'Escape' })`)
		b.waitFor("nothing selected", `picked() === 0`)
		if b.eval(fmt.Sprintf(`grid.focus(), press(grid, { %s: true })`, mod)) != true {
			t.Fatalf("%s+A on the grid was not taken by the editor", mod)
		}
		b.waitFor(mod+"+A selects every square", `squares() === 29 && picked() === 29`)
	}

	// One square picked, then the name field focused: the chord there must leave the grid's selection as it is.
	b.eval(`press(grid, { key: 'Escape', code: 'Escape' }); press(grid, { key: 'ArrowDown', code: 'ArrowDown' })`)
	b.waitFor("one square selected", `picked() === 1`)
	b.eval(`document.getElementById('ed-name').focus(); window.before = picked(); true`)
	if b.eval(`press(document.getElementById('ed-name'), { ctrlKey: true }) || press(document.getElementById('ed-name'), { metaKey: true })`) != false {
		t.Fatal("Cmd or Ctrl+A in the name field was taken from the field")
	}
	b.waitFor("the grid selection is untouched", `picked() === before && picked() < 29`)
	b.check()
}
