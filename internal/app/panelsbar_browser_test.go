package app

import (
	"testing"
)

// The one page header of /panels, the editor, the Live Editor and /log (panelsbar.js). It checks the links in order,
// Home and the status. The Lock bar, the site name and the status share one line inside the content's edges. A long
// site name is cut and moves nothing. It also checks the editor's unsaved-edits rule and the phone row.
func TestChromePanelsTopBar(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := makePanel(t, a, ana, `{"name":"Studio grid","type":"grid","sources":{"members":"all"},"destinations":{"members":"all"}}`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	links := `window.links = (root) => [...root.querySelectorAll('a.pb-btn--icon, .pb-divider--vertical, #ed-live-screen')].filter((n) => !n.classList.contains('sode-gridpanel__whorule')).map((n) =>
		n.matches('.pb-divider--vertical') ? '|' : n.id === 'ed-live-screen' ? 'screen' : n.getAttribute('href') + ':' + (n.querySelector('use')?.getAttribute('href') || 'logo')).join(' ');
		window.noBack = () => !document.body.textContent.includes('Back to Sode');
		// box: the element whose content box is the page's edges; else the header's siblings set them.
		window.lined = (head, box) => {
			const r = (n) => n.getBoundingClientRect(), mid = (n) => r(n).top + r(n).height / 2;
			const lock = document.querySelector('.sode-lock__who-slot'), title = head.querySelector('.sode-panels__title'), conn = head.querySelector(':scope > .sode__conn');
			const content = [...head.parentElement.children].filter((n) => n !== head && n.getClientRects().length && r(n).width > 0);
			const cs = box && getComputedStyle(box);
			const left = box ? r(box).left + parseFloat(cs.paddingLeft) : Math.min(...content.map((n) => r(n).left));
			const right = box ? r(box).right - parseFloat(cs.paddingRight) : Math.max(...content.map((n) => r(n).right));
			return [lock, title, conn].every((n) => Math.abs(mid(n) - mid(head)) < 2) && Math.abs(r(lock).left - r(head).left) < 1.5 && r(title).left >= r(document.querySelector('.sode-lock__who')).right
				&& Math.abs(r(head).left - left) < 1.5 && Math.abs(r(head).right - right) < 1.5 && Math.abs(r(conn).right - r(head).right) < 1.5;
		}; true`
	b.open(srv.URL + "/panels")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	b.eval(links)
	b.waitFor("/panels has Home and Settings, and the status at the right", `(() => {
		const head = document.querySelector('.sode-panels__head'), conn = head.querySelector(':scope > .sode__conn');
		return links(head) === './:#pb-i-house ./#settings:#pb-i-settings' && noBack() && !!conn && conn.getBoundingClientRect().right > head.getBoundingClientRect().right - 2
			&& /^Sode \((Alt\+Shift\+|⌥⇧)h\)$/.test(document.querySelector('.sode-bar__home').dataset.pbTip) && document.querySelector('.sode-bar__home').getAttribute('aria-keyshortcuts') === 'Alt+Shift+H';
	})()`)
	b.waitFor("the Lock bar, the site name and the status are on one line, inside the content's edges", `lined(document.querySelector('.sode-panels__head'))`)
	b.eval(`window.connAt = document.querySelector('.sode-panels__head > .sode__conn').getBoundingClientRect().left; document.querySelector('.sode-panels__title .sode-bar__name').textContent = 'SodeExtremev2222222222'.repeat(4) + ' Panels'; true`)
	b.waitFor("a long site name is cut and moves nothing", `(() => {
		const head = document.querySelector('.sode-panels__head'), name = head.querySelector('.sode-bar__name');
		return name.scrollWidth > name.clientWidth && head.querySelector(':scope > .sode__conn').getBoundingClientRect().left === connAt && lined(head);
	})()`)
	b.shot("panels")
	b.eval(`document.querySelector('.sode-bar__home').click(); true`)
	b.waitFor("Home opens Sode", `location.pathname === '/'`)

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the lists show", `!!document.getElementById('ed-sources-col')?.offsetParent`)
	b.eval(links)
	b.waitFor("the editor has Home, Panels and Settings", `links(document.querySelector('.sode-ed > .sode-panels__head')) === './:#pb-i-house panels:logo ./#settings:#pb-i-settings' && noBack()`)
	b.shot("editor")
	b.eval(`{ const n = document.getElementById('ed-name'); n.value = 'Studio grid 2'; n.dispatchEvent(new Event('input', { bubbles: true })); } true`)
	b.waitFor("an unsaved edit", `!document.getElementById('ed-save').disabled`)
	b.eval(`window.stays = 1; document.querySelector('.sode-ed .sode-bar__home').click(); true`)
	b.waitFor("Home with an unsaved edit warns and stays", `window.stays === 1 && location.pathname.endsWith('/editor') && document.getElementById('ed-status').textContent === 'Unsaved changes. Choose again to discard.'`)
	b.eval(`document.querySelector('.sode-ed .sode-bar__home').click(); true`)
	b.waitFor("a second Home discards and opens Sode", `location.pathname === '/'`)

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the lists show", `!!document.getElementById('ed-sources-col')?.offsetParent`)
	b.eval(links + `; document.getElementById('ed-live').click(); true`)
	b.waitFor("the Live Editor keeps the page header: Home, Panels, Settings, a rule, Full screen, and the run has no header row", `(() => {
		const head = document.querySelector('.sode-ed > .sode-panels__head'), bar = document.getElementById('ed-live-bar');
		if (!head?.getClientRects().length || !bar) return false;
		const r = head.getBoundingClientRect(), b = bar.getBoundingClientRect();
		return links(head) === './:#pb-i-house panels:logo ./#settings:#pb-i-settings | screen' && b.top >= r.bottom && r.top < 40 && lined(head, document.querySelector('#ed-live-view .sode-gridpanel__deck'))
			&& !document.querySelector('#ed-live-view .sode-gridpanel__headrow') && !document.querySelector('#ed-live-view .sode__conn') && !!head.querySelector(':scope > .sode-gridpanel__slot');
	})()`)
	b.shot("live")
	b.eval(`document.querySelector('#ed-sources-list li[data-id="3"] [data-act="pin"]').click(); true`)
	b.waitFor("an unsaved edit in the Live Editor", `!document.getElementById('ed-live-save').hidden`)
	b.eval(`window.stays = 1; document.querySelector('.sode-ed > .sode-bar .sode-bar__panels').click(); true`)
	b.waitFor("Panels with an unsaved edit warns in the Live Editor's bar and stays", `window.stays === 1 && !document.getElementById('ed-live-view').hidden && document.getElementById('ed-live-state').textContent === 'Unsaved changes. Choose again to discard.'`)
	b.eval(`document.querySelector('.sode-ed > .sode-bar .sode-bar__panels').click(); true`)
	b.waitFor("a second Panels discards and opens /panels", `location.pathname === '/panels'`)

	b.open(srv.URL + "/log")
	b.eval(links)
	b.waitFor("/log has the same header: Home and Settings, Inbox and the status at the right, on the Lock bar's line", `(() => {
		const head = document.querySelector('.sode-log__head');
		return head.classList.contains('sode-bar') && links(head) === './:#pb-i-house ./#settings:#pb-i-settings' && noBack() && !!head.querySelector('[data-inbox-button]') && lined(head);
	})()`)
	b.shot("log")

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/panels")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	b.waitFor("the phone bar is one row: the title, the icons and the status dots, with no words and no side scroll", `(() => {
		const head = document.querySelector('.sode-panels__head'), parts = [head.querySelector('.sode-panels__title'), ...head.querySelectorAll('.sode-bar__tools > a'), head.querySelector('.sode__conn')];
		const mid = (n) => { const r = n.getBoundingClientRect(); return r.top + r.height / 2; };
		const dots = [...head.querySelectorAll('.sode__conn .sode__state')].filter((d) => d.getClientRects().length);
		const words = [...head.querySelectorAll('.sode__conn .pb-status > :not(.sode__state), .sode__conn-addr')].filter((n) => n.getClientRects().length);
		return parts.every((n) => n && n.getClientRects().length && Math.abs(mid(n) - mid(parts[0])) < 12) && dots.length === 2 && !words.length
			&& document.documentElement.scrollWidth <= innerWidth && head.getBoundingClientRect().right <= innerWidth;
	})()`)
	b.shot("phone")
	b.check()
}
