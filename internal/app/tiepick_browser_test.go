package app

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Tie line picker on the main page, with two simulators: a take from router A to router B opens the picker. Esc
// cancels and sends nothing. A tap on the wire chooses the tie line, and the Take button sends the take. It lands on
// both simulators: the hop on A, then the destination on B.
func TestChromeTiePickerTakesByMouse(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	const tie = "aaaaaaaaaaaaaaa1"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{{ID: tie, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}}}
	})

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	pick := func(id, text, want string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+strings.ReplaceAll(want, " ", "")+`'; })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	pick("src", simSrcName, "A.11 "+simSrcName)
	pick("dst", "B4", "B.4 Stage Mon")
	press := func() {
		b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
		b.waitFor("the picker opens with TIE 1", `!document.getElementById('tiepick').hidden && !!document.querySelector('.sode-tiepick__choice[data-tie="`+tie+`"]')`)
	}

	press()
	b.waitFor("the picker shows the route, its step and the Affected destinations",
		`(() => { const t = document.getElementById('tiepick').textContent; return t.includes('Step 1: router A to router B') && t.includes('Affected destinations') && t.includes('Will carry A.11 `+simSrcName+`'); })()`)
	b.waitFor("the confirm with the letter stays closed", `document.getElementById('confirm').hidden`)
	b.eval(`document.getElementById('tiepick-dialog').dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', code: 'Escape', bubbles: true, cancelable: true})); true`)
	b.waitFor("Esc closes the picker", `document.getElementById('tiepick').hidden`)

	press()
	b.eval(`document.querySelector('.sode-tiepick__choice[data-tie="` + tie + `"] .sode-tiepick__wire').click(); true`)
	b.waitFor("the wire is lit", `document.querySelector('.sode-tiepick__choice[data-tie="`+tie+`"]').getAttribute('aria-selected') === 'true'`)
	b.waitFor("Take unlocks after the lockout", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the picker closes", `document.getElementById('tiepick').hidden`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.waitFor("the result says Success with the tie line", `(() => { const t = document.getElementById('result').textContent; return t.includes('Success') && t.includes('Via TIE 1'); })()`)
	b.check()
}

// The open picker updates live, with two simulators. Another take moves TIE 1: the picker shows what TIE 1 carries
// now, marks its row and starts the lockout again, and the selection stays. Then a move that the page does not hear
// in time (its plan messages are dropped) makes the take 409 tie_line_changed. The page says so and opens the picker
// again on the new state, and that take goes.
func TestChromeTiePickerLiveUpdate(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 3 Stage Mon"))
	// A long lockout, so the check sees Take off again after a change.
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.ConfirmLockoutMs = 2000 })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	const tie = "aaaaaaaaaaaaaaa1"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{{ID: tie, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}}}
	})
	// moveTie routes source n of router A to the upstream end of TIE 1 on every level, as another user does.
	moveTie := func(n int) {
		body := fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":%q,"n":5},"levels":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17]}]}`, uids[0], n, uids[0])
		if rec := apiPost(a, "/api/v1/takes", ana, body, nil); rec.Code != http.StatusOK {
			t.Fatalf("move TIE 1: %d %s", rec.Code, rec.Body)
		}
		routerTallyHas(t, a, ana, uids[0], 5, n)
	}

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	pick := func(id, text, want string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+strings.ReplaceAll(want, " ", "")+`'; })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	pick("src", simSrcName, "A.11 "+simSrcName)
	pick("dst", "B4", "B.4 Stage Mon")
	b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
	row := `document.querySelector('.sode-tiepick__choice[data-tie="` + tie + `"]')`
	shows := func(n int) string {
		return fmt.Sprintf(`(%s?.querySelector('.sode-tiepick__now')?.textContent || '').startsWith('Now A.%d ')`, row, n+1)
	}
	b.waitFor("the picker opens with TIE 1 and what it carries", `!document.getElementById('tiepick').hidden && `+shows(5))
	b.waitFor("Take unlocks after the lockout", `!document.getElementById('tiepick-take').disabled`)
	b.waitFor("no row is marked", `!document.querySelector('.sode-tiepick__mark')`)

	moveTie(6)
	b.waitFor("the picker shows the move, marks the row, starts the lockout again and keeps the selection",
		shows(6)+` && !!`+row+`.querySelector('.sode-tiepick__mark') && document.getElementById('tiepick-take').disabled && `+row+`.getAttribute('aria-selected') === 'true'`)
	b.waitFor("Take unlocks again", `!document.getElementById('tiepick-take').disabled`)

	// The page stops its plan messages, so it does not see the next move before the take.
	b.eval(`(() => {
		const send = WebSocket.prototype.send;
		window.sodeDropped = 0;
		window.sodeDrop = true;
		WebSocket.prototype.send = function (d) {
			if (window.sodeDrop && String(d).includes('"t":"tiePlan"')) { window.sodeDropped++; return; }
			return send.call(this, d);
		};
		return true;
	})()`)
	moveTie(7)
	b.waitFor("the page asked for a plan, and it was dropped", `window.sodeDropped > 0`)
	b.waitFor("Take stays on", `!document.getElementById('tiepick-take').disabled && `+shows(6))
	b.eval(`window.sodeDrop = false; document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the result names the tie line that changed", `(() => { const t = document.getElementById('result').textContent; return t.includes('Tie line changed') && t.includes('TIE 1 changed'); })()`)
	b.waitFor("the picker opens again on the new state, with TIE 1 chosen and marked",
		`!document.getElementById('tiepick').hidden && `+shows(7)+` && !!`+row+`.querySelector('.sode-tiepick__mark') && `+row+`.getAttribute('aria-selected') === 'true'`)
	routerTallyHas(t, a, ana, uids[1], 3, 3)
	b.waitFor("Take unlocks", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the picker closes", `document.getElementById('tiepick').hidden`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.waitFor("the result says Success", `document.getElementById('result').textContent.includes('Success')`)
	b.check()
}
