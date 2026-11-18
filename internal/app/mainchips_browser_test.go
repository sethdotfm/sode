package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// The Last chip has the look of the destination chips under the box. A Ctrl-click on a History
// destination or a Destination match row adds it as + does. On a Mac, a Ctrl-click is a
// contextmenu event. A plain click still fills the box.
func TestChromeMainChips(t *testing.T) {
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
	b.waitFor("History has the take", `!!document.querySelector('#history .sode__end--dst.sode__end--pick')`)
	b.waitFor("Last destination chip", `!!document.querySelector('[data-last="dst"] button.sode__last:not(.sode__next)')`)

	// Last takes the edge, text size and padding of a destination chip in the same box.
	if got := b.eval(`(() => {
		const st = (n) => { const c = getComputedStyle(n); return [c.borderTopColor, c.fontSize, c.paddingLeft, c.paddingTop].join(' '); };
		const box = document.getElementById('dst-chips');
		const chip = document.createElement('span');
		chip.className = 'pb-chip pb-chip--static';
		chip.textContent = '1 DSM1';
		box.hidden = false;
		box.append(chip);
		const want = st(chip);
		chip.remove();
		box.hidden = true;
		const last = st(document.querySelector('[data-last="dst"] button.sode__last:not(.sode__next)'));
		return last === want || last + ' / ' + want;
	})()`); got != true {
		t.Fatalf("Last chip and destination chip differ (Last / destination): %v", got)
	}

	// Ctrl-click: a click with ctrlKey, or on a Mac the contextmenu that a Ctrl-click fires instead.
	ctrlClick := func(sel string) string {
		return `(() => { const n = ` + sel + `; const mac = /Mac|iPhone|iPad/.test(navigator.platform);
			n.dispatchEvent(new MouseEvent('mousedown', { bubbles: true, cancelable: true, ctrlKey: true }));
			n.dispatchEvent(new MouseEvent(mac ? 'contextmenu' : 'click', { bubbles: true, cancelable: true, ctrlKey: true }));
			return true; })()`
	}
	chips := `[...document.querySelectorAll('#dst-chips > .pb-chip')].map((c) => c.firstChild.textContent).join('|')`

	pick("dst", "DSM2")
	b.eval(ctrlClick(`document.querySelector('#history .sode__end--dst.sode__end--pick')`))
	b.waitFor("Ctrl-click on History adds DSM1 beside DSM2", chips+`.includes('DSM2') && `+chips+`.includes('DSM1')`)
	if got := b.eval(`document.getElementById('dst').value`); got != "" {
		t.Fatalf("Destination box holds %q after the Ctrl-click, want it empty", got)
	}

	b.waitFor("DSM3 match list", `(() => { const t = document.getElementById('dst'); t.focus(); t.value = 'DSM3'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('dst-opt-0'); })()`)
	b.eval(ctrlClick(`document.getElementById('dst-opt-0')`))
	b.waitFor("Ctrl-click on a match row adds DSM3", chips+`.includes('DSM3')`)

	b.eval(`document.querySelector('#history .sode__end--dst.sode__end--pick').click(); true`)
	b.waitFor("a plain click fills the box", `document.getElementById('dst').value.includes('DSM1')`)

	// A folder's "N destinations" chip adds every destination, with a plain click.
	pick("src", simSrcName)
	b.eval(`(() => { const t = document.getElementById('lvl'); t.focus(); t.value = '1'; t.dispatchEvent(new Event('input', {bubbles: true})); t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); return true; })()`)
	b.waitFor("confirm unlocks again", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	// The folder's own chip, not any earlier row: History redraws once per frame, so it can come a frame late.
	folderChip := `[...document.querySelectorAll('#history .sode__hist-route button.sode__end--dst')].find((n) => /destinations/.test(n.textContent))`
	b.waitFor("History has the folder", `!!`+folderChip)
	b.eval(`document.querySelectorAll('#dst-chips .pb-chip__remove').forEach((x) => x.click()); true`)
	b.waitFor("chips empty", chips+` === ''`)
	b.eval(folderChip + `.click(); true`)
	b.waitFor("the folder chip adds all three", chips+`.includes('DSM1') && `+chips+`.includes('DSM2') && `+chips+`.includes('DSM3')`)

	b.check()
}
