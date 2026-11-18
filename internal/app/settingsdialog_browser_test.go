package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// A dialog opened low in Settings keeps the page where it was. A real wheel over its scrim does
// not scroll the page behind it. With overflow:hidden on html, body (patchbay's height:100%,
// overflow-x:auto) scrolled on its own, so the scroll range shrank and Settings jumped up.
func TestChromeSettingsDialogKeepsScroll(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	b.waitFor("Manage users shown", `!!document.getElementById('users-manage')?.offsetParent`)
	b.eval(`(() => { const r = document.getElementById('users-manage').getBoundingClientRect(); window.scrollTo({ top: scrollY + r.top - innerHeight / 2, behavior: 'instant' }); return true; })()`)
	b.waitFor("page scrolled down", `scrollY > 1500`)
	y := b.eval(`scrollY`)

	b.eval(`document.getElementById('users-manage').focus({ preventScroll: true }); document.getElementById('users-manage').click(); true`)
	b.waitFor("Users dialog open", `!!document.querySelector('.sode__users-dlg')?.offsetParent`)
	if got := b.eval(`scrollY`); got != y {
		t.Fatalf("opening the dialog moved Settings from %v to %v", y, got)
	}

	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": 20, "y": 200, "deltaX": 0, "deltaY": 600})
	if got := b.eval(`new Promise((r) => setTimeout(() => r(scrollY), 400))`); got != y {
		t.Fatalf("a wheel over the scrim scrolled Settings from %v to %v", y, got)
	}

	b.eval(`document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true`)
	b.waitFor("Users dialog closed", `!document.querySelector('.sode__users-dlg')?.offsetParent`)
	if got := b.eval(`scrollY`); got != y {
		t.Fatalf("closing the dialog left Settings at %v, not %v", got, y)
	}
	b.check()
}
