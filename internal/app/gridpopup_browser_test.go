package app

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// popTyped types text into a patch box and takes the best match with Tab.
func popTyped(b *tab, id, text string) {
	b.t.Helper()
	b.eval(`document.getElementById('` + id + `').focus(); true`)
	b.call("Input.insertText", map[string]any{"text": text})
	b.waitFor(id+" lists matches", `!document.getElementById('`+id+`-menu').hidden`)
	realKey(b, "Tab", "Tab", 9)
}

// lastGroup waits for the newest Sode Group over REST to have n routes, and returns its first entry's source.
func lastGroup(t *testing.T, a *App, key string, n int) {
	t.Helper()
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		rec := apiDo(a, "GET", "/api/v1/history/groups/last", key, nil)
		if rec.Code == 200 {
			body := apiBody(t, rec)
			if es, _ := body["entries"].([]any); len(es) == n {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("the last Group never had %d routes: %.400s", n, rec.Body.String())
		}
	}
}

// The Grid popup on the main page. Ctrl+e opens it centred on the patch boxes. It is the grid and its search band, with
// no Levels box, Take or Clear. A Diagonal stages four routes that stay in the staged list, and the crosshair fills the
// boxes from a row. One confirm sends one Group, and Ctrl+Z prepares its undo. The main Take sends the staged routes
// with the boxes' route, at the main page's levels.
func TestChromeMainGridPopup(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("17 level picks", `document.querySelectorAll('#lvl-pick > button').length === 17`)

	shown := `!document.getElementById('grid-pop')?.hidden`
	cursor := `(() => { const c = document.querySelector('#grid-pop .sode-gridpanel__cursor'); return c ? c.dataset.i + ',' + c.dataset.j : ''; })()`

	// A valid pair in the boxes: the popup opens with the cursor on that crosspoint.
	popTyped(b, "src", "CAM 1")
	popTyped(b, "dst", "DSM1")
	b.waitFor("the boxes hold the pair", `document.getElementById('src').value.endsWith('CAM 1') && document.getElementById('dst').value.endsWith('DSM1')`)
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("Ctrl+e opens the Grid popup", shown)
	b.waitFor("the grid draws", `!!document.querySelector('#grid-pop .sode-gridpanel__xp:not([hidden])')`)
	// DSM1 is destination 2 (router numbers from 0). Every item is in the Rights and none is pinned, so a display index
	// is the router number.
	b.waitFor("the cursor is on DSM1", cursor+`.startsWith('2,')`)
	b.waitFor("and on CAM 1", `(() => { const j = `+cursor+`.split(',')[1]; return document.querySelector('#grid-pop .sode-gridpanel__src[data-src="' + j + '"]')?.textContent.trim() === 'CAM 1'; })()`)
	b.waitFor("the popup fills the window but for a small margin", `(() => { const r = document.querySelector('.sode-gridpop__box').getBoundingClientRect(); return r.width > innerWidth * 0.9 && r.right < innerWidth && r.height > innerHeight * 0.9 && r.bottom < innerHeight; })()`)
	b.waitFor("no title, and the dialog keeps its name", `!document.querySelector('#grid-pop .pb-modal__title') && document.querySelector('#grid-pop [role=dialog]').getAttribute('aria-label') === 'Grid'`)
	b.waitFor("no Levels box, Take or Clear", `!document.getElementById('panel-levels').offsetParent && !document.getElementById('panel-take')?.offsetParent && !document.getElementById('panel-clear')?.offsetParent`)
	b.waitFor("the search boxes are side by side in the band, out of the corner", `(() => {
		const f = (k) => document.querySelector('#grid-pop .sode-gridpop__band .sode-gridpanel__find[data-kind="' + k + '"]')?.getBoundingClientRect();
		const s = f('srcs'), d = f('dsts');
		return !!s && !!d && Math.abs(s.top - d.top) < 1 && d.left >= s.right && s.width > 200 && !document.querySelector('#grid-pop .sode-gridpanel__head .sode-gridpanel__find');
	})()`)
	b.waitFor("the corner shows its whole Destinations word", `(() => { const w = document.querySelector('#grid-pop .sode-gridpanel__label--dst'); return !!w && w.scrollWidth <= w.clientWidth + 1; })()`)

	// A 4 by 4 Diagonal: sources 1 to 4 picked, then a press on source 1's crosspoint with destination 5.
	click := func(sel, mods string) {
		b.eval(sel + `.dispatchEvent(new MouseEvent('click', { bubbles: true, ` + mods + ` })); true`)
	}
	name := func(n int) string {
		return fmt.Sprintf(`document.querySelector('#grid-pop .sode-gridpanel__src[data-src="%d"]')`, n)
	}
	gridShow(b, 1, 5)
	click(name(1), "")
	click(name(4), "shiftKey: true")
	b.waitFor("the range is picked", name(4)+`.dataset.picked === ''`)
	b.eval(gridXp(1, 5) + `.click()`)
	staged := func(s, d int) string { return gridXp(s, d) + `?.dataset.s === 'staged'` }
	b.waitFor("the press stages the Diagonal", staged(1, 5)+` && `+staged(2, 6)+` && `+staged(3, 7)+` && `+staged(4, 8))

	// Esc drops the picks first, then closes; the staged routes stay in the list under the boxes.
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("the first Esc drops the picks", `!document.querySelector('#grid-pop .sode-gridpanel__src[data-picked]') && `+shown)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("the next Esc closes the popup", `document.getElementById('grid-pop').hidden`)
	b.waitFor("the staged list shows 4 rows", `!document.getElementById('grid-staged').hidden && document.querySelectorAll('#grid-staged-list li').length === 4`)
	if got := b.eval(`document.getElementById('grid-staged-count').textContent`); got != "4 routes" {
		t.Fatalf("the list's count says %q", got)
	}

	// Leaving the popup puts focus back in the patch boxes. So the next Enter is the one Take: it confirms the staged routes
	// and the boxes' route together.
	b.waitFor("focus is back in a patch box", `['src', 'dst', 'lvl'].includes(document.activeElement.id)`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("the confirm opens", `!document.getElementById('confirm').hidden`)
	if body := b.eval(`document.getElementById('confirm-body').textContent`).(string); !strings.Contains(body, "5 destinations") || !strings.Contains(body, "DSM1") {
		t.Fatalf("Enter in the boxes confirms %q, want the 4 staged routes and the boxes", body)
	}
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	b.waitFor("the list stays", `document.querySelectorAll('#grid-staged-list li').length === 4`)

	// The crosshair fills the boxes from the second row and opens nothing. Ctrl+e then finds the boxes on the grid.
	b.eval(`document.querySelectorAll('#grid-staged-list .sode__staged-find')[1].click(); true`)
	b.waitFor("the boxes hold the row", `document.getElementById('src').value === document.querySelector('#grid-staged-list li:nth-child(2) .sode__end--src').textContent && document.getElementById('dst').value !== ''`)
	if b.eval(shown).(bool) {
		t.Fatal("the crosshair opened the popup")
	}
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("Ctrl+e opens the popup", shown)
	b.waitFor("the cursor is on the row's crosspoint", cursor+` === '6,2'`)
	b.waitFor("reopened, the routes still show as staged", staged(2, 6))
	b.waitFor("a help line, and the staged badge at the right next to Close", `(() => {
		const help = document.querySelector('#grid-pop .sode-gridpop__help'), n = document.getElementById('panel-staged'), x = document.querySelector('#grid-pop .sode-gridpop__close');
		const h = help.getBoundingClientRect(), r = n.getBoundingClientRect(), c = x.getBoundingClientRect();
		return /Esc or Ctrl\+e closes the grid/.test(help.textContent) && h.right <= r.left && r.right <= c.left && c.left - r.right < 40;
	})()`)

	// One confirm sends one take: History shows one Group of 4 routes. The boxes hold one of them, so it goes once.
	// The Result shows on the main page, never in the popup.
	b.eval(`document.querySelector('#grid-pop .sode-gridpanel__scroll').focus(); true`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter opens the main page's confirm for 4 destinations", `!document.getElementById('confirm').hidden && document.getElementById('confirm-body').textContent.includes('4 destinations')`)
	gridConfirm(b)
	for s := 1; s <= 4; s++ {
		tallyHas(t, a, ana, s+4, s)
	}
	lastGroup(t, a, ana, 4)
	b.waitFor("the main page shows the Result", `/Success/.test(document.getElementById('result').textContent)`)
	if b.eval(`!!document.querySelector('#grid-pop .sode-gridpop__msg') || /Success/.test(document.getElementById('grid-pop').textContent)`).(bool) {
		t.Fatal("the popup shows the take result")
	}
	b.waitFor("the staged list goes", `document.getElementById('grid-staged').hidden && !document.querySelector('#grid-pop .sode-gridpanel__xp[data-s="staged"]:not([hidden])')`)
	b.waitFor("the boxes clear, as after a take", `!document.getElementById('src').value && !document.getElementById('dst').value`)

	// Ctrl+Z in the popup prepares the undo of the whole Group.
	realKeyMods(b, "z", "KeyZ", 90, modCtrl)
	b.waitFor("the undo confirm covers the Group", `!document.getElementById('confirm').hidden && /undo/i.test(document.getElementById('confirm-title').textContent) && document.getElementById('confirm-body').textContent.includes('4 destinations')`)
	gridConfirm(b)
	for s := 1; s <= 4; s++ {
		tallyHas(t, a, ana, s+4, s+4)
	}

	// The main page's Clear drops the staged routes; its Take sends them.
	gridShow(b, 9, 9)
	b.eval(gridXp(9, 9) + `.click()`)
	b.waitFor("one route staged", staged(9, 9))
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("Ctrl+e closes the popup", `document.getElementById('grid-pop').hidden`)
	b.waitFor("the list has the route", `document.getElementById('grid-staged-count').textContent === '1 route'`)
	b.eval(`document.getElementById('clear-all').click(); true`)
	b.waitFor("Clear empties the list", `document.getElementById('grid-staged').hidden && document.getElementById('clear-all').disabled`)
	b.eval(`document.getElementById('grid-open').click(); true`)
	b.waitFor("the icon bar's Grid button opens the popup", shown)
	gridShow(b, 9, 9)
	b.eval(gridXp(9, 9) + `.click()`)
	b.waitFor("staged again", staged(9, 9))
	b.eval(`document.querySelector('#grid-pop .sode-gridpop__close').click(); true`)
	b.waitFor("Close closes the popup", `document.getElementById('grid-pop').hidden`)
	b.eval(`document.getElementById('patch').click(); true`)
	gridConfirm(b)
	tallyHas(t, a, ana, 9, 9)
	b.waitFor("Take empties the list", `document.getElementById('grid-staged').hidden`)

	// With no valid pair, the popup comes back as it was left: its search and scroll.
	b.eval(`document.getElementById('grid-open').click(); true`)
	b.waitFor("open", shown)
	b.eval(`document.querySelector('#grid-pop .sode-gridpanel__find-input').focus(); true`)
	b.call("Input.insertText", map[string]any{"text": "src 1"})
	b.waitFor("the search box holds the text", `document.querySelector('#grid-pop .sode-gridpanel__find-input').value === 'src 1'`)
	b.eval(`document.querySelector('#grid-pop .sode-gridpanel__scroll').focus(); true`)
	left := b.eval(`new Promise((r) => setTimeout(() => r(document.querySelector('#grid-pop .sode-gridpanel__scroll').scrollLeft), 1600))`).(float64)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", `document.getElementById('grid-pop').hidden`)
	for _, id := range []string{"src", "dst"} {
		b.eval(`document.getElementById('` + id + `').focus(); true`)
		realKey(b, "Escape", "Escape", 27)
	}
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("the boxes are empty", `!document.getElementById('src').value && !document.getElementById('dst').value && document.getElementById('dst-chips').hidden`)
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("reopened", shown)
	b.waitFor("the search comes back", `document.querySelector('#grid-pop .sode-gridpanel__find-input').value === 'src 1'`)
	if got := b.eval(`document.querySelector('#grid-pop .sode-gridpanel__scroll').scrollLeft`).(float64); got != left {
		t.Errorf("reopened at scrollLeft %v, want %v as left", got, left)
	}
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed again", `document.getElementById('grid-pop').hidden`)

	// With the boxes empty, a press stages at the main page's levels and the main Take sends it.
	b.eval(`(() => { const i = document.getElementById('lvl'); i.focus(); i.value = '3'; i.dispatchEvent(new Event('input', { bubbles: true })); i.blur(); return true; })()`)
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("reopened for the main Take", shown)
	gridShow(b, 11, 11)
	b.eval(gridXp(11, 11) + `.click()`)
	b.waitFor("staged at level 3", staged(11, 11))
	b.eval(`document.querySelector('#grid-pop .sode-gridpop__close').click(); true`)
	b.waitFor("the staged row has only level 3", `document.querySelector('#grid-staged-list .sode__bar')?.getAttribute('aria-label') === 'Levels 3'`)
	b.eval(`document.getElementById('patch').click(); true`)
	gridConfirm(b)
	b.waitFor("the main Take empties the staged list", `document.getElementById('grid-staged').hidden`)
	b.check()

	// The phone layout: a button opens the popup over the whole window.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/")
	b.waitFor("the phone layout", `document.documentElement.dataset.pbLayout === 'phone' && !!document.getElementById('m-grid').offsetParent`)
	b.waitFor("names", `document.getElementById('m-ws-dot').classList.contains('pb-dot--ok')`)
	b.eval(`document.getElementById('m-grid').click(); true`)
	b.waitFor("the button opens the popup full screen", shown+` && (() => { const r = document.querySelector('.sode-gridpop__box').getBoundingClientRect(), v = visualViewport; return r.left <= 0 && r.top <= 0 && r.width >= v.width && r.height >= v.height; })()`)
	b.check()
}

// The shield on a Grid popup destination name sets Sode's Protect through the Confirm, then offers Clear Protect.
func TestChromeMainGridPopupShield(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.ProtectDevice = 126
	})
	srv := seamServer(t, a)
	key := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, key)
	sodeRows := func() int {
		return strings.Count(apiDo(a, "GET", "/api/v1/protected", key, nil).Body.String(), `"sodeUser":"Ana"`)
	}
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("Settings button", `!!document.getElementById('settings-open')`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)

	b.eval(`document.getElementById('grid-open').click(); true`)
	b.waitFor("the popup", `!document.getElementById('grid-pop').hidden && !!document.querySelector('#grid-pop .sode-gridpanel__dst[data-dst="1"]:not([hidden])')`)
	name := `document.querySelector('#grid-pop .sode-gridpanel__dst[data-dst="1"]:not([hidden])')`
	hover := func() {
		r := b.eval(`(() => { const r = ` + name + `.getBoundingClientRect(); return [r.left + 10, r.top + r.height / 2]; })()`).([]any)
		b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": r[0], "y": r[1]})
	}
	hover()
	b.waitFor("the shield offers Protect", `document.querySelector('.sode-gridpop__shield')?.dataset.protect === 'set' && !document.querySelector('.sode-gridpop__shield').hidden`)
	b.eval(`document.querySelector('.sode-gridpop__shield').click(); true`)
	b.waitFor("Confirm protect", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm protect'`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	for deadline := time.Now().Add(10 * time.Second); sodeRows() != 17; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("Sode's Protect on %d levels, want 17", sodeRows())
		}
	}
	b.waitFor("the name shows Protected", name+`.classList.contains('pb-xpt--protected')`)
	hover()
	b.waitFor("the shield offers Clear Protect", `document.querySelector('.sode-gridpop__shield')?.dataset.protect === 'clear'`)
	b.eval(`document.querySelector('.sode-gridpop__shield').click(); true`)
	b.waitFor("Confirm clear protect", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm clear protect'`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	for deadline := time.Now().Add(10 * time.Second); sodeRows() != 0; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("Sode's Protect left on %d levels", sodeRows())
		}
	}
	b.check()
}
