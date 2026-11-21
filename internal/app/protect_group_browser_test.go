package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The Protect picker protects 32 destinations in one Confirm. History shows one Group row with the Protect chip and
// "32 destinations". It opens to 32 rows. A second Protect soon after folds into the same row. Its Undo all frees
// all 34 in one Confirm.
func TestChromeProtectGroupInHistory(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.ProtectDevice = 126
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	key := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, key)
	sodeRows := func() int {
		return strings.Count(apiDo(a, "GET", "/api/v1/protected", key, nil).Body.String(), `"sodeUser":"Ana"`)
	}

	b.open(srv.URL + "/")
	b.waitFor("Settings button", `!!document.getElementById('settings-open')`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)

	pick := func() {
		b.eval(`document.activeElement.blur(); true`)
		b.eval(ctrlShiftP)
		b.waitFor("the Palette", paletteShown)
		paletteFind(b, "Protect", "action:protect")
		paletteKey(b, "Enter", false)
		b.waitFor("the picker", `!document.getElementById('protpick').hidden && document.activeElement.id === 'protpick-find'`)
	}
	confirm := func(title string) {
		b.waitFor(title, `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === '`+title+`'`)
		b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
		b.eval(`document.getElementById('confirm-ok').click(); true`)
	}
	top := `document.querySelectorAll('#history > .sode__hist-row')`
	topRow := `(() => { const r = document.querySelector('#history > .sode__hist-row'); return r ? { chip: r.querySelector('.sode__hist-route > .pb-badge')?.textContent, dst: r.querySelector('.sode__end--dst')?.textContent, note: r.querySelector('.sode__hist-route .sode__hist-note')?.textContent || '', status: r.querySelector('.sode__status-text')?.textContent } : null; })()`

	// A click, then a Shift+click 32 pickable rows on: the Router lock on 12 is left out.
	pick()
	b.eval(`(() => { const rows = [...document.querySelectorAll('#protpick-list [role=option]')].filter((r) => r.getAttribute('aria-disabled') !== 'true'); rows[0].click(); rows[31].dispatchEvent(new MouseEvent('click', { shiftKey: true, bubbles: true })); return true; })()`)
	b.waitFor("32 picked", `document.getElementById('protpick-go').textContent === 'Protect 32'`)
	b.eval(`document.getElementById('protpick-go').click(); true`)
	confirm("Confirm protect")
	b.waitFor("the result", `document.getElementById('result')?.textContent.includes('32 Protected')`)
	if n := sodeRows(); n != 32*17 {
		t.Fatalf("Sode's Protect on %d levels, want 32 × 17", n)
	}
	b.waitFor("one History row for the 32", top+`.length === 1 && (() => { const r = `+topRow+`; return r.chip === 'Protect' && r.dst === '32 destinations' && r.status === 'ok'; })()`)
	b.eval(`document.querySelector('#history > .sode__hist-row [aria-controls^="hist-folder-"]').click(); true`)
	b.waitFor("it opens to 32 rows", `document.querySelectorAll('#history .sode__hist-folder > .sode__hist-row').length === 32`)

	// A second Protect within the stack gap folds into the same row.
	pick()
	b.eval(`document.querySelector('#protpick-list [data-dst="40"]').click(); document.querySelector('#protpick-list [data-dst="41"]').click(); true`)
	b.waitFor("2 picked", `document.getElementById('protpick-go').textContent === 'Protect 2'`)
	b.eval(`document.getElementById('protpick-go').click(); true`)
	confirm("Confirm protect")
	b.waitFor("the result", `document.getElementById('result')?.textContent.includes('2 Protected')`)
	b.waitFor("the two actions fold into one row", top+`.length === 1 && (() => { const r = `+topRow+`; return r.dst === '34 destinations' && r.note.includes('2 actions'); })()`)

	// Undo all frees all 34 in one Confirm. The router's Protect state reaches the page a moment after the entries, so
	// this waits until each row inside offers its Undo.
	b.eval(`(() => { const d = document.querySelector('#history > .sode__hist-row [aria-controls^="hist-folder-"]'); if (d.getAttribute('aria-expanded') !== 'true') d.click(); return true; })()`)
	b.waitFor("each of the 34 offers Undo", `[...document.querySelectorAll('#history .sode__hist-folder > .sode__hist-row .sode__hist-actions button')].filter((x) => x.textContent.includes('Undo') && !x.disabled).length === 34`)
	b.eval(`[...document.querySelectorAll('#history > .sode__hist-row > .sode__hist-actions button')].find((x) => x.textContent.includes('Undo all')).click(); true`)
	b.waitFor("the Confirm asks for all 34", `!document.getElementById('confirm').hidden && document.getElementById('confirm').textContent.includes('Undo the Protects: free the Protect on these 34 destinations?')`)
	confirm("Confirm free protect")
	b.waitFor("the result", `document.getElementById('result')?.textContent.includes('34 Protects freed')`)
	if n := sodeRows(); n != 0 {
		t.Fatalf("Sode's Protect left on %d levels", n)
	}
	b.waitFor("one Free Protect row for the undo", `(() => { const r = `+topRow+`; return r.chip === 'Free Protect' && r.dst === '34 destinations' && r.note.includes('undo'); })()`)
	b.check()
}
