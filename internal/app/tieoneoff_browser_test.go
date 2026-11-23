package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// A One-off tie line in the Tie line picker, with two simulators and no saved tie line. The picker opens with no
// path and offers a One-off tie line. The person types A.6 and B.12: the picker chooses the path through it, marks
// what it feeds unknown, and the take lands on both simulators. Sode keeps it while B.12 is routed. An Administrator
// then saves it in Settings > Routers > Tie lines, with the name of the downstream source.
func TestChromeTiePickerOneOff(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")

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
	b.waitFor("the picker opens with no path and Take off", `!document.getElementById('tiepick').hidden && document.getElementById('tiepick').textContent.includes('No tie line from router A to router B')
		&& document.getElementById('tiepick-take').disabled`)

	b.eval(`document.getElementById('tiepick-oneoff').click(); true`)
	b.waitFor("the form names the routers", `!document.getElementById('tiepick-oneoff-form').hidden && document.getElementById('tiepick-oneoff-form').textContent.includes('Destination on router A')
		&& document.getElementById('tiepick-oneoff-form').textContent.includes('Source on router B')`)
	const typeIn = `((id, v) => { const i = document.getElementById(id); i.value = v; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })`
	b.eval(typeIn + `('tiepick-oneoff-from', '6')`)
	b.eval(typeIn + `('tiepick-oneoff-to', '12')`)
	b.eval(`document.getElementById('tiepick-oneoff-use').click(); true`)
	row := `[...document.querySelectorAll('.sode-tiepick__choice')].find((r) => r.textContent.includes('One-off A.6'))`
	b.waitFor("the picker chooses the One-off tie line and marks what it feeds unknown", `(() => { const r = `+row+`; return !!r && r.getAttribute('aria-selected') === 'true'
		&& document.getElementById('tiepick').textContent.includes('unknown downstream of a One-off tie line') && document.getElementById('tiepick-oneoff-form').hidden; })()`)
	b.waitFor("Take unlocks after the lockout", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the picker closes", `document.getElementById('tiepick').hidden`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.waitFor("the result says Success with the One-off tie line", `(() => { const t = document.getElementById('result').textContent; return t.includes('Success') && t.includes('Via One-off A.6'); })()`)
	if n := len(a.r().cfg.TieLines); n != 0 {
		t.Fatalf("%d saved tie lines after the take, want 0", n)
	}
	es, _ := a.hist.Page(-1, 5)
	if len(es) == 0 || len(es[0].Hops) != 1 || es[0].Hops[0].OneOff == nil || es[0].Hops[0].OneOff.To.N != 11 {
		t.Fatalf("entries %+v", es)
	}

	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("Sign in in Settings", `!!document.getElementById('account-login')?.offsetParent`)
	b.eval(`document.getElementById('account-login').click(); true`)
	signInAs(b, "Ana", "1234")
	b.waitFor("the One-off tie line in use", `(() => { const r = document.querySelector('#tielines-oneoffs-list [data-oneoff]'); return !!r?.offsetParent && r.textContent.includes('A.6')
		&& r.textContent.includes('B.12 From A'); })()`)
	b.eval(`document.querySelector('#tielines-oneoffs-list [data-oneoff] button').click(); true`)
	b.waitFor("saved as a tie line", `document.getElementById('tielines-status').textContent === 'Added From A.' && document.querySelectorAll('#tielines-list [data-id]').length === 1`)
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		got := a.r().cfg.TieLines
		if len(got) == 1 && got[0].Name == "From A" && got[0].From.Router == uids[0] && got[0].From.N == 5 && got[0].To.Router == uids[1] && got[0].To.N == 11 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("saved %+v", got)
		}
	}
	b.waitFor("the saved tie line is no One-off tie line in use", `!document.querySelector('#tielines-oneoffs-list [data-oneoff]')`)
	b.check()
}
