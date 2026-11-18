package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Compact view hides History. While both links are up, it fades the footer, the Lock bar and the
// status (hover or focus brings each back). It centres the block, and neither the match list nor a
// failed take moves the block. After a reload the Last chips fill from the stored Last, and a failed
// take and a lost link still show. Alt+Shift+C toggles Compact view from a box, and Settings shows
// it under This browser.
func TestChromeMainCompact(t *testing.T) {
	b := startBrowser(t)
	// The simulator ACKs connects to DSM1 but never routes it, so a take there fails.
	t.Setenv("SIM_IGNORE_DSTS", fmt.Sprint(simDst))
	sim, stopSim := startSimStoppable(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	shown := func(sel string) string {
		return `(() => { const n = document.querySelector('` + sel + `'); return !!n && getComputedStyle(n).display !== 'none'; })()`
	}
	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.waitFor("History shown before Compact view", shown("#patch-view > .sode__history"))
	b.eval(fmt.Sprintf(`localStorage.setItem('sode-compact', 'on'); localStorage.setItem('sode-last-take', JSON.stringify({src:[%d,'old'],dsts:[[%d,'old']],levels:[1]})); true`, simSrc, simDst))

	b.open(srv.URL + "/")
	b.waitFor("data-compact set before the modules run", `document.documentElement.hasAttribute('data-compact')`)
	b.waitFor("History hidden", `!(`+shown("#patch-view > .sode__history")+`)`)
	for _, sel := range []string{"#patch-view .sode__boxes", "#patch-view [data-presets]", "#patch-view .sode__footer", "#settings-open", "#patch"} {
		b.waitFor(sel+" still shown", shown(sel))
	}

	// Real pointer moves: :hover ignores synthetic events. The rest point is the page centre, away
	// from the corners where the faded chrome sits.
	mouse := func(x, y any) {
		b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y})
	}
	hover := func(sel string) {
		xy := b.eval(`(() => { const r = document.querySelector('` + sel + `').getBoundingClientRect(); return [r.x + r.width / 2, r.y + r.height / 2]; })()`).([]any)
		mouse(xy[0], xy[1])
	}
	// near points 5rem (100px at 125%) past the item's inner corner, inside the apron but off the item.
	near := func(sel string, dx, dy int) {
		xy := b.eval(fmt.Sprintf(`(() => { const r = document.querySelector('%s').getBoundingClientRect(); return [(%d > 0 ? r.right : r.left) + %d, (%d > 0 ? r.bottom : r.top) + %d]; })()`, sel, dx, dx, dy, dy)).([]any)
		mouse(xy[0], xy[1])
	}
	rest := func() { mouse(640, 400) }
	opacity := func(sel, want string) string {
		return `getComputedStyle(document.querySelector('` + sel + `')).opacity === '` + want + `'`
	}
	rest()
	b.waitFor("status marked ok with both links up", `document.getElementById('conn').classList.contains('sode__conn--ok')`)
	for _, c := range []struct {
		sel    string
		dx, dy int
	}{{".sode__footer", 100, -80}, {"#conn", -100, 80}} {
		sel := c.sel
		b.waitFor(sel+" faded", opacity(sel, "0")+` && `+shown(sel))
		hover(sel)
		b.waitFor(sel+" back on hover", opacity(sel, "1"))
		rest()
		b.waitFor(sel+" faded again", opacity(sel, "0"))
		near(sel, c.dx, c.dy)
		b.waitFor(sel+" back with the pointer near it", opacity(sel, "1"))
		rest()
		b.waitFor(sel+" faded once the pointer leaves", opacity(sel, "0"))
	}
	// The aprons stop short of the controls: each one's centre still hits the control itself.
	for _, sel := range []string{"#src", "#dst", "#patch", "#patch-view [data-presets]"} {
		b.waitFor(sel+" not under an apron", `(() => { const e = document.querySelector('`+sel+`'), r = e.getBoundingClientRect(), h = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2); return e === h || e.contains(h); })()`)
	}
	if got := b.eval(`[document.getElementById('settings-open').getAttribute('aria-label'), document.getElementById('settings-open').dataset.pbTip].join('|')`); got != "Settings|Settings" {
		t.Fatalf("faded Settings button lost its name or tip: %v", got)
	}
	b.eval(`document.getElementById('settings-open').focus(); true`)
	b.waitFor("footer back on focus", opacity(".sode__footer", "1"))
	b.eval(`document.getElementById('settings-open').blur(); true`)
	b.waitFor("footer faded after blur", opacity(".sode__footer", "0"))
	dur := `getComputedStyle(document.querySelector('.sode__footer')).transitionDuration`
	b.waitFor("reveal fades over a quarter second", dur+` === '0.25s'`)
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-reduced-motion", "value": "reduce"}}})
	b.waitFor("reveal snaps under reduced motion", dur+` === '0s'`)
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{}})

	// Centred: the span from Source's left edge to Levels' right edge, at the height of the boxes,
	// sits at the viewport's centre. This check ignores where the title and Presets fall.
	const boxes = `(() => { const a = document.getElementById('src').getBoundingClientRect(), z = document.getElementById('lvl').getBoundingClientRect(); return [(a.left + z.right) / 2 - innerWidth / 2, (a.top + a.bottom) / 2 - innerHeight / 2]; })()`
	b.waitFor("boxes at the viewport centre", `(() => { const [x, y] = `+boxes+`; return Math.abs(x) <= 1 && Math.abs(y) <= 1; })()`)
	const presets = `(() => { const p = document.querySelector('#patch-view [data-presets]'), k = [...p.children].filter((c) => c.getBoundingClientRect().width > 0).map((c) => c.getBoundingClientRect()); return (Math.min(...k.map((r) => r.left)) + Math.max(...k.map((r) => r.right))) / 2 - innerWidth / 2; })()`
	b.waitFor("Presets centred under the boxes", `Math.abs(`+presets+`) <= 1`)
	b.shot("compact")
	const boxesTop = `Math.round(document.getElementById('patch').getBoundingClientRect().top)`
	top := b.eval(boxesTop)
	b.eval(`(() => { const s = document.getElementById('src'); s.focus(); s.value = 'a'; s.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	b.waitFor("match list open", `!document.getElementById('src-menu').hidden`)
	if got := b.eval(boxesTop); got != top {
		t.Fatalf("the match list moved the boxes: top %v, was %v", got, top)
	}
	b.eval(`(() => { const s = document.getElementById('src'); s.value = ''; s.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	b.waitFor("match list closed", `document.getElementById('src-menu').hidden`)
	b.waitFor("Last source chip after reload", `(document.querySelector('[data-last="src"] .sode__last .sode__last-text')?.textContent || '').includes('`+simSrcName+`')`)
	b.waitFor("Last destination chip after reload", `(document.querySelector('[data-last="dst"] .sode__last .sode__last-text')?.textContent || '').includes('DSM1')`)

	key := func(target, code, key, mods string) string {
		return `(() => { const t = document.getElementById('` + target + `'); t.focus(); const e = new KeyboardEvent('keydown', {code: '` + code + `', key: '` + key + `', ` + mods + `, bubbles: true, cancelable: true}); t.dispatchEvent(e); return e.defaultPrevented; })()`
	}
	for _, c := range []string{"s", "d", "f"} {
		b.eval(key("src", "Key"+strings.ToUpper(c), c, "ctrlKey: true"))
	}
	b.waitFor("boxes filled from Last", `document.getElementById('src').value.includes('`+simSrcName+`') && document.getElementById('lvl').value !== ''`)
	b.eval(`document.getElementById('lvl').focus(); document.getElementById('lvl').dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); true`)
	b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("failed take shows in Compact view", `[...document.querySelectorAll('#result .pb-alert--error')].some((n) => n.textContent.includes('Take failed'))`)
	b.waitFor("result shown", shown("#result"))
	if got := b.eval(boxesTop); got != top {
		t.Fatalf("the failed take moved the boxes: top %v, was %v", got, top)
	}
	b.shot("compact-failed")
	// Ctrl+Z still reaches undo: its confirm, or Nothing to undo when the failed take moved nothing.
	b.eval(`document.body.focus(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 'z', code: 'KeyZ', ctrlKey: true, bubbles: true, cancelable: true})); true`)
	b.waitFor("Ctrl+Z answers", `!document.getElementById('confirm').hidden || document.getElementById('result').textContent.includes('Nothing to undo')`)
	if b.eval(`!document.getElementById('confirm').hidden`) == true {
		b.eval(`document.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', code: 'Escape', bubbles: true, cancelable: true})); true`)
		b.waitFor("Esc closes the confirm", `document.getElementById('confirm').hidden`)
	}

	// Esc clears the boxes, then with them empty starts no keyboard mode while History is hidden.
	b.eval(`for (let i = 0; i < 6; i++) { const t = document.getElementById('src'); t.focus(); t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Escape', code: 'Escape', bubbles: true, cancelable: true})); } true`)
	b.waitFor("Esc cleared the boxes", `document.getElementById('src').value === '' && document.getElementById('lvl').value === '' && document.getElementById('dst-chips').hidden`)
	if got := b.eval(`document.getElementById('history').classList.contains('sode__hist--keys') || document.activeElement.id`); got != "src" {
		t.Fatalf("Esc with empty boxes started keyboard mode in Compact view (%v)", got)
	}

	if got := b.eval(key("src", "KeyC", "C", "altKey: true, shiftKey: true")); got != true {
		t.Fatalf("Alt+Shift+C in a box not prevented: %v", got)
	}
	b.waitFor("Alt+Shift+C shows History", shown("#patch-view > .sode__history"))
	b.waitFor("toggle writes This browser off", `localStorage.getItem('sode-compact') === 'off'`)
	b.eval(key("src", "KeyC", "C", "altKey: true, shiftKey: true"))
	b.waitFor("Alt+Shift+C hides History again", `!(`+shown("#patch-view > .sode__history")+`)`)

	b.eval(key("src", "Comma", ",", "ctrlKey: true"))
	b.waitFor("Ctrl+, opens Settings", `document.documentElement.dataset.view === 'settings'`)
	b.waitFor("Compact view shown under This browser", `(() => { const s = document.getElementById('cfg-compact'); return s?.closest('fieldset')?.querySelector('legend')?.textContent === 'This browser' && !!s.offsetParent; })()`)
	b.waitFor("Settings shows this browser's value", `document.getElementById('cfg-compact').value === 'on'`)
	b.waitFor("Global defaults has it too", `!!document.querySelector('select[data-gpref="compact"]')`)
	b.eval(`document.getElementById('account-login').click()`)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click(); true`)
	b.waitFor("signed in", `document.getElementById('account-login').hidden && !document.querySelector('.sode-lock__who').hidden`)
	b.eval(`document.getElementById('settings-back').click(); true`)
	b.waitFor("back on the main page", shown("#patch-view"))
	rest()
	const bar = ".sode-lock__who-bar"
	b.waitFor("Lock bar faded", opacity(bar, "0"))
	b.shot("compact-signed-in")
	if got := b.eval(`getComputedStyle(document.querySelector('.sode-lock__who')).backgroundColor === getComputedStyle(document.body).backgroundColor`); got != true {
		t.Fatal("the faded Lock bar leaves a box: its background differs from the page's")
	}
	hover(".sode-lock__who")
	b.waitFor("Lock bar back on hover", opacity(bar, "1"))
	rest()
	b.waitFor("Lock bar faded again", opacity(bar, "0"))
	near(".sode-lock__who", 100, 80)
	b.waitFor("Lock bar back with the pointer near it", opacity(bar, "1"))
	rest()
	b.waitFor("Lock bar faded once the pointer leaves", opacity(bar, "0"))
	b.eval(`document.querySelector('.sode-lock__who .pb-btn').focus(); true`)
	b.waitFor("Lock bar back on focus", opacity(bar, "1"))
	b.eval(`document.activeElement.blur(); true`)

	// A preset picking from History brings back the usual page.
	b.eval(`document.documentElement.dataset.sodePick = ''; true`)
	b.waitFor("History back while picking", shown("#patch-view > .sode__history"))
	b.waitFor("not centred while picking", `getComputedStyle(document.querySelector('.sode')).display !== 'grid' && getComputedStyle(document.getElementById('result')).position !== 'absolute'`)
	b.eval(`delete document.documentElement.dataset.sodePick; true`)
	b.waitFor("Compact again", `!(`+shown("#patch-view > .sode__history")+`)`)

	stopSim()
	b.waitFor("lost link shows in Compact view", `!document.getElementById('conn-down').hidden && `+shown("#conn-down"))
	b.waitFor("status no longer marked ok, so not faded", `!document.getElementById('conn').classList.contains('sode__conn--ok') && `+opacity("#conn", "1"))
	b.check()
}
