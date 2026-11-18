package app

import (
	"fmt"
	"strings"
	"testing"
)

// bandRects keeps the band's control rects in window.__band; bandSame is true while none has moved.
const bandRects = `(() => {
	const ids = ['#panel-lvl', '#panel-lvl-all', '#panel-lvl-pick', '.sode-gridpanel__find[data-kind="srcs"] input', '.sode-gridpanel__find[data-kind="dsts"] input',
		'.sode-gridpanel__find-mode', '#panel-clear', '#panel-take', '.sode-gridpanel__scroll', '#panel-title'];
	window.__bandAt = () => ids.map((s) => { const r = document.querySelector(s).getBoundingClientRect(); return [r.left, r.top, r.width, r.height].map(Math.round).join(','); }).join('|');
	window.__band = window.__bandAt();
	return true;
})()`

const bandSame = `window.__bandAt() === window.__band`

// bandFits is true when the deck's controls are whole, inside the deck and apart from each other, and the page does
// not scroll sideways.
const bandFits = `(() => {
	const deck = document.querySelector('.sode-gridpanel__deck').getBoundingClientRect();
	const sel = ['#panel-title', '.sode-gridpanel__tools', '#panel-lvl-pick', '#panel-lvl', '#panel-lvl-all', '.sode-gridpanel__find[data-kind="srcs"]',
		'.sode-gridpanel__find[data-kind="dsts"]', '.sode-gridpanel__find-mode', '#panel-clear', '#panel-take'];
	const rs = sel.map((s) => document.querySelector(s).getBoundingClientRect());
	const inside = rs.every((r) => r.width > 0 && r.left >= deck.left && r.right <= deck.right + 0.5 && r.top >= deck.top && r.bottom <= deck.bottom);
	const apart = rs.every((a, i) => rs.every((b, j) => j <= i || a.right <= b.left + 0.5 || b.right <= a.left + 0.5 || a.bottom <= b.top + 0.5 || b.bottom <= a.top + 0.5));
	const whole = ['#panel-title', '#panel-lvl-all', '#panel-clear', '#panel-take'].every((s) => { const n = document.querySelector(s); return n.scrollWidth <= n.clientWidth + 1; });
	return inside && apart && whole && document.documentElement.scrollWidth === innerWidth;
})()`

