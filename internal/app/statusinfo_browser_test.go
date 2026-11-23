package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// statusInfoPage opens Home at a desktop size with a spy on the socket's sends, and returns a hover at the status.
func statusInfoPage(t *testing.T, locks config.Locks) (*tab, func()) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Locks = locks })
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("both links up", `document.getElementById('conn').classList.contains('sode__conn--ok')`)
	b.eval(`window.sent = []; const send = WebSocket.prototype.send; WebSocket.prototype.send = function (d) { sent.push(JSON.parse(d)); return send.call(this, d); }; true`)
	hover := func() {
		xy := b.eval(`(() => { const r = document.getElementById('conn').getBoundingClientRect(); return [r.x + r.width / 2, r.y + r.height / 2]; })()`).([]any)
		b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": xy[0], "y": xy[1]})
	}
	return b, hover
}

const statusInfoOpen = `!document.querySelector('#conn > .sode-statusinfo').hidden`

// With Log rights the details show the Server run time, the Router link and the 24-hour strip. Escape closes them,
// focus opens them and the native title is gone.
func TestChromeStatusInfoWithLogRights(t *testing.T) {
	b, hover := statusInfoPage(t, config.Locks{})
	b.waitFor("no native title", `!document.getElementById('conn').hasAttribute('title') && document.getElementById('conn').tabIndex === 0`)
	b.waitFor("a Server and a Router icon, each with its state badge, spoken name and visible word", `(() => {
		const st = [...document.querySelectorAll('#conn .sode__state')];
		const use = (n, sel) => n.querySelector(sel)?.getAttribute('href');
		return st.length === 2 && st.every((d) => d.dataset.state === 'ok' && d.getAttribute('role') === 'img' && use(d, '.sode__state-badge use') === '#pb-i-circle-check')
			&& use(st[0], ':scope > svg use') === '#pb-i-server' && use(st[1], ':scope > svg use') === '#pb-i-server-nas'
			&& st.map((d) => d.getAttribute('aria-label')).join('|') === 'Server connected|Router connected'
			&& [...document.querySelectorAll('#conn .pb-status')].map((s) => s.textContent).join('|') === 'Server|Router';
	})()`)
	hover()
	b.waitFor("details open", statusInfoOpen)
	b.waitFor("Server run time from api/health", `/Server run time\s*\d+ (second|minute)/.test(document.querySelector('.sode-statusinfo').innerText)`)
	b.waitFor("Router link and address", `/Router link\s*Connected/.test(document.querySelector('.sode-statusinfo').innerText) && /Address\s*127\.0\.0\.1:/.test(document.querySelector('.sode-statusinfo').innerText)`)
	b.waitFor("the strip", `document.querySelectorAll('.sode-statusinfo .sode-up__bar').length > 0 && /Router link up for/.test(document.querySelector('.sode-statusinfo').innerText)`)
	b.waitFor("one uptime ask, above log.js's ids", `sent.filter((m) => m.t === 'uptime').length === 1 && sent.find((m) => m.t === 'uptime').id >= 1e9`)
	b.eval(`document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true`)
	b.waitFor("Escape closes", `!(`+statusInfoOpen+`)`)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 700, "y": 600})
	b.eval(`document.getElementById('conn').focus(); true`)
	b.waitFor("focus opens", statusInfoOpen)
	b.eval(`document.getElementById('conn').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true`)
	b.waitFor("Escape closes and keeps focus", `!(`+statusInfoOpen+`) && document.activeElement.id === 'conn'`)
	b.check()
}

// Without Log rights (a Guest under the log lock) the details are text only. The page never asks for uptime, since a
// refused ask writes a locked row to the Communication Log.
func TestChromeStatusInfoWithoutLogRights(t *testing.T) {
	b, hover := statusInfoPage(t, config.Locks{Log: true})
	hover()
	b.waitFor("details open", statusInfoOpen)
	b.waitFor("Server run time", `/Server run time\s*\d+ (second|minute)/.test(document.querySelector('.sode-statusinfo').innerText)`)
	b.eval(`new Promise((r) => setTimeout(r, 500))`)
	if got := b.eval(`[document.querySelectorAll('.sode-statusinfo .sode-up__bar').length, sent.filter((m) => m.t === 'uptime').length]`).([]any); got[0] != float64(0) || got[1] != float64(0) {
		t.Fatalf("bars, uptime asks = %v, want none", got)
	}
	b.check()
}
