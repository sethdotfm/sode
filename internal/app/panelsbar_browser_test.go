package app

import (
	"testing"
)

// The top bar of /panels, the editor and the Live Editor (panelsbar.js). It checks the links in order, Home and the
// status. It also checks the editor's unsaved-edits rule and the phone row.
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
		window.noBack = () => !document.body.textContent.includes('Back to Sode'); true`
	b.open(srv.URL + "/panels")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	b.eval(links)
	b.waitFor("/panels has Settings and Home, and the status at the right", `(() => {
		const head = document.querySelector('.sode-panels__head'), conn = head.querySelector(':scope > .sode__conn');
		return links(head) === './#settings:#pb-i-settings ./:#pb-i-house' && noBack() && !!conn && conn.getBoundingClientRect().right > head.getBoundingClientRect().right - 2
			&& document.querySelector('.sode-bar__home').dataset.pbTip === 'Sode';
	})()`)
	b.shot("panels")
	b.eval(`document.querySelector('.sode-bar__home').click(); true`)
	b.waitFor("Home opens Sode", `location.pathname === '/'`)

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the lists show", `!!document.getElementById('ed-sources-col')?.offsetParent`)
	b.eval(links)
	b.waitFor("the editor has Settings, Panels and Home", `links(document.querySelector('.sode-ed > .sode-panels__head')) === './#settings:#pb-i-settings panels:logo ./:#pb-i-house' && noBack()`)
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
	b.waitFor("the Live Editor's top bar is the Grid head row: Settings, Panels, Home, a rule, Full screen", `(() => {
		const row = document.querySelector('#ed-live-view .sode-gridpanel__headrow'), bar = document.getElementById('ed-live-bar');
		if (!row || !bar) return false;
		const r = row.getBoundingClientRect(), b = bar.getBoundingClientRect();
		return links(row) === './#settings:#pb-i-settings panels:logo ./:#pb-i-house | screen' && b.top >= r.bottom - 1 && r.top < 40
			&& !document.querySelector('.sode-ed > .sode-panels__head').getClientRects().length && !!row.querySelector('.sode__conn');
	})()`)
	b.shot("live")
	b.eval(`document.querySelector('#ed-sources-list li[data-id="3"] [data-act="pin"]').click(); true`)
	b.waitFor("an unsaved edit in the Live Editor", `!document.getElementById('ed-live-save').hidden`)
	b.eval(`window.stays = 1; document.querySelector('#ed-live-view .sode-bar__panels').click(); true`)
	b.waitFor("Panels with an unsaved edit warns in the Live Editor's bar and stays", `window.stays === 1 && !document.getElementById('ed-live-view').hidden && document.getElementById('ed-live-state').textContent === 'Unsaved changes. Choose again to discard.'`)
	b.eval(`document.querySelector('#ed-live-view .sode-bar__panels').click(); true`)
	b.waitFor("a second Panels discards and opens /panels", `location.pathname === '/panels'`)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/panels")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	b.waitFor("the phone bar is one row: the title, the icons and the status dots, with no words and no side scroll", `(() => {
		const head = document.querySelector('.sode-panels__head'), parts = [head.querySelector('.sode-panels__title'), ...head.querySelectorAll('.sode-bar__tools > a'), head.querySelector('.sode__conn')];
		const mid = (n) => { const r = n.getBoundingClientRect(); return r.top + r.height / 2; };
		const dots = [...head.querySelectorAll('.sode__conn .pb-dot')].filter((d) => d.getClientRects().length);
		const words = [...head.querySelectorAll('.sode__conn .pb-status > :not(.pb-dot), .sode__conn-addr')].filter((n) => n.getClientRects().length);
		return parts.every((n) => n && n.getClientRects().length && Math.abs(mid(n) - mid(parts[0])) < 12) && dots.length === 2 && !words.length
			&& document.documentElement.scrollWidth <= innerWidth && head.getBoundingClientRect().right <= innerWidth;
	})()`)
	b.shot("phone")
	b.check()
}
