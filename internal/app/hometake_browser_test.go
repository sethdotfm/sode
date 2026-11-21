package app

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
	"testing"
)

// The Home page's one Take and one Clear with the Grid popup's staged routes. Ctrl+e finds one filled box on the grid.
// A staged route's chips fill the boxes. Esc closes the popup only, then clears the boxes and the staged routes, and
// with nothing to clear does nothing. Take sends the staged routes and the boxes' route in one Confirm, and Clear drops them all.
// Take counts its routes at one width. Only a whole route in the boxes makes Last. Ctrl+e and the Grid button pick typed
// text first.
func TestChromeMainOneTake(t *testing.T) {
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
	hidden := `document.getElementById('grid-pop').hidden`
	cursor := `(() => { const c = document.querySelector('#grid-pop .sode-gridpanel__cursor'); return c ? c.dataset.i + ',' + c.dataset.j : ''; })()`
	scroll := `document.querySelector('#grid-pop .sode-gridpanel__scroll')`
	esc := func(id string) {
		b.eval(`document.getElementById('` + id + `').focus(); true`)
		realKey(b, "Escape", "Escape", 27)
	}
	clearOff := `document.getElementById('clear-all').disabled`
	stage := func(pairs ...[2]int) {
		b.eval(`document.getElementById('grid-open').click(); true`)
		b.waitFor("the popup", shown+` && !!document.querySelector('#grid-pop .sode-gridpanel__xp:not([hidden])')`)
		for _, p := range pairs {
			gridShow(b, p[0], p[1])
			b.eval(gridXp(p[0], p[1]) + `.click()`)
			b.waitFor("staged", gridXp(p[0], p[1])+`?.dataset.s === 'staged'`)
		}
		b.eval(`document.querySelector('#grid-pop .sode-gridpop__close').click(); true`)
		b.waitFor("closed", hidden)
		b.waitFor("the staged list", `document.querySelectorAll('#grid-staged-list li').length === `+strconv.Itoa(len(pairs)))
	}
	b.waitFor("nothing to clear", clearOff)
	take := `document.getElementById('patch').textContent`
	takeWidth := `document.getElementById('patch').getBoundingClientRect().width`
	lastText := func(k string) string {
		return `(document.querySelector('[data-last="` + k + `"] .sode__last:not(.sode__next) .sode__last-text')?.textContent || '')`
	}
	if w := b.eval(take).(string); w != "Take" {
		t.Fatalf("Take with nothing staged says %q", w)
	}
	width := b.eval(takeWidth).(float64)

	// Only the Source box: Ctrl+e opens the popup on that source, far to the right.
	popTyped(b, "src", "SRC 150")
	b.waitFor("Source holds it", `document.getElementById('src').value.endsWith('SRC 150')`)
	b.waitFor("Clear can clear", `!`+clearOff)
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the popup", shown)
	b.waitFor("the cursor is on the source", cursor+` === '-1,149'`)
	b.waitFor("the grid scrolls to it", scroll+`.scrollLeft > 0`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("the first Esc closes the popup only", hidden+` && document.getElementById('src').value.endsWith('SRC 150')`)

	// Only the Destination box, on a reopen: it scrolls down to that destination.
	esc("src")
	b.waitFor("Esc clears the box", `!document.getElementById('src').value && `+clearOff)
	popTyped(b, "dst", "100")
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the popup", shown)
	b.waitFor("the cursor is on the destination", cursor+` === '99,-1'`)
	b.waitFor("the grid scrolls to it", scroll+`.scrollTop > 0`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", hidden)

	// A staged route's chips fill the boxes, as a History row's do.
	stage([2]int{3, 5}, [2]int{4, 6})
	row := func(n int, end string) string {
		return `document.querySelector('#grid-staged-list li:nth-child(` + strconv.Itoa(n) + `) .sode__end--` + end + `')`
	}
	b.eval(row(1, "src") + `.click(); true`)
	b.waitFor("the source chip fills Source", `document.getElementById('src').value === `+row(1, "src")+`.textContent`)
	b.eval(row(2, "dst") + `.click(); true`)
	b.waitFor("the destination chip fills Destination", `document.getElementById('dst').value === `+row(2, "dst")+`.textContent`)
	if b.eval(shown).(bool) {
		t.Fatal("a chip opened the popup")
	}

	// One Esc clears the boxes and the staged routes.
	esc("src")
	b.waitFor("Esc clears everything", `!document.getElementById('src').value && !document.getElementById('dst').value && document.getElementById('grid-staged').hidden && `+clearOff)

	// One Take: the staged routes and the boxes' route in one Confirm, sent as one Group. Take counts the routes it
	// sends, and its width does not change. Partial boxes add nothing to the count.
	stage([2]int{3, 5}, [2]int{4, 6})
	b.waitFor("Take counts the staged routes", take+` === 'Take 2'`)
	popTyped(b, "src", "CAM 1")
	b.waitFor("a lone Source adds no route", take+` === 'Take 2'`)
	popTyped(b, "dst", "DSM1")
	b.waitFor("Take counts the boxes' route", take+` === 'Take 3'`)
	if w := b.eval(takeWidth).(float64); w != width {
		t.Fatalf("Take is %vpx wide with a count, %vpx without", w, width)
	}
	b.eval(`document.getElementById('patch').click(); true`)
	b.waitFor("one confirm", `!document.getElementById('confirm').hidden`)
	if body := b.eval(`document.getElementById('confirm-body').textContent`).(string); !strings.Contains(body, "3 destinations") || !strings.Contains(body, "DSM1") {
		t.Fatalf("the confirm says %q, want the 2 staged routes and the boxes' route", body)
	}
	// The Confirm keeps the boxes' route apart from the staged routes, which each say Staged, and names every
	// destination in its question.
	parts := b.eval(`(() => {
		const body = document.getElementById('confirm-body');
		const items = [...body.querySelectorAll('.sode__plan-item')].map((s) => [s.querySelector('.sode__plan-head strong').textContent, s.querySelector('.sode__plan-staged')?.textContent || '']);
		const labels = [...body.children].filter((n) => n.classList.contains('sode__label')).map((n) => n.textContent);
		return JSON.stringify({ q: body.firstElementChild.textContent, items, labels });
	})()`).(string)
	if !strings.Contains(parts, `"labels":["Patch boxes","Staged routes"]`) || !strings.Contains(parts, `DSM1",""],[`) || strings.Count(parts, `"Staged"]`) != 2 {
		t.Fatalf("the confirm parts are %s, want the boxes' route first with no badge, then 2 routes marked Staged", parts)
	}
	if q := b.eval(`document.getElementById('confirm-body').firstElementChild.textContent`).(string); !strings.Contains(q, "DSM1") || strings.Count(q, ",")+strings.Count(q, " and ") < 2 {
		t.Fatalf("the question is %q, want each destination by name", q)
	}
	// The Confirm's Take has the one Confirm button height (3rem): lower than the main Take, higher than Cancel. The
	// dialog has a fixed place: a shorter plan later must not move it.
	heights := `(() => {
		const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
		const h = (id) => document.getElementById(id).getBoundingClientRect().height / rem;
		return JSON.stringify({ ok: h('confirm-ok'), take: h('patch'), cancel: h('confirm-cancel') });
	})()`
	var hs struct{ OK, Take, Cancel float64 }
	if err := json.Unmarshal([]byte(b.eval(heights).(string)), &hs); err != nil || math.Abs(hs.OK-3) > 0.05 || hs.OK >= hs.Take || hs.OK <= hs.Cancel {
		t.Fatalf("the Confirm's Take, the main Take and Cancel are %+v rem high, want 3 rem, between the other two", hs)
	}
	dialogAt := `(() => { const r = document.getElementById('confirm-dialog').getBoundingClientRect(); return [r.top, r.left, r.width].map(Math.round).join(','); })()`
	tall := b.eval(dialogAt).(string)
	gridConfirm(b)
	lastGroup(t, a, ana, 3)
	tallyHas(t, a, ana, 5, 3)
	tallyHas(t, a, ana, 6, 4)
	b.waitFor("the take clears the boxes and the staged list", `document.getElementById('grid-staged').hidden && !document.getElementById('src').value && !document.getElementById('dst').value && `+clearOff)
	b.waitFor("Take has no count again", take+` === 'Take'`)
	// The boxes held a whole route, so it is Last, as a patch of the boxes makes it.
	b.waitFor("the boxes' route is Last", lastText("src")+`.endsWith('CAM 1') && `+lastText("dst")+`.endsWith('DSM1')`)

	// With empty boxes, the staged routes alone make no Last: Last stays as it was.
	stage([2]int{8, 9}, [2]int{9, 10})
	b.waitFor("Take counts the staged routes", take+` === 'Take 2'`)
	b.eval(`document.getElementById('patch').click(); true`)
	b.waitFor("the confirm", `!document.getElementById('confirm').hidden`)
	if short := b.eval(dialogAt).(string); short != tall {
		t.Fatalf("the Confirm moved with a shorter plan: top, left and width %s, was %s", short, tall)
	}
	gridConfirm(b)
	lastGroup(t, a, ana, 2)
	b.waitFor("the staged routes went", `document.getElementById('grid-staged').hidden`)
	if src, dst := b.eval(lastText("src")).(string), b.eval(lastText("dst")).(string); !strings.HasSuffix(src, "CAM 1") || !strings.HasSuffix(dst, "DSM1") {
		t.Fatalf("Last is %q to %q after a take of staged routes alone, want CAM 1 to DSM1", src, dst)
	}

	// With staged routes, an Enter that picks a match only picks it.
	stage([2]int{7, 7})
	popTyped(b, "src", "CAM 1")
	b.eval(`document.getElementById('dst').focus(); true`)
	b.call("Input.insertText", map[string]any{"text": "DSM1"})
	b.waitFor("dst lists matches", `!document.getElementById('dst-menu').hidden`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("the route is whole, so Enter takes all", `!document.getElementById('confirm').hidden && document.getElementById('confirm-body').textContent.includes('2 destinations')`)
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)

	// The one Clear drops the boxes and the staged routes.
	b.eval(`document.getElementById('clear-all').click(); true`)
	b.waitFor("Clear clears everything", `document.getElementById('grid-staged').hidden && !document.getElementById('src').value && !document.getElementById('dst').value && `+clearOff)

	// Nothing to clear: Esc starts Keyboard mode (TestChromeMainMultiAdd checks Alt+Shift+k).
	esc("src")
	b.waitFor("Esc starts Keyboard mode", `document.activeElement === document.getElementById('history')`)

	// Ctrl+e with typed text not yet picked: it picks the best match, as Enter does, then finds it on the grid.
	typed := func(id, text string) {
		b.eval(`document.getElementById('` + id + `').focus(); true`)
		b.call("Input.insertText", map[string]any{"text": text})
		b.waitFor(id+" lists matches", `!document.getElementById('`+id+`-menu').hidden`)
	}
	typed("src", "SRC 150")
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the popup", shown)
	b.waitFor("Ctrl+e picks the source", `/^\d+ SRC 150$/.test(document.getElementById('src').value)`)
	b.waitFor("the cursor is on the source", cursor+` === '-1,149'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", hidden)
	esc("src")
	b.waitFor("Esc clears the box", `!document.getElementById('src').value`)
	typed("dst", "MV 107")
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the popup", shown)
	b.waitFor("Ctrl+e picks the destination", `/^\d+ MV 107$/.test(document.getElementById('dst').value)`)
	b.waitFor("the cursor is on the destination", cursor+` === '119,-1'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", hidden)
	esc("dst")
	// Both boxes typed: Ctrl+e picks both and aims at their crosspoint.
	typed("src", "CAM 2")
	typed("dst", "DSM3")
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the popup", shown)
	b.waitFor("Ctrl+e picks both", `/^\d+ CAM 2$/.test(document.getElementById('src').value) && /^\d+ DSM3$/.test(document.getElementById('dst').value)`)
	b.waitFor("the cursor is on their crosspoint", cursor+` === '4,13'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", hidden)
	esc("src")
	// Text with no match stays as typed, and the popup aims at nothing new.
	b.eval(`document.getElementById('src').focus(); true`)
	b.call("Input.insertText", map[string]any{"text": "qqqqzzzz"})
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the popup", shown)
	if v := b.eval(`document.getElementById('src').value`).(string); v != "qqqqzzzz" {
		t.Fatalf("Ctrl+e with no match changed the Source box to %q", v)
	}
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", hidden)
	esc("src")

	// The Grid button at the bottom left picks typed text first, as Ctrl+e does.
	b.waitFor("the boxes are empty", `!document.getElementById('src').value && `+clearOff)
	typed("dst", "MV 107")
	b.eval(`document.getElementById('grid-open').click(); true`)
	b.waitFor("the popup", shown)
	b.waitFor("the Grid button picks the destination", `/^\d+ MV 107$/.test(document.getElementById('dst').value)`)
	b.waitFor("the cursor is on the destination", cursor+` === '119,-1'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", hidden)
	b.check()
}
