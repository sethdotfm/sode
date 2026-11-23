package app

import (
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
