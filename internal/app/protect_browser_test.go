package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The route view of one destination with no source offers Protect. It goes through the Confirm, the simulator
// holds it on every level, and the view then says "Protected by Sode (Ana)". Shift+x in Keyboard mode clears it.
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
