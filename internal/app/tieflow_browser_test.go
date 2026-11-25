package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// A take with plain routes and tie routes, with two simulators (docs/tielines-design.md, Takes with plain routes and
// tie routes). A route with no path opens the picker with the reason by name. A take from A.11 to A.3, B.4 and B.5
// asks one question for the hop A to B of B.4 and B.5. Then the churning-letter confirm opens with every route. The
// split gives each destination its own question, Clear this route drops B.4, and a cancel of the confirm sends
// nothing. Then B.4 and B.5 share TIE 1 in one question. The take lands on both simulators.
func TestChromeTiePickerMixedTake(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 3 Stage Mon", "dst 4 Side Mon"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Side Mon")
	const tie = "aaaaaaaaaaaaaaa1"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{{ID: tie, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}}}
	})

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	match := func(id, text, want string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+strings.ReplaceAll(want, " ", "")+`'; })()`)
	}
	pick := func(id, text, want string) {
		match(id, text, want)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	add := func(text, want string) {
		match("dst", text, want)
		b.eval(`document.getElementById('dst-add').click(); true`)
		b.waitFor("destination added: "+want, `[...document.querySelectorAll('#dst-chips .sode__dst-chip')].some((c) => c.textContent.replace(/\s/g, '').includes('`+strings.ReplaceAll(want, " ", "")+`'))`)
	}
	press := func() {
		b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
	}
	picker := func(head string) {
		b.waitFor("the picker opens: "+head, `!document.getElementById('tiepick').hidden && document.querySelector('#tiepick .sode-tiepick__count').textContent === '`+head+`' && !!document.querySelector('.sode-tiepick__choice[data-tie="`+tie+`"]')`)
	}
	next := func() {
		b.eval(`document.querySelector('.sode-tiepick__choice[data-tie="` + tie + `"] .sode-tiepick__wire').click(); true`)
		b.waitFor("Next unlocks", `!document.getElementById('tiepick-take').disabled && document.getElementById('tiepick-take').textContent.startsWith('Next')`)
		b.eval(`document.getElementById('tiepick-take').click(); true`)
	}
	confirmBody := `document.getElementById('confirm-body').textContent`

	// No tie line goes from B to A: the picker names the reason, and Take stays off.
	pick("src", "B12", "B.12 From A")
	pick("dst", "DSM1", "A.3 DSM1")
	press()
	b.waitFor("the picker names the reason", `!document.getElementById('tiepick').hidden && (() => {
		const t = document.querySelector('#tiepick .pb-alert--error')?.textContent || '';
		return t.includes('No usable path') && t.includes('No tie line from router B to router A'); })()`)
	b.waitFor("Take stays off, and a take of one route has no Clear this route",
		`document.getElementById('tiepick-take').disabled && document.getElementById('tiepick-clear').hidden`)
	b.eval(`document.getElementById('tiepick-dialog').dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', code: 'Escape', bubbles: true, cancelable: true})); true`)
	b.waitFor("Esc closes the picker", `document.getElementById('tiepick').hidden`)

	pick("src", simSrcName, "A.11 "+simSrcName)
	add("DSM1", "A.3 DSM1")
	add("B4", "B.4 Stage Mon")
	pick("dst", "B5", "B.5 Side Mon")

	// Per destination, then Clear this route drops B.4. The confirm lists A.3 and B.5 Via TIE 1, and its cancel sends
	// nothing.
	press()
	picker("A → B, for B.4 B.5")
	b.waitFor("Clear these routes shows", `!document.getElementById('tiepick-clear').hidden && document.getElementById('tiepick-clear').textContent === 'Clear these routes'`)
	b.eval(`document.getElementById('tiepick-split').click(); true`)
	picker("A → B, for B.4")
	b.eval(`document.getElementById('tiepick-clear').click(); true`)
	picker("A → B, for B.5")
	b.waitFor("the picker is for B.5", `document.querySelector('#tiepick .sode-tiepick__route').textContent.includes('B.5')`)
	next()
	b.waitFor("the confirm lists the plain route and the tie route Via TIE 1",
		`!document.getElementById('confirm').hidden && `+confirmBody+`.split('Via TIE 1').length === 2 && `+confirmBody+`.includes('A.3') && `+confirmBody+`.includes('B.5') && !`+confirmBody+`.includes('B.4')`)
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	routerTallyHas(t, a, ana, uids[0], 5, 5)

	// B.4 and B.5 share TIE 1 in one question.
	press()
	picker("A → B, for B.4 B.5")
	next()
	b.waitFor("the confirm lists every route, the tie routes Via TIE 1",
		`!document.getElementById('confirm').hidden && `+confirmBody+`.split('Via TIE 1').length === 3 && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	routerTallyHas(t, a, ana, uids[0], simDst, simSrc)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	routerTallyHas(t, a, ana, uids[1], 4, 11)
	b.waitFor("the result says Success with the tie line", `(() => { const t = document.getElementById('result').textContent; return t.includes('Success') && t.includes('Via TIE 1'); })()`)
	b.check()
}
