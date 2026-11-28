package app

import (
	"strings"
	"testing"
)

// Link state with two routers (docs/tielines-design.md, Link state on the page). Router B's simulator stops: Sode is
// Degraded. The one Routers icon is amber and says "1 of 2 down", the note names router B's destinations, and the Lost
// link banner does not show. Compact view shows only the Routers icon. Take stays on for router A. /log has an uptime line and link rows for each
// router. Then router A's simulator stops too: Sode is Down, and the banner shows a node for each router.
func TestChromeDegradedRouter(t *testing.T) {
	b := startBrowser(t)
	simA, stopA := startSimStoppable(t)
	simB, stopB := startSimStoppable(t)
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "DSM1")

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.eval(`localStorage.setItem('sode-compact', 'on'); true`)
	b.open(srv.URL + "/")
	b.waitFor("Compact view", `document.documentElement.hasAttribute('data-compact')`)
	const states = `[...document.querySelectorAll('#conn-routers .sode__state')].map((d) => d.dataset.state).join(',')`
	b.waitFor("one Routers icon, with no color", states+` === 'ok' && document.getElementById('conn-degraded').hidden`)

	stopB()
	b.waitFor("Degraded: the Routers icon is amber and counts router B", states+` === 'warn' && document.querySelector('#conn-routers .sode__conn-open').textContent === '1 of 2 down'`)
	b.waitFor("the note names router B and its destinations", `(() => {
		const n = document.getElementById('conn-degraded');
		return !n.hidden && n.classList.contains('pb-note--warn') && /Router B .*is down\. You cannot patch its \d+ destinations now: B\.1 /.test(n.textContent);
	})()`)
	if b.eval(`!document.getElementById('conn-down').hidden || document.documentElement.dataset.conn !== 'up'`).(bool) {
		t.Fatal("the Lost link banner shows while Sode is Degraded")
	}
	b.waitFor("Compact view shows only the Routers icon", `(() => {
		const o = (n) => getComputedStyle(n).opacity;
		const srv = document.getElementById('conn-srv-dot').parentElement;
		const rt = document.querySelector('#conn-routers > .sode__conn-open');
		return o(srv) === '0' && o(rt) === '1' && !document.getElementById('conn').classList.contains('sode__conn--ok');
	})()`)
	if b.eval(`document.getElementById('patch').getAttribute('aria-disabled')`) != "false" {
		t.Fatal("Take is off while router A is up")
	}
	b.shot("degraded")

	b.open(srv.URL + "/log")
	b.waitFor("an uptime line for each router", `(() => {
		const bars = [...document.querySelectorAll('.sode-up__bars[data-router]')];
		const heads = [...document.querySelectorAll('.sode-log__up-router:not([hidden])')].map((h) => h.textContent);
		return bars.length === 2 && bars.every((x) => x.querySelector('.sode-up__bar')) && heads.length === 2 && heads[1].startsWith('Router B');
	})()`)
	b.waitFor("router B's link row names it", `[...document.querySelectorAll('#log-rows tr')].some((r) => r.textContent.includes('Router B down'))`)
	b.waitFor("a Routers chip for each router", `!document.getElementById('log-routers-line').hidden && document.querySelectorAll('#log-routers .pb-chip').length === 3`)
	b.eval(`[...document.querySelectorAll('#log-routers .pb-chip')].find((c) => c.textContent.startsWith('Router A')).click(); true`)
	b.waitFor("the Routers filter hides router B's link rows", `(() => {
		const rows = [...document.querySelectorAll('#log-rows tr')].map((r) => r.textContent);
		return rows.length > 0 && !rows.some((t) => t.includes('Router B down'));
	})()`)

	b.open(srv.URL + "/")
	stopA()
	b.waitFor("Down: the banner has a node for each router", `(() => {
		const box = document.getElementById('conn-down');
		const nodes = [...box.querySelectorAll('.sode__down-node--down')].map((n) => n.textContent);
		return !box.hidden && box.querySelector('.sode__down-title').textContent === 'Lost link: every Router link is down' &&
			nodes.length === 2 && nodes[0].startsWith('Router A') && nodes[1].startsWith('Router B') && !nodes.some((n) => /Router (.) Router \1/.test(n)) &&
			getComputedStyle(box.querySelector('.sode__down-why code')).whiteSpace === 'pre-line' && document.getElementById('conn-degraded').hidden;
	})()`)
	b.waitFor("Down: the full status shows in Compact view", `getComputedStyle(document.getElementById('conn-srv-dot').parentElement).opacity === '1'`)

	for _, e := range b.problems() {
		if !strings.Contains(e, "net::ERR_CONNECTION_REFUSED") {
			t.Errorf("page error: %s", e)
		}
	}
}
