package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Find in the panel editor: typing opens it, the preview shows in the grid before anything changes, a click
// on one square fills it and moves on, and Read-only Rights grey a result and refuse it.
func TestChromeEditorFind(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim, func(c *config.Config) {
		zoom := []int{10, 11}
		c.Users[0].Rights = config.Rights{Sources: &zoom}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Find panel","rows":4,"cols":8,"cells":[]}`)
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("editor grid", `document.querySelectorAll('#ed-grid .sode-ed__sq').length === 32`)

	b.eval(`window.press = (el, init) => el.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init }));
		window.grid = document.getElementById('ed-grid');
		window.box = () => document.querySelector('.sode-find input');
		window.find = (text) => { box().value = text; box().dispatchEvent(new Event('input')); };
		window.rows = () => [...document.querySelectorAll('.sode-find__row')].map((r) => r.textContent);
		true`)

	// The grid is drawn before the panel loads and Find attaches, and the lock's welcome keeps the page inert.
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Find panel'`)
	b.waitFor("Find in the menu", `(document.querySelector('.is-sel') || press(grid, { key: 'ArrowDown' }), !!document.querySelector('.sode-find'))`)
	// Two squares selected, then typing opens Find with that letter.
	b.eval(`press(grid, { key: 'ArrowRight', shiftKey: true }); press(grid, { key: 'z' })`)
	b.waitFor("Find open with the typed letter", `box() === document.activeElement && box().value === 'z'`)
	b.eval(`find('zoom')`)
	b.waitFor("Sources first, then Destinations", `rows()[0].includes('Zoom 1') && rows()[1].includes('Zoom 2') && rows()[2].includes('Zoom Return')`)
	b.waitFor("the fill previewed in the grid", `document.querySelectorAll('.sode-ed__sq.is-preview').length === 2 && document.querySelectorAll('.sode-ed__sq--source').length === 0`)
	b.shot("preview")
	b.eval(`press(box(), { key: 'Enter' })`)
	b.waitFor("two Sources placed as previewed", `[...document.querySelectorAll('.sode-ed__sq--source .sode-ed__text')].map((n) => n.textContent).join() === 'Zoom 1,Zoom 2' && !document.querySelector('.is-preview')`)

	// One square: a click fills it, the selection moves on and the next result is highlighted.
	b.eval(`press(grid, { key: 'Escape' }); press(grid, { key: 'ArrowDown' }); press(grid, { key: 'ArrowDown' }); press(grid, { key: 'Enter' })`)
	b.waitFor("Find open on the second row", `box() === document.activeElement && !!document.querySelector('.is-sel[data-r="2"][data-c="1"]')`)
	b.eval(`find('zoom'); press(box(), { key: 'j', code: 'KeyJ', ctrlKey: true })`)
	b.waitFor("Ctrl+J stepped to the second result", `document.querySelector('.sode-find__row.is-hi')?.dataset.i === '1'`)
	b.eval(`press(box(), { key: 'k', code: 'KeyK', ctrlKey: true }); document.querySelector('.sode-find__row.is-hi').click()`)
	b.waitFor("one square filled and the selection moved on", `document.querySelector('.sode-ed__sq[data-r="2"][data-c="1"]')?.textContent.includes('Zoom 1') && !!document.querySelector('.is-sel[data-r="2"][data-c="2"]')`)
	b.waitFor("the next result highlighted and Find kept open", `document.querySelector('.sode-find__row.is-hi')?.textContent.includes('Zoom 2') && box() === document.activeElement`)
	b.eval(`document.querySelector('.sode-find__row.is-hi').click()`)
	b.waitFor("the next square filled", `document.querySelector('.sode-ed__sq[data-r="2"][data-c="2"]')?.textContent.includes('Zoom 2')`)

	// Read-only Rights: outside ones are greyed and marked, and choosing one places nothing.
	b.eval(`find('cg1')`)
	b.waitFor("greyed and marked", `document.querySelectorAll('.sode-find__row.sode__menu-off').length === 2 && [...document.querySelectorAll('.sode-find__row.sode__menu-off')].every((r) => r.textContent.includes('Outside rights'))`)
	before := b.eval(`document.querySelectorAll('.sode-ed__sq.has-cell').length`)
	b.eval(`document.querySelector('.sode-find__row.sode__menu-off').click()`)
	if after := b.eval(`document.querySelectorAll('.sode-ed__sq.has-cell').length`); after != before {
		t.Errorf("a greyed result placed a cell: %v cells before, %v after", before, after)
	}
	b.shot("greyed")
	b.check()
}
