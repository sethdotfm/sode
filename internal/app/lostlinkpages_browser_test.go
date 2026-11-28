package app

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// /log and /panels show the same Lost link banner as the main page when the Sode server stops. On /log the Load older
// and Download buttons stay off while the link is down, and the header starts below the banner. All comes back
// when Sode answers again.
func TestChromeLostLinkOnEveryPage(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})

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
	// A hijacked socket outlives Close, so the page drops its own (keepSockets); its next try finds no Sode.
	down := func() {
		live.CloseClientConnections()
		live.Close()
		b.eval(`(window.sodeSockets || []).forEach((s) => s.close()); true`)
	}
	up()
	t.Cleanup(func() { live.CloseClientConnections(); live.Close() })
	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": keepSockets})
	base := "http://" + addr

	const banner = `!document.getElementById('conn-down').hidden && document.querySelector('#conn-down .sode__down-title')?.textContent === 'Lost link: the Server link is down'
		&& document.documentElement.dataset.conn === 'down'`
	const noBanner = `document.getElementById('conn-down').hidden && document.documentElement.dataset.conn === 'up'`
	const buttons = `['log-older', 'log-download-rows', 'log-download-full'].map((id) => document.getElementById(id).disabled)`

	b.open(base + "/log")
	b.waitFor("/log runs with no banner and its buttons on", noBanner+` && `+buttons+`.join() === 'false,false,false'`)
	down()
	b.waitFor("/log shows the Lost link banner and its buttons are off", banner+` && `+buttons+`.join() === 'true,true,true'`)
	b.waitFor("the header starts below the banner", `document.querySelector('.sode-log__head').getBoundingClientRect().top >= document.getElementById('conn-down').getBoundingClientRect().bottom`)
	b.shot("log")
	up()
	b.waitFor("/log goes back with no banner and its buttons on", noBanner+` && `+buttons+`.join() === 'false,false,false'`)

	b.open(base + "/panels")
	signInOnPad(b)
	b.waitFor("/panels runs with no banner", `!document.documentElement.dataset.sodeLock && !!document.getElementById('panels-new') && `+noBanner)
	down()
	b.waitFor("/panels shows the Lost link banner", banner)
	b.waitFor("the /panels header starts below the banner", `document.querySelector('.sode-panels__head').getBoundingClientRect().top >= document.getElementById('conn-down').getBoundingClientRect().bottom`)
	up()
	b.waitFor("/panels goes back with no banner", noBanner)

	// The refused connections while Sode is down are the point of the test; anything else is a real error.
	for _, e := range b.problems() {
		if !strings.Contains(e, "ERR_CONNECTION_REFUSED") && !strings.Contains(e, "WebSocket") && !strings.Contains(e, "/ws") {
			t.Errorf("page error: %s", e)
		}
	}
}
