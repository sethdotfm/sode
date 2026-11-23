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
