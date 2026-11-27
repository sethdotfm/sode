package app

import (
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Protect from the Tie line picker, with two simulators (docs/tielines-design.md, Protect in a tie take). Without
// the Protect right the picker has no shield and P does nothing. Signed in with the right, P asks for the Protect of
// TIE 1. The take lands on both simulators, and then Sode protects the upstream end of TIE 1 on router A. The next
// picker shows TIE 1 grey with who set the Protect and when. A take of the source that TIE 1 already carries can
// still go through it.
func TestChromeTiePickerProtect(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 From A", "dst 3 Stage Mon", "dst 4 Desk Mon"))
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Routers[0].ProtectDevice = 126 // router A only
	})
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
	row := `document.querySelector('.sode-tiepick__choice[data-tie="` + tie + `"]')`
	shield := `document.querySelector('.sode-tiepick__shield[data-tie="` + tie + `"]')`
	open := func(src, srcWant, dst, dstWant string) {
		pick("src", src, srcWant)
		pick("dst", dst, dstWant)
		b.eval(`(() => { const t = document.getElementById('lvl'); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); document.getElementById('patch').click(); return true; })()`)
		b.waitFor("the picker opens with TIE 1", `!document.getElementById('tiepick').hidden && !!`+row)
	}
	key := func(k, code string) {
		b.eval(`document.getElementById('tiepick-dialog').dispatchEvent(new KeyboardEvent('keydown', {key: '` + k + `', code: '` + code + `', bubbles: true, cancelable: true})); true`)
	}

	// Not signed in: no Protect right.
	open(simSrcName, "A.11 "+simSrcName, "B4", "B.4 Stage Mon")
	b.waitFor("no shield and no P hint", `!`+shield+` && !document.querySelector('.sode-tiepick__keys').innerText.includes('protect')`)
	key("p", "KeyP")
	b.waitFor("P asks for nothing", `!document.querySelector('.sode-tiepick__protect')`)
	key("Escape", "Escape")
	b.waitFor("Esc closes the picker", `document.getElementById('tiepick').hidden`)

	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)

	open(simSrcName, "A.11 "+simSrcName, "B4", "B.4 Stage Mon")
	b.waitFor("the shield and the P hint show", `!!`+shield+` && `+shield+`.getAttribute('aria-pressed') === 'false' && document.querySelector('.sode-tiepick__keys').innerText.includes('protect')`)
	key("p", "KeyP")
	b.waitFor("P asks for the Protect of TIE 1", shield+`.getAttribute('aria-pressed') === 'true' && `+row+`.textContent.includes('Protect after the take')`)
	// The shield does the same as P.
	b.eval(shield + `.click(); true`)
	b.waitFor("the shield stops asking", shield+`.getAttribute('aria-pressed') === 'false' && !document.querySelector('.sode-tiepick__protect')`)
	b.eval(shield + `.click(); true`)
	b.waitFor("the shield asks again", shield+`.getAttribute('aria-pressed') === 'true'`)
	b.waitFor("Take unlocks after the lockout", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the picker closes", `document.getElementById('tiepick').hidden`)
	routerTallyHas(t, a, ana, uids[0], 5, simSrc)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	b.waitFor("the result says Success and names the Protect", `(() => { const t = document.getElementById('result').textContent; return t.includes('Success') && t.includes('then Protect TIE 1'); })()`)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		body := apiDo(a, "GET", "/api/v1/protected?router="+uids[0], ana, nil).Body.String()
		if strings.Contains(body, `"dst":5,`) && strings.Contains(body, `"sodeUser":"Ana"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("A.5 is not Protected by Ana: %s", body)
		}
	}
	if es, _ := a.hist.Page(-1, 3); len(es) != 3 || es[0].Protect != "set" || !es[0].OK || es[0].Group == "" || es[0].GroupN != 3 || es[2].Group != es[0].Group {
		t.Fatalf("the Protect is not in the Group of the take: %+v", es)
	}

	// Another source: TIE 1 is grey and says who and when; Take stays off.
	open("Zoom 2", "A.12 Zoom 2", "B5", "B.5 Desk Mon")
	b.waitFor("TIE 1 is grey with who and when", row+`.classList.contains('sode-tiepick__choice--held') && /Protected by Ana at \d\d:\d\d/.test(`+row+`.textContent)`)
	b.waitFor("its shield is off", shield+`.disabled`)
	time.Sleep(700 * time.Millisecond) // past the lockout
	b.waitFor("Take stays off", `document.getElementById('tiepick-take').disabled`)
	key("Escape", "Escape")
	b.waitFor("Esc closes the picker", `document.getElementById('tiepick').hidden`)

	// The source that TIE 1 carries: TIE 1 stays usable, and nothing goes upstream.
	open(simSrcName, "A.11 "+simSrcName, "B5", "B.5 Desk Mon")
	b.waitFor("TIE 1 is usable", `!`+row+`.classList.contains('sode-tiepick__choice--held') && `+row+`.textContent.includes('Carries the source now')`)
	b.waitFor("Take unlocks", `!document.getElementById('tiepick-take').disabled`)
	b.eval(`document.getElementById('tiepick-take').click(); true`)
	b.waitFor("the picker closes", `document.getElementById('tiepick').hidden`)
	routerTallyHas(t, a, ana, uids[1], 4, 11)
	b.waitFor("the result says Success", `document.getElementById('result').textContent.includes('Success')`)
	b.check()
}
