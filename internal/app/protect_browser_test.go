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
// holds it on every level, and the view then says "Protected by you", with the badge's owner mark. Shift+x in
// Keyboard mode frees it. History then offers Undo only on the newest Protect entry, and follows a later change.
// Ctrl+Z undoes this tab's newest Free Protect.
// Ana is an Administrator here, for Free Sode's Protects.
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
	b.waitFor("Protected by you", `[...document.querySelectorAll('#dst-under .pb-badge')].some((n) => n.textContent === 'Protected by you' && n.dataset.owner === 'you')`)
	b.waitFor("Free Protect offered", `!!document.querySelector('#dst-under [data-protect="clear"]')`)
	if n := sodeRows(); n != 17 {
		t.Fatalf("Sode's Protect on %d levels, want 17", n)
	}
	b.waitFor("History names the Protect", `[...document.querySelectorAll('#history .pb-badge')].some((n) => n.textContent === 'Protect')`)
	// Its detail lists the sources on DSM1 when it ran, in place of Before.
	b.eval(`[...document.querySelectorAll('#history .sode__hist-row')].find((r) => r.querySelector('.sode__hist-route > .pb-badge')?.textContent === 'Protect').querySelector('.pb-disclosure, [aria-controls^="hist-before-"]').click(); true`)
	b.waitFor("Source on DSM1", `[...document.querySelectorAll('#history .sode__hist-before .sode__label')].some((n) => /^Sources? on .*DSM1/.test(n.textContent))`)

	// Keyboard mode: j starts it, Shift+x opens the Confirm for Free Protect.
	b.eval(`document.body.focus(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'j', code: 'KeyJ', bubbles: true, cancelable: true})); true`)
	b.eval(`document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'X', code: 'KeyX', shiftKey: true, bubbles: true, cancelable: true})); true`)
	b.waitFor("Confirm free protect", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm free protect'`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the shield again", `!!document.querySelector('#dst-under [data-protect="set"]')`)
	if n := sodeRows(); n != 0 {
		t.Fatalf("Sode's Protect left on %d levels", n)
	}
	// Only the newest Protect entry of a destination offers Undo, and only while the opposite is possible.
	undoOn := `((word) => { const r = [...document.querySelectorAll('#history .sode__hist-row')].find((r) => [...r.querySelectorAll('.pb-badge')].some((b) => b.textContent === word)); const u = r && [...r.querySelectorAll('.sode__hist-actions button')].find((b) => b.textContent.includes('Undo')); return u ? (u.disabled ? 'off' : 'on') : 'none'; })`
	b.waitFor("Undo on the Free Protect, not on the older Protect", undoOn+`('Free Protect') === 'on' && `+undoOn+`('Protect') === 'off'`)
	// A newer Protect entry (here from the API) turns that Undo off at once.
	if rec := apiSend(a, "POST", "/api/v1/protected/2", key, ""); rec.Code != 200 {
		t.Fatalf("API Protect: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("the Free Protect's Undo follows", undoOn+`('Free Protect') === 'off'`)
	if rec := apiSend(a, "DELETE", "/api/v1/protected/2", key, ""); rec.Code != 200 {
		t.Fatalf("API Free Protect: %d %s", rec.Code, rec.Body.String())
	}
	// The newest Free Protect row has the badge where the source box is, with no box and no arrow. It must be the
	// top row, the API's. The tab's own older Free Protect row also names DSM1. Ctrl+Z before the API's entry comes
	// finds nothing to undo.
	b.waitFor("Free Protect row", `(() => { const r = document.querySelector('#history .sode__hist-route'); return r?.querySelector(':scope > .pb-badge')?.textContent === 'Free Protect' && !r.querySelector('.sode__end--src, .sode__arrow') && r.querySelector('.sode__end--dst')?.textContent.includes('DSM1'); })()`)

	// Ctrl+Z steps back only through this tab's takes, and the API's Free Protect is newer than this tab's. So this tab
	// protects and frees DSM1 once more with the route view's shield.
	for _, step := range []struct{ offer, title string }{{"set", "Confirm protect"}, {"clear", "Confirm free protect"}} {
		b.waitFor("the shield offers "+step.offer, `!!document.querySelector('#dst-under [data-protect="`+step.offer+`"]')`)
		b.eval(`document.querySelector('#dst-under [data-protect="` + step.offer + `"]').click(); true`)
		b.waitFor(step.title, `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === '`+step.title+`' && !document.getElementById('confirm-ok').disabled`)
		b.eval(`document.getElementById('confirm-ok').click(); true`)
		b.waitFor(step.title+" closes", `document.getElementById('confirm').hidden`)
	}
	b.waitFor("the shield after the Free Protect", `!!document.querySelector('#dst-under [data-protect="set"]')`)
	b.waitFor("four Protect entries from this tab", `JSON.parse(sessionStorage.getItem('sode-own-takes') || '[]').length === 4`)
	// Ctrl+Z (Cmd+Z on a Mac) undoes this tab's Free Protect: the Confirm sets the Protect again.
	b.eval(`(() => { const mac = /Mac/.test(navigator.platform); document.body.focus(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'z', code: 'KeyZ', ctrlKey: !mac, metaKey: mac, bubbles: true, cancelable: true})); return true; })()`)
	b.waitFor("Confirm protect for the undo", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm protect' && document.getElementById('confirm').textContent.includes('Undo the Free Protect')`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("Free Protect offered again", `!!document.querySelector('#dst-under [data-protect="clear"]')`)
	if n := sodeRows(); n != 17 {
		t.Fatalf("after the undo, Sode's Protect on %d levels, want 17", n)
	}

	// Settings > Routers > Protects lists one set over the API, and Free Sode's Protects asks the PIN again.
	if rec := apiSend(a, "POST", "/api/v1/protected/3", key, ""); rec.Code != 200 {
		t.Fatalf("API Protect: %d %s", rec.Code, rec.Body.String())
	}
	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("the Protects list", `[...document.querySelectorAll('#protects-list .sode__prow')].some((n) => n.textContent.includes('DSM2') && n.textContent.includes('Protected by you'))`)
	b.eval(`document.getElementById('protects-clear-all').click(); true`)
	b.waitFor("the Confirm unlocks", `!!document.querySelector('#protects-free-go:not(:disabled)')`)
	b.eval(`document.getElementById('protects-free-go').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("cleared", `document.getElementById('protects-status').textContent.startsWith("Sode freed")`)
	if n := sodeRows(); n != 0 {
		t.Fatalf("Free Sode's Protects left %d levels", n)
	}
	b.check()
}

// The Palette's Protect opens the picker on Home. A find narrows the list, and two picks go through one Confirm. The
// simulator holds both on every level. Free Protect lists only those two; Pick all shown and Enter free them.
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

	pick()
	b.waitFor("All destinations is the filter", `document.querySelector('#protpick [data-filter="all"]').getAttribute('aria-pressed') === 'true'`)
	b.eval(`(() => { const i = document.getElementById('protpick-find'); i.value = 'DSM'; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.waitFor("the find narrows the list", `[...document.querySelectorAll('#protpick-list [role=option]')].every((r) => r.textContent.includes('DSM'))`)
	b.eval(`document.querySelector('#protpick-list [data-dst="2"]').click(); true`)
	b.waitFor("one picked, in the neutral chip", `document.getElementById('protpick-go').textContent === 'Protect 1' && document.getElementById('protpick-count').textContent === '1 picked' && document.getElementById('protpick-count').classList.contains('pb-badge') && !document.getElementById('protpick-count').matches('[class*="pb-badge--"]')`)
	// After typing, Space types and Cmd+Space does nothing; Ctrl+J outlines the next row, and Ctrl+Space picks it.
	findKey := func(key, code string, ctrl, meta bool) {
		b.eval(fmt.Sprintf(`document.getElementById('protpick-find').dispatchEvent(new KeyboardEvent('keydown', { key: %q, code: %q, ctrlKey: %v, metaKey: %v, bubbles: true, cancelable: true })); true`, key, code, ctrl, meta))
	}
	findKey(" ", "Space", false, false)
	findKey(" ", "Space", false, true)
	if got := b.eval(`document.getElementById('protpick-go').textContent`).(string); got != "Protect 1" {
		t.Fatalf("Space or Cmd+Space after typing picked a row: %q", got)
	}
	findKey("j", "KeyJ", true, false)
	b.waitFor("Ctrl+J outlines the next row", `document.querySelector('#protpick-list [data-cue]')?.dataset.dst === '3'`)
	findKey(" ", "Space", true, false)
	b.waitFor("Ctrl+Space picks it", `document.getElementById('protpick-go').textContent === 'Protect 2'`)
	findKey("Enter", "Enter", false, false)
	b.waitFor("the picker closes", `!document.getElementById('protpick')`)
	if got := b.eval(`document.querySelector('#confirm-body > div').textContent`).(string); got != "Protect these 2 destinations on every level?" {
		t.Fatalf("Confirm asks %q", got)
	}
	confirm("Confirm protect")
	b.waitFor("the result", `document.getElementById('result')?.textContent.includes('2 Protected')`)
	if n := sodeRows(); n != 34 {
		t.Fatalf("Sode's Protect on %d levels, want 2 × 17", n)
	}

	// Currently protected lists Sode's two and the simulator's Router lock, which shows its owner and cannot be picked.
	pick()
	b.eval(`document.querySelector('#protpick [data-filter="protected"]').click(); true`)
	b.waitFor("the protected ones", `[...document.querySelectorAll('#protpick-list [role=option]')].map((r) => r.dataset.dst).join() === '2,3,12'`)
	b.waitFor("the Router lock cannot be picked", `(() => { const r = document.querySelector('#protpick-list [data-dst="12"]'); return r.getAttribute('aria-disabled') === 'true' && r.textContent.includes('another system'); })()`)
	// A click, then a Shift+click: the range picks Sode's two and leaves the Router lock out.
	b.eval(`document.querySelector('#protpick-list [data-dst="2"]').click(); true`)
	b.eval(`document.querySelector('#protpick-list [data-dst="12"]').dispatchEvent(new MouseEvent('click', { shiftKey: true, bubbles: true })); true`)
	b.waitFor("both picked", `document.getElementById('protpick-go').textContent === 'Free 2' && document.getElementById('protpick-alt').hidden`)
	// With a free one picked too, Protect comes first, and Free is the second button.
	b.eval(`document.querySelector('#protpick [data-filter="free"]').click(); true`)
	b.waitFor("only free ones", `!document.querySelector('#protpick-list [data-dst="2"]') && !!document.querySelector('#protpick-list [data-dst="5"]')`)
	b.eval(`document.querySelector('#protpick-list [data-dst="5"]').click(); true`)
	b.waitFor("a mix protects first", `document.getElementById('protpick-go').textContent === 'Protect 1' && !document.getElementById('protpick-alt').hidden && document.getElementById('protpick-alt').textContent === 'Free 2' && document.getElementById('protpick-count').textContent === '3 picked'`)
	b.eval(`document.getElementById('protpick-alt').click(); true`)
	confirm("Confirm free protect")
	b.waitFor("the result", `document.getElementById('result')?.textContent.includes('2 Protects freed')`)
	if n := sodeRows(); n != 0 {
		t.Fatalf("Sode's Protect left on %d levels", n)
	}
}
