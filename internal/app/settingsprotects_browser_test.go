package app

import (
	"strconv"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// protectsRouters: an app with one router on each simulator, every simulator with router-side Protects. Router A
// (device 126) has Sode's on A.4, a Protect of device 300 on A.5 and a Router lock on A.6. Router B (device 127) has
// Sode's on B.3 and B.7 and a Protect of device 44 on B.8. A third simulator, when asked for, is router C (device
// 128) with Sode's on C.2. Ana is an Administrator. It gives the tab in Settings with the list drawn.
func protectsRouters(t *testing.T, withC bool) (b *tab, a *App, uids []string, stops []func()) {
	t.Helper()
	b = startBrowser(t)
	specs := []string{"SIM_PROTECTS=12:0:0,3:1:126,4:1:300,5:2:9", "SIM_PROTECTS=12:0:0,2:1:127,6:1:127,7:1:44"}
	if withC {
		specs = append(specs, "SIM_PROTECTS=12:0:0,1:1:128")
	}
	var addrs []string
	for _, s := range specs {
		addr, stop := startSimStoppable(t, s)
		addrs = append(addrs, addr)
		stops = append(stops, stop)
	}
	a = simRoutersApp(t, addrs, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].ProtectDevice = 126
		for i := range c.Routers[1:] {
			c.Routers[i+1].ProtectDevice = 127 + i
		}
	})
	srv := seamServer(t, a)
	key := userKey(t, a, "Ana")
	uids = routerUIDs(a)
	for _, uid := range uids {
		waitRouterRead(t, a, key, uid, "DSM1")
	}
	b.open(srv.URL + "/")
	b.waitFor("Settings button", `!!document.getElementById('settings-open')`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.documentElement.dataset.view === 'settings' || document.getElementById('settings-open').click(); true`)
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	b.eval(`document.querySelector('fieldset[data-section="protects"]').scrollIntoView({block: 'start', behavior: 'instant'}); true`)
	n := 6
	if withC {
		n = 7
	}
	b.waitFor("every row", `document.querySelectorAll('#protects-list .sode__prow[data-hold]').length === `+strconv.Itoa(n))
	return b, a, uids, stops
}

// The list in Settings: a group for each router with its heading, a count line, the find box and the filters.
// Another device's Protect is plain words with a magenta shield, no amber badge. Free on a row turns that row into the
// Confirm, in place: the list does not move. The letter frees it. The row says "Protect freed" with no color, and
// the status line under the list keeps it. Home's banner gets nothing. Then the row goes.
func TestChromeSettingsProtectsFreeInRow(t *testing.T) {
	b, _, uids, _ := protectsRouters(t, false)
	count := `document.getElementById('protects-count').textContent`
	b.waitFor("the count line", count+` === '6 Protects and Router locks on 2 routers. Sode set 3.'`)
	b.waitFor("a group for each router, in order", `[...document.querySelectorAll('#protects-list .sode__pgroup-prefix')].map((n) => n.textContent).join('|') === 'A|B'`)
	b.waitFor("another device's Protect: plain words and a magenta shield", `(() => {
		const by = document.querySelector('#protects-list .sode__prow[data-dst="4"]:not([data-router]) .sode__prow-by');
		const shield = by?.querySelector('.sode__prow-shield');
		const probe = document.body.appendChild(Object.assign(document.createElement('span'), { style: 'color: var(--pb-protect)' }));
		const magenta = getComputedStyle(probe).color;
		probe.remove();
		return by?.dataset.owner === 'other' && by.textContent === 'Protected by device 300' && !!shield &&
			getComputedStyle(shield).color === magenta && !document.querySelector('#protects-list .pb-badge');
	})()`)

	// The find box and the filters narrow the list, and the count line says so.
	b.eval(`(() => { const i = document.getElementById('protects-find'); i.focus(); i.value = 'B.8'; i.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	b.waitFor("find B.8", count+` === 'Shows 1 of 6.' && document.querySelectorAll('#protects-list .sode__prow[data-hold]').length === 1`)
	b.eval(`(() => { const i = document.getElementById('protects-find'); i.value = ''; i.dispatchEvent(new Event('input', {bubbles: true})); i.blur(); return true; })()`)
	b.waitFor("the find cleared", count+` === '6 Protects and Router locks on 2 routers. Sode set 3.'`)
	b.eval(`document.querySelector('#protects-tools [data-act="kind-other"]').click(); true`)
	b.waitFor("Other devices", count+` === 'Shows 2 of 6.'`)
	b.eval(`document.querySelector('#protects-tools [data-act="router-` + uids[1] + `"]').click(); true`)
	b.waitFor("Other devices on router B", count+` === 'Shows 1 of 6.' && document.querySelectorAll('#protects-list .sode__pgroup').length === 1`)
	b.eval(`document.querySelector('#protects-tools [data-act="kind-all"]').click(); document.querySelector('#protects-tools [data-act="router-all"]').click(); true`)
	b.waitFor("no filter", count+` === '6 Protects and Router locks on 2 routers. Sode set 3.'`)

	// Free on B.3 (dst 2 of router B): the row is the Confirm, and the list stays where it is.
	row := `document.querySelector('#protects-list .sode__prow[data-router="` + uids[1] + `"][data-dst="2"]')`
	b.eval(`(() => { const l = document.getElementById('protects-list'); l.style.height = '9rem'; l.scrollTop = 40; window.__top = l.scrollTop; window.__y = window.scrollY; return true; })()`)
	b.eval(row + `.querySelector('[data-act="free"]').click(); true`)
	b.waitFor("the row is the Confirm", row+`?.hasAttribute('data-confirm') && /Free the Protect on B\.3 /.test(`+row+`.textContent) && !!`+row+`.querySelector('kbd')`)
	if moved := b.eval(`document.getElementById('protects-list').scrollTop !== window.__top || window.scrollY !== window.__y`).(bool); moved {
		t.Fatal("the list or the page moved when the Confirm opened")
	}
	b.waitFor("the lockout ends", `!`+row+`.querySelector('[data-act="confirm"]').disabled`)
	b.eval(`(() => { const k = ` + row + `.querySelector('kbd').textContent.toLowerCase(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: k, bubbles: true, cancelable: true})); return true; })()`)
	b.waitFor("Protect freed, in the row and the status line", row+`?.dataset.done === 'ok' && `+row+`.textContent.includes('Protect freed') &&
		document.getElementById('protects-status').textContent === 'Protect freed: B.3 DSM1, every level.' && document.getElementById('protects-status').dataset.state === ''`)
	if b.eval(`/Protect freed/.test(document.getElementById('result')?.textContent || '')`).(bool) {
		t.Fatal("the result went to Home's banner")
	}
	b.waitFor("the row goes", `!`+row+` && document.getElementById('protects-count').textContent === '5 Protects and Router locks on 2 routers. Sode set 2.'`)
	if !strings.HasPrefix(b.eval(`document.getElementById('protects-status').textContent`).(string), "Protect freed") {
		t.Fatal("the status line lost the result")
	}
	b.check()
}

// Free Sode's Protects by router: the dialog lists each router with its count and a tick box. Router C is down. Its
// rows are greyed and marked last known, and its Free is off. The dialog lists it with the reason and no tick.
// With A unticked, the button frees 2 Protects on 1 router. After the PIN recheck the result shows in B's heading
// and the status line, and A's Protect stays.
func TestChromeSettingsProtectsFreeByRouter(t *testing.T) {
	b, _, uids, stops := protectsRouters(t, true)
	stops[2]()
	group := func(uid string) string {
		return `document.querySelector('#protects-list .sode__pgroup[data-router="` + uid + `"]')`
	}
	b.waitFor("router C: last known rows, Free off", group(uids[2])+`?.hasAttribute('data-stale') && /These are the last known rows, read at \d\d:\d\d:\d\d\./.test(`+group(uids[2])+`.textContent) &&
		`+group(uids[2])+`.querySelector('.sode__prow[data-stale] [data-act="free"]').disabled`)

	b.eval(`document.getElementById('protects-clear-all').click(); true`)
	b.waitFor("the dialog", `!!document.getElementById('protects-free-go')`)
	box := func(uid string) string {
		return `document.querySelector('#protects-free input[data-router="` + uid + `"]')`
	}
	goBtn := `document.getElementById('protects-free-go')`
	b.waitFor("A and B ticked, C off with its reason", box("")+`.checked && `+box(uids[1])+`.checked && !`+box(uids[2])+`.checked && `+box(uids[2])+`.disabled &&
		/not connected/.test(`+box(uids[2])+`.closest('.sode__pfree-row').textContent) && /^Free 3 Protects on 2 routers/.test(`+goBtn+`.textContent)`)
	b.eval(box("") + `.click(); true`)
	b.waitFor("the button follows the ticks", `/^Free 2 Protects on 1 router \(/.test(`+goBtn+`.textContent)`)
	b.waitFor("the lockout ends", `!`+goBtn+`.disabled`)
	b.eval(goBtn + `.click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("the result for each router", `document.getElementById('protects-status').textContent === 'Sode freed 2 Protects: 2 on B.' &&
		/Sode freed 2 Protects at/.test(`+group(uids[1])+`.querySelector('.sode__pgroup-result').textContent)`)
	b.waitFor("A's Protect stays, B's go", `!!document.querySelector('#protects-list .sode__prow[data-dst="3"]:not([data-router]) [data-act="free"]') &&
		!document.querySelector('#protects-list .sode__prow[data-router="`+uids[1]+`"] [data-act="free"]')`)
	b.check()
}
