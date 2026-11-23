package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// While the page is offline, the Lock takes the offline colors and says "Offline, locked", in either order. The main
// page (desktop and phone) and a panel page each get one order.
func TestChromeOfflineLocked(t *testing.T) {
	b := startBrowser(t)
	sim1, stop1 := startSimStoppable(t)
	sim2, stop2 := startSimStoppable(t)
	sim3, stop3 := startSimStoppable(t)
	a := simApp(t, sim1, func(c *config.Config) { c.Users[0].Role = config.RoleTrusted })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := makePanel(t, a, ana, `{"name":"Wall","rows":1,"cols":2,"cells":[{"r":1,"c":1,"kind":"take"}]}`)

	// Reduced motion stops the Lock's transitions, so a color reads the same at any time.
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-reduced-motion", "value": "reduce"}}})
	const red = `(() => { const d = document.createElement('div'); d.style.color = 'var(--pb-status-error)'; document.body.append(d); const c = getComputedStyle(d).color; d.remove(); return c; })()`
	const lockColor = `(() => { const s = getComputedStyle(document.querySelector('.sode-lock')); return s.borderTopWidth !== '0px' && s.borderTopColor; })()`
	const offlineText = `(() => { const n = document.querySelector('.sode-lock__offline'); return !!n && !!n.offsetParent && n.textContent === 'Offline, locked' && document.querySelector('.sode-lock').getAttribute('aria-label') === 'Offline, locked'; })()`
	const onlineText = `(() => { const n = document.querySelector('.sode-lock__offline'); return (!n || !n.offsetParent) && document.querySelector('.sode-lock').getAttribute('aria-label') === 'Locked'; })()`
	// The bus event is sent again until the page has its hello and raises the Lock.
	lockUp := func(where string) {
		b.waitFor(where+": the Lock is up", `!!document.querySelector('.sode-lock.is-on') || (import('./core.js').then((m) => m.emit('localLock')), false)`)
	}
	offlineLook := func(where string) {
		b.waitFor(where+": the Lock has the offline colors", lockColor+` === `+red)
		b.waitFor(where+": the Lock says Offline, locked", offlineText)
	}

	// Main page, desktop: the Lock, then offline. Back online, the Lock's own look returns.
	b.open(srv.URL + "/")
	b.waitFor("online", `document.documentElement.dataset.conn === 'up'`)
	lockUp("main page")
	locked := b.eval(lockColor)
	if locked == false || locked == b.eval(red) {
		t.Fatalf("the Lock's own border is %v, want a color that is not the offline red", locked)
	}
	stop1()
	b.waitFor("offline", `document.documentElement.dataset.conn === 'down'`)
	offlineLook("main page, Lock then offline")
	if err := a.applyConfig(func(c *config.Config) error { c.Address = sim2; return nil }); err != nil {
		t.Fatal(err)
	}
	b.waitFor("online again", `document.documentElement.dataset.conn === 'up'`)
	b.waitFor("the Lock's own colors return", lockColor+` === `+strconv.Quote(locked.(string)))
	b.waitFor("the Lock says only Locked", onlineText)

	// Main page, phone layout: offline, then the Lock.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/")
	b.waitFor("the phone layout, online", `document.documentElement.dataset.layout === 'phone' && document.documentElement.dataset.conn === 'up'`)
	stop2()
	b.waitFor("the phone layout, offline", `document.documentElement.dataset.conn === 'down'`)
	lockUp("phone layout")
	offlineLook("phone layout, offline then Lock")
	b.call("Emulation.clearDeviceMetricsOverride", map[string]any{})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": false})

	// A panel page, signed in: offline, then the Lock.
	if err := a.applyConfig(func(c *config.Config) error { c.Address = sim3; return nil }); err != nil {
		t.Fatal(err)
	}
	waitNamesAndTally(t, a, ana)
	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the panel page asks for a sign-in", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("signed in, online", `!document.querySelector('.sode-lock.is-on') && document.querySelectorAll('.sode-panel__cell').length >= 1 && document.getElementById('panel-offline').hidden`)
	stop3()
	b.waitFor("the panel page's Offline cover", `!document.getElementById('panel-offline').hidden && document.documentElement.dataset.conn === 'down'`)
	lockUp("panel page")
	offlineLook("panel page, offline then Lock")
	b.check()
}
