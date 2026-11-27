package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings > Routers: the list and the detail view. Every router has the same settings in the same place, and each one
// saves through the one Save. Search finds a setting once for each router and selects that router.
func TestChromeSettingsRouterDetail(t *testing.T) {
	b := startBrowser(t)
	simA, simB, simC := startSim(t), startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB, simC}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].Title, c.Routers[1].Title, c.Routers[2].Title = "Studio A", "Stage 2", "MCR"
	})
	srv := seamServer(t, a)
	uids := routerUIDs(a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)

	b.waitFor("three rows of two lines, Default on the first, a count line and an entry for each router in the sidebar", `(() => {
		const rows = [...document.querySelectorAll('#routers-list .sode__rrow')];
		const toc = [...document.querySelectorAll('#settings-toc .sode__toc-router')].map((x) => x.textContent);
		return rows.length === 3 && rows[0].textContent.includes('Default') && !rows[1].textContent.includes('Default')
			&& rows.every((r) => r.querySelectorAll('.sode__rrow-line').length === 2)
			&& document.getElementById('routers-count').textContent === '3 routers. 3 connected.'
			&& toc.join('|') === 'A Studio A|B Stage 2|C MCR' && !!document.getElementById('cfg-rt-prefix').offsetParent;
	})()`)

	// Down on the list moves the selection to router B.
	b.eval(`document.querySelector('#routers-list [data-router="` + uids[0] + `"]').focus();
		document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })); true`)
	b.waitFor("router B selected and focused", `document.activeElement?.dataset.router === '`+uids[1]+`'
		&& document.getElementById('router-detail-name').textContent === 'B Stage 2' && document.getElementById('cfg-rt-prefix').value === 'B'`)

	typeField(b, "cfg-rt-title", "Stage 9")
	typeField(b, "cfg-rt-protectDevice", "321")
	b.eval(`(() => { const f = document.getElementById('cfg-rt-fireMode'); f.value = 'native'; f.dispatchEvent(new Event('change', { bubbles: true })); return true; })()`)
	b.waitFor("router B's row is Edited and shows the new title", `(() => {
		const r = document.querySelector('#routers-list [data-router="`+uids[1]+`"]');
		return r.textContent.includes('Stage 9') && r.textContent.includes('Edited') && document.getElementById('router-detail-name').textContent === 'B Stage 9';
	})()`)
	// Router A keeps its own values: one set of inputs shows the router in view.
	pickRouter(b, uids[0])
	b.waitFor("router A's values", `document.getElementById('cfg-rt-title').value === 'Studio A' && document.getElementById('cfg-rt-fireMode').value === 'sequential'`)
	pickRouter(b, uids[1])
	b.waitFor("router B keeps its edits", `document.getElementById('cfg-rt-title').value === 'Stage 9' && document.getElementById('cfg-rt-protectDevice').value === '321'`)

	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review names router B in each row", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const rows = [...d.querySelectorAll('tbody tr')].map((r) => r.cells[0]?.textContent || '');
		return rows.includes('B Stage 2: Router title') && rows.includes("B Stage 2: Sode's device number") && rows.includes('B Stage 2: Multi destination')
			&& d.textContent.includes('Router salvo') && document.getElementById('review-save').textContent === 'Save 3 changes';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		rb := a.r().cfg.Routers[1]
		if rb.Title == "Stage 9" && rb.ProtectDevice == 321 && rb.FireMode == "native" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("router B %+v", rb)
		}
	}
	if ra := a.r().cfg.Routers[0]; ra.Title != "Studio A" || ra.FireMode != "sequential" {
		t.Errorf("router A changed: %+v", ra)
	}
	b.waitFor("nothing left unsaved, B not Edited", `!document.querySelector('#routers-list [data-router="`+uids[1]+`"]').textContent.includes('Edited') && document.getElementById('settings-unsaved').hidden`)

	// The page checks Matrix against Name requests when a person leaves the field.
	typeField(b, "cfg-rt-matrix", "20")
	b.eval(`document.getElementById('cfg-rt-matrix').dispatchEvent(new FocusEvent('focusout', { bubbles: true })); true`)
	b.waitFor("the error under Matrix", `document.getElementById('cfg-rt-matrix').closest('.sode__field').querySelector('.sode__field-err')?.textContent.startsWith('Standard name requests allow matrix 0 to 15')`)
	typeField(b, "cfg-rt-matrix", "0")
	b.eval(`document.getElementById('cfg-rt-matrix').dispatchEvent(new FocusEvent('focusout', { bubbles: true })); true`)
	b.waitFor("the error goes", `!document.querySelector('#router-detail .sode__field-err')`)

	// Search: a hit for each router names it, and a pick selects that router and focuses the field.
	b.waitFor("Address found once for each router", `(() => {
		const f = document.getElementById('settings-find');
		f.value = 'address'; f.dispatchEvent(new Event('input'));
		const hits = [...document.querySelectorAll('#settings-found li')].filter((li) => li.firstChild.textContent === 'Address').map((li) => li.lastChild.textContent);
		return hits.join('|') === 'in Routers › A Studio A › Routers|in Routers › B Stage 9 › Routers|in Routers › C MCR › Routers';
	})()`)
	b.eval(`[...document.querySelectorAll('#settings-found li')].find((li) => li.lastChild.textContent.includes('C MCR')).click(); true`)
	b.waitFor("router C selected, its Address focused", `document.querySelector('#routers-list [aria-selected="true"]')?.dataset.router === '`+uids[2]+`'
		&& document.activeElement?.id === 'cfg-rt-address' && document.activeElement.value === '`+simC+`'`)
	b.check()
}

// With one router the list hides and the detail view fills the section, with no Prefix. Search names no router.
func TestChromeSettingsRouterDetailOne(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator; c.Routers[0].Title = "Studio A" })
	srv := seamServer(t, a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("no list, no Prefix, no count line, the detail view named by its title", `!document.getElementById('routers-list-wrap').offsetParent
		&& !document.getElementById('cfg-rt-prefix').offsetParent && document.getElementById('routers-count').textContent === ''
		&& document.getElementById('router-detail-name').textContent === 'Studio A' && !document.getElementById('router-ties-group').offsetParent
		&& !document.querySelector('#settings-toc .sode__toc-router')`)
	b.waitFor("Address found once, with no router name", `(() => {
		const f = document.getElementById('settings-find');
		f.value = 'address'; f.dispatchEvent(new Event('input'));
		const hits = [...document.querySelectorAll('#settings-found li')].filter((li) => li.firstChild.textContent === 'Address').map((li) => li.lastChild.textContent);
		return hits.join('|') === 'in Routers › Routers';
	})()`)
	b.check()
}
