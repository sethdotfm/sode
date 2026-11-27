package app

import (
	"net"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// statusInfoPage opens Home at a desktop size with a spy on the socket's sends.
func statusInfoPage(t *testing.T, locks config.Locks) *tab {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Locks = locks })
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("both links up", `document.getElementById('conn').classList.contains('sode__conn--ok')`)
	b.eval(`window.sent = []; const send = WebSocket.prototype.send; WebSocket.prototype.send = function (d) { sent.push(JSON.parse(d)); return send.call(this, d); }; true`)
	return b
}

// key presses one key on the element with focus, as a person does (Enter and Space on a button click it).
func (b *tab) key(name, code string, vk int, text string) {
	b.t.Helper()
	down := map[string]any{"type": "keyDown", "key": name, "code": code, "windowsVirtualKeyCode": vk}
	if text != "" {
		down["text"] = text
	}
	b.call("Input.dispatchKeyEvent", down)
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": name, "code": code, "windowsVirtualKeyCode": vk})
}

const (
	routerListOpen = `!document.querySelector('#conn > .sode-statusinfo').hidden`
	routerListText = `document.querySelector('#conn > .sode-statusinfo').innerText`
)

// One router: one Routers icon, a button. Enter opens the router list, which stays open. A row opens with Enter to
// show the address, the 24-hour strip (Log rights) and a Communication Log link. A person who is not an Administrator
// sees no Router settings link. Escape closes the list and gives focus back to the button. A click outside closes it.
func TestChromeStatusInfoWithLogRights(t *testing.T) {
	b := statusInfoPage(t, config.Locks{})
	b.waitFor("a Server icon and one Router icon in a button, each with its state badge, spoken name and visible word", `(() => {
		const st = [...document.querySelectorAll('#conn .sode__state')];
		const use = (n, sel) => n.querySelector(sel)?.getAttribute('href');
		const open = document.querySelector('#conn-routers > button.sode__conn-open');
		return st.length === 2 && st.every((d) => d.dataset.state === 'ok' && d.getAttribute('role') === 'img' && use(d, '.sode__state-badge use') === '#pb-i-circle-check')
			&& use(st[0], ':scope > svg use') === '#pb-i-server' && use(st[1], ':scope > svg use') === '#pb-i-server-nas'
			&& st.map((d) => d.getAttribute('aria-label')).join('|') === 'Server connected|Router connected'
			&& [...document.querySelectorAll('#conn .pb-status')].map((s) => s.textContent).join('|') === 'Server|Router'
			&& !!open && open.contains(st[1]) && open.getAttribute('aria-expanded') === 'false' && !document.getElementById('conn').hasAttribute('title');
	})()`)
	b.eval(`document.querySelector('#conn .sode__conn-open').focus(); true`)
	b.key("Enter", "Enter", 13, "\r")
	b.waitFor("Enter opens the list, and focus goes to its row", routerListOpen+` && document.querySelector('#conn .sode__conn-open').getAttribute('aria-expanded') === 'true'
		&& document.activeElement.classList.contains('sode-routerlist__head')`)
	b.waitFor("Server run time from api/health", `/Server run time\s*\d+ (second|minute)/.test(`+routerListText+`)`)
	b.waitFor("one row, Connected in plain text, no find box", `(() => {
		const rows = [...document.querySelectorAll('#conn .sode-routerlist__item')];
		const st = rows[0]?.querySelector('.sode-routerlist__state');
		return rows.length === 1 && st.textContent === 'Connected' && !st.dataset.state && document.querySelector('#conn .sode-routerlist__find').hidden;
	})()`)
	b.eval(`new Promise((r) => setTimeout(r, 300))`)
	if b.eval(`sent.some((m) => m.t === 'uptime')`).(bool) {
		t.Fatal("an uptime ask before a row opened")
	}
	b.key("Enter", "Enter", 13, "\r")
	b.waitFor("the row opens: address, strip and Communication Log, no Router settings", `(() => {
		const body = document.querySelector('#conn .sode-routerlist__body');
		const links = [...body.querySelectorAll('a')].map((a) => a.textContent + ' ' + a.getAttribute('href'));
		return !body.hidden && /Address\s*127\.0\.0\.1:/.test(body.innerText) && body.querySelectorAll('.sode-up__bar').length > 0
			&& /Router link up for/.test(body.innerText) && links.join('|') === 'Communication Log log';
	})()`)
	b.waitFor("one uptime ask, above log.js's ids", `sent.filter((m) => m.t === 'uptime').length === 1 && sent.find((m) => m.t === 'uptime').id >= 1e9`)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 700, "y": 600})
	b.eval(`new Promise((r) => setTimeout(r, 400))`)
	if !b.eval(routerListOpen).(bool) {
		t.Fatal("the list closed when the pointer left it")
	}
	b.key("Escape", "Escape", 27, "")
	b.waitFor("Escape closes and gives focus back to the button", `!(`+routerListOpen+`) && document.activeElement === document.querySelector('#conn .sode__conn-open')`)
	b.key(" ", "Space", 32, " ")
	b.waitFor("Space opens it again", routerListOpen)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mousePressed", "x": 700, "y": 600, "button": "left", "clickCount": 1})
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseReleased", "x": 700, "y": 600, "button": "left", "clickCount": 1})
	b.waitFor("a click outside closes it", `!(`+routerListOpen+`)`)
	b.check()
}

