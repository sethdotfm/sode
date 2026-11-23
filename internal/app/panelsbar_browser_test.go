package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
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
		};
		// The Lock bar scrolls with the header: after a scroll it keeps its offset to the bar's first button.
		window.sticks = () => {
			const pad = document.body.appendChild(Object.assign(document.createElement('div'), { style: 'height: 3000px' }));
			const first = document.querySelector('.sode-bar .sode-bar__tools > *'), lock = document.querySelector('.sode-lock__who-slot');
			const off = () => lock.getBoundingClientRect().top - first.getBoundingClientRect().top;
			const before = off();
			window.scrollTo({ left: 0, top: 400, behavior: 'instant' });
			const ok = window.scrollY === 400 && first.getBoundingClientRect().top < 0 && Math.abs(off() - before) < 1;
			window.scrollTo({ left: 0, top: 0, behavior: 'instant' });
			pad.remove();
			return ok;
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
	b.waitFor("on /panels the Lock bar scrolls with the header", `sticks()`)
	b.eval(`window.connAt = document.querySelector('.sode-panels__head > .sode__conn').getBoundingClientRect().left; document.querySelector('.sode-panels__title .sode-bar__site').textContent = 'SodeExtremev2222222222'.repeat(4); true`)
	b.waitFor("a long site name is cut and moves nothing", `(() => {
		const head = document.querySelector('.sode-panels__head'), name = head.querySelector('.sode-bar__name');
		return name.scrollWidth > name.clientWidth && head.querySelector(':scope > .sode__conn').getBoundingClientRect().left === connAt && lined(head);
	})()`)
	b.shot("panels")
	b.eval(`document.querySelector('.sode-bar__home').click(); true`)
	b.waitFor("Home opens Sode", `location.pathname === '/'`)
	b.eval(links)
	b.waitFor("on Home the Lock bar scrolls with the header", `!document.querySelector('.sode-lock__who').hidden && sticks()`)

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
	b.waitFor("on /log the Lock bar scrolls with the header", `sticks()`)
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

// The bar's title off Home (panelsbar.js barTitle, bar.css): the site name in bold, a space and the page name. It is
// in the middle of the whole bar. A Lock bar that grows, Inbox that goes and a long name do not move it, and it never runs
// under a button. A narrow window puts it back at the left. More brand events do not repeat the site name. The Lock
// flies its site name from the title's site name, and on /log it fades.
func TestChromeBarTitleCentred(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.BrandText = "Sode v2 Dev" })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := makePanel(t, a, ana, `{"name":"Studio grid","type":"grid","sources":{"members":"all"},"destinations":{"members":"all"}}`)
	size := func(w int) {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	}
	size(1440)
	helpers := `window.r = (n) => n.getBoundingClientRect();
		window.head = () => document.querySelector('.sode-bar');
		window.title = () => head().querySelector('.sode-panels__title');
		window.mid = (n) => r(n).left + r(n).width / 2;
		window.centred = () => Math.abs(mid(title()) - mid(head())) < 1;
		// The title clears every part beside it.
		window.clear = () => {
			const t = r(title()), parts = [...head().children].filter((n) => n !== title() && !n.matches('[class*="__spacer"]') && n.getClientRects().length && r(n).width > 0);
			return parts.every((n) => r(n).right <= t.left || r(n).left >= t.right);
		};
		window.named = (site, page) => {
			const t = title(), s = t.querySelector('.sode-bar__site'), p = t.querySelector('.sode-bar__page'), w = (n) => Number(getComputedStyle(n).fontWeight);
			return t.textContent === site + ' ' + page && t.dataset.pbTip === site + ' ' + page && s.textContent === site && p.textContent === page && w(s) >= 600 && w(p) < 600;
		};
		window.frames = null;
		window.watchBrand = () => {
			const brand = document.querySelector('.sode-lock__brand'), run = brand.animate;
			window.frames = null;
			brand.animate = function (f, o) { if (!window.frames) window.frames = { f, at: r(brand) }; return run.call(this, f, o); };
			return true;
		};
		window.lockNow = () => import('./core.js').then((m) => { m.emit('localLock'); return true; });
		true`
	unlock := func(what string) {
		t.Helper()
		b.waitFor(what+": the held pad", `!!document.querySelector('.sode-lock.is-on') && !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
		for _, d := range "1234" {
			b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click(); true`)
		b.waitFor(what+": the Lock ends", `!document.documentElement.dataset.sodeLock && !document.querySelector('.sode-lock.is-on')`)
	}
	// The Lock's way in starts at from (a selector): at its centre and its scale.
	fliesFrom := func(what, from string) {
		t.Helper()
		b.eval(`window.from = r(document.querySelector('` + from + `')); watchBrand()`)
		b.eval(`lockNow()`)
		b.waitFor(what+": the Lock flies its site name from "+from, `(() => {
			if (!frames) return false;
			const m = /translate\(([-\d.e]+)px, ([-\d.e]+)px\) scale\(([-\d.e]+)\)/.exec(frames.f[0].transform || ''), at = frames.at;
			return !!m && Math.abs(+m[1] - (from.left + from.width / 2 - (at.left + at.width / 2))) < 2 && Math.abs(+m[3] - from.width / at.width) < 0.02;
		})()`)
		unlock(what)
	}

	b.open(srv.URL + "/panels")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	b.eval(helpers)
	b.waitFor("/panels: the site name in bold, a space and Panels, in the middle of the bar and clear of its buttons", `named('Sode v2 Dev', 'Panels') && document.title === 'Sode v2 Dev Panels' && centred() && clear()`)
	b.eval(`import('./brand.js').then((m) => { m.showBrand('Sode v2 Dev'); m.showBrand('Sode v2 Dev'); return true; })`)
	b.waitFor("two more brand events do not repeat the site name", `named('Sode v2 Dev', 'Panels') && document.title === 'Sode v2 Dev Panels'`)
	b.eval(`window.at = mid(title()); document.documentElement.style.setProperty('--sode-who-w', '420px'); document.querySelector('.sode-bar > .sode-inbox__slot').hidden = true; true`)
	b.waitFor("a wider Lock bar and no Inbox do not move the title, and it stays clear", `Math.abs(mid(title()) - at) < 1 && clear()`)
	b.eval(`document.documentElement.style.removeProperty('--sode-who-w'); document.querySelector('.sode-bar > .sode-inbox__slot').hidden = false; title().querySelector('.sode-bar__site').textContent = 'Sode Studio West Gallery'.repeat(4); true`)
	b.waitFor("a long name is cut with an ellipsis in the middle, clear of the buttons", `(() => {
		const n = title().querySelector('.sode-bar__name');
		return n.scrollWidth > n.clientWidth && centred() && clear();
	})()`)
	size(800)
	b.waitFor("a narrow window puts the cut title back at the left, clear of the buttons", `!head().hasAttribute('data-sode-centre') && r(title()).right <= r(head().querySelector('.sode-bar__tools')).left && clear()`)
	size(1440)
	b.open(srv.URL + "/panels")
	b.eval(helpers)
	b.waitFor("the title shows again", `centred() && named('Sode v2 Dev', 'Panels')`)
	fliesFrom("/panels", ".sode-bar__site")

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the lists show", `!!document.getElementById('ed-sources-col')?.offsetParent`)
	b.eval(helpers)
	b.waitFor("the editor: Panel Editor in the middle", `named('Sode v2 Dev', 'Panel Editor') && centred() && clear()`)

	b.open(srv.URL + "/log")
	b.eval(helpers)
	b.waitFor("/log: Communication Log in the middle", `named('Sode v2 Dev', 'Communication Log') && document.title === 'Sode v2 Dev Communication Log' && centred() && clear()`)
	b.eval(`watchBrand()`)
	b.eval(`lockNow()`)
	b.waitFor("/log: the Lock's site name fades in, with no flight", `!!frames && frames.f.every((k) => !k.transform)`)
	unlock("/log")

	// A panel page keeps its own header: the panel's name at the left, where the Lock flies from.
	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the panel page shows the panel's name", `document.getElementById('panel-title')?.textContent === 'Studio grid'`)
	b.eval(helpers)
	b.eval(`import('./brand.js').then((m) => { m.showBrand('Sode v2 Dev'); return true; })`)
	b.waitFor("a brand event leaves the panel's name", `document.getElementById('panel-title').textContent === 'Studio grid'`)
	fliesFrom("/panels/<id>", "#panel-title")
	b.check()
}
