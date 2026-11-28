package app

import (
	"fmt"
	"math"
	"regexp"
	"testing"
)

func regexpMatch(pattern, s string) bool {
	ok, _ := regexp.MatchString(pattern, s)
	return ok
}

const (
	firstStrip = `document.getElementById('log-uptime-bars')`
	stripBars  = `[...document.getElementById('log-uptime-bars').querySelectorAll('.sode-up__bar')]`
	focusedBar = stripBars + `.indexOf(document.activeElement)`
)

// Each uptime strip is one Tab stop with the arrow keys inside it, and every bar has a name. The picked bar keeps its
// color and No data bars show. The words around the strip name one thing one way, and a bar's tip stays inside the strip.
func TestChromeLogUptimeStrip(t *testing.T) {
	b := logTablePage(t, mouseFlags...)
	b.waitFor("both strips", `document.querySelectorAll('.sode-up__bars[data-router]').length === 2 && [...document.querySelectorAll('.sode-up__bars[data-router]')].every((s) => s.childElementCount > 0)`)

	// Every bar has a name: its tip text.
	if bad := b.eval(`[...document.querySelectorAll('.sode-up__bars button')].filter((x) => !x.getAttribute('aria-label') || x.getAttribute('aria-label') !== x.dataset.pbTip).length`).(float64); bad != 0 {
		t.Errorf("%v bars with no name", bad)
	}

	// Tab from the last range chip to the first Show chip: one stop for each strip.
	b.eval(`document.querySelector('#log-uptime-presets > :last-child').focus(); true`)
	stops, strips := 0, map[string]bool{}
	for i := 0; i < 600; i++ {
		realKey(b, "Tab", "Tab", 9)
		at := b.eval(`(() => { const a = document.activeElement; return a.closest('#log-kinds') ? 'show' : a.classList.contains('sode-up__bar') ? 'bar ' + a.parentElement.dataset.router : 'other ' + a.outerHTML.slice(0, 60); })()`).(string)
		if at == "show" {
			break
		}
		if at[:3] != "bar" {
			t.Fatalf("Tab stop %d is %s", i+1, at)
		}
		stops++
		strips[at] = true
	}
	if stops != 2 || len(strips) != 2 {
		t.Errorf("%d Tab stops on %d strips between the range chips and Show, want 2 on 2", stops, len(strips))
	}

	// The stop is the newest bar; the arrows, Home and End move in the strip and take the Tab stop with them.
	b.eval(`document.querySelector('#log-uptime-presets > :last-child').focus(); true`)
	realKey(b, "Tab", "Tab", 9)
	n := int(b.eval(stripBars + `.length`).(float64))
	if got := int(b.eval(focusedBar).(float64)); got != n-1 {
		t.Errorf("Tab lands on bar %d of %d, want the newest", got, n)
	}
	moves := []struct {
		key, code string
		vk, want  int
	}{{"ArrowLeft", "ArrowLeft", 37, n - 2}, {"Home", "Home", 36, 0}, {"ArrowLeft", "ArrowLeft", 37, 0}, {"ArrowRight", "ArrowRight", 39, 1}, {"End", "End", 35, n - 1}}
	for _, m := range moves {
		realKey(b, m.key, m.code, m.vk)
		if got := int(b.eval(focusedBar).(float64)); got != m.want {
			t.Errorf("%s: focus on bar %d, want %d", m.key, got, m.want)
		}
		if got := b.eval(stripBars + `.filter((x) => x.tabIndex === 0).map((x) => ` + stripBars + `.indexOf(x)).join()`).(string); got != fmt.Sprint(m.want) {
			t.Errorf("%s: Tab stop on bars %q, want %d", m.key, got, m.want)
		}
	}

	// A bar's tip opens over the strip and stays inside its edges at both ends.
	if bad := b.eval(`(() => {
		const bs = ` + stripBars + `, n = bs.length;
		return bs.filter((x, i) => !x.classList.contains('pb-tip--top') || x.classList.contains('pb-tip--start') !== (i < n / 3) || x.classList.contains('pb-tip--end') !== (i >= n - n / 3)).length;
	})()`).(float64); bad != 0 {
		t.Errorf("%v bars with the wrong tip side", bad)
	}
	b.eval(`document.activeElement.blur(); true`)
	for _, at := range []string{"0", "n - 1"} {
		r := b.eval(`(() => { const bs = ` + stripBars + `, n = bs.length, r = bs[` + at + `].getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`).([]any)
		b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": r[0], "y": r[1]})
		b.waitFor("the bar's tip", `!document.querySelector('.sode__tip').hidden`)
		if bad := b.eval(`(() => {
			const tip = document.querySelector('.sode__tip').getBoundingClientRect(), s = ` + firstStrip + `.getBoundingClientRect();
			const out = [];
			if (tip.left < s.left - 0.5 || tip.right > s.right + 0.5) out.push('x ' + tip.left.toFixed(1) + ' to ' + tip.right.toFixed(1) + ', strip ' + s.left.toFixed(1) + ' to ' + s.right.toFixed(1));
			if (tip.bottom > s.top + 0.5) out.push('tip bottom ' + tip.bottom.toFixed(1) + ' under the strip top ' + s.top.toFixed(1));
			return out.join('; ');
		})()`).(string); bad != "" {
			t.Errorf("bar %s tip: %s", at, bad)
		}
		if at == "n - 1" {
			b.shot("tip")
		}
	}
	if tip := b.eval(stripBars + `.at(-1).dataset.pbTip`).(string); !regexpMatch(`^\d{4}-\d\d-\d\d \d\d:\d\d to \d\d:\d\d: `, tip) {
		t.Errorf("newest bar tip %q, want the date once and to", tip)
	}
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 5, "y": 1070})

	// The words: the chips name the range only, the chip tip says how long a bar is, the count says what it counts.
	if got := b.eval(`[...document.querySelectorAll('#log-uptime-presets > button')].map((x) => x.textContent + '|' + x.dataset.pbTip).join(', ')`).(string); got != "1 hour|Each bar is 1 min, 24 hours|Each bar is 15 min, 7 days|Each bar is 1 hour, 30 days|Each bar is 6 hours" {
		t.Errorf("range chips %q", got)
	}
	if got := b.eval(`document.getElementById('log-count').textContent`).(string); !regexpMatch(`^\d+ rows shown, [\d,]+ in the log$`, got) {
		t.Errorf("count %q", got)
	}
	if got := b.eval(`document.querySelector('.sode-log__legend .sode-up__bar--warn').parentElement.dataset.pbTip`).(string); got != "Down for part of the bar's time, or messages failed or were slow" {
		t.Errorf("Partial tip %q", got)
	}

	// No data shows on the page: an outline, and not yet differs from it.
	if bad := b.eval(`(() => {
		const key = getComputedStyle(document.querySelector('.sode-log__legend .sode-up__bar--none'));
		const f = document.createElement('span');
		f.className = 'sode-up__bar sode-up__bar--future';
		document.body.append(f);
		const fut = getComputedStyle(f).boxShadow;
		f.remove();
		const out = [];
		if (key.backgroundColor !== 'rgba(0, 0, 0, 0)' || !key.boxShadow.includes('inset')) out.push('No data mark ' + key.backgroundColor + ' ' + key.boxShadow);
		if (fut === key.boxShadow) out.push('not yet looks like No data');
		return out.join('; ');
	})()`).(string); bad != "" {
		t.Error(bad)
	}

	// A picked bar keeps its whole color: the ring is outside it. The strip's range chip stays pressed.
	b.eval(stripBars + `.at(-2).click(); true`)
	b.waitFor("the One bar note", `!document.getElementById('log-slot').hidden && !!document.querySelector('#log-uptime-bars .sode-up__bar--sel')`)
	b.shot("picked")
	if bad := b.eval(`(() => {
		const sel = document.querySelector('#log-uptime-bars .sode-up__bar--sel'), s = getComputedStyle(sel);
		const twin = [...sel.parentElement.children].find((x) => x !== sel && x.className === sel.className.replace(' sode-up__bar--sel', ''));
		const want = twin ? getComputedStyle(twin).boxShadow : 'no unpicked bar of its color';
		return s.boxShadow === want && s.outlineStyle === 'solid' && parseFloat(s.outlineWidth) > 0 ? '' : 'shadow ' + s.boxShadow + ' (unpicked ' + want + '), outline ' + s.outlineStyle + ' ' + s.outlineWidth;
	})()`).(string); bad != "" {
		t.Errorf("picked bar: %s", bad)
	}
	if got := b.eval(`[...document.querySelectorAll('#log-uptime-presets > [aria-pressed="true"]')].map((x) => x.textContent).join()`).(string); got != "24 hours" {
		t.Errorf("pressed range chips in a bar's view %q, want 24 hours", got)
	}
	if got := b.eval(`(() => { const n = document.getElementById('log-slot'); return n.querySelector('strong').textContent + '|' + n.querySelector('button').textContent; })()`).(string); got != "One bar|Show all bars" {
		t.Errorf("bar note %q", got)
	}
	// The note is a box like the other notes, not loose text.
	pad := b.tokenPx("#log-slot", "--pb-space-3")
	if got := b.eval(`(() => { const s = getComputedStyle(document.getElementById('log-slot')); return [parseFloat(s.paddingTop), parseFloat(s.paddingLeft), parseFloat(s.borderTopWidth)]; })()`).([]any); math.Abs(got[0].(float64)-pad) > layoutTol || got[1].(float64) < pad || got[2].(float64) <= 0 {
		t.Errorf("bar note padding and border %v, want %.1f px padding and a border", got, pad)
	}
	b.check()
}

