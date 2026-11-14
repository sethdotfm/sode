package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// A patch made through the page is stored per browser: after a reload the Last chips show it, the
// Next chips step from it, and Ctrl+S and Ctrl+Shift+S fill Source from them.
func TestChromeMainLastSurvivesReload(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.waitFor("no Last before any take", `document.querySelector('[data-last="src"]').childElementCount === 0`)

	// Typing then Tab takes the box's best match, as a person would. The input repeats until the
	// page has the router's names.
	pick := func(id, text string) {
		b.waitFor(id+" match list", `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('`+id+`-opt-0'); })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	pick("src", simSrcName)
	pick("dst", "DSM1")
	b.eval(`(() => { const t = document.getElementById('lvl'); t.focus(); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); return true; })()`)
	b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the patch is stored", `JSON.parse(localStorage.getItem('sode-last-take') || 'null')?.src?.[1] === '`+simSrcName+`'`)

	b.open(srv.URL + "/")
	text := func(slot, chip string) string {
		return `(document.querySelector('[data-last="` + slot + `"] ` + chip + ` .sode__last-text')?.textContent || '')`
	}
	b.waitFor("Last source chip after reload", text("src", "button.sode__last:not(.sode__next)")+`.includes('`+simSrcName+`')`)
	b.waitFor("Last destination chip after reload", text("dst", "button.sode__last:not(.sode__next)")+`.includes('DSM1')`)
	b.waitFor("Next source chip steps from Last", text("src", ".sode__next")+`.includes('Zoom 2')`)
	b.waitFor("Next destination chip steps from Last", text("dst", ".sode__next")+`.includes('DSM2')`)
	ctrlS := func(shift string) {
		b.eval(`document.getElementById('src').focus(); document.dispatchEvent(new KeyboardEvent('keydown', {key: 's', code: 'KeyS', ctrlKey: true, shiftKey: ` + shift + `, bubbles: true})); true`)
	}
	ctrlS("false")
	b.waitFor("Ctrl+S fills Source from Last", `document.getElementById('src').value.includes('`+simSrcName+`')`)
	ctrlS("true")
	b.waitFor("Ctrl+Shift+S fills Source from Next", `document.getElementById('src').value.includes('Zoom 2')`)

	b.eval(`localStorage.setItem('sode-last-take', '{broken'); true`)
	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	b.waitFor("a corrupt value is no Last", `document.querySelector('[data-last="src"]').childElementCount === 0`)
	b.check()
}
