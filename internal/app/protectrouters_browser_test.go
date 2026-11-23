package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The route view of a destination on router B offers Protect, as on router A. The Confirm sends it to router B, and
// the view then says "Protected by you" from router B's own Protect state, with Free Protect offered.
func TestChromeProtectSecondRouter(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.ProtectDevice = 126
		c.MoreRouters[0].ProtectDevice = 127
	})
	srv := seamServer(t, a)
	key := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, key, uids[1], "Stage Mon")
	sodeRows := func() int {
		return strings.Count(apiDo(a, "GET", "/api/v1/protected?router="+uids[1], key, nil).Body.String(), `"sodeUser":"Ana"`)
	}

	b.open(srv.URL + "/")
	b.waitFor("Settings button", `!!document.getElementById('settings-open')`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)

	b.waitFor("B4 typed", `(() => { const t = document.getElementById('dst'); t.focus(); t.value = 'B4'; t.dispatchEvent(new Event('input', {bubbles: true}));
		return (document.getElementById('dst-opt-0')?.textContent || '').includes('Stage Mon'); })()`)
	b.eval(`document.getElementById('dst').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
	b.waitFor("the shield on B.4", `!!document.querySelector('#dst-under [data-protect="set"]')`)
	b.eval(`document.querySelector('#dst-under [data-protect="set"]').click(); true`)
	b.waitFor("Confirm protect", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm protect'
		&& document.getElementById('confirm').textContent.includes('Stage Mon')`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("Protected by you on B.4", `[...document.querySelectorAll('#dst-under .pb-badge')].some((n) => n.textContent === 'Protected by you' && n.dataset.owner === 'you')`)
	b.waitFor("Free Protect offered", `!!document.querySelector('#dst-under [data-protect="clear"]')`)
	if n := sodeRows(); n == 0 {
		t.Fatal("no Protect of Sode on router B")
	}
	if strings.Contains(apiDo(a, "GET", "/api/v1/protected?router="+uids[0], key, nil).Body.String(), `"sodeUser"`) {
		t.Fatal("the Protect went to router A")
	}
}
