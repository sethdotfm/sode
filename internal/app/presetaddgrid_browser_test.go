package app

import (
	"fmt"
	"strings"
	"testing"
)

// The preset editor's Add from Patch Boxes at 1440 by 900, signed in. The bar's controls sit on the accent with no box
// behind them. Take reads Add at the same width, adds the boxes' patch and sends nothing. The Grid popup says it adds:
// a crosspoint click adds that route and a destination name adds the source it has now. No take reaches the router.
func TestChromePresetAddFromGrid(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	// Source 3 on destination 5 at the default levels, for the name click.
	if rec := apiPost(a, "/api/v1/takes", ana, `{"routes":[{"src":3,"dst":5}]}`, nil); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	tallyHas(t, a, ana, 5, 3)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("17 level picks", `document.querySelectorAll('#lvl-pick > button').length === 17`)
	realClick(b, "#lock-chip-signin")
	b.waitFor("the pad", `document.activeElement?.id === 'lock-name'`)
	typeNameThenPin(b)
	b.waitFor("Signed in as Ana, the Lock gone", `document.querySelector('.sode-lock__who-name')?.textContent === 'Signed in as Ana' && !document.documentElement.hasAttribute('data-sode-lock') && !document.documentElement.hasAttribute('data-sode-lock-on')`)

	// Every take message the page sends from here on.
	b.eval(`(() => {
		window.sentTakes = [];
		const send = WebSocket.prototype.send;
		WebSocket.prototype.send = function (d) {
			if (typeof d === 'string' && /"t":"(take|undo|redo|retry|salvo|protect)\w*"/.test(d)) window.sentTakes.push(d);
			return send.call(this, d);
		};
		return true;
	})()`)
	tallyBefore := apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String()
	takeW := b.eval(`document.getElementById('patch').getBoundingClientRect().width`).(float64)

	b.eval(`import('/presets.js').then((m) => (m.openPresets(), true))`)
	b.waitFor("the preset editor", `[...document.querySelectorAll('button')].some((x) => x.offsetParent && x.textContent === 'Add from Patch Boxes…')`)
	b.eval(`[...document.querySelectorAll('button')].find((x) => x.offsetParent && x.textContent === 'Add from Patch Boxes…').click(); true`)
	b.waitFor("the add bar", `document.documentElement.dataset.sodePick === 'boxes' && !document.querySelector('.sode__pickbar').hidden`)

	b.waitFor("the bar's controls have no box behind them, and Done is an outline on the accent", `(() => {
		const clear = (n) => getComputedStyle(n).backgroundColor === 'rgba(0, 0, 0, 0)';
		const done = [...document.querySelectorAll('.sode__pickbar .pb-btn')].find((x) => !x.hidden && x.textContent === 'Done');
		const add = [...document.querySelectorAll('.sode__pickbar .pb-btn')].find((x) => x.textContent === 'Add');
		return clear(document.querySelector('.sode__pickbar-controls')) && !!done && clear(done) && !!add && !add.offsetParent;
	})()`)
	b.waitFor("the lock chip has no box on the bar, and the title clears it at the content's left edge", `(() => {
		const who = document.querySelector('.sode-lock__who'), t = document.querySelector('.sode__pickbar-title').getBoundingClientRect();
		const page = document.getElementById('patch-view').getBoundingClientRect();
		return getComputedStyle(who).backgroundColor === 'rgba(0, 0, 0, 0)' && t.left >= who.getBoundingClientRect().right && t.left >= page.left - 1;
	})()`)
	b.waitFor("Take reads Add at the same width, in the add colour", `(() => {
		const p = document.getElementById('patch');
		return p.textContent === 'Add' && p.classList.contains('pb-btn--ok') && !p.classList.contains('pb-btn--danger') && !!p.offsetParent && Math.abs(p.getBoundingClientRect().width - `+fmt.Sprint(takeW)+`) < 1;
	})()`)
	settle(b)
	b.shot("bar-light")
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": "dark"}}})
	settle(b)
	b.shot("bar-dark")
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{}})
	settle(b)

	// Add adds the boxes' patch to the preset and clears the boxes.
	popTyped(b, "src", "CAM 1")
	popTyped(b, "dst", "DSM1")
	b.eval(`document.getElementById('patch').click(); true`)
	b.waitFor("Add adds a row", `document.querySelector('.sode__pickbar [role=status]').textContent === 'Added 1 row' && document.getElementById('src').value === ''`)

	// The bar covers the page's Grid button, so it has its own: Switch to grid view opens the popup. It says it adds to
	// the preset; its Take is Add and Done ends the mode.
	b.waitFor("Switch to grid view on the bar, before Done", `(() => {
		const g = document.querySelector('.sode__pickbar .sode__pickbar-grid'), d = [...document.querySelectorAll('.sode__pickbar .pb-btn')].find((x) => x.textContent === 'Done');
		return !!g?.offsetParent && g.textContent === 'Switch to grid view' && !!g.querySelector('use[href$="sode-grid"]') && g.getBoundingClientRect().right <= d.getBoundingClientRect().left;
	})()`)
	b.eval(`document.querySelector('.sode__pickbar .sode__pickbar-grid').click(); true`)
	b.waitFor("the popup", `!document.getElementById('grid-pop').hidden && !!document.querySelector('#grid-pop .sode-gridpanel__xp:not([hidden])')`)
	b.waitFor("the popup says it adds, Take reads Add, Clear goes and Done shows", `(() => {
		const q = (s) => document.querySelector('#grid-pop ' + s);
		return q('.sode-gridpop__adding')?.offsetParent && /^Add patches to /.test(q('.sode-gridpop__adding-msg').textContent) &&
			q('.sode-gridpop__take').textContent === 'Add' && q('.sode-gridpop__take').classList.contains('pb-btn--ok') &&
			!q('.sode-gridpop__clear').offsetParent && !!q('.sode-gridpop__done').offsetParent && !q('.sode-gridpop__shield:not([hidden])')?.offsetParent;
	})()`)
	count := `(document.querySelector('#grid-pop .sode-gridpop__rows')?.textContent ?? '')`
	b.waitFor("the band counts the preset's rows", count+` === '1 row in the preset' && !!document.querySelector('#grid-pop .sode-gridpop__rows').offsetParent`)

	note := `(document.querySelector('#grid-pop .sode-panel__status')?.textContent ?? '')`
	// A crosspoint adds its route and stages nothing.
	gridShow(b, 4, 7)
	b.eval(gridXp(4, 7) + `.click(); true`)
	b.waitFor("the crosspoint adds its route", note+`.startsWith('Added ') && document.querySelector('.sode__pickbar [role=status]').textContent === 'Added 2 rows'`)
	if b.eval(gridXp(4, 7) + `.dataset.s === 'staged'`).(bool) {
		t.Fatal("the click staged the route")
	}
	b.waitFor("the added route shows as in the preset, and the count goes up", gridXp(4, 7)+`.dataset.mark === '' && `+count+` === '2 rows in the preset' && /in the preset$/.test(`+gridXp(4, 7)+`.getAttribute('aria-label'))`)
	b.waitFor("its names show that they have a row", `document.querySelector('#grid-pop .sode-gridpanel__src[data-src="4"]')?.dataset.mark === '' && document.querySelector('#grid-pop .sode-gridpanel__dst[data-dst="7"]')?.dataset.mark === ''`)
	if b.eval(gridXp(5, 7) + `?.dataset.mark === ''`).(bool) {
		t.Fatal("a route not in the preset shows as in it")
	}
	// A click on a route already in the preset adds nothing and says so; the bar keeps its count.
	b.eval(gridXp(4, 7) + `.click(); true`)
	b.waitFor("a click on a marked route says it is there", note+` === 'Already in the preset.' && `+count+` === '2 rows in the preset' && document.querySelector('.sode__pickbar [role=status]').textContent === 'Added 2 rows'`)
	// Enter on the cursor's crosspoint adds as a click does, and confirms nothing. A second Enter finds it there.
	b.eval(`document.querySelector('#grid-pop .sode-gridpanel__scroll').focus(); true`)
	realKey(b, "ArrowRight", "ArrowRight", 39)
	realKey(b, "ArrowDown", "ArrowDown", 40)
	b.waitFor("the cursor on a crosspoint", `(() => { const c = document.querySelector('#grid-pop .sode-gridpanel__cursor'); return !!c && c.dataset.i === '0' && c.dataset.j === '0'; })()`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter adds the cursor's route", `document.querySelector('.sode__pickbar [role=status]').textContent === 'Added 3 rows'`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("a second Enter adds nothing", note+` === 'Already in the preset.'`)
	// The name of destination 5 adds the source it has now.
	b.eval(`document.querySelector('#grid-pop .sode-gridpanel__dst[data-dst="5"]').click(); true`)
	b.waitFor("the destination's name adds its current route", note+`.includes(' from ') && document.querySelector('.sode__pickbar [role=status]').textContent === 'Added 4 rows'`)
	b.waitFor("its name says in preset, and the count is 4", `document.querySelector('#grid-pop .sode-gridpanel__dst[data-dst="5"] .sode-gridpanel__held')?.textContent === 'in preset' && `+count+` === '4 rows in the preset'`)
	gridShow(b, 3, 5)
	b.waitFor("its current route shows as in the preset", gridXp(3, 5)+`.dataset.mark === ''`)
	b.shot("popup-light")
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": "dark"}}})
	settle(b)
	b.shot("popup-dark")
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{}})
	settle(b)

	// Esc closes the popup only; the add mode stays.
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc closes the popup", `document.getElementById('grid-pop').hidden && document.documentElement.dataset.sodePick === 'boxes'`)
	realKeyMods(b, "e", "KeyE", 69, modCtrl)
	b.waitFor("the popup again", `!document.getElementById('grid-pop').hidden`)
	b.eval(`document.querySelector('#grid-pop .sode-gridpop__done').click(); true`)
	b.waitFor("Done closes the popup and ends the mode", `document.getElementById('grid-pop').hidden && !document.documentElement.dataset.sodePick && document.getElementById('patch').textContent === 'Take'`)

	rows := b.eval(`[...document.querySelectorAll('.sode__preset-row')].map((r) => [...r.querySelectorAll('input')].map((i) => i.value).join(' | ')).join('\n')`).(string)
	// Display numbers (router number + 1). A crosspoint stores no levels, as the boxes' Add does with an empty Levels
	// box; a name stores the levels its route has now.
	if want := "5 | 8 | \n1 | 1 | \n4 | 6 | 1-17"; !strings.HasSuffix(rows, "\n"+want) || strings.Count(rows, "\n") != 3 {
		t.Fatalf("the rows are %q, want the boxes' row, then %q", rows, want)
	}
	if got := b.eval(`window.sentTakes.length`).(float64); got != 0 {
		t.Fatalf("the page sent %v take messages: %v", got, b.eval(`window.sentTakes`))
	}
	if now := apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(); now != tallyBefore {
		t.Fatal("the router's tally changed while adding to the preset")
	}
	// The spy sees a real take: Take again takes.
	// The rows are unsaved, so Cancel warns once and closes on the second press.
	cancel := `[...document.querySelectorAll('.sode__presets-dlg button')].find((x) => x.textContent === 'Cancel').click()`
	b.eval(cancel + `; ` + cancel + `; true`)
	b.waitFor("the editor closes", `!document.querySelector('.sode__presets-dlg')?.offsetParent`)
	popTyped(b, "src", "CAM 1")
	popTyped(b, "dst", "DSM1")
	b.eval(`document.getElementById('patch').click(); true`)
	gridConfirm(b)
	b.waitFor("the spy sees the take", `window.sentTakes.length === 1`)
	b.check()
}

// settle waits for the theme's colour transitions to end before a shot.
func settle(b *tab) {
	b.eval(`new Promise((r) => setTimeout(() => requestAnimationFrame(() => r(true)), 600))`)
}