// The Grid panel page at 1440 by 900. The header, the Levels box and the search boxes are one band, the deck. Its
// header row is laid out as the main page. At the left are the title, a rule and the instance name, on the Lock chip's
// centre line. Then come Settings, the Panels link, Edit, a rule and Full screen. The main page's Server and Router status is
// at the right. The Lock chip names the user as on the main page. Under it are the Levels box with Select all under it, the search boxes and the
// Filter switch. Then come Clear and the red Take with the note over them. Nothing in the band moves as words
// change. It checks the Sources and Destinations words in the corner, the picked row and column washes and the staged ring. The
// cursor ring, the staged ring and Take pulse in one phase. The grid goes down to the window's bottom and Full width
// and Kiosk mode are gone. The band fits at 1280 by 800 and 1920 by 1080, and 50 pins on each edge leave a short
// window room to scroll. Then it checks the panels list's picture and words for a Grid panel.
func TestChromeGridPanelLook(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Studio grid","type":"grid","settings":{"mode":"take","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on') && !!`+gridXp(3, 3))

	b.waitFor("the header row has the main page's status at its right end and no other details", `(() => {
		const t = (s) => [...document.querySelectorAll(s)].map((n) => n.textContent.trim()).join('|');
		const conn = document.querySelector('.sode-gridpanel__headrow > .sode__conn');
		const dots = [...conn.querySelectorAll('.pb-dot')];
		const row = conn.parentNode.getBoundingClientRect(), c = conn.getBoundingClientRect();
		return t('.sode-gridpanel__headrow > .sode__conn .pb-status') === 'Server|Router' && dots.length === 2 && dots.every((d) => d.classList.contains('pb-dot--ok')) &&
			!document.querySelector('.sode-gridpanel__meta, .sode-gridpanel__stat') && t('#panel-instance') === 'Desk' &&
			getComputedStyle(conn).position === 'static' && conn.parentNode.lastElementChild === conn && row.right - c.right < 2;
	})()`)
	b.waitFor("Settings, the Panels link, Edit, a rule and Full screen follow the title and instance name at the left", `(() => {
		const tools = document.querySelector('.sode-gridpanel__tools'), title = document.getElementById('panel-title').getBoundingClientRect();
		const s = tools.querySelector('a.sode-gridpanel__settings'), r = tools.getBoundingClientRect();
		const shown = [...tools.querySelectorAll('a, button, .pb-divider')].filter((n) => n.getClientRects().length);
		const ids = shown.map((n) => (n.classList.contains('pb-divider') ? '|' : n.id || n.getAttribute('aria-label')));
		const edit = tools.querySelector('.sode-gridpanel__edit');
		const name = [...document.querySelector('.sode-gridpanel__name').children].filter((n) => n.getClientRects().length).map((n) => n.id || (n.classList.contains('pb-divider') ? '|' : '?'));
		return !!s && new URL(s.href).hash === '#settings' && new URL(s.href).pathname === '/' && s.classList.contains('pb-btn--ghost') &&
			ids.join() === 'Settings,Panels,Edit panel,|,panel-screen' && new URL(edit.href).pathname === '/panels/editor' &&
			name.join() === 'panel-title,panel-instance,|' && r.left > title.right && r.right < innerWidth / 2;
	})()`)
	b.waitFor("the title, the instance name and the buttons are on the Lock chip's centre line", `(() => {
		const mid = (s) => { const r = document.querySelector(s).getBoundingClientRect(); return (r.top + r.bottom) / 2; };
		const c = mid('.sode-lock__who');
		return ['#panel-title', '#panel-instance', '.sode-gridpanel__settings', '#panel-screen'].every((s) => Math.abs(mid(s) - c) < 0.5);
	})()`)
	b.waitFor("one deck at the top holds the header, the Levels box and the search boxes, under 176 px high (the banner's two lines took 4.5 px)", `(() => {
		const deck = document.querySelector('.sode-gridpanel__deck');
		const r = deck.getBoundingClientRect();
		const inside = ['#panel-title', '.sode-gridpanel__tools', '#panel-lvl', '#panel-lvl-all', '.sode-gridpanel__find[data-kind="srcs"] input', '.sode-gridpanel__find-mode', '#panel-take']
			.every((s) => { const n = deck.querySelector(s); const b = n?.getBoundingClientRect(); return b && b.top >= r.top && b.bottom <= r.bottom; });
		return r.top === 0 && r.height < 176 && inside && !document.querySelector('.sode-gridpanel__head .sode-gridpanel__find');
	})()`)
	b.waitFor("the Lock chip does not cover the title or the Levels box", `(() => {
		const c = document.querySelector('.sode-lock__who').getBoundingClientRect();
		const off = (s) => { const n = document.querySelector(s).getBoundingClientRect(); return n.top >= c.bottom || n.left >= c.right; };
		return off('#panel-title') && off('#panel-lvl-pick') && off('#panel-levels .sode__box-head');
	})()`)
	b.waitFor("the Levels box is the main page's, large words that fit, with a short Select all as wide as it under it, both in the picks' height", `(() => {
		const i = document.getElementById('panel-lvl'), p = document.getElementById('panel-lvl-pick').getBoundingClientRect();
		const r = i.getBoundingClientRect(), cs = getComputedStyle(i), a = document.getElementById('panel-lvl-all').getBoundingClientRect();
		const c = document.createElement('canvas').getContext('2d');
		c.font = cs.fontWeight + ' ' + cs.fontSize + ' ' + cs.fontFamily;
		const fits = c.measureText(i.placeholder).width + parseFloat(cs.paddingLeft) + parseFloat(cs.paddingRight) <= i.clientWidth;
		return i.classList.contains('sode__input') && r.width >= 151 && cs.fontSize === '24.75px' && fits &&
			Math.abs(r.top - p.top) < 1 && Math.abs(a.bottom - p.bottom) < 1 && a.top > r.bottom && a.height < r.height &&
			Math.abs(a.left - r.left) < 1 && Math.abs(a.width - r.width) < 1;
	})()`)
	b.waitFor("nothing in the deck is cut: the title, its line, Select none on one line", `(() => {
		const cut = (n) => n.scrollWidth > n.clientWidth + 1;
		const all = document.getElementById('panel-lvl-all');
		const line = [...all.children].map((n) => n.getBoundingClientRect().top);
		return !cut(document.getElementById('panel-title')) && !cut(all) &&
			/^Select(All|None)$/.test(all.textContent) && Math.abs(line[0] - line[1]) < 1 && document.documentElement.scrollWidth === innerWidth;
	})()`)
	b.waitFor("the Lock chip names the user, as on the main page", `(() => {
		const name = document.querySelector('.sode-lock__who-name');
		return !!document.querySelector('.sode-lock__who .pb-btn') && name.getClientRects().length > 0 && name.textContent === 'Signed in as Ana';
	})()`)
	b.waitFor("the search boxes have the border all the way round, with the role colour as a strip inside it, as on the main page", `[...document.querySelectorAll('.sode-gridpanel__find-input')].every((i) => {
		const c = getComputedStyle(i);
		return c.borderLeftWidth === c.borderRightWidth && c.borderLeftColor === c.borderRightColor && c.backgroundImage.startsWith('linear-gradient');
	})`)
	b.waitFor("the switch is one switch, off, that says Find, as tall as both search boxes; the boxes are at most 14 rem wide", `(() => {
		const m = document.querySelector('.sode-gridpanel__find-mode');
		const sw = m.querySelector('input[role="switch"]');
		const top = document.querySelector('.sode-gridpanel__find[data-kind="srcs"] input').getBoundingClientRect();
		const bot = document.querySelector('.sode-gridpanel__find[data-kind="dsts"] input').getBoundingClientRect();
		const r = m.getBoundingClientRect();
		const boxes = [...document.querySelectorAll('.sode-gridpanel__find')].map((n) => n.getBoundingClientRect().width);
		return m.tagName === 'LABEL' && m.textContent === 'Find' && sw.getAttribute('aria-label') === 'Filter' && sw.type === 'checkbox' && !sw.checked &&
			boxes.length === 2 && boxes.every((w) => w > 100 && w <= 14 * 18 + 1) &&
			Math.abs(r.top - top.top) < 2 && Math.abs(r.bottom - bot.bottom) < 2;
	})()`)
	b.waitFor("Full screen is a ghost icon, Full width and Kiosk mode are gone and the Panels link is with the header buttons", `(() => {
		const s = document.getElementById('panel-screen');
		return getComputedStyle(s, '::before').content === 'none' && s.getBoundingClientRect().width < 48 &&
			s.classList.contains('pb-btn--ghost') && !s.classList.contains('pb-btn--outline') &&
			document.getElementById('panel-kiosk').getClientRects().length === 0 &&
			document.getElementById('panel-full').getClientRects().length === 0 &&
			document.querySelector('.sode-gridpanel__tools').contains(document.getElementById('panel-home')) &&
			!!document.querySelector('#panel-home a[href="panels"]');
	})()`)
	b.waitFor("big scroll bars: 25 px on both axes", `(() => {
		const x = document.querySelector('.sode-gridpanel__frame > .sode-bigscroll--x'), y = document.querySelector('.sode-gridpanel__frame > .sode-bigscroll--y');
		return !!x && !!y && !x.hidden && !y.hidden && x.offsetHeight === 25 && y.offsetWidth === 25;
	})()`)
	b.waitFor("the grid reaches the window's bottom", `Math.abs(document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect().bottom - innerHeight) < 2 && document.documentElement.scrollHeight === innerHeight`)
	b.waitFor("each name shows its router number before it, in the number font, and its text is the name alone", `(() => {
		const probe = document.createElement('i'); probe.style.fontFamily = 'var(--pb-font-num)'; document.body.append(probe);
		const num = getComputedStyle(probe).fontFamily; probe.remove();
		const names = [...document.querySelectorAll('.sode-gridpanel__src:not([hidden]), .sode-gridpanel__dst:not([hidden])')];
		const offs = new Set(names.map((n) => Number(n.dataset.num) - Number(n.dataset.src ?? n.dataset.dst)));
		const src = document.querySelector('.sode-gridpanel__src[data-src="0"]');
		return names.length > 10 && offs.size === 1 && [0, 1].includes([...offs][0]) && src.textContent === 'Black' &&
			names.every((n) => { const b = getComputedStyle(n, '::before'); return b.content === '"' + n.dataset.num + '"' && b.fontFamily === num; });
	})()`)
	b.waitFor("the corner names both bars: Destinations along its bottom, Sources on its side down its right edge, each word and its arrow centred", `(() => {
		const head = document.querySelector('.sode-gridpanel__head'), h = head.getBoundingClientRect();
		const src = head.querySelector(':scope > .sode-gridpanel__label--src'), dst = head.querySelector(':scope > .sode-gridpanel__label--dst');
		const s = src.getBoundingClientRect(), d = dst.getBoundingClientRect();
		const mid = (outer, inner, ax) => Math.abs((outer[ax] + outer[ax === 'left' ? 'right' : 'bottom']) / 2 - (inner[ax] + inner[ax === 'left' ? 'right' : 'bottom']) / 2) < 2;
		const sw = src.getBoundingClientRect(), dw = dst.getBoundingClientRect();
		const words = (n) => { const r = [...n.children].map((k) => k.getBoundingClientRect()); return { top: Math.min(...r.map((x) => x.top)), bottom: Math.max(...r.map((x) => x.bottom)), left: Math.min(...r.map((x) => x.left)), right: Math.max(...r.map((x) => x.right)) }; };
		const arrows = [src, dst].map((n) => n.querySelector('svg use')?.getAttribute('href')).join();
		return src.textContent === 'Sources' && dst.textContent === 'Destinations' && head.lastChild === dst && !document.querySelector('.sode-gridpanel__bar') &&
			getComputedStyle(src).writingMode === 'vertical-rl' && s.height > s.width && s.right <= h.right && s.right > h.right - 12 &&
			d.bottom <= h.bottom && d.bottom > h.bottom - 12 && d.right <= s.left && mid(sw, words(src), 'top') && mid(dw, words(dst), 'left') &&
			arrows === '#pb-i-arrow-left,#pb-i-arrow-down';
	})()`)

	b.waitFor("the Grid page's root is 112.5%: 72 px crosspoints with 15 px names", `getComputedStyle(document.documentElement).fontSize === '18px' && `+
		`Math.round(document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width) === 72 && `+
		`getComputedStyle(document.querySelector('.sode-gridpanel__src')).fontSize === '14.625px'`)

	// The band's controls, to check that nothing moves as their words change.
	b.eval(bandRects)
	b.eval(`document.querySelector('.sode-gridpanel__dst[data-dst="2"]').click(); document.querySelector('.sode-gridpanel__src[data-src="4"]').click(); true`)
	b.waitFor("the picked row and column are washed", gridXp(6, 2)+`.hasAttribute('data-row') && !`+gridXp(6, 2)+`.hasAttribute('data-col') && `+
		gridXp(4, 5)+`.hasAttribute('data-col') && `+gridXp(4, 2)+`.hasAttribute('data-row') && `+gridXp(4, 2)+`.hasAttribute('data-col') && !`+gridXp(6, 5)+`.hasAttribute('data-row') && `+
		`getComputedStyle(`+gridXp(6, 2)+`).backgroundColor !== getComputedStyle(`+gridXp(6, 5)+`).backgroundColor`)
	b.eval(`document.querySelector('.sode-gridpanel__dst[data-dst="2"]').click(); document.querySelector('.sode-gridpanel__src[data-src="4"]').click(); true`)
	b.waitFor("an unpicked row is plain again", `!`+gridXp(6, 2)+`.hasAttribute('data-row')`)

	b.eval(gridXp(5, 3) + `.click()`)
	b.waitFor("the staged ring is a pie of the staged levels", `(() => {
		const r = `+gridXp(5, 3)+`.querySelector('.sode-gridpanel__stage');
		const bg = r.style.background;
		return `+gridXp(5, 3)+`.dataset.s === 'staged' && getComputedStyle(r).display === 'block' && bg.includes('conic-gradient') && (bg.match(/transparent/g) || []).length > 3;
	})()`)
	b.waitFor("Take says only Take at the right of the deck; Clear and Take are as tall as the picks, past the Filter switch; the note is over them", `(() => {
		const t = document.getElementById('panel-take').getBoundingClientRect();
		const c = document.getElementById('panel-clear').getBoundingClientRect();
		const f = document.querySelector('.sode-gridpanel__find-mode').getBoundingClientRect();
		const p = document.getElementById('panel-lvl-pick').getBoundingClientRect();
		const n = document.querySelector('.sode-gridpanel__chip').getBoundingClientRect();
		const same = (r) => Math.abs(r.top - p.top) < 1 && Math.abs(r.height - p.height) < 1;
		return same(t) && same(c) && innerWidth - t.right < 24 && f.right <= c.left && c.right <= t.left &&
			n.bottom <= t.top && Math.abs(n.right - t.right) < 2 && document.getElementById('panel-take').textContent === 'Take';
	})()`)
	b.waitFor("the V pick is one pick wide and two picks high, as on the main page, filled when on", `(() => {
		const [v, two] = document.querySelectorAll('#panel-lvl-pick > .sode__pick');
		const r = v.getBoundingClientRect(), p = two.getBoundingClientRect();
		return Math.abs(r.width - p.width) < 2 && r.height > 2 * p.height && v.getAttribute('aria-pressed') === 'true' && getComputedStyle(v).backgroundColor !== 'rgba(0, 0, 0, 0)';
	})()`)
	b.waitFor("an armed Take is the main page's red Take, between --pb-status-error and --pb-danger-hover, with its words", `(() => {
		const t = document.getElementById('panel-take');
		if (!t.classList.contains('pb-btn--danger')) return false;
		// Canvas turns any colour form (oklab from color-mix) into RGB; a probe reads the tokens in this theme.
		const rgb = (c) => { const x = document.createElement('canvas').getContext('2d'); x.fillStyle = c; x.fillRect(0, 0, 1, 1); return [...x.getImageData(0, 0, 1, 1).data]; };
		const token = (v) => { const p = document.createElement('i'); p.style.color = 'var(' + v + ')'; document.body.append(p); const c = getComputedStyle(p).color; p.remove(); return rgb(c); };
		const bg = rgb(getComputedStyle(t).backgroundColor), lo = token('--pb-status-error'), hi = token('--pb-danger-hover');
		const ink = rgb(getComputedStyle(t).color), want = token('--pb-text-on-accent');
		const near = (a, b) => a.slice(0, 3).every((v, k) => Math.abs(v - b[k]) <= 3);
		return t.classList.contains('sode-panel__take--armed') && bg.slice(0, 3).every((v, k) => v >= Math.min(lo[k], hi[k]) - 3 && v <= Math.max(lo[k], hi[k]) + 3) && near(ink, want);
	})()`)
	held := `[...document.querySelectorAll('.sode-gridpanel__held:not([hidden])')].map((n) => n.parentNode.dataset.dst + ':' + n.textContent).join(',')`
	note := `[...document.querySelectorAll('.sode-gridpanel__note > :not([hidden])')].map((n) => n.textContent).join('|')`
	b.waitFor("the staged destination says so, and the note counts it with no help words", held+` === '3:staged' && `+note+` === '1 staged'`)
	// Over a crosspoint the cursor is a ring. All three count from time zero (gridpanelview.js joinPulse).
	b.eval(`(() => {
		const r = ` + gridXp(6, 5) + `.getBoundingClientRect();
		document.querySelector('.sode-gridpanel__scroll').dispatchEvent(new PointerEvent('pointermove', { clientX: r.x + 20, clientY: r.y + 20, bubbles: true, pointerType: 'mouse' }));
		return true;
	})()`)
	b.waitFor("the cursor ring, the staged ring and the armed Take pulse in one phase", `(() => {
		const phase = (n) => n.getAnimations().filter((a) => a.animationName === 'sode-panel-pulse').map((a) => a.currentTime % 2000);
		const t = [...phase(document.querySelector('.sode-gridpanel__ring')), ...phase(`+gridXp(5, 3)+`), ...phase(document.getElementById('panel-take'))];
		return t.length === 3 && Math.max(...t) - Math.min(...t) < 20;
	})()`)
	b.waitFor("the search boxes name the mode and show their keys", `[...document.querySelectorAll('.sode-gridpanel__find-input')].map((n) => n.placeholder).join('|') === 'Find sources|Find destinations' && `+
		`[...document.querySelectorAll('.sode-gridpanel__find-key')].map((n) => n.textContent).join('') === 'sd'`)
	b.waitFor("the lock screen has faded", `!document.documentElement.hasAttribute("data-sode-lock")`)
	b.shot("look")
	b.eval(`document.querySelector('.sode-gridpanel__dst[data-dst="2"]').click(); document.querySelector('.sode-gridpanel__dst[data-dst="3"]').dispatchEvent(new MouseEvent('click', { bubbles: true, ctrlKey: true })); ` +
		`document.querySelector('.sode-gridpanel__src[data-src="5"]').click(); true`)
	b.waitFor("a pick shows first in the note, whole", note+` === '2 destinations, 1 source picked' && (() => { const c = document.querySelector('.sode-gridpanel__chip'); return c.scrollWidth <= c.clientWidth; })()`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 800, "deviceScaleFactor": 1, "mobile": false})
	b.waitFor("at 1280 the note is whole too", `innerWidth === 1280 && (() => { const c = document.querySelector('.sode-gridpanel__chip'); return c.scrollWidth <= c.clientWidth; })()`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.waitFor("back at 1440", `innerWidth === 1440`)
	b.eval(`document.getElementById('panel-lvl-all').click(); document.querySelector('.sode-gridpanel__find-mode input').click(); true`)
	b.waitFor("Select all changes its words and Filter is on, and the switch says so", `document.getElementById('panel-lvl-all').textContent === 'SelectNone' && document.querySelector('.sode-gridpanel__find-mode input').checked && `+
		`document.querySelector('.sode-gridpanel__find-mode').textContent === 'Filter'`)
	b.waitFor("nothing in the band moved", bandSame)
	b.waitFor("the switch has moved", `!document.getAnimations().some((a) => a instanceof CSSTransition)`)
	b.shot("filter-on")
	b.eval(`document.getElementById('panel-lvl-all').click(); document.querySelector('.sode-gridpanel__find-mode input').click(); ` +
		`document.querySelector('.sode-gridpanel__dst[data-dst="3"]').dispatchEvent(new MouseEvent('click', { bubbles: true, ctrlKey: true })); document.querySelector('.sode-gridpanel__src[data-src="5"]').click(); true`)
	b.waitFor("one pick left", note+` === '1 destination picked'`)
	b.eval(`document.getElementById('panel-clear').click(); true`)
	b.waitFor("Clear drops the picks first", note+` === '1 staged' && !document.querySelector('.sode-gridpanel__dst[data-picked]')`)
	b.eval(`document.getElementById('panel-clear').click(); true`)
	b.waitFor("then the staged routes, and has nothing left to drop", held+` === '' && document.getElementById('panel-take').disabled && document.getElementById('panel-clear').disabled && `+
		note+` === ''`)

	for _, size := range [][2]int{{1280, 800}, {1920, 1080}} {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": size[0], "height": size[1], "deviceScaleFactor": 1, "mobile": false})
		b.waitFor(fmt.Sprintf("at %d by %d the band fits: nothing cut, nothing over another, no sideways scroll", size[0], size[1]), `innerWidth === `+fmt.Sprint(size[0])+` && `+bandFits)
		b.shot(fmt.Sprintf("band-%d", size[0]))
	}

	// 50 pins on each edge on a short window: the pinned bands leave room to scroll.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 800, "deviceScaleFactor": 1, "mobile": false})
	pinList := make([]string, 50)
	for i := range pinList {
		pinList[i] = fmt.Sprintf(`"%d":{"pinned":true}`, i+1)
	}
	pins := "{" + strings.Join(pinList, ",") + "}"
	pinned := makePanel(t, a, ana, `{"name":"Pinned grid","type":"grid","settings":{"mode":"take","levels":[1]},`+
		`"sources":{"members":"all","items":`+pins+`},"destinations":{"members":"all","items":`+pins+`}}`)
	b.open(srv.URL + "/panels/" + pinned + "?instance=Desk")
	b.waitFor("the pinned grid", `!!document.querySelector('.sode-gridpanel__xp')`)
	b.waitFor("the pinned bands take at most half the room past the names, so the rest scrolls, and each band has its own little bar", `(() => {
		const f = document.querySelector('.sode-gridpanel__field'), sc = document.querySelector('.sode-gridpanel__scroll');
		const v = (k) => Number(getComputedStyle(f).getPropertyValue(k).trim());
		const xp = document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width;
		const name = document.querySelector('.sode-gridpanel__probe').getBoundingClientRect();
		const rows = v('--sode-row-pins'), cols = v('--sode-col-pins');
		const bars = ['x', 'y'].map((a) => document.querySelector('.sode-gridpanel__pins--' + a + ' + .sode-bigscroll--little')?.getBoundingClientRect());
		const p = sc.getBoundingClientRect();
		return rows >= 1 && rows < 50 && cols >= 1 && rows * xp <= (sc.clientHeight - name.height) / 2 && cols * xp <= (sc.clientWidth - name.width) / 2 &&
			bars.every((r) => r && r.width > 0 && r.height > 0) && Math.abs(bars[0].width - cols * xp) < 1 && Math.abs(bars[1].height - rows * xp) < 1 &&
			Math.abs(bars[0].left - (p.left + name.width)) < 1 && Math.abs(bars[1].top - (p.top + name.height)) < 1;
	})()`)
	b.shot("pins-1280")

	members := func(n int) string {
		l := make([]string, n)
		for i := range l {
			l[i] = fmt.Sprint(i + 1)
		}
		return `{"members":[` + strings.Join(l, ",") + `]}`
	}
	makePanel(t, a, ana, `{"name":"Small grid","type":"grid","settings":{"mode":"take","levels":[1]},"sources":`+members(4)+`,"destinations":`+members(2)+`}`)
	makePanel(t, a, ana, `{"name":"Medium grid","type":"grid","settings":{"mode":"take","levels":[1]},"sources":`+members(40)+`,"destinations":`+members(20)+`}`)

	b.open(srv.URL + "/panels")
	b.waitFor("the Grid panel in the list, on a page that keeps the 125% root", `!!document.querySelector('.sode-ov__item[data-id="`+id+`"]') && getComputedStyle(document.documentElement).fontSize === '20px'`)
	b.eval(`document.querySelector('.sode-ov__item[data-id="` + id + `"]').click(); true`)
	b.waitFor("its picture and words", `(() => {
		const big = document.querySelector('.sode-ov__big');
		return big?.querySelector('.pb-hint')?.textContent === 'Grid, all sources by all destinations' && !!big.querySelector('.sode-ov__thumb--grid .sode-ov__k-on') &&
			big.querySelector('.sode-ov__thumb--grid').dataset.size === 'large' && !big.querySelector('.sode-ov__count') && [...big.querySelectorAll('.sode-ov__thumb--grid .sode-ov__tile')].map((x) => x.textContent).join() === 'A,L,L,A,L,L' &&
			!!document.querySelector('.sode-ov__item[data-id="`+id+`"] .sode-ov__thumb--grid');
	})()`)
	b.waitFor("each size shows its counts one character per square in the two bars, the side bar turned and reading upward, with no N by M label", `(() => {
		const item = (name) => [...document.querySelectorAll('.sode-ov__item')].find((n) => n.textContent.includes(name));
		const tiles = (name) => [...item(name).querySelectorAll('.sode-ov__tile')].map((x) => x.textContent).join();
		const dst = [...item('Medium grid').querySelectorAll('.sode-ov__tile--destination')];
		const src = [...item('Medium grid').querySelectorAll('.sode-ov__tile--source')];
		return tiles('Small grid') === '4,2' && tiles('Medium grid') === '4,0,2,0' && !document.querySelector('.sode-ov__thumb--grid .sode-ov__count') &&
			dst.length === 2 && dst.every((x) => getComputedStyle(x).rotate === '-90deg') && dst[0].textContent === '2' && dst[0].getBoundingClientRect().top > dst[1].getBoundingClientRect().top &&
			src[0].getBoundingClientRect().left < src[1].getBoundingClientRect().left;
	})()`)
	b.waitFor("the large preview keeps each character in a square of its own in both bars, with a gap between neighbours and no shift", `(() => {
		const t = (k) => [...document.querySelectorAll('.sode-ov__big .sode-ov__tile--' + k)].map((x) => x.getBoundingClientRect());
		const src = t('source'), dst = t('destination');
		const apart = (r, a, b) => r.slice(1).every((x, i) => x[a] - r[i][b] > 1);
		const sq = (r) => r.every((x) => Math.abs(x.width - x.height) < 1.5);
		const cell = document.querySelector('.sode-ov__big .sode-ov__thumb--grid .sode-ov__k-on').getBoundingClientRect();
		const tiles = [...document.querySelectorAll('.sode-ov__big .sode-ov__tile')];
		return src.length === 3 && dst.length === 3 && apart(src, 'left', 'right') && apart(dst.slice().reverse(), 'top', 'bottom') && sq(src) && sq(dst) &&
			Math.abs(src[0].width - cell.width) < 1.5 && tiles.every((x) => getComputedStyle(x).translate === 'none');
	})()`)
	b.shot("panels-list")
	// One square size in every picture size: the list's pictures alike, and the large preview of a Large and a Small panel alike.
	b.waitFor("the list's Small, Medium and Large pictures have squares of one size, and the pictures grow with them", `(() => {
		const w = (name) => [...document.querySelectorAll('.sode-ov__item')].find((n) => n.textContent.includes(name)).querySelector('.sode-ov__thumb--grid > i').getBoundingClientRect().width;
		const item = (name) => [...document.querySelectorAll('.sode-ov__item')].find((n) => n.textContent.includes(name));
		const box = (name) => item(name).querySelector('.sode-ov__thumb--grid').getBoundingClientRect();
		const who = (name) => item(name).querySelector('.sode-ov__who').getBoundingClientRect().left;
		const big = document.querySelector('.sode-ov__item[data-id="`+id+`"] .sode-ov__thumb--grid > i').getBoundingClientRect().width;
		const small = w('Small grid'), medium = w('Medium grid');
		return small > 4 && Math.abs(small - big) < 1 && Math.abs(medium - big) < 1 && box('Small grid').width < box('Medium grid').width &&
			Math.abs(who('Small grid') - who('Medium grid')) < 1;
	})()`)
	b.eval(`window.__bigSq = document.querySelector('.sode-ov__big .sode-ov__thumb--grid > i').getBoundingClientRect().width; [...document.querySelectorAll('.sode-ov__item')].find((n) => n.textContent.includes('Small grid')).click(); true`)
	b.waitFor("the Small panel's large preview has the Large panel's square size", `(() => {
		const big = document.querySelector('.sode-ov__big .sode-ov__thumb--grid');
		return big?.dataset.size === 'small' && Math.abs(big.querySelector(':scope > i').getBoundingClientRect().width - window.__bigSq) < 1;
	})()`)
	b.check()
}
