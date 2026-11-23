package app

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The Lost link banner draws three nodes and two wires with their status, and the reason. A lost wire is still. A good
// wire moves until the page asks for reduced motion. The red outline does not move.
func TestChromeLostLinkBanner(t *testing.T) {
	b := startBrowser(t)
	sim, stopSim := startSimStoppable(t)
	// An Administrator, so no setup note sits under the banner in the review shots.
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	b.open(srv.URL + "/")
	b.waitFor("both links up", `document.documentElement.dataset.conn === 'up' && document.getElementById('conn-down').hidden`)

	// One string per part, so a miss names what the banner drew.
	const shape = `(() => {
		const box = document.getElementById('conn-down');
		const nodes = [...box.querySelectorAll('.sode__down-node')].map((n) => n.className.replace('sode__down-node', '').trim() || 'ok');
		const wires = [...box.querySelectorAll('.sode__down-link')].map((w) => {
			const flow = getComputedStyle(w.querySelector('.sode__down-wire')).animationName;
			const glyph = w.querySelector('.sode__down-word use')?.getAttribute('href') || '';
			return w.querySelector('.sode__down-word').textContent + ':' + glyph + ':' + flow;
		});
		const frame = getComputedStyle(document.querySelector('.sode__frame')).animationName;
		return [box.querySelector('.sode__down-title').textContent, nodes.join(','), wires.join(','), 'frame:' + frame].join(' | ');
	})()`
	want := func(what, got string) {
		t.Helper()
		b.waitFor(what, shape+` === `+strconv.Quote(got))
	}
	why := func(what, part string) {
		t.Helper()
		b.waitFor(what, `(document.querySelector('#conn-down .sode__down-why code')?.textContent || '').includes(`+strconv.Quote(part)+`)`)
	}
	media := func(f ...map[string]string) {
		b.call("Emulation.setEmulatedMedia", map[string]any{"features": append([]map[string]string{}, f...)})
	}
	// Desktop light and dark, then the phone: its banner keeps to the top of the screen. The shots are for review
	// (SODE_BROWSER_SHOTS).
	views := func(name string) {
		t.Helper()
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 2, "mobile": false})
		b.waitFor(name+": desktop layout", `innerWidth === 1440 && document.documentElement.dataset.layout === 'desktop'`)
		b.shot(name + "-desktop-light")
		media(map[string]string{"name": "prefers-color-scheme", "value": "dark"})
		b.shot(name + "-desktop-dark")
		media()
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 2, "mobile": true})
		b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
		b.waitFor(name+": the phone banner shows", `document.documentElement.dataset.layout === 'phone' && !document.getElementById('m-down').hidden && document.getElementById('m-down').offsetHeight > 0`)
		if h := b.eval(`document.getElementById('m-down').offsetHeight / innerHeight`).(float64); h > 0.25 {
			t.Errorf("%s: the phone banner is %.0f%% of the screen, want at most 25%%", name, h*100)
		}
		if b.eval(`document.getElementById('m-down').scrollWidth > document.getElementById('m-down').clientWidth`).(bool) {
			t.Errorf("%s: the phone banner scrolls sideways", name)
		}
		b.shot(name + "-phone")
		b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": false})
		b.call("Emulation.clearDeviceMetricsOverride", map[string]any{})
		b.waitFor(name+": back to desktop", `document.documentElement.dataset.layout === 'desktop'`)
	}

	stopSim()
	want("Router link down: the router node is past the break, the good wire moves",
		"Lost link: Router link down | ok,ok,sode__down-node--down | ok:#pb-i-circle-check:sode-down-flow,lost:#pb-i-circle-x:none | frame:none")
	why("the router's own error shows", "connection refused")
	if txt := b.eval(`document.getElementById('conn-down').textContent`).(string); strings.Contains(txt, "Reconnecting") || strings.Contains(txt, "tries again") {
		t.Fatalf("the banner still names the next try: %q", txt)
	}
	if b.eval(`!!document.querySelector('#conn-down use[href="#pb-i-router"]')`).(bool) {
		t.Fatal("the router node uses Lucide's Wi-Fi router icon")
	}
	media(map[string]string{"name": "prefers-reduced-motion", "value": "reduce"})
	want("reduced motion stops the good wire",
		"Lost link: Router link down | ok,ok,sode__down-node--down | ok:#pb-i-circle-check:none,lost:#pb-i-circle-x:none | frame:none")
	media()
	views("router")

	srv.CloseClientConnections()
	srv.Close()
	// httptest forgets a hijacked connection, so the open socket outlives Close: drop it as a dead server would.
	b.eval(`import('./core.js').then((m) => { m.S.ws?.close(); return true; })`)
	want("Server link down: the server node is past the break, the router is unknown",
		"Lost link: Server link down | ok,sode__down-node--down,sode__down-node--unknown | lost:#pb-i-circle-x:none,unknown:#pb-i-circle-minus:none | frame:none")
	why("the socket's close shows", "connection closed (code ")
	views("server")

	// With Sode down the page cannot reach the socket or healthz: only those refused loads are expected.
	for _, e := range b.problems() {
		if !strings.Contains(e, "net::ERR_CONNECTION_REFUSED") {
			t.Errorf("page error: %s", e)
		}
	}
}
