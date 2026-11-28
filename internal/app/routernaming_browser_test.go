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
		c.Routers[0].Title, c.Routers[1].Title = "Studio A", "Stage 2"
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
	pickRouter(b, uid)
	set("cfg-rt-address", "127.0.0.1:2")
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

	pickRouter(b, routerUIDs(a)[0])
	set("cfg-rt-address", "127.0.0.1:1")
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
	if c := a.r().cfg; c.Routers[0].Address != simA || c.Routers[1].Address != simB {
		t.Fatalf("nothing is sent before the PIN: %q %q", c.Routers[0].Address, c.Routers[1].Address)
	}
	b.check()
}

// twoNamedRouters: an Administrator Settings page on two routers, "A Studio A" and "B Stage 2". edit runs last. It
// gives the App and router B's UID.
func twoNamedRouters(t *testing.T, b *tab, edit ...func(*config.Config)) (*App, string) {
	t.Helper()
	simA, simB := startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB}, append([]func(*config.Config){func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].Title, c.Routers[1].Title = "Studio A", "Stage 2"
	}}, edit...)...)
	srv := seamServer(t, a)
	uid := routerUIDs(a)[1]
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("router B's row", `!!document.querySelector('#routers-list [data-router="`+uid+`"]')`)
	return a, uid
}

// pickRouter selects router uid in the list of Settings > Routers and waits for its detail view.
func pickRouter(b *tab, uid string) {
	b.t.Helper()
	b.waitFor("the row of router "+uid, `!!document.querySelector('#routers-list [data-router="`+uid+`"]')`)
	b.eval(`document.querySelector('#routers-list [data-router="` + uid + `"]').click(); true`)
	b.waitFor("router "+uid+" selected", `document.querySelector('#routers-list [aria-selected="true"]')?.dataset.router === '`+uid+`'`)
}

// typeField types v into the field and sends its input event.
func typeField(b *tab, id, v string) {
	b.t.Helper()
	b.eval(`(() => { const f = document.getElementById('` + id + `'); f.value = '` + v + `'; f.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
}

// With two routers, review rows and refusals name each router by prefix and Router title, the first router too.
func TestChromeReviewNamesEveryRouter(t *testing.T) {
	b := startBrowser(t)
	a, uid := twoNamedRouters(t, b)
	pickRouter(b, routerUIDs(a)[0])
	typeField(b, "cfg-rt-address", "nohost")
	pickRouter(b, uid)
	typeField(b, "cfg-rt-address", "nohost2")
	// The page checks the address: Save names each router with an error, shows the first and goes no further.
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the error line names both routers, and A's field shows its error", `(() => {
		const e = document.getElementById('settings-err');
		const t = e.textContent;
		return !e.hidden && t.startsWith('2 settings have an error.') && t.includes('Router A Studio A, Address: enter the address as host:port')
			&& t.includes('Router B Stage 2, Address: enter') && !document.getElementById('review')?.offsetParent
			&& document.querySelector('#routers-list [aria-selected="true"]')?.dataset.router === '`+routerUIDs(a)[0]+`'
			&& document.querySelector('.sode__field-err')?.textContent.startsWith('Enter the address');
	})()`)
	typeField(b, "cfg-rt-address", "127.0.0.1:1")
	pickRouter(b, uid)
	typeField(b, "cfg-rt-address", "127.0.0.1:2")
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("each row names its router", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const rows = [...d.querySelectorAll('tbody tr')].map((r) => r.cells[0]?.textContent || '');
		return rows.includes('A Studio A: Address') && rows.includes('B Stage 2: Address');
	})()`)
	b.check()
}

