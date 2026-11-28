package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The Tie line picker at 1920x1080 with one tie line. Best is the badge only, with no outline round the choice. A
// step's head has room above its choices. The dialog takes the height of what it holds, not 80% of the window, and
// the wire says what it shows.
func TestChromeTiePickerLayout(t *testing.T) {
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

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
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
	b.waitFor("the picker with TIE 1 as Best", `!document.getElementById('tiepick').hidden && !!document.querySelector('.sode-tiepick__choice--best[data-tie="`+tie+`"]')`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	b.shot("picker")

	if got := b.eval(`getComputedStyle(document.querySelector('.sode-tiepick__choice--best')).outlineStyle`); got != "none" {
		t.Errorf("the Best choice has a %v outline, want the badge only", got)
	}
	b.gapY("step head to its choices", "calc(2 * var(--pb-space-3))", ".sode-tiepick__stephead", ".sode-tiepick__choices")
	if got := b.eval(`(() => { const d = document.getElementById('tiepick-dialog'); const body = d.querySelector('.sode-tiepick__body');
		const blank = body.getBoundingClientRect().bottom - [...body.children].at(-1).getBoundingClientRect().bottom;
		return blank <= parseFloat(getComputedStyle(body).paddingBottom) + 1 || blank; })()`); got != true {
		t.Errorf("the picker body has %v px blank under its last part, want the dialog the height of what it holds", got)
	}
	if got := b.eval(`document.querySelector('.sode-tiepick__choice .sode-tiepick__wire').dataset.pbTip || ''`); got == "" {
		t.Error("the wire has no tip")
	}
	// The step and the ends name their routers as the buttons do.
	b.waitFor("routers named as on the buttons", `(() => { const t = document.getElementById('tiepick').textContent; return t.includes('Step 1: Router A to Router B') && t.includes('Source on Router A'); })()`)
	b.check()
}
