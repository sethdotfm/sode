package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// The editor's corner grip extends numbering with a tip, the fill order picker changes the preview, the toolbar
// counts an unassigned cell, and Test it live saves and opens a panel that leaves that cell out.
func TestChromeEditorGripOrderLive(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Grip panel","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"source","n":10},
		{"r":1,"c":2,"kind":"source","n":11},
		{"r":4,"c":8,"kind":"source"}]}`)
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}
	signIn := func() {
		b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
		for _, d := range "1234" {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
		b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	}
	b.open(srv.URL + "/panels/editor?panel=" + id)
	signIn()
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Grip panel'`)
	b.eval(`window.press = (el, init) => el.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));
		window.grid = document.getElementById('ed-grid');
		window.sq = (r, c) => document.querySelector('.sode-ed__sq[data-r="' + r + '"][data-c="' + c + '"]');
		window.mid = (n) => { const r = n.getBoundingClientRect(); return { clientX: r.left + r.width / 2, clientY: r.top + r.height / 2 }; };
		window.ptr = (el, type, at) => el.dispatchEvent(new PointerEvent(type, { bubbles: true, button: 0, pointerId: 1, ...at }));
		true`)

	// Select the two Sources and drag the grip three squares right.
	b.eval(`press(grid, { key: 'ArrowDown' }); press(grid, { key: 'ArrowRight', shiftKey: true })`)
	b.waitFor("grip on the selection", `!!document.querySelector('.sode-ed__grip')`)
	b.eval(`ptr(document.querySelector('.sode-ed__grip'), 'pointerdown', mid(document.querySelector('.sode-ed__grip'))); ptr(document, 'pointermove', mid(sq(1, 5)))`)
	b.waitFor("tip with the first and last name and the count", `/^3 squares, .+ to .+$/.test(document.querySelector('.sode-ed__tip')?.textContent || '')`)
	b.waitFor("preview in the squares", `document.querySelectorAll('.sode-ed__sq.is-preview').length === 3`)
	b.shot("grip")
	b.eval(`ptr(document, 'pointerup', mid(sq(1, 5)))`)
	b.waitFor("numbering extended and selected", `document.querySelectorAll('.sode-ed__sq--source:not(.is-unassigned)').length === 5 && !!document.querySelector('.is-sel[data-r="1"][data-c="5"]') && !document.querySelector('.is-preview')`)

	// Fill order: a 2x2 block previews down as 1 then 2 in the first column.
	b.eval(`press(grid, { key: 'Escape' }); for (const k of ['ArrowDown', 'ArrowDown', 'ArrowDown']) press(grid, { key: k }); press(grid, { key: 'ArrowRight', shiftKey: true }); press(grid, { key: 'ArrowDown', shiftKey: true }); press(grid, { key: 'z' })`)
	b.waitFor("Find open", `document.querySelector('.sode-find input') === document.activeElement`)
	b.eval(`const i = document.querySelector('.sode-find input'); i.value = 'zoom'; i.dispatchEvent(new Event('input'))`)
	b.waitFor("across by default", `sq(3, 2)?.textContent.includes('Zoom 2') && !!document.querySelector('.sode-find__orders [data-order="across"][aria-pressed="true"]')`)
	b.eval(`document.querySelector('.sode-find__orders [data-order="down"]').click()`)
	b.waitFor("down order previewed", `sq(4, 1)?.textContent.includes('Zoom 2') && !sq(3, 2)?.textContent.includes('Zoom 2') && !!document.querySelector('.sode-find__orders [data-order="down"][aria-pressed="true"]')`)
	b.shot("order")

	// Test it live saves the edits, then runs the saved panel in the editor; the unassigned cell is not on it.
	b.eval(`window.opened = ''; window.open = (u) => { window.opened = u; return null }; document.getElementById('ed-live').click()`)
	b.waitSaved("saved before the live view")
	b.waitFor("live shown after saving", `!document.getElementById('ed-live-view').hidden`)
	b.waitFor("only assigned cells live", `document.querySelectorAll('#ed-live-view .sode-panel__cell').length === 5`)
	if got := b.eval(`window.opened`); got != "" {
		t.Errorf("a window was opened: %v", got)
	}
	b.check()
}
