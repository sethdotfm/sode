package app

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
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

// A pointer held still for 1 s on the Next chip keeps it open: it does not open and close over and over. On alpha the
// centre strobed, and on main the left edge did. Opening only adds room on the left, so the chip never moves its edge
// away from the pointer. With reduced motion the chip opens and closes with no transition.
func TestChromeMainNextChipHoldsOpen(t *testing.T) {
	b := mainWithLast(t)
	r := b.eval(`(() => { const r = document.querySelector('[data-last="dst"] .sode__next').getBoundingClientRect(); return [r.left, r.top, r.right, r.bottom]; })()`).([]any)
	l, tp, rt, bt := r[0].(float64), r[1].(float64), r[2].(float64), r[3].(float64)
	cy := (tp + bt) / 2
	move := func(x, y float64) {
		b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y})
	}
	// Open is what the person sees: the Next tag in the accent, not in Last's muted color. Neither has a transition.
	state := `(() => { const c = (s) => getComputedStyle(document.querySelector('[data-last="dst"] ' + s + ' > .sode__tag')).color; return c('.sode__next') !== c('button.sode__last:not(.sode__next)') ? 'open' : 'shut'; })()`
	for _, p := range []struct {
		name string
		x, y float64
	}{{"centre", (l + rt) / 2, cy}, {"left edge", l + 0.5, cy}} {
		move(10, 700)
		b.waitFor("the chip is shut", state+` === 'shut'`)
		move(p.x, p.y)
		var seq []string
		for start := time.Now(); time.Since(start) < time.Second; time.Sleep(25 * time.Millisecond) {
			seq = append(seq, b.eval(state).(string))
		}
		// The first samples can come before the hover lands. After the chip opens it must stay open.
		opened := -1
		for i, s := range seq {
			if s == "open" && opened < 0 {
				opened = i
			}
			if s != "open" && opened >= 0 {
				t.Fatalf("pointer still on the %s: the chip opened at sample %d, then sample %d of %d is %q: %v", p.name, opened, i, len(seq), s, seq)
			}
		}
		if opened < 0 || opened > 4 {
			t.Fatalf("pointer still on the %s: the chip opened at sample %d of %d, want in the first 100 ms: %v", p.name, opened, len(seq), seq)
		}
	}
	if got := b.eval(`document.querySelector('[data-last="dst"] .sode__next .sode__last-text').getBoundingClientRect().width > 0`); got != true {
		t.Fatal("the held chip does not show its value")
	}

	move(10, 700)
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-reduced-motion", "value": "reduce"}}})
	if got := b.eval(`(() => {
		const q = (s) => getComputedStyle(document.querySelector('[data-last="dst"] ' + s)).transitionDuration.split(',').every((d) => parseFloat(d) === 0);
		return q('.sode__next-open') && q('button.sode__last:not(.sode__next)');
	})()`); got != true {
		t.Fatal("with reduced motion, the Next chip or Last still has a transition")
	}
	b.check()
}

// The Last chip of the Levels box shows "Last 0–10" whole with two routers, also on a narrow window. There it runs on
// past the text box over the blank head of Take and Clear, never over the Levels label. A press on that part still
// reaches the chip.
func TestChromeMainLevelsLastFits(t *testing.T) {
	b := startBrowser(t)
	a := simRoutersApp(t, []string{startSim(t), startSim(t)}, func(c *config.Config) { c.Routers[0].LevelNumbering = "0" })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitRouterRead(t, a, ana, routerUIDs(a)[0], "DSM1")

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
	b.eval(`(() => { const t = document.getElementById('lvl'); t.focus(); t.value = '0-10'; t.dispatchEvent(new Event('input', {bubbles: true})); t.dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); return true; })()`)
	b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("the Levels box's Last chip says 0–10", `document.querySelector('[data-last="lvl"] .sode__last-text')?.textContent === '0–10'`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)
	b.eval(`document.activeElement.blur(); true`)

	for _, w := range []int{1920, 1100, 900} {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
		b.eval(`new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(() => r(true))))`)
		if got := b.eval(`(() => {
			const chip = document.querySelector('[data-last="lvl"] > .sode__last:not(.sode__next)');
			const text = chip.querySelector('.sode__last-text');
			const c = chip.getBoundingClientRect();
			const label = document.querySelector('.sode__box--levels .sode__label').getBoundingClientRect();
			const clear = document.getElementById('clear-all').getBoundingClientRect();
			const bad = [];
			if (text.scrollWidth > text.clientWidth) bad.push('the text is cut: ' + text.clientWidth + ' of ' + text.scrollWidth + ' px');
			if (c.left < label.right) bad.push('the chip is over the Levels label');
			if (c.right > clear.right + 0.5) bad.push('the chip runs past Clear');
			const at = document.elementFromPoint(c.right - 4, (c.top + c.bottom) / 2);
			if (!chip.contains(at)) bad.push('a press at its right end reaches ' + (at?.className || at?.tagName));
			return bad.length === 0 || bad.join('; ');
		})()`); got != true {
			t.Fatalf("at %d px the Levels box's Last chip does not fit: %v", w, got)
		}
	}
	b.check()
}