// Swap… on router B's row opens the Swap dialog on router B. The staged Swap changes router B's preset, and its review
// row names the router.
func TestChromeSwapOnSecondRouter(t *testing.T) {
	b := startBrowser(t)
	const ub = "00000000000000bb"
	_, uid := twoNamedRouters(t, b, func(c *config.Config) {
		c.Routers[1].UID = ub
		ref := func(router string, n int) config.Ref { return config.Ref{Router: router, N: n} }
		c.Presets = []config.Preset{{ID: "pb", Name: "On B", Items: []config.PresetItem{{Src: ref(ub, 0), Dsts: []config.Ref{ref(ub, 4)}}}}}
	})
	if uid != ub {
		t.Fatalf("router B's UID is %q", uid)
	}
	pickRouter(b, ub)
	b.eval(`document.getElementById('router-swap').click(); true`)
	b.waitFor("the dialog opens on router B", `!!document.getElementById('swap-router')?.offsetParent && document.getElementById('swap-router').value === '`+ub+`'
		&& document.getElementById('swap-router').selectedOptions[0].textContent === 'B Stage 2'`)
	typeField(b, "swap-from", "5")
	b.eval(`document.getElementById('swap-find').click(); true`)
	b.waitFor("router B's preset listed", `document.querySelector('.sode__swap-list')?.textContent.includes('Preset: On B')`)
	typeField(b, "swap-to", "6")
	b.waitFor("one item to stage", `document.getElementById('swap-go').textContent === 'Stage 1 item' && !document.getElementById('swap-go').disabled`)
	b.eval(`document.getElementById('swap-go').click(); true`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review names router B", `(() => {
		const d = document.getElementById('review');
		return !!d?.offsetParent && [...d.querySelectorAll('tbody tr')].some((r) => r.cells[0]?.textContent === 'Swap destination 5 for 6 on B Stage 2');
	})()`)
	b.check()
}

// Every router's title field is labelled Router title, and the detail view of each router names it the same way.
func TestChromeRouterTitleLabel(t *testing.T) {
	b := startBrowser(t)
	a, uid := twoNamedRouters(t, b)
	b.waitFor("one label for the title field", `(() => {
		const label = (id) => document.querySelector('label[for="' + id + '"]')?.textContent;
		return label('cfg-rt-title') === 'Router title' && document.querySelector('#cfg-routershow option[value="name"]').textContent === 'Router title'
			&& document.getElementById('router-detail-name').textContent === 'A Studio A';
	})()`)
	pickRouter(b, uid)
	b.waitFor("router B's name", `document.getElementById('router-detail-name').textContent === 'B Stage 2'`)
	pickRouter(b, routerUIDs(a)[0])
	typeField(b, "cfg-rt-title", "Studio 1")
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review row", `[...document.querySelectorAll('#review tbody tr')].some((r) => r.cells[0]?.textContent === 'A Studio A: Router title')`)
	b.check()
}

// Each row of Settings > Routers shows its router's link state in words, live. Connected has no color. A router that
// was never up is Never connected, and one that was up and stops is Down; both are red.
func TestChromeRoutersListLinkState(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	simC, stopC := startSimStoppable(t)
	a := simRoutersApp(t, []string{simA, simB, simC}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[1].Address = "127.0.0.1:1"
	})
	srv := seamServer(t, a)
	uids := routerUIDs(a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	link := func(uid string) string {
		return `document.querySelector('#routers-list [data-router="` + uid + `"] .sode__rstate')`
	}
	b.waitFor("A and C Connected with no color, B Never connected in red, and the count line", `(() => {
		const a = `+link(uids[0])+`, b = `+link(uids[1])+`, c = `+link(uids[2])+`;
		return a?.textContent === 'Connected' && !a.dataset.state && c?.textContent === 'Connected'
			&& b?.textContent === 'Never connected' && b.dataset.state === 'error' && !!b.querySelector('svg')
			&& document.getElementById('routers-count').textContent === '3 routers. 2 connected, 1 down.';
	})()`)
	pickRouter(b, uids[1])
	b.waitFor("B's detail view shows the state and the error", `(() => { const l = document.getElementById('router-detail-link'); return l.textContent.startsWith('Never connected') && l.textContent.length > 'Never connected'.length; })()`)
	stopC()
	b.waitFor("C goes Down, live", `(() => { const c = `+link(uids[2])+`; return c.textContent === 'Down' && c.dataset.state === 'error'; })()`)
	b.check()
}
