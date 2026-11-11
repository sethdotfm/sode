package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The Lock bar floats over the panel page on the desktop layout: with it present the page is no taller, the grid,
// the Panels link and the status line keep their place, and the bar covers no cell or header control.
func TestChromePanelLockBarFloats(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)
	t.Cleanup(srv.CloseClientConnections)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"take"},{"r":1,"c":2,"kind":"take"},{"r":2,"c":1,"kind":"take"}]}`)

	const geo = `(() => {
		const r = (s) => { const e = document.querySelector(s); const x = e.getBoundingClientRect(); return [x.left, x.top, x.width, x.height].map(Math.round).join(','); };
		return [document.documentElement.scrollHeight, innerHeight, r('#panel-grid'), r('#panel-home'), r('#panel-foot')].join('|');
	})()`

	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the plain link asks for a sign-in", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("signed in with the Lock bar", `document.documentElement.hasAttribute('data-sode-chip') && !document.querySelector('.sode-lock__who').hidden && document.querySelectorAll('.sode-panel__cell').length >= 3 && !document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	with := b.eval(geo).(string)
	// The bar gone, as for a signed-out page or a kiosk: nothing else moves.
	b.eval(`document.documentElement.removeAttribute('data-sode-chip'); document.querySelector('.sode-lock__who').hidden = true`)
	if got := b.eval(geo); got != with {
		t.Fatalf("the Lock bar moved the page: with %v, without %v", with, got)
	}
	b.eval(`document.documentElement.setAttribute('data-sode-chip', ''); document.querySelector('.sode-lock__who').hidden = false`)

	b.waitFor("the Lock bar covers no cell or header control", `(() => {
		const w = document.querySelector('.sode-lock__who').getBoundingClientRect();
		if (!w.width) return false;
		const hit = (e) => { const x = e.getBoundingClientRect(); return x.width && x.left < w.right && x.right > w.left && x.top < w.bottom && x.bottom > w.top; };
		return ![...document.querySelectorAll('.sode-panel__cell, .sode-panels__head > *:not(.sode-panels__spacer):not([hidden])')].some(hit);
	})()`)
	b.shot("lockbar")

	// A real sign-out and sign-in leave the header row where it was. Signing out ends the page's session key, so the
	// page's own note may push the grid down; that is the page, not the bar.
	const head = `(() => { const x = document.querySelector('.sode-panels__head').getBoundingClientRect(); return [x.top, x.height].map(Math.round).join(','); })()`
	headIn := b.eval(head).(string)
	b.eval(`import('./core.js').then((m) => m.emit('logout')), 0`)
	b.waitFor("signed out to the PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	if got := b.eval(head); got != headIn {
		t.Fatalf("signing out moved the header: signed in %v, signed out %v", headIn, got)
	}
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("signed in again", `!document.querySelector('.sode-lock__who').hidden && !document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	if got := b.eval(head); got != headIn {
		t.Fatalf("signing in again moved the header: first %v, again %v", headIn, got)
	}

	// The phone layout leaves the header alone: the bar shows only under a lock screen there.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 400, "height": 800, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	if pad := b.eval(`getComputedStyle(document.querySelector('.sode-panels__head')).paddingInlineStart`); pad != "0px" {
		t.Fatalf("the phone header clears the Lock bar: padding %v", pad)
	}
	b.check()
}
