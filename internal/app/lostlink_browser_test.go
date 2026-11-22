package app

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// The Lost link banner draws three nodes and two wires. A lost wire is still and has an X on it. A good wire moves
// until the page asks for reduced motion. The red outline does not move.
func TestChromeLostLinkBanner(t *testing.T) {
	b := startBrowser(t)
	sim, stopSim := startSimStoppable(t)
	a := simApp(t, sim)
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
			const flow = getComputedStyle(w.querySelector('.sode__down-wire'), '::before').animationName;
			const x = w.querySelector('use')?.getAttribute('href') || '';
			return w.querySelector('.sode__down-word').textContent + ':' + flow + ':' + x;
		});
		const frame = getComputedStyle(document.querySelector('.sode__frame')).animationName;
		return [box.querySelector('.sode__down-title').textContent, nodes.join(','), wires.join(','), 'frame:' + frame].join(' | ');
	})()`
	want := func(what, got string) {
		t.Helper()
		b.waitFor(what, shape+` === `+strconv.Quote(got))
	}

	stopSim()
	want("Router link down: the router node is past the break, the good wire moves",
		"Lost link: Router link down | ok,ok,sode__down-node--down | ok:sode-down-flow:,lost:none:#pb-i-x | frame:none")
	if txt := b.eval(`document.getElementById('conn-down').textContent`).(string); strings.Contains(txt, "Reconnecting") || strings.Contains(txt, "tries again") {
		t.Fatalf("the banner still names the next try: %q", txt)
	}
	if b.eval(`!!document.querySelector('#conn-down use[href="#pb-i-router"]')`).(bool) {
		t.Fatal("the router node uses Lucide's Wi-Fi router icon")
	}

	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-reduced-motion", "value": "reduce"}}})
	want("reduced motion stops the good wire",
		"Lost link: Router link down | ok,ok,sode__down-node--down | ok:none:,lost:none:#pb-i-x | frame:none")
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{}})

	srv.CloseClientConnections()
	srv.Close()
	// httptest forgets a hijacked connection, so the open socket outlives Close: drop it as a dead server would.
	b.eval(`import('./core.js').then((m) => { m.S.ws?.close(); return true; })`)
	want("Server link down: the server node is past the break, the router is unknown",
		"Lost link: Server link down | ok,sode__down-node--down,sode__down-node--unknown | lost:none:#pb-i-x,unknown:none: | frame:none")

	// With Sode down the page cannot reach the socket or healthz: only those refused loads are expected.
	for _, e := range b.problems() {
		if !strings.Contains(e, "net::ERR_CONNECTION_REFUSED") {
			t.Errorf("page error: %s", e)
		}
	}
}
