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

// twoNamedRouters: an Administrator Settings page on two routers, "A Studio A" and "B Stage 2". edit runs last. It
// gives the App and router B's UID.
func twoNamedRouters(t *testing.T, b *tab, edit ...func(*config.Config)) (*App, string) {
	t.Helper()
	simA, simB := startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB}, append([]func(*config.Config){func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Title, c.MoreRouters[0].Title = "Studio A", "Stage 2"
	}}, edit...)...)
	srv := seamServer(t, a)
	uid := routerUIDs(a)[1]
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("router B's row", `!!document.getElementById('cfg-rt-`+uid+`-address')`)
	return a, uid
}

// typeField types v into the field and sends its input event.
func typeField(b *tab, id, v string) {
	b.t.Helper()
	b.eval(`(() => { const f = document.getElementById('` + id + `'); f.value = '` + v + `'; f.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
}

// With two routers, review rows and refusals name each router by prefix and Router title, the first router too.
func TestChromeReviewNamesEveryRouter(t *testing.T) {
	b := startBrowser(t)
	_, uid := twoNamedRouters(t, b)
	typeField(b, "cfg-router", "nohost")
	typeField(b, "cfg-rt-"+uid+"-address", "nohost2")
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("each row names its router", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const rows = [...d.querySelectorAll('tbody tr')].map((r) => r.cells[0]?.textContent || '');
		return rows.includes('A Studio A: Address') && rows.includes('B Stage 2: Address');
	})()`)
	// Save, Save and reconnect, then the PIN recheck.
	b.waitFor("PIN recheck asked", `(() => {
		if (document.getElementById('review')?.offsetParent) document.getElementById('review-save').click();
		return !!document.getElementById('recheck-pin')?.offsetParent;
	})()`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("each refusal names its router and says address", `(() => {
		const m = (k) => document.querySelector('.sode__refused[data-refused="' + k + '"]')?.textContent || '';
		return m('router') === 'Not saved. A Studio A: address must be host:port, for example 10.0.0.50:8910.'
			&& m('router/`+uid+`/address') === 'Not saved. B Stage 2: address must be host:port, for example 10.0.0.50:8910.';
	})()`)
	b.check()
}

// Swap… on router B's row opens the Swap dialog on router B. The staged Swap changes router B's preset, and its review
// row names the router.
func TestChromeSwapOnSecondRouter(t *testing.T) {
	b := startBrowser(t)
	const ub = "00000000000000bb"
	_, uid := twoNamedRouters(t, b, func(c *config.Config) {
		c.MoreRouters[0].UID = ub
		ref := func(router string, n int) config.Ref { return config.Ref{Router: router, N: n} }
		c.Presets = []config.Preset{{ID: "pb", Name: "On B", Items: []config.PresetItem{{Src: ref(ub, 0), Dsts: []config.Ref{ref(ub, 4)}}}}}
	})
	if uid != ub {
		t.Fatalf("router B's UID is %q", uid)
	}
	b.eval(`[...document.querySelectorAll('[data-router="` + ub + `"] .sode__routers-head button')].find((x) => x.textContent === 'Swap…').click(); true`)
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

// Every router's title field is labelled Router title, and the first router's heading names it as the others are
// named.
func TestChromeRouterTitleLabel(t *testing.T) {
	b := startBrowser(t)
	_, uid := twoNamedRouters(t, b)
	b.waitFor("one label for the title field", `(() => {
		const label = (id) => document.querySelector('label[for="' + id + '"]')?.textContent;
		return label('cfg-routername') === 'Router title' && label('cfg-rt-`+uid+`-title') === 'Router title'
			&& document.querySelector('#cfg-routershow option[value="name"]').textContent === 'Router title'
			&& document.getElementById('router-1-title').textContent === 'A Studio A'
			&& document.querySelector('[data-router="`+uid+`"] .sode__routers-head .sode__label').textContent === 'B Stage 2';
	})()`)
	typeField(b, "cfg-routername", "Studio 1")
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
		c.MoreRouters[0].Address = "127.0.0.1:1"
	})
	srv := seamServer(t, a)
	uids := routerUIDs(a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	link := func(uid string) string {
		return `document.querySelector('#routers-list [data-link="` + uid + `"]')`
	}
	b.waitFor("A and C Connected with no color, B Never connected in red", `(() => {
		const a = `+link(uids[0])+`, b = `+link(uids[1])+`, c = `+link(uids[2])+`;
		return a?.textContent === 'Connected' && a.className === 'sode__routers-link' && c?.textContent === 'Connected'
			&& b?.textContent === 'Never connected' && b.classList.contains('pb-badge--error') && !!b.dataset.pbTip;
	})()`)
	stopC()
	b.waitFor("C goes Down, live", `(() => { const c = `+link(uids[2])+`; return c.textContent === 'Down' && c.classList.contains('pb-badge--error'); })()`)
	b.check()
}
