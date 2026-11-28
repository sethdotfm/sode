package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// A panel page and the main page that this browser opened also open while Sode is down. Each shows its Offline state,
// then goes live when Sode answers again, with no reload.
func TestChromeServiceWorkerOpensWithSodeDown(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleTrusted })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"api","rows":1,"cols":2,"cells":[{"r":1,"c":1,"kind":"take"}]}`)
	secret := instanceKey(t, a, id, ana, "Hall")

	// The page's Sode: the same address each time it starts, as a restarted Sode has. seamServer runs the app.
	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	var live *httptest.Server
	up := func() {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		live = &httptest.Server{Listener: l, Config: &http.Server{Handler: a.Handler(web.FS)}}
		live.Start()
	}
	down := func() {
		live.CloseClientConnections()
		live.Close()
	}
	up()
	t.Cleanup(func() { down() })
	base := "http://" + addr

	b.open(base + "/panels/" + id + "?instance=Hall#key=" + strings.TrimPrefix(secret, keyPrefix))
	const cell = `document.querySelectorAll('.sode-panel__cell').length >= 1`
	b.waitFor("the panel page runs", cell+` && document.getElementById('panel-offline').hidden`)
	b.waitFor("the service worker has the page and its files", `navigator.serviceWorker.controller !== null && caches.match(location.pathname).then((r) => !!r) && caches.match('panelpage.js').then((r) => !!r)`)
	b.open(base + "/")
	b.waitFor("the main page runs", `document.documentElement.dataset.conn === 'up'`)
	b.waitFor("the service worker has the main page", `caches.match('./').then((r) => !!r)`)

	// Sode down: the main page opens from the worker and says the Server link is lost.
	down()
	b.open(base + "/")
	b.waitFor("the main page opens with Sode down", `!document.getElementById('conn-down').hidden && document.getElementById('conn-down').textContent.includes('the Server link is down')`)
	b.eval(`window.sodeStayed = 1`)
	up()
	b.waitFor("the main page connects with no reload", `document.documentElement.dataset.conn === 'up' && document.getElementById('conn-down').hidden && window.sodeStayed === 1`)

	// The panel page opens on its stored key, shows its Offline cover and goes live when Sode answers.
	down()
	b.open(base + "/panels/" + id + "?instance=Hall")
	b.waitFor("the panel page opens with Sode down", `!document.getElementById('panel-offline').hidden && document.querySelector('.sode-panel__offline-title').textContent === 'Offline'`)
	b.eval(`window.sodeStayed = 1`)
	up()
	b.waitFor("the panel page goes live with no reload", cell+` && document.getElementById('panel-offline').hidden && window.sodeStayed === 1`)
	tallyHas(t, a, ana, simDst, simDst)

	// With Sode down the pages could not reach the socket, healthz or the manifest: only those refused loads are expected.
	for _, e := range b.problems() {
		if !strings.Contains(e, "net::ERR_CONNECTION_REFUSED") {
			t.Errorf("page error: %s", e)
		}
	}
}
