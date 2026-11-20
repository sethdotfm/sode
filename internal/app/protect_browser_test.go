package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The route view of one destination with no source offers Protect. It goes through the Confirm, the simulator
// holds it on every level, and the view then says "Protected by Sode (Ana)". Shift+x in Keyboard mode clears it,
// and Ctrl+Z undoes the clear.
// Ana is an Administrator here, for Clear Sode's Protects.
func TestChromeProtectFromRouteView(t *testing.T) {
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

	b.waitFor("DSM1 typed", `(() => { const t = document.getElementById('dst'); t.focus(); t.value = 'DSM1'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('dst-opt-0'); })()`)
	b.eval(`document.getElementById('dst').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
	b.waitFor("the shield", `!!document.querySelector('#dst-under [data-protect="set"]')`)
	b.eval(`document.querySelector('#dst-under [data-protect="set"]').click(); true`)
	b.waitFor("Confirm protect", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm protect'`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("Protected by Sode (Ana)", `[...document.querySelectorAll('#dst-under .pb-badge')].some((n) => n.textContent === 'Protected by Sode (Ana)')`)
	b.waitFor("Clear Protect offered", `!!document.querySelector('#dst-under [data-protect="clear"]')`)
	if n := sodeRows(); n != 17 {
		t.Fatalf("Sode's Protect on %d levels, want 17", n)
	}
	b.waitFor("History names the Protect", `[...document.querySelectorAll('#history .pb-badge')].some((n) => n.textContent === 'Protect')`)

	// Keyboard mode: j starts it, Shift+x opens the Confirm for Clear Protect.
	b.eval(`document.body.focus(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'j', code: 'KeyJ', bubbles: true, cancelable: true})); true`)
	b.eval(`document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'X', code: 'KeyX', shiftKey: true, bubbles: true, cancelable: true})); true`)
	b.waitFor("Confirm clear protect", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm clear protect'`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the shield again", `!!document.querySelector('#dst-under [data-protect="set"]')`)
	if n := sodeRows(); n != 0 {
		t.Fatalf("Sode's Protect left on %d levels", n)
	}
	// The Clear Protect row has the badge where the source box is, with no box and no arrow.
	b.waitFor("Clear Protect row", `(() => { const r = [...document.querySelectorAll('#history .sode__hist-route')].find((n) => n.querySelector(':scope > .pb-badge')?.textContent === 'Clear Protect'); return !!r && !r.querySelector('.sode__end--src, .sode__arrow') && r.querySelector('.sode__end--dst')?.textContent.includes('DSM1'); })()`)

	// Ctrl+Z (Cmd+Z on a Mac) undoes the Clear Protect: the Confirm sets the Protect again.
	b.eval(`(() => { const mac = /Mac/.test(navigator.platform); document.body.focus(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'z', code: 'KeyZ', ctrlKey: !mac, metaKey: mac, bubbles: true, cancelable: true})); return true; })()`)
	b.waitFor("Confirm protect for the undo", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm protect' && document.getElementById('confirm').textContent.includes('Undo the Clear Protect')`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("Clear Protect offered again", `!!document.querySelector('#dst-under [data-protect="clear"]')`)
	if n := sodeRows(); n != 17 {
		t.Fatalf("after the undo, Sode's Protect on %d levels, want 17", n)
	}

	// Settings > Routers > Protects lists one set over the API, and Clear Sode's Protects asks the PIN again.
	if rec := apiSend(a, "POST", "/api/v1/protected/3", key, ""); rec.Code != 200 {
		t.Fatalf("API Protect: %d %s", rec.Code, rec.Body.String())
	}
	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("the Protects list", `[...document.querySelectorAll('#protects-list li')].some((n) => n.textContent.includes('DSM2') && n.textContent.includes('Protected by Sode (Ana)'))`)
	b.eval(`document.getElementById('protects-clear-all').click(); true`)
	b.waitFor("the Confirm unlocks", `!!document.querySelector('#protects-confirm .pb-btn--ok:not(:disabled)')`)
	b.eval(`document.querySelector('#protects-confirm .pb-btn--ok').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("cleared", `!!document.querySelector('#protects-confirm + .sode__hint')?.textContent.startsWith("Sode's Protects cleared")`)
	if n := sodeRows(); n != 0 {
		t.Fatalf("Clear Sode's Protects left %d levels", n)
	}
	b.check()
}

// The Palette's Protect opens the picker on Home. A find narrows the list, and two picks go through one Confirm. The
// simulator holds both on every level. Clear Protect lists only those two; Pick all shown and Enter clear them.
func TestChromePaletteProtectPick(t *testing.T) {
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

	pick := func(row, title string) {
		b.eval(`document.activeElement.blur(); true`)
		b.eval(ctrlShiftP)
		b.waitFor("the Palette", paletteShown)
		if row == "action:protect" {
			paletteFind(b, title, row)
			paletteKey(b, "Enter", false)
		} else {
			// A danger row: Enter does not pick it first, so a press on it does.
			b.eval(fmt.Sprintf(`(() => { const i = document.querySelector('.sode-palette__q'); i.value = %q; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`, title))
			b.waitFor("the danger row", `!!document.querySelector('#palette-list [data-key="`+row+`"]')`)
			b.eval(`document.querySelector('#palette-list [data-key="` + row + `"]').click(); true`)
		}
		b.waitFor("the picker", `!document.getElementById('protpick').hidden && document.activeElement.id === 'protpick-find'`)
	}
	confirm := func(title string) {
		b.waitFor(title, `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === '`+title+`'`)
		b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
		b.eval(`document.getElementById('confirm-ok').click(); true`)
	}

	pick("action:protect", "Protect")
	b.eval(`(() => { const i = document.getElementById('protpick-find'); i.value = 'DSM'; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.waitFor("the find narrows the list", `[...document.querySelectorAll('#protpick-list [role=option]')].every((r) => r.textContent.includes('DSM'))`)
	b.eval(`document.querySelector('#protpick-list [data-dst="2"]').click(); true`)
	b.waitFor("one picked", `document.getElementById('protpick-go').textContent === 'Protect 1'`)
	// After typing, Space types; after Ctrl+J, Space picks the outlined row, and Enter goes to the Confirm.
	findKey := func(key, code string, ctrl bool) {
		b.eval(fmt.Sprintf(`document.getElementById('protpick-find').dispatchEvent(new KeyboardEvent('keydown', { key: %q, code: %q, ctrlKey: %v, bubbles: true, cancelable: true })); true`, key, code, ctrl))
	}
	findKey(" ", "Space", false)
	if got := b.eval(`document.getElementById('protpick-go').textContent`).(string); got != "Protect 1" {
		t.Fatalf("Space after typing picked a row: %q", got)
	}
	findKey("j", "KeyJ", true)
	b.waitFor("Ctrl+J outlines the next row", `document.querySelector('#protpick-list [data-cue]')?.dataset.dst === '3'`)
	findKey(" ", "Space", false)
	b.waitFor("Space picks it", `document.getElementById('protpick-go').textContent === 'Protect 2'`)
	findKey("Enter", "Enter", false)
	b.waitFor("the picker closes", `!document.getElementById('protpick')`)
	if got := b.eval(`document.querySelector('#confirm-body > div').textContent`).(string); got != "Protect these 2 destinations on every level?" {
		t.Fatalf("Confirm asks %q", got)
	}
	confirm("Confirm protect")
	b.waitFor("the result", `document.getElementById('result')?.textContent.includes('2 Protected')`)
	if n := sodeRows(); n != 34 {
		t.Fatalf("Sode's Protect on %d levels, want 2 × 17", n)
	}

	pick("action:unprotect", "Clear Protect")
	b.waitFor("only Sode's two", `[...document.querySelectorAll('#protpick-list [role=option]')].map((r) => r.dataset.dst).join() === '2,3'`)
	b.eval(`document.getElementById('protpick-all').click(); true`)
	b.waitFor("both picked", `document.getElementById('protpick-go').textContent === 'Clear 2'`)
	b.eval(`document.getElementById('protpick-find').dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true, cancelable: true })); true`)
	confirm("Confirm clear protect")
	b.waitFor("the result", `document.getElementById('result')?.textContent.includes('2 Protects cleared')`)
	if n := sodeRows(); n != 0 {
		t.Fatalf("Sode's Protect left on %d levels", n)
	}
}
