package app

import (
	"fmt"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Panels and tie lines, with two simulators (docs/tielines-design.md, Panels). Each panel Reserves its own tie line
// from router A to router B. A Buttons panel in Take mode shows what its Reserved tie line also changes, and a second
// Take sends the take on it. A Grid panel in Instant mode sends on its Reserved tie line with no confirm. On the main
// page, the Tie line picker shows the Reserve, and Take asks one more time before it sends on it.
func TestChromePanelTieLinesReserved(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "src 12 From A 2", "dst 3 Stage Mon", "dst 4 Stage Mon 2"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	const tie1, tie2 = "aaaaaaaaaaaaaaa1", "aaaaaaaaaaaaaaa2"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{
			{ID: tie1, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}},
			{ID: tie2, Name: "TIE 2", From: config.Ref{Router: uids[0], N: 6}, To: config.Ref{Router: uids[1], N: 12}},
		}
	})
	buttons := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Lobby","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","router":%[2]q,"n":3},
		{"r":1,"c":2,"kind":"source","router":%[1]q,"n":%[3]d},
		{"r":1,"c":3,"kind":"take"}],"settings":{"mode":"take"},"tieLines":{"mode":"reserved","reserved":[%[4]q]}}`, uids[0], uids[1], simSrc, tie1))
	grid := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Wall","type":"grid","settings":{"mode":"instant","levels":{%[1]q:[1]}},
		"sources":{"members":[{"router":%[1]q,"n":%[3]d}]},
		"destinations":{"members":[{"router":%[2]q,"n":4}]},"tieLines":{"mode":"reserved","reserved":[%[4]q]}}`, uids[0], uids[1], simSrc, tie2))

	// The Buttons panel in Take mode: the first Take shows the tie line and what it also changes.
	b.open(srv.URL + "/panels/" + buttons + "?instance=Desk")
	signInOnPad(b)
	cell := func(r, c int) string {
		return fmt.Sprintf(`document.querySelector('.sode-panel__cell[style*="grid-area: %d / %d"]')`, r, c)
	}
	status := `document.getElementById('panel-status').textContent`
	b.waitFor("the cells", `document.querySelectorAll('.sode-panel__cell').length === 3`)
	b.waitFor("router B's names", cell(1, 1)+`?.querySelector('span')?.textContent === 'Stage Mon'`)
	b.eval(cell(1, 1) + `.click()`)
	b.eval(cell(1, 2) + `.click()`)
	b.waitFor("Take armed: a source on router A stages for router B", `!`+cell(1, 3)+`.disabled`)
	b.eval(cell(1, 3) + `.click()`)
	b.waitFor("the first Take shows the Reserved tie line", status+`.includes('Via TIE 1') && `+status+`.includes('Press Take again to send')`)
	b.eval(cell(1, 3) + `.click()`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)

	// The Grid panel in Instant mode: a press sends on TIE 2, with no confirm.
	b.open(srv.URL + "/panels/" + grid + "?instance=Desk")
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	bDst := itemSpan + 4
	b.waitFor("the crosspoint across routers", gridXp(simSrc, bDst)+` !== null`)
	b.eval(gridXp(simSrc, bDst) + `.click(); true`)
	routerTallyHas(t, a, ana, uids[0], 6, simSrc)
	routerTallyHas(t, a, ana, uids[1], 4, 12)
	b.waitFor("the confirm stays closed", `document.getElementById('confirm').hidden`)

	// The main page: TIE 1 and TIE 2 already carry the source, so the picker sends nothing upstream on them. A second
	// source needs one of them, and each is Reserved by its panel.
	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	pick := func(id, text string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '').startsWith('`+text+`'); })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	pick("src", "A.2")
	pick("dst", "B.4")
	b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
	b.waitFor("the picker shows the Reserve, and the choice is not grey", `(() => {
		const c = document.querySelector('.sode-tiepick__choice[aria-selected="true"]');
		return !!c && c.textContent.includes('Reserved by panel') && !c.classList.contains('sode-tiepick__choice--held');
	})()`)
	b.waitFor("Take unlocks after the lockout", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("Take asks one more time", `!document.getElementById('tiepick').hidden && document.querySelector('.sode-tiepick__reserve')?.textContent.includes('Press Take again')`)
	b.waitFor("the lockout starts again", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the picker closes", `document.getElementById('tiepick').hidden`)
	// TIE 1 is the cheaper path: B.4 already shows it, so only the hop on router A sends.
	routerTallyHas(t, a, ana, uids[0], 5, 1)

	// The editor's Panel tab: the Buttons panel Reserves TIE 1, and TIE 2 is the Grid panel's, so it cannot be chosen.
	b.open(srv.URL + "/panels/editor?panel=" + buttons)
	b.waitFor("the lock goes in the editor", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the tie line setting", fmt.Sprintf(`(() => {
		const s = document.getElementById('ed-ties-%s-%s');
		const two = s && [...s.options].find((o) => o.value === %q);
		return document.getElementById('ed-set-ties-reserved')?.checked && s.value === %q && two?.disabled && two.textContent.includes('Reserved by panel Wall');
	})()`, uids[0], uids[1], tie2, tie1))
	// A panel asks the plan with POST /takes/plan, which answers 409 tie_line_needed (paneltierules.js). Each take of
	// a panel gives one such load. Anything else is a real error.
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 409") || !strings.HasSuffix(e, "/api/v1/takes/plan") {
			t.Errorf("page error: %s", e)
		}
	}
}
