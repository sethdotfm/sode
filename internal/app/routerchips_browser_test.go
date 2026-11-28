package app

import (
	"strconv"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The level chips of the router detail view have one width, from the widest label. Level 1 and level 15 are the same
// width. With 64 levels (the most chips), level 1 and level 64 are too. Each label fits in its chip.
func TestChromeSettingsLevelChipsOneWidth(t *testing.T) {
	b := startBrowser(t)
	a := simApp(t, startSim(t), func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	same := func(n, i, j int) string {
		return `(() => {
			const c = [...document.querySelectorAll('#router-chips > .sode__slot')];
			if (c.length !== ` + strconv.Itoa(n) + ` || !c[0].offsetParent) return false;
			const w = (x) => x.getBoundingClientRect().width;
			if (w(c[` + strconv.Itoa(i) + `]) !== w(c[` + strconv.Itoa(j) + `])) throw new Error('chip widths ' + w(c[` + strconv.Itoa(i) + `]) + ' and ' + w(c[` + strconv.Itoa(j) + `]));
			return c.every((x) => x.scrollWidth <= x.clientWidth);
		})()`
	}
	b.waitFor("levels 1 and 15 the same width", same(17, 0, 14))
	typeField(b, "cfg-rt-levels", "64")
	b.waitFor("levels 1 and 64 the same width", same(64, 0, 63))
	b.check()
}

// Settings > Routers on the desktop: the routers are a row of chips above the detail view, which has the full width of
// the section. A chip shows its link state only when the router is not connected. Right and Left move the selection.
func TestChromeSettingsRouterChipsRow(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	a := simRoutersApp(t, []string{simA, simB, "127.0.0.1:1"}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].Title, c.Routers[1].Title, c.Routers[2].Title = "Studio A", "Stage 2", "MCR"
	})
	srv := seamServer(t, a)
	uids := routerUIDs(a)
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	chip := func(uid string) string { return `document.querySelector('#routers-list [data-router="` + uid + `"]')` }
	b.waitFor("one row of chips above a detail view of the full width", `(() => {
		const r = (id) => document.getElementById(id).getBoundingClientRect();
		const rows = [...document.querySelectorAll('#routers-list .sode__rrow')];
		if (rows.length !== 3 || !rows[0].offsetParent) return false;
		const tops = new Set(rows.map((x) => Math.round(x.getBoundingClientRect().top)));
		const list = r('routers-list-wrap'), detail = r('router-detail'), panes = r('routers-panes');
		if (tops.size !== 1) throw new Error('the chips are on ' + tops.size + ' lines');
		if (list.bottom > detail.top) throw new Error('the list ends at ' + list.bottom + ', below the top of the detail view at ' + detail.top);
		if (Math.abs(detail.width - panes.width) > 1) throw new Error('the detail view is ' + detail.width + ' wide in ' + panes.width);
		return true;
	})()`)
	b.waitFor("A and B show no state, C shows Never connected", `(() => {
		const st = (uid) => document.querySelector('#routers-list [data-router="' + uid + '"] .sode__rstate');
		const a = st('`+uids[0]+`'), b = st('`+uids[1]+`'), c = st('`+uids[2]+`');
		return !!a && !a.offsetParent && !!b && !b.offsetParent && c?.offsetParent && c.textContent === 'Never connected'
			&& !`+chip(uids[0])+`.querySelector('.sode__rrow-addr').offsetParent && `+chip(uids[0])+`.textContent.includes('Default');
	})()`)
	b.eval(chip(uids[0]) + `.focus(); document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true })); true`)
	b.waitFor("Right selects router B", `document.activeElement?.dataset.router === '`+uids[1]+`' && document.getElementById('router-detail-name').textContent === 'B Stage 2'`)
	b.eval(`document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', bubbles: true })); true`)
	b.waitFor("Left selects router A", `document.activeElement?.dataset.router === '`+uids[0]+`' && document.getElementById('router-detail-name').textContent === 'A Studio A'`)
	b.check()
}
