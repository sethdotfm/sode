package app

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Ctrl+Z steps back through this tab's takes and Ctrl+Shift+Z goes forward again (stepback.js). A newer take over the
// API is never a step. A reload keeps both lists.
func TestChromeMainStepBack(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	key := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, key)

	b.open(srv.URL + "/")
	b.waitFor("Source box", `!!document.getElementById('src')`)
	pick := func(id, text string) {
		b.waitFor(id+" match list", `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('`+id+`-opt-0'); })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	confirm := func(what, verb, dst string) {
		b.waitFor(what, `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm `+verb+`' && /`+dst+`\b/.test(document.getElementById('confirm').textContent) && !document.getElementById('confirm-ok').disabled`)
		b.eval(`document.getElementById('confirm-ok').click(); true`)
		b.waitFor(what+" closes", `document.getElementById('confirm').hidden`)
	}
	own := `JSON.parse(sessionStorage.getItem('sode-own-takes') || '[]').length`
	take := func(src, dst string, n int) {
		pick("src", src)
		pick("dst", dst)
		b.eval(`(() => { const t = document.getElementById('lvl'); t.focus(); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); return true; })()`)
		confirm("take to "+dst, "take", dst)
		b.waitFor("the entry is this tab's", own+` === `+strconv.Itoa(n))
	}
	// Ctrl+Z, or Cmd+Z on a Mac, from the page itself.
	press := func(shift bool) {
		s := "false"
		if shift {
			s = "true"
		}
		b.eval(`(() => { const mac = /Mac/.test(navigator.platform); document.body.focus(); document.body.dispatchEvent(new KeyboardEvent('keydown', {key: ` + map[bool]string{false: `'z'`, true: `'Z'`}[shift] + `, code: 'KeyZ', ctrlKey: !mac, metaKey: mac, shiftKey: ` + s + `, bubbles: true, cancelable: true})); return true; })()`)
	}

	take(simSrcName, "DSM1", 1)
	take(simSrcName, "DSM2", 2)
	// A newer take from another client is never a step here.
	if rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":3,"dst":4,"levels":[1]}]}`, nil); rec.Code != 200 {
		t.Fatalf("API take: %d %s", rec.Code, rec.Body)
	}
	b.waitFor("the API take in History", `document.querySelectorAll('#history .sode__hist-row').length >= 3`)

	press(false)
	confirm("Ctrl+Z undoes DSM2", "undo", "DSM2")
	b.waitFor("one step to redo", `JSON.parse(sessionStorage.getItem('sode-redo-steps') || '[]').length === 1`)
	press(false)
	confirm("a second Ctrl+Z undoes DSM1", "undo", "DSM1")
	b.waitFor("the second undo is in", own+` === 4`)
	press(false)
	b.waitFor("nothing left to undo", `document.getElementById('result').textContent.includes('Nothing to undo')`)

	b.open(srv.URL + "/")
	b.waitFor("Source box after reload", `!!document.getElementById('src') && document.querySelectorAll('#history .sode__hist-row').length >= 5`)
	press(true)
	confirm("Ctrl+Shift+Z redoes DSM1", "redo", "DSM1")
	press(true)
	confirm("a second Ctrl+Shift+Z redoes DSM2", "redo", "DSM2")
	b.waitFor("the redo list is empty", `JSON.parse(sessionStorage.getItem('sode-redo-steps') || '[]').length === 0`)
	press(true)
	b.waitFor("nothing left to redo", `document.getElementById('result').textContent.includes('Nothing to redo')`)

	// A new take after an undo clears the redo list.
	b.waitFor("both redos are in", own+` === 6`)
	press(false)
	confirm("Ctrl+Z undoes the redo of DSM2", "undo", "DSM2")
	b.waitFor("one step to redo", `JSON.parse(sessionStorage.getItem('sode-redo-steps') || '[]').length === 1`)
	take(simSrcName, "DSM3", 8)
	b.waitFor("the take cleared the redo list", `JSON.parse(sessionStorage.getItem('sode-redo-steps') || '[]').length === 0`)
	b.check()
}
