package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// mainWithLast opens the desktop main page at 1920 by 1080 after one take from Zoom 1 to DSM1. Then Last and Next
// show in the Source and Destination boxes.
func mainWithLast(t *testing.T) *tab {
	t.Helper()
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

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
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
	b.waitFor("Next destination chip", `!!document.querySelector('[data-last="dst"] .sode__next')`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	b.eval(`document.activeElement.blur(); true`)
	return b
}

// The Last and Next chips show no key hint: the key is in the tooltip and the keys list. The open Next chip is as
// wide as its tag and its value, and no wider.
func TestChromeMainNextChipNoKeys(t *testing.T) {
	b := mainWithLast(t)
	b.shot("closed")
	b.eval(`document.documentElement.dataset.shift = ''; true`)
	// The open part grows over the normal motion time; 400 ms is past its end.
	b.eval(`new Promise((r) => setTimeout(() => r(true), 400))`)
	b.shot("open")

	if got := b.eval(`document.querySelectorAll('[data-last] kbd').length`); got != float64(0) {
		t.Fatalf("Last and Next chips hold %v kbd, want none", got)
	}
	if got := b.eval(`[...document.querySelectorAll('[data-last] button')].every((n) => /\((Ctrl|⌃)/.test(n.dataset.pbTip || ''))`); got != true {
		t.Fatal("a Last or Next chip has no key in its tooltip")
	}
	if got := b.eval(`(() => {
		const n = document.querySelector('[data-last="dst"] .sode__next');
		const c = getComputedStyle(n);
		const w = (s) => n.querySelector(s).getBoundingClientRect().width;
		const frame = ['paddingLeft', 'paddingRight', 'borderLeftWidth', 'borderRightWidth'].reduce((s, k) => s + parseFloat(c[k]), 0);
		const room = w('.sode__tag') + w('.sode__last-text') + frame + 2 * parseFloat(c.columnGap || c.gap || 0);
		const r = n.getBoundingClientRect().width;
		return r <= room + 1 || [r, room].join(' > ');
	})()`); got != true {
		t.Fatalf("the open Next chip is wider than its tag and its value: %v", got)
	}
	b.check()
}
