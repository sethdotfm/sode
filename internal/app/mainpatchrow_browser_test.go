package app

import (
	"math"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The desktop main page's patch row and top bar at 1920x1080, with the layout checks. The Last and Next chips are an
// action high with body text, and have the flex gap between them. "Last 0–10" fits even on a narrow window. Every box
// head has one height. The space from Source to Destination is the space from + to Levels, and Levels has the row's
// gap to Take. Select All ends under Clear on one line. The boxes keep no key note room out of keyboard mode. A
// disabled Clear has patchbay's disabled look, as a disabled Take has. The routes under Source and under Destination
// start on one line. In the top bar, Inbox has the text and icon size of the status. The status ends on the bar's right
// edge. When the link is lost, the rule before the status goes too. At 3840x2160 the Levels text box stops at 14rem.
func TestChromeMainPatchRow(t *testing.T) {
	b := startBrowser(t)
	sim, stopSim := startSimStoppable(t)
	a := simApp(t, sim, func(c *config.Config) { c.Routers[0].LevelNumbering = "0" })
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	size := func(w, h int) {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": h, "deviceScaleFactor": 1, "mobile": false})
	}
	size(1920, 1080)
	b.open(srv.URL + "/")
	b.waitFor("17 level picks", `document.querySelectorAll('#lvl-pick > button').length === 17`)
	// Levels 1 to 11 show as "0–10" with levelNumbering 0.
	b.eval(`localStorage.setItem('sode-last-take', JSON.stringify({ src: [10, 'Zoom 1'], dsts: [[2, 'DSM1']], levels: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11] })); true`)
	b.open(srv.URL + "/")
	b.waitFor("17 level picks", `document.querySelectorAll('#lvl-pick > button').length === 17`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	b.waitFor("Last 0–10", `document.querySelector('[data-last="lvl"] .sode__last')?.textContent === 'Last0–10'`)
	b.eval(`document.querySelector('.sode-lock__who-signin').click(); true`)
	signInAs(b, "Ana", "1234")
	b.waitFor("Inbox shows", `!!document.querySelector('[data-inbox-button]')?.offsetParent`)
	b.eval(`document.activeElement.blur(); true`)

	// A2-03, A2-13: Last and Next.
	for _, k := range []string{"src", "dst"} {
		slot := `[data-last="` + k + `"]`
		b.gapX(k+": Last to Next", "--pb-space-3", slot+" > .sode__last", slot+" .sode__next")
		b.sameLeft(k+": the closed Next chip fills its slot", slot+" .sode__next-size", slot+" .sode__next")
	}
	chips := `[data-last] .pb-chip.sode__last:not(.sode__next-size)`
	for _, x := range b.boxes(chips) {
		if h, want := x.height(), b.tokenPx(".sode__boxes", "--sode-h-action"); math.Abs(h-want) > layoutTol {
			t.Errorf("%s is %.2f px high, want --sode-h-action (%.2f px)", x.Name, h, want)
		}
	}
	if got, want := b.eval(`parseFloat(getComputedStyle(document.querySelector('[data-last="src"] .sode__last')).fontSize)`).(float64), b.tokenPx(".sode__boxes", "--pb-text-md"); math.Abs(got-want) > layoutTol {
		t.Errorf("the Last chip's text is %v px, want --pb-text-md (%v px)", got, want)
	}
	b.sameHeight("every box head", ".sode__boxes .sode__box-head")
	b.noClip("Last 0–10", `[data-last="lvl"] .sode__last-text`)

	// A2-10, A2-26: the gaps of the row.
	b.gapX("Levels to Take", "--pb-space-6", "#lvl", "#patch")
	b.gapX("Source to Destination", "calc(2 * var(--pb-space-6) + var(--sode-h-action))", "#src", "#dst")
	b.gapX("+ to Levels", "calc(2 * var(--pb-space-6) + var(--sode-h-action))", "#dst-add", "#lvl")

	// A2-23: Select All ends under Clear, its words on one line.
	if x := b.boxes("#lvl-all", "#clear-all"); len(x) != 2 || math.Abs(x[0].Right-x[1].Right) > layoutTol {
		t.Errorf("Select All and Clear end at %v, want one right edge", x)
	}
	b.sameMiddle("Select None on one line", "#lvl-all > span")
	b.noClip("Select None", "#lvl-all")

	// A2-11: out of keyboard mode the boxes keep no room for the hidden key note.
	for _, id := range []string{"src", "dst", "lvl"} {
		if got, want := b.eval(`parseFloat(getComputedStyle(document.getElementById('`+id+`')).paddingRight)`).(float64), b.tokenPx(".sode__boxes", "--pb-space-3"); math.Abs(got-want) > layoutTol {
			t.Errorf("#%s has a right pad of %v px, want --pb-space-3 (%v px)", id, got, want)
		}
	}

	// A2-22: a disabled Clear and a disabled Take have patchbay's disabled look.
	look := `((n) => { const c = getComputedStyle(n); return [c.backgroundColor, c.borderTopColor, c.color, c.opacity].join(' '); })`
	b.eval(`(() => { const p = Object.assign(document.createElement('button'), { className: 'pb-btn', disabled: true, id: 'probe-off' }); document.getElementById('patch').after(p); document.getElementById('patch').disabled = true; return true; })()`)
	b.waitFor("Take's color change ends", `document.getElementById('patch').getAnimations().length === 0`)
	if probe, clear, take := b.eval(look+`(document.getElementById('probe-off'))`), b.eval(look+`(document.getElementById('clear-all'))`), b.eval(look+`(document.getElementById('patch'))`); clear != probe || take != probe {
		t.Errorf("disabled looks: Clear %v, Take %v, want patchbay's %v", clear, take, probe)
	}
	b.eval(`document.getElementById('probe-off').remove(); document.getElementById('patch').disabled = false; true`)

	// A2-16: the top bar.
	if got, want := b.eval(`getComputedStyle(document.querySelector('.sode-inbox__word')).fontSize`), b.eval(`getComputedStyle(document.getElementById('conn-srv-text')).fontSize`); got != want {
		t.Errorf("Inbox text is %v, the status text %v", got, want)
	}
	b.sameHeight("Inbox icon and status icons", "[data-inbox-button] .pb-icon", "#conn .sode__state > .pb-icon")

	// A2-01: the routes under Source and Destination start on one line.
	pick := func(id, text string) {
		b.waitFor(id+" match list", `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('`+id+`-opt-0'); })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', { key: 'Tab', code: 'Tab', bubbles: true, cancelable: true })); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	pick("src", simSrcName)
	pick("dst", "DSM1")
	b.waitFor("routes under both boxes", `!!document.querySelector('#src-under > .sode__under-head') && !!document.querySelector('#dst-under > .sode__under-head')`)
	b.sameMiddle("routes under Source and Destination", "#src-under > .sode__under-head", "#dst-under > .sode__under-head")

	// A2-13: on a narrow window the Levels Last chip keeps room for "Last 0–10".
	size(1100, 900)
	b.waitFor("1100 px", `innerWidth === 1100`)
	b.noClip("Last 0–10 at 1100 px", `[data-last="lvl"] .sode__last-text`)

	// A2-06: at 3840x2160 the Levels text box stops at 14rem, and Take keeps its gap.
	size(3840, 2160)
	b.waitFor("3840 px", `innerWidth === 3840`)
	if w, max := b.boxes("#lvl")[0], b.tokenPx(".sode__boxes", "14rem"); w.Right-w.Left > max+layoutTol {
		t.Errorf("at 3840 px the Levels text box is %.1f px wide, want at most 14rem (%.1f px)", w.Right-w.Left, max)
	}
	b.gapX("Levels to Take at 3840 px", "--pb-space-6", "#lvl", "#patch")

	// A2-16: two routers leave the address empty, and the status still ends on the bar's edge. A lost link
	// takes the rule before the status with it.
	size(1920, 1080)
	b.waitFor("1920 px", `innerWidth === 1920`)
	if err := a.applyConfig(func(c *config.Config) error {
		rc := c.DefaultRouter()
		rc.UID, rc.Prefix, rc.Title, rc.Address = "", "B", "Router B", "127.0.0.1:1"
		c.Routers = append(c.Routers, rc)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b.waitFor("no address with two routers", `document.getElementById('conn-addr').textContent === ''`)
	if x := b.boxes("#conn-routers .pb-status", "#clear-all"); len(x) != 2 || math.Abs(x[0].Right-x[1].Right) > layoutTol {
		t.Errorf("the status ends at %v, want the page edge (Clear's right edge)", x)
	}
	b.waitFor("the rule before the status", `!!document.querySelector('#home-bar > .sode-bar__rule--end')?.getClientRects().length`)
	stopSim()
	b.waitFor("the router link is lost", `document.documentElement.dataset.conn === 'down'`)
	if n := b.eval(`document.querySelector('#home-bar > .sode-bar__rule--end').getClientRects().length`); n != 0.0 {
		t.Errorf("the rule before the status still shows with the link lost")
	}
}