// Without Log rights (a Guest under the log lock) an open row is text only. The page never asks for uptime, since a
// refused ask writes a locked row to the Communication Log.
func TestChromeStatusInfoWithoutLogRights(t *testing.T) {
	b := statusInfoPage(t, config.Locks{Log: true})
	b.eval(`document.querySelector('#conn .sode__conn-open').click(); true`)
	b.waitFor("list open", routerListOpen)
	b.waitFor("Server run time", `/Server run time\s*\d+ (second|minute)/.test(`+routerListText+`)`)
	b.eval(`document.querySelector('#conn .sode-routerlist__head').click(); true`)
	b.waitFor("the row opens", `/Address/.test(document.querySelector('#conn .sode-routerlist__body').innerText)`)
	b.eval(`new Promise((r) => setTimeout(r, 500))`)
	if got := b.eval(`[document.querySelectorAll('.sode-statusinfo .sode-up__bar').length, sent.filter((m) => m.t === 'uptime').length]`).([]any); got[0] != float64(0) || got[1] != float64(0) {
		t.Fatalf("bars, uptime asks = %v, want none", got)
	}
	b.check()
}

// Nine routers: A and B up, C to I at an address where nothing listens, so they never connected. The bar has one
// amber Routers icon with "7 of 9 down", and no prefixes. The Degraded note is one sentence with Show routers, which
// opens the list. The list puts the lost routers first, has a find box, and an open row of a router that never
// connected says "Check the address.". An Administrator's Router settings opens Settings at that router's row. On the
// phone the header fits, a press opens the same list, and it has no Router settings link.
func TestChromeRouterListManyRouters(t *testing.T) {
	b := startBrowser(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	dead := l.Addr().String()
	l.Close()
	addrs := []string{startSim(t), startSim(t)}
	for range 7 {
		addrs = append(addrs, dead)
	}
	a := simRoutersApp(t, addrs, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].Title = "Studio A"
		for i := range c.Routers[1:] {
			c.Routers[i+1].Title = "Studio " + c.Routers[i+1].Prefix
		}
	})
	srv := seamServer(t, a)
	uids := routerUIDs(a)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("Home", `document.documentElement.dataset.view !== 'settings'`)

	b.waitFor("one amber Routers icon that counts the lost routers", `(() => {
		const st = [...document.querySelectorAll('#conn-routers .sode__state')];
		const open = document.querySelector('#conn-routers > .sode__conn-open');
		return st.length === 1 && st[0].dataset.state === 'warn' && open.textContent === '7 of 9 down'
			&& st[0].getAttribute('aria-label') === 'Routers: 7 of 9 down' && document.getElementById('conn-addr').textContent === '';
	})()`)
	b.waitFor("the Degraded note is one sentence with Show routers", `(() => {
		const n = document.getElementById('conn-degraded');
		return !n.hidden && n.querySelector('.sode__degraded-lines').textContent === '7 routers are down: C, D, E, F, G, H, I. You cannot patch their destinations now.'
			&& n.querySelector('button')?.textContent === 'Show routers';
	})()`)
	b.shot("bar")
	b.eval(`document.querySelector('#conn-degraded button').click(); true`)
	b.waitFor("Show routers opens the list with focus in the find box", routerListOpen+` && document.activeElement === document.querySelector('#conn .sode-routerlist__find')`)
	b.waitFor("lost routers first, in config order; Connected has no color", `(() => {
		const rows = [...document.querySelectorAll('#conn .sode-routerlist__item')];
		const name = (r) => r.querySelector('.sode-routerlist__name').textContent;
		const st = (r) => r.querySelector('.sode-routerlist__state');
		const lost = rows.slice(0, 7), ok = rows.slice(7);
		return rows.length === 9 && lost.map(name).join(',') === 'C Studio C,D Studio D,E Studio E,F Studio F,G Studio G,H Studio H,I Studio I'
			&& lost.every((r) => st(r).textContent === 'Never connected' && st(r).dataset.state === 'error')
			&& ok.map(name).join(',') === 'A Studio A,B Studio B' && ok.every((r) => st(r).textContent === 'Connected' && !st(r).dataset.state);
	})()`)
	if h := b.eval(`document.querySelector('#conn .sode-routerlist').getBoundingClientRect().height / innerHeight`).(float64); h > 0.61 {
		t.Fatalf("the list is %.0f%% of the window, want a fixed height", h*100)
	}
	b.eval(`(() => { const f = document.querySelector('#conn .sode-routerlist__find'); f.value = 'studio b'; f.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.waitFor("the find box finds router B", `[...document.querySelectorAll('#conn .sode-routerlist__item')].map((r) => r.dataset.router).join() === '`+uids[1]+`'`)
	b.eval(`(() => { const f = document.querySelector('#conn .sode-routerlist__find'); f.value = ''; f.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.key("ArrowDown", "ArrowDown", 40, "")
	b.waitFor("Down moves to the first row", `document.activeElement === document.querySelector('#conn .sode-routerlist__head')`)
	b.key("Enter", "Enter", 13, "\r")
	b.waitFor("router C's row: address, error, the note and both links", `(() => {
		const body = document.querySelector('#conn .sode-routerlist__item .sode-routerlist__body');
		const links = [...body.querySelectorAll('a')].map((a) => a.textContent);
		return !body.hidden && body.innerText.includes('`+dead+`') && /Error/.test(body.innerText)
			&& body.querySelector('.sode-routerlist__note')?.textContent === 'Check the address.' && links.join('|') === 'Router settings|Communication Log';
	})()`)
	b.shot("list")
	b.eval(`document.querySelector('#conn .sode-routerlist__body a').click(); true`)
	b.waitFor("Settings opens at router C's row", `(() => {
		const row = document.querySelector('#routers-list > [data-router="`+uids[2]+`"]');
		if (document.documentElement.dataset.view !== 'settings' || !row) return false;
		const r = row.getBoundingClientRect();
		return r.top >= 0 && r.bottom <= innerHeight && row.contains(document.activeElement) && document.querySelector('#conn > .sode-statusinfo').hidden;
	})()`)

	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/")
	b.waitFor("the phone header fits, with one Routers icon", `(() => {
		if (document.documentElement.dataset.layout !== 'phone') return false;
		const st = document.querySelectorAll('#m-routers .sode__state');
		const s = document.getElementById('m-settings').getBoundingClientRect();
		return st.length === 1 && st[0].dataset.state === 'warn' && s.right <= innerWidth && s.width > 0
			&& document.querySelector('#m-routers .sode__conn-open').textContent === '7 of 9 down';
	})()`)
	b.eval(`document.querySelector('#m-routers .sode__conn-open').click(); true`)
	b.waitFor("a press opens the list", `document.querySelector('.sode-m__head > .sode-statusinfo')?.hidden === false`)
	b.eval(`document.querySelector('.sode-m__head .sode-routerlist__head').click(); true`)
	b.waitFor("the open row has no Router settings link, and the list fits the screen", `(() => {
		const pop = document.querySelector('.sode-m__head > .sode-statusinfo');
		const links = [...pop.querySelectorAll('.sode-routerlist__body a')].map((a) => a.textContent);
		const r = pop.getBoundingClientRect();
		return links.join('|') === 'Communication Log' && r.left >= 0 && r.right <= innerWidth;
	})()`)
	b.shot("phone")
	for _, e := range b.problems() {
		// api/health answers 503 while Sode is Degraded, with the same body (statusinfo.js askHealth).
		if !strings.Contains(e, "net::ERR_CONNECTION_REFUSED") && !strings.Contains(e, "503 (Service Unavailable)") {
			t.Errorf("page error: %s", e)
		}
	}
}
