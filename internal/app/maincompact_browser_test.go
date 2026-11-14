package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// Compact view hides only History: after a reload the Last chips fill from the stored Last, a
// failed take and a lost link still show, Alt+Shift+C toggles it from a box, and Settings shows it
// under This browser.
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

	ana := userKey(t, a, "Ana")
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}

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
	b.waitFor("Last source chip after reload", `(document.querySelector('[data-last="src"] .sode__last .sode__last-text')?.textContent || '').includes('`+simSrcName+`')`)
	b.waitFor("Last destination chip after reload", `(document.querySelector('[data-last="dst"] .sode__last .sode__last-text')?.textContent || '').includes('DSM1')`)

	key := func(target, code string, mods string) string {
		return `(() => { const t = document.getElementById('` + target + `'); t.focus(); const e = new KeyboardEvent('keydown', {code: '` + code + `', key: '` + strings.ToLower(code[len(code)-1:]) + `', ` + mods + `, bubbles: true, cancelable: true}); t.dispatchEvent(e); return e.defaultPrevented; })()`
	}
	for _, c := range []string{"KeyS", "KeyD", "KeyF"} {
		b.eval(key("src", c, "ctrlKey: true"))
	}
	b.waitFor("boxes filled from Last", `document.getElementById('src').value.includes('`+simSrcName+`') && document.getElementById('lvl').value !== ''`)
	b.eval(`document.getElementById('lvl').focus(); document.getElementById('lvl').dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); true`)
	b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("failed take shows in Compact view", `[...document.querySelectorAll('#result .pb-alert--error')].some((n) => n.textContent.includes('Take failed'))`)
	b.waitFor("result shown", shown("#result"))
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

	if got := b.eval(key("src", "KeyC", "altKey: true, shiftKey: true")); got != true {
		t.Fatalf("Alt+Shift+C in a box not prevented: %v", got)
	}
	b.waitFor("Alt+Shift+C shows History", shown("#patch-view > .sode__history"))
	b.waitFor("toggle writes This browser off", `localStorage.getItem('sode-compact') === 'off'`)
	b.eval(key("src", "KeyC", "altKey: true, shiftKey: true"))
	b.waitFor("Alt+Shift+C hides History again", `!(`+shown("#patch-view > .sode__history")+`)`)

	b.eval(strings.Replace(key("src", "Comma", "ctrlKey: true"), "key: 'a'", "key: ','", 1))
	b.waitFor("Ctrl+, opens Settings", `document.documentElement.dataset.view === 'settings'`)
	b.waitFor("Compact view under This browser", `document.getElementById('cfg-compact')?.closest('fieldset')?.querySelector('legend')?.textContent === 'This browser'`)
	b.waitFor("Settings shows this browser's value", `document.getElementById('cfg-compact').value === 'on'`)
	b.waitFor("Global defaults has it too", `!!document.querySelector('select[data-gpref="compact"]')`)
	b.eval(`document.getElementById('settings-back').click(); true`)
	b.waitFor("back on the main page", shown("#patch-view"))

	stopSim()
	b.waitFor("lost link shows in Compact view", `!document.getElementById('conn-down').hidden && `+shown("#conn-down"))
	b.check()
}