// /log on a phone: the page fits 375 px with one right edge, touch-sized controls, wider bars and native scroll bars.
func TestChromeLogPhoneLayout(t *testing.T) {
	b := startBrowser(t)
	url := logTableURL(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 375, "height": 812, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.call("Emulation.setScrollbarsHidden", map[string]any{"hidden": true})
	b.open(url + "/log")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	waitLogRows(b)
	b.waitFor("the bars", `document.querySelectorAll('#log-uptime-bars .sode-up__bar').length > 0`)
	b.shot("phone")

	if w := b.eval(`document.documentElement.scrollWidth - innerWidth`).(float64); w > 0.5 {
		t.Errorf("the page scrolls sideways by %.1f px", w)
	}
	b.noClip("page title", ".sode-log__title")
	// One right edge for the time fields and the text filters.
	if bad := b.eval(`(() => {
		const r = ['#log-from', '#log-to', '#log-src', '#log-dst'].map((s) => [s, document.querySelector(s).getBoundingClientRect().right]);
		return r.some(([, x]) => Math.abs(x - r[0][1]) > 0.5) ? r.map(([s, x]) => s + ' ' + x.toFixed(1)).join(', ') : '';
	})()`).(string); bad != "" {
		t.Errorf("right edges: %s", bad)
	}
	b.sameLeft("time fields", "#log-from", "#log-to")
	b.sameMiddle("download buttons", "#log-download-rows", "#log-download-full")
	h := b.tokenPx(".sode-log__filters", "--pb-control-h")
	if bad := b.eval(`[...document.querySelectorAll('.sode-log__chip, .sode-log__filters .pb-btn')].filter((x) => x.getClientRects().length && x.getBoundingClientRect().height < ` + fmt.Sprint(h) + ` - 0.5).map((x) => x.textContent + ' ' + x.getBoundingClientRect().height.toFixed(1)).join(', ')`).(string); bad != "" {
		t.Errorf("controls under %.1f px high: %s", h, bad)
	}
	if b.eval(`!!document.querySelector('#log-kinds .sode-log__sep').getClientRects().length`).(bool) {
		t.Error("the Show divider shows on the phone")
	}
	if b.eval(`!!document.querySelector('.sode-log__table th[data-sort="hex"]').getClientRects().length`).(bool) {
		t.Error("the Bytes column shows on the phone")
	}
	// Fewer, wider bars: each one can take a tap.
	if w := b.eval(`Math.min(...[...document.querySelectorAll('#log-uptime-bars .sode-up__bar')].map((x) => x.getBoundingClientRect().width))`).(float64); w < 7 {
		t.Errorf("bars %.1f px wide on the phone", w)
	}
	// The table keeps the native bar.
	if b.eval(`[...document.querySelectorAll('.sode-bigscroll')].some((x) => !x.hidden && x.getClientRects().length)`).(bool) {
		t.Error("a big scroll bar shows on the phone")
	}
	if s := b.eval(`getComputedStyle(document.querySelector('.sode-log__scroll')).overflowX`).(string); s != "auto" {
		t.Errorf("table scroller overflow-x %q", s)
	}
	b.check()
}
