package app

import (
	"fmt"
	"testing"
)

// A saved change and a Default artwork change reach an open Grid panel by the usual live update. With nothing staged
// it applies after the countdown and keeps the search, the picks and the cursor on the same items. With a route
// staged it waits until Apply now, which drops the staged route and keeps the search. The banner shows in the gap
// between the search boxes and Clear, with the main page's countdown bar, and the grid never moves. A take result
// shows over it, and a new update notice takes the place of the result.
func TestChromeGridPanelLiveUpdate(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	body := func(name, dsts string) string {
		return fmt.Sprintf(`{"name":%q,"type":"grid","settings":{"mode":"take","levels":[1]},"sources":{"members":"all"},"destinations":{"members":%s}}`, name, dsts)
	}
	id := makePanel(t, a, ana, body("Before", `"all"`))
	save := func(name, dsts string) {
		t.Helper()
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body(name, dsts)); rec.Code != 200 {
			t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
		}
	}
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(3, 2)+` !== null`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)
	b.eval(`window.__top = () => [document.querySelector('.sode-gridpanel__scroll'), document.querySelector('.sode-gridpanel__deck')].map((n) => Math.round(n.getBoundingClientRect().top) + ':' + Math.round(n.getBoundingClientRect().height)).join('|'); window.__was = window.__top(); true`)
	still := `window.__top() === window.__was`
	// The banner's words and buttons show whole inside the message box at 1280 by 800, the browser's size here.
	whole := `(() => { const u = document.getElementById('panel-update'), m = u.parentNode.getBoundingClientRect(); const cut = (n) => n.scrollWidth > n.clientWidth + 1; ` +
		`const inside = [...u.querySelectorAll('.pb-alert, .pb-btn')].every((n) => { const r = n.getBoundingClientRect(); return r.top >= m.top - 0.5 && r.bottom <= m.bottom + 0.5 && r.left >= m.left - 0.5 && r.right <= m.right + 0.5; }); ` +
		`return innerWidth === 1280 && inside && !cut(u.querySelector('.pb-alert')) && !cut(u.querySelector('.sode__banner-body')); })()`

	// CAM 1 is source 12, the first match. Two rows down is destination 1.
	realKey(b, "s", "KeyS", 83)
	b.call("Input.insertText", map[string]any{"text": "cam 1"})
	gridCursorOn(b, "Find moves the cursor to the first match", -1, 12)
	b.eval(`document.activeElement.blur(); true`)
	realKey(b, "j", "KeyJ", 74)
	realKey(b, "j", "KeyJ", 74)
	gridCursorOn(b, "the cursor on destination 1", 1, 12)
	dst := func(n int) string {
		return fmt.Sprintf(`document.querySelector('.sode-gridpanel__dst[data-dst="%d"]:not([hidden])')`, n)
	}
	b.eval(dst(3) + `.click(); true`)
	b.waitFor("destination 3 is picked", dst(3)+`.dataset.picked === ''`)

	kept := func(what string, i int) {
		t.Helper()
		b.waitFor(what+": the search box keeps its text and mode", gridFindBox("srcs")+`.value === 'cam 1' && document.querySelector('.sode-gridpanel__find[data-kind="srcs"]').dataset.mode === 'find'`)
		b.waitFor(what+": Find still marks the match", gridFound("srcs")+`.split(',').includes('12')`)
		gridCursorOn(b, what+": the cursor stays on destination 1 and CAM 1", i, 12)
	}

	// Destination 0 leaves the panel, so destination 1 moves up a row.
	save("After", `[1,2,3,4,5,6]`)
	text := `document.getElementById('panel-update-text').textContent`
	b.waitFor("the countdown banner", `/Refreshing in \d+ s\./.test(`+text+`)`)
	b.waitFor("the banner is in the gap between the search boxes and Clear, whole, with its countdown bar, and the grid did not move", `(() => {
		const u = document.getElementById('panel-update'), r = u.getBoundingClientRect();
		const f = document.querySelector('.sode-gridpanel__deck .sode-gridpanel__find-mode').getBoundingClientRect(), c = document.querySelector('.sode-gridpanel__clear').getBoundingClientRect();
		const bar = u.querySelector('#panel-update-bar .sode__banner-bar');
		return u.parentNode.classList.contains('sode-gridpanel__msg') && r.left > f.right && r.right < c.left &&
			!!bar && getComputedStyle(bar).animationName === 'sode-countdown' && bar.getClientRects().length > 0;
	})() && `+whole+` && `+still)
	b.shot("banner")
	b.waitFor("the old copy shows until the countdown ends", `document.getElementById('panel-title').textContent === 'Before'`)
	b.waitFor("the new copy applies, and the grid did not move", `document.getElementById('panel-title').textContent === 'After' && document.getElementById('panel-update').hidden && `+still)
	b.waitFor("the new members show", `getComputedStyle(document.querySelector('.sode-gridpanel__field')).getPropertyValue('--sode-rows').trim() === '6'`)
	kept("a saved change", 0)
	b.waitFor("the pick stays", dst(3)+`.dataset.picked === ''`)

	// A Default artwork change moves no rev but is an update too.
	n := 12
	if _, err := a.artDefaults.set(ArtDefault{Kind: artKindSource, N: &n, Art: "icon:tv"}, nil); err != nil {
		t.Fatal(err)
	}
	b.waitFor("the countdown banner for it", `!document.getElementById('panel-update').hidden`)
	b.waitFor("the Default artwork reaches CAM 1", `document.querySelector('.sode-gridpanel__src[data-src="12"]:not([hidden]) .sode-gridpanel__art:not([hidden]) use')?.getAttribute('href') === '#pb-i-tv'`)
	kept("a Default artwork change", 0)
	b.waitFor("the pick stays", dst(3)+`.dataset.picked === ''`)
	b.shot("after-updates")

	// Esc drops the pick first, so the press stages one route.
	gridKey(b, "Escape")
	b.waitFor("the pick goes", `!document.querySelector('.sode-gridpanel__dst[data-picked]:not([hidden])')`)
	b.waitFor("crosspoint CAM 1, destination 2", gridXp(12, 2)+` !== null && !`+gridXp(12, 2)+`.hidden`)
	b.eval(gridXp(12, 2) + `.click(); true`)
	b.waitFor("a route staged", gridXp(12, 2)+`.dataset.s === 'staged'`)
	b.waitFor("the Take button is on", `document.getElementById('panel-take').textContent === 'Take' && !document.getElementById('panel-take').disabled`)
	save("Third", `[1,2,3,4,5,6]`)
	// The poll and the countdown together come near one wait's limit, so each gets its own.
	b.waitFor("the countdown banner for the third copy", `/Refreshing in \d+ s\./.test(`+text+`)`)
	b.waitFor("the banner waits for the staged route, its longest line whole, and the grid did not move", `/take or clear/.test(`+text+`) && `+whole+` && `+still)
	b.waitFor("the staged route and the old copy stay", `document.getElementById('panel-title').textContent === 'After' && `+gridXp(12, 2)+`.dataset.s === 'staged'`)
	b.eval(`document.getElementById('panel-apply').click(); true`)
	b.waitFor("Apply now applies and drops the staged route", `document.getElementById('panel-title').textContent === 'Third' && !document.querySelector('.sode-gridpanel__xp[data-s="staged"]:not([hidden])') && document.getElementById('panel-staged').hidden`)
	b.waitFor("the banner goes, and the grid did not move", `document.getElementById('panel-update').hidden && `+still)
	b.waitFor("the Take button is off", `document.getElementById('panel-take').textContent === 'Take' && document.getElementById('panel-take').disabled`)
	kept("Apply now", 0)

	// A take's result is the main page's banner in the same box, with its countdown, and the grid does not move.
	b.eval(gridXp(12, 2) + `.click(); true`)
	gridKey(b, "Enter")
	gridConfirm(b)
	b.waitFor("the take's result shows in the box with the main page's countdown bar, and the grid did not move", gridMessage+`.startsWith('Taken') && (() => {
		const n = document.querySelector('.sode-gridpanel__msg .sode__banner'), bar = n.querySelector('.sode__banner-bar');
		return !!n.querySelector('.pb-alert--ok') && !!n.querySelector('.sode__banner-x') && !!bar && getComputedStyle(bar).animationName === 'sode-countdown';
	})() && `+still)
	b.shot("result")
	b.check()
}
