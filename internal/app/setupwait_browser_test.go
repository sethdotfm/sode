package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// While another browser runs setup, a phone sees the wait page with Use desktop layout and no Take
// control. Use desktop layout gives it Take control, which opens the wizard here and tells the
// other browser.
func TestChromeSetupWaitPage(t *testing.T) {
	b := startBrowser(t)
	a := setupApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	first := dialPeer(t, a)
	if r := first.ask("setupClaim", 1, nil); r["ok"] != true {
		t.Fatalf("first browser's claim: %v", r)
	}

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	wait := `document.getElementById('setup-wait')`
	b.waitFor("the phone's wait page", `(() => {
		const w = `+wait+`;
		return !!w?.offsetParent && w.textContent.includes('Open Sode on a computer to set it up.')
			&& !!w.querySelector('[data-desktop-layout]') && !document.getElementById('setup-take')
			&& !document.getElementById('setup-dialog').offsetParent;
	})()`)
	b.waitFor("the desktop-only note is no box in the modal's box", `(() => {
		const s = getComputedStyle(`+wait+`.querySelector('.sode__desktop-only'));
		return s.borderTopWidth === '0px' && s.paddingTop === '0px' && s.paddingLeft === '0px';
	})()`)
	b.shot("setup-wait-phone")

	b.eval(wait + `.querySelector('[data-desktop-layout]').click(); true`)
	b.waitFor("the desktop layout", `document.documentElement.dataset.layout === 'desktop'`)
	b.waitFor("Take control, since the first browser holds setup", `!!document.getElementById('setup-take')?.offsetParent && !document.getElementById('setup-dialog').offsetParent`)
	b.shot("setup-wait-desktop")

	b.eval(`document.getElementById('setup-take').click(); true`)
	b.waitFor("the wizard here", `!!document.getElementById('setup-dialog').offsetParent && document.getElementById('setup-title').textContent === 'Set up Sode' && `+wait+`.hidden`)
	first.until("setupTaken")
	b.check()
}

// Take control moves setup to the other browser, and the first one keeps its answers in its own tab
// (sessionStorage, never the PINs). When it takes control again, the answers come back.
func TestChromeSetupTakeControlKeepsAnswers(t *testing.T) {
	a := startBrowser(t)
	b := startBrowser(t)
	app := setupApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { app.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(app.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	type_ := func(tab *tab, id, text string) {
		tab.eval(`(() => { const i = document.getElementById('` + id + `'); i.focus(); i.value = '` + text + `'; i.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	}
	wizard := `!!document.getElementById('setup-dialog').offsetParent && document.getElementById('setup-title').textContent === 'Set up Sode'`

	a.open(srv.URL + "/")
	a.waitFor("A runs setup", wizard)
	type_(a, "setup-brandText", "Studio B")
	type_(a, "setup-name", "Ada")
	type_(a, "setup-pin", "2468")
	type_(a, "setup-pin2", "2468")
	a.waitFor("A keeps its answers in this tab, with no PIN", `(() => {
		const k = JSON.parse(sessionStorage.getItem('sode-setup-answers') || 'null');
		return k?.name === 'Ada' && k.brandText === 'Studio B' && !('pin' in k) && !('pin2' in k);
	})()`)

	b.open(srv.URL + "/")
	b.waitFor("B waits", `document.getElementById('setup-wait-title').textContent === 'Setup is running in another browser' && !!document.getElementById('setup-take')?.offsetParent`)
	b.waitFor("the Take control tip says nothing is lost", `document.getElementById('setup-take').dataset.pbTip === 'The other browser keeps its answers'`)
	b.eval(`document.getElementById('setup-take').click(); true`)
	b.waitFor("B runs setup with its own answers", wizard+` && document.getElementById('setup-name').value === ''`)

	a.waitFor("A says its answers stay", `document.getElementById('setup-taken')?.textContent === 'Another browser took control. This browser keeps your answers.'
		&& document.getElementById('setup-wait-title').textContent === 'Setup is running in another browser'`)
	a.eval(`document.getElementById('setup-take').click(); true`)
	a.waitFor("A gets its answers back, and no PIN", wizard+` && document.getElementById('setup-name').value === 'Ada'
		&& document.getElementById('setup-brandText').value === 'Studio B' && document.getElementById('setup-pin').value === ''`)
	a.check()
	b.check()
}
