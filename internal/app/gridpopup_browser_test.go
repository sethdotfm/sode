package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
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

// The Grid popup on the main page. Ctrl+e opens it centred on the patch boxes. A Diagonal stages four routes that stay
// in the staged list, and the crosshair finds a row. One confirm sends one Group, and Ctrl+Z prepares its undo.
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
	b.waitFor("the popup is about 80% of the window", `(() => { const r = document.querySelector('.sode-gridpop__box').getBoundingClientRect(); return r.width > innerWidth * 0.7 && r.width < innerWidth * 0.9 && r.height > innerHeight * 0.7; })()`)
	b.waitFor("the Levels box is there", `!document.getElementById('panel-levels').hidden`)

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
	if got := b.eval(`document.getElementById('grid-staged-take').textContent`); got != "Take 4 routes" {
		t.Fatalf("the list's Take says %q", got)
	}

	// Enter in the boxes sends only what the boxes hold.
	b.eval(`document.getElementById('dst').focus(); true`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("the confirm opens", `!document.getElementById('confirm').hidden`)
	if body := b.eval(`document.getElementById('confirm-body').textContent`).(string); strings.Contains(body, "destinations") || !strings.Contains(body, "DSM1") {
		t.Fatalf("Enter in the boxes confirms %q, want only the boxes", body)
	}
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	b.waitFor("the list stays", `document.querySelectorAll('#grid-staged-list li').length === 4`)

	// The crosshair fills the boxes from the second row and opens the grid on it.
	b.eval(`document.querySelectorAll('#grid-staged-list .sode__staged-find')[1].click(); true`)
	b.waitFor("the crosshair opens the popup", shown)
	b.waitFor("the boxes hold the row", `document.getElementById('src').value === document.querySelector('#grid-staged-list li:nth-child(2) .sode__end--src').textContent && document.getElementById('dst').value !== ''`)
	b.waitFor("the cursor is on the row's crosspoint", cursor+` === '6,2'`)
	b.waitFor("reopened, the routes still show as staged", staged(2, 6))

	// One confirm sends one take: History shows one Group of 4 routes. The popup banner shows the Result.
	b.eval(`document.querySelector('#grid-pop .sode-gridpanel__scroll').focus(); true`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter opens the main page's confirm for 4 destinations", `!document.getElementById('confirm').hidden && document.getElementById('confirm-body').textContent.includes('4 destinations')`)
	gridConfirm(b)
	for s := 1; s <= 4; s++ {
		tallyHas(t, a, ana, s+4, s)
	}
	lastGroup(t, a, ana, 4)
	b.waitFor("the popup banner shows the Result", `/Success/.test(document.querySelector('.sode-gridpop__msg')?.textContent || '')`)
	b.waitFor("the staged list goes", `document.getElementById('grid-staged').hidden && !document.querySelector('#grid-pop .sode-gridpanel__xp[data-s="staged"]:not([hidden])')`)

	// Ctrl+Z in the popup prepares the undo of the whole Group.
	realKeyMods(b, "z", "KeyZ", 90, modCtrl)
	b.waitFor("the undo confirm covers the Group", `!document.getElementById('confirm').hidden && /undo/i.test(document.getElementById('confirm-title').textContent) && document.getElementById('confirm-body').textContent.includes('4 destinations')`)
	gridConfirm(b)
	for s := 1; s <= 4; s++ {
		tallyHas(t, a, ana, s+4, s+4)
	}

	// The list's own Take and Clear.
	gridShow(b, 9, 9)
	b.eval(gridXp(9, 9) + `.click()`)
	b.waitFor("one route staged", staged(9, 9))
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("Ctrl+e closes the popup", `document.getElementById('grid-pop').hidden`)
	b.waitFor("the list has the route", `document.getElementById('grid-staged-take').textContent === 'Take 1 route'`)
	b.eval(`document.getElementById('grid-staged-clear').click(); true`)
	b.waitFor("Clear empties the list", `document.getElementById('grid-staged').hidden`)
	b.eval(`document.getElementById('grid-open').click(); true`)
	b.waitFor("the Grid button opens the popup", shown)
	// The boxes still hold the crosshair's pair, so the popup opened on it.
	b.waitFor("the cursor is on the boxes' pair", cursor+` === '6,2'`)
	gridShow(b, 9, 9)
	b.eval(gridXp(9, 9) + `.click()`)
	b.waitFor("staged again", staged(9, 9))
	b.eval(`document.querySelector('#grid-pop .sode-gridpop__head button').click(); true`)
	b.waitFor("Close closes the popup", `document.getElementById('grid-pop').hidden`)
	b.eval(`document.getElementById('grid-staged-take').click(); true`)
	gridConfirm(b)
	tallyHas(t, a, ana, 9, 9)
	b.waitFor("the list's Take empties the list", `document.getElementById('grid-staged').hidden`)

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
