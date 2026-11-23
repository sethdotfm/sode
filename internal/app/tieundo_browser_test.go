package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Ctrl+Z and Ctrl+Shift+Z over a tie take, with two simulators (docs/tielines-design.md, Undo; Redo and Retry). A tie
// take sent from the picker is this tab's take. Ctrl+Z undoes its destination through the Undo confirm and leaves
// TIE 1 as it is. The row says Undone and that the tie lines are left as they are. Ctrl+Shift+Z opens the picker on
// TIE 1, the tie line that the take used last time, also when TIE 2 costs less now. The picker's Take routes it again.
func TestChromeTieUndoRedoKeys(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t, simNames(t, "src 3 Cam Three"))
	simB := startSim(t, simNames(t, "src 11 From A", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	const tie, tie2 = "aaaaaaaaaaaaaaa1", "aaaaaaaaaaaaaaa2"
	lifeCommit(t, a, func(c *config.Config) {
		c.TieLines = []config.TieLine{
			{ID: tie, Name: "TIE 1", From: config.Ref{Router: uids[0], N: 5}, To: config.Ref{Router: uids[1], N: 11}},
			{ID: tie2, Name: "TIE 2", From: config.Ref{Router: uids[0], N: 7}, To: config.Ref{Router: uids[1], N: 15}},
		}
	})

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	pick := func(id, text, want string) {
		b.waitFor(id+" match for "+text, `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true}));
			return (document.getElementById('`+id+`-opt-0')?.textContent || '').replace(/\s/g, '') === '`+strings.ReplaceAll(want, " ", "")+`'; })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	chosen := `document.querySelector('.sode-tiepick__choice[data-tie="` + tie + `"]')`
	// takeInPicker sends the take through TIE 1. click: choose TIE 1 first, as the picker opens on the cheapest path.
	takeInPicker := func(what string, click bool) {
		b.waitFor(what+": the picker opens", `!document.getElementById('tiepick').hidden && !!`+chosen)
		if click {
			b.eval(chosen + `.click(); true`)
		}
		b.waitFor(what+": TIE 1 is chosen", chosen+`.getAttribute('aria-selected') === 'true'`)
		b.waitFor(what+": Take unlocks", `!document.getElementById('tiepick-take').disabled`)
		b.eval(`document.getElementById('tiepick-take').click(); true`)
		b.waitFor(what+": the picker closes", `document.getElementById('tiepick').hidden`)
	}
	press := func(shift bool) {
		key, s := `'z'`, "false"
		if shift {
			key, s = `'Z'`, "true"
		}
		b.eval(`(() => { const mac = /Mac/.test(navigator.platform); document.body.focus(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: ` + key + `, code: 'KeyZ', ctrlKey: !mac, metaKey: mac, shiftKey: ` + s + `, bubbles: true, cancelable: true})); return true; })()`)
	}
	own := `JSON.parse(sessionStorage.getItem('sode-own-takes') || '[]')`

	pick("src", simSrcName, "A.11 "+simSrcName)
	pick("dst", "B4", "B.4 Stage Mon")
	b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
	takeInPicker("the take", true)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	// The take straight from the picker is this tab's take: its destination entry is in the list.
	es, _ := a.hist.Page(-1, 5)
	var dest Entry
	for _, e := range es {
		if e.Origin != nil {
			dest = e
			break
		}
	}
	if dest.Origin == nil || len(dest.Prev) == 0 {
		t.Fatalf("no destination entry in %+v", es)
	}
	b.waitFor("the tie take is this tab's", own+`.some((k) => k.startsWith('`+strconv.Itoa(dest.ID)+`@'))`)

	press(false)
	b.waitFor("Ctrl+Z opens the Undo confirm for B.4", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm undo'
		&& document.getElementById('confirm').textContent.includes('Stage Mon') && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the confirm closes", `document.getElementById('confirm').hidden`)
	routerTallyHas(t, a, ana, uids[1], 3, dest.Prev[0][1])
	// Undo never switches TIE 1 back: its upstream end keeps the source of the take.
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	row := `[...document.querySelectorAll('#history > li')].find((r) => r.querySelector('.sode__hist-route')?.textContent.includes('Via TIE 1'))`
	b.waitFor("the tie row says Undone and that the tie lines are left", `(() => { const r = `+row+`; if (!r) return false; const t = r.textContent;
		return t.includes('Undone') && t.includes('tie lines left as they are'); })()`)

	// Make TIE 1 cost more than TIE 2: it carries Cam Three now, and two more destinations of B take its end.
	all := `[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17]`
	for _, body := range []string{
		fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":3},"dst":{"router":%q,"n":5},"levels":%s}]}`, uids[0], uids[0], all),
		fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":11},"dst":{"router":%q,"n":0},"levels":[1]},{"src":{"router":%q,"n":11},"dst":{"router":%q,"n":1},"levels":[1]}]}`, uids[1], uids[1], uids[1], uids[1]),
	} {
		if rec := apiPost(a, "/api/v1/takes", ana, body, nil); rec.Code != http.StatusOK {
			t.Fatalf("API take: %d %s", rec.Code, rec.Body)
		}
	}
	routerTallyHas(t, a, ana, uids[0], 5, 3)
	routerTallyHas(t, a, ana, uids[1], 1, 11)
	plan := apiPost(a, "/api/v1/takes", ana, fmt.Sprintf(`{"routes":[{"src":{"router":%q,"n":%d},"dst":{"router":%q,"n":3},"levels":[1]}]}`, uids[0], simSrc, uids[1]), nil)
	var paths tieBodyIn
	if err := json.Unmarshal(plan.Body.Bytes(), &paths); err != nil || len(paths.Routes) != 1 || len(paths.Routes[0].Paths) < 2 || paths.Routes[0].Paths[0].Hops[0].Tie.ID != tie2 {
		t.Fatalf("TIE 2 is not the cheapest now: %d %s", plan.Code, plan.Body)
	}
	press(true)
	b.waitFor("Ctrl+Shift+Z opens the picker on TIE 1, used last time", `!document.getElementById('tiepick').hidden && !!`+chosen+` && `+chosen+`.textContent.includes('Used last time')
		&& `+chosen+`.getAttribute('aria-selected') === 'true' && !document.querySelector('#tiepick .sode-tiepick__stored')`)
	takeInPicker("Ctrl+Shift+Z", false)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.waitFor("the Redo list is empty", `JSON.parse(sessionStorage.getItem('sode-redo-steps') || '[]').length === 0`)

	// B.4 now has Zoom 1 Via TIE 1. A take of Cam Three switches TIE 1, and B.4 stays on its end. The Undo of B.4
	// alone would give it Cam Three. So the Undo confirm offers to put TIE 1 back to Zoom 1, in a box that is off.
	pick("src", "Cam Three", "A.4 Cam Three")
	pick("dst", "B4", "B.4 Stage Mon")
	b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
	takeInPicker("the take of Cam Three", true)
	routerTallyHas(t, a, ana, uids[0], 5, 3)
	// The answer of the take makes it this tab's newest take.
	b.waitFor("the take of Cam Three is this tab's", own+`.length === 4`)
	press(false)
	b.waitFor("the Undo confirm offers to put TIE 1 back", `(() => { const c = document.getElementById('confirm'); const x = c.querySelector('.sode__plan-extra');
		return !c.hidden && !!x && x.textContent.includes('Also put TIE 1 back to Zoom 1') && !x.querySelector('input').checked && !document.getElementById('confirm-ok').disabled; })()`)
	b.eval(`document.querySelector('#confirm .sode__plan-extra input').click(); document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the confirm closes after the put-back", `document.getElementById('confirm').hidden`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.check()
}
