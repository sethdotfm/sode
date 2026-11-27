package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The one Save with two routers: the Save and reconnect confirm names each router that reconnects, with its new
// address. It comes before the PIN recheck.
func TestChromeReconnectConfirmNamesEachRouter(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Title, c.MoreRouters[0].Title = "Studio A", "Stage 2"
	})
	srv := seamServer(t, a)
	uid := routerUIDs(a)[1]

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	set := func(id, v string) {
		b.t.Helper()
		b.eval(`(() => { const f = document.getElementById('` + id + `'); f.value = '` + v + `'; f.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	}
	b.waitFor("router B's row", `!!document.getElementById('cfg-rt-`+uid+`-address')`)
	set("cfg-rt-"+uid+"-address", "127.0.0.1:2")
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review", `!!document.getElementById('review')?.offsetParent && document.getElementById('review-save').textContent === 'Save 1 change'`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("the confirm names router B and its new address, before the PIN", `(() => {
		const t = document.getElementById('review-title').textContent;
		return t === 'Save and reconnect B Stage 2 to 127.0.0.1:2?' && document.getElementById('review-save').textContent === 'Save and reconnect'
			&& !document.getElementById('recheck-pin')?.offsetParent;
	})()`)
	b.eval(`document.getElementById('review-back').click(); true`)
	b.waitFor("back on the review", `document.getElementById('review-back').textContent === 'Keep editing'`)
	b.eval(`document.getElementById('review-back').click(); true`)
	b.waitFor("review closed", `!document.getElementById('review').offsetParent`)

	set("cfg-router", "127.0.0.1:1")
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review of two", `!!document.getElementById('review')?.offsetParent && document.getElementById('review-save').textContent === 'Save 2 changes'`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("the confirm names both routers", `(() => {
		const d = document.getElementById('review');
		const lines = [...d.querySelectorAll('.sode__review-stops li')].map((li) => li.textContent);
		return document.getElementById('review-title').textContent === 'Save and reconnect 2 routers?'
			&& lines.join('|') === 'A Studio A to 127.0.0.1:1|B Stage 2 to 127.0.0.1:2';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("then the PIN recheck", `!!document.getElementById('recheck-pin')?.offsetParent`)
	if c := a.r().cfg; c.Address != simA || c.MoreRouters[0].Address != simB {
		t.Fatalf("nothing is sent before the PIN: %q %q", c.Address, c.MoreRouters[0].Address)
	}
	b.check()
}
