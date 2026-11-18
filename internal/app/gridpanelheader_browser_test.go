package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Grid page's header buttons and kiosk mode at 1280 by 800. Ana can use Ben's panel but not change it. Its Title bar
// setting is off, but Settings, the Panels link and Copy show. Settings hides only in kiosk mode. Copy makes Ana's own copy and opens it in the editor. In kiosk mode on her own panel the header row goes, so
// the grid moves up, and a take's banner never moves it. Rules follow the Lock chip, the instance name and the Find box,
// and the banner is as tall as the Find box.
func TestChromeGridPanelHeader(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users = append(c.Users, mkUser(t, "Ben", "5678")) })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	ben := userKey(t, a, "Ben")
	bens := makePanel(t, a, ben, `{"name":"Ben grid","type":"grid","share":{"mode":"everyone"},"settings":{"mode":"take","levels":[1],"titleBar":false}}`)
	own := makePanel(t, a, ana, `{"name":"Ana grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + bens + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on') && !!`+gridXp(3, 3))
	b.waitFor("with no title bar, Settings, the Panels link and Copy show clear of the Lock chip, and not Edit", `(() => {
		const tools = document.querySelector('.sode-gridpanel__tools');
		const ids = [...tools.querySelectorAll('a, button')].filter((n) => n.getClientRects().length).map((n) => n.id || n.getAttribute('aria-label'));
		return document.querySelector('.sode-panels__head').hidden && !document.getElementById('panel-title').getClientRects().length &&
			ids.join() === 'Settings,Panels,Save a copy,panel-screen' &&
			[...tools.querySelectorAll('a, button')].filter((n) => n.getClientRects().length).every((n) => { const r = n.getBoundingClientRect(); return n.contains(document.elementFromPoint((r.left + r.right) / 2, (r.top + r.bottom) / 2)); });
	})()`)
	b.shot("header-copy")
	b.eval(`document.querySelector('.sode-gridpanel__copy').click(); true`)
	b.waitFor("Copy opens Ana's copy in the editor", `location.pathname === '/panels/editor' && /^\?panel=p-/.test(location.search)`)
	list := apiDo(a, "GET", "/api/v1/panels", ana, nil).Body.String()
	if !strings.Contains(list, `"name":"Ben grid copy"`) || !strings.Contains(list, `"owner":"Ana"`) {
		t.Errorf("after Copy, Ana's panels are %s, want Ana's own Ben grid copy", list)
	}

	b.open(srv.URL + "/panels/" + own + "?instance=Desk")
	b.waitFor("Ana's panel", `!!`+gridXp(3, 3)+` && !document.querySelector('.sode-gridpanel__edit').hidden && document.querySelector('.sode-gridpanel__copy').hidden`)
	b.waitFor("rules: after the Lock chip, after the instance name and after the Find box; the message box is as tall as the Find box", `(() => {
		const r = (s) => document.querySelector(s).getBoundingClientRect();
		const who = r('.sode-lock__who-name'), wr = r('.sode-gridpanel__whorule'), title = r('#panel-title');
		const inst = r('#panel-instance'), nr = r('.sode-gridpanel__namerule'), tools = r('.sode-gridpanel__tools');
		const mode = r('.sode-gridpanel__deck .sode-gridpanel__find-mode'), r3 = r('.sode-gridpanel__rule--3'), msg = r('.sode-gridpanel__msg');
		return wr.height > 0 && wr.left >= who.right && wr.right <= title.left &&
			nr.height > 0 && nr.left >= inst.right && nr.right <= tools.left &&
			r3.height > 0 && r3.left >= mode.right && r3.right <= msg.left &&
			Math.abs(msg.top - mode.top) < 0.5 && Math.abs(msg.bottom - mode.bottom) < 0.5;
	})()`)
	// The reload ends the page's script, so the sizes before kiosk mode go in the check's words.
	was := b.eval(`document.querySelector('.sode-gridpanel__deck').getBoundingClientRect().height + ',' + document.querySelector('.sode-gridpanel__headrow').getBoundingClientRect().height`).(string)
	b.eval(`location.hash = 'kiosk=1'; location.reload(); true`)
	b.waitFor("kiosk mode", `document.documentElement.hasAttribute('data-sode-kiosk') && !!`+gridXp(3, 3))
	b.waitFor("in kiosk mode the header row, Settings and the Panels link go, and the grid moves up by the row", `(() => {
		const deck = document.querySelector('.sode-gridpanel__deck').getBoundingClientRect(), sc = document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect();
		const gone = ['.sode-gridpanel__headrow', '.sode-gridpanel__settings', '.sode-panels__home'].every((s) => !document.querySelector(s).getClientRects().length || !document.querySelector(s).getBoundingClientRect().height);
		const [d, row] = [`+was+`];
		return gone && deck.top === 0 && d - deck.height >= row && sc.top - deck.bottom < 2;
	})()`)
	b.eval(`window.__top = () => [document.querySelector('.sode-gridpanel__scroll'), document.querySelector('.sode-gridpanel__deck')].map((n) => Math.round(n.getBoundingClientRect().top) + ':' + Math.round(n.getBoundingClientRect().height)).join('|'); window.__was = window.__top(); true`)
	b.eval(gridXp(3, 3) + `.click(); true`)
	gridKey(b, "Enter")
	gridConfirm(b)
	b.waitFor("in kiosk mode the take's banner shows and the grid did not move", gridMessage+`.startsWith('Taken') && window.__top() === window.__was`)
	b.waitFor("the banner fills the message box, top to bottom", `(() => {
		const m = document.querySelector('.sode-gridpanel__msg').getBoundingClientRect();
		const al = document.querySelector('.sode-gridpanel__msg .sode__banner:not(.sode__banner--out) .pb-alert').getBoundingClientRect();
		return al.top - m.top < 1 && m.bottom - al.bottom < 8;
	})()`)
	b.shot("header-kiosk")
	b.check()
}
