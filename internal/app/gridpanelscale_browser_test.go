package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// gridNodesMax is the most nodes a Grid panel page holds at 1280 by 800, whatever the router's size. That is the
// crosspoints that show plus overscan, the names and the pinned bands. All 2500 by 750 would be 1.9 million.
const gridNodesMax = 4000

// gridSweep scrolls the grid corner to corner in 60 steps. It counts the elements added and removed, the most nodes, the
// steps with no crosspoint mid-view, and the median and worst step time.
const gridSweep = `(async () => {
	const grid = document.getElementById('panel-grid');
	const sc = grid.querySelector('.sode-gridpanel__scroll');
	const count = () => grid.querySelectorAll('*').length;
	let added = 0, removed = 0, most = count();
	const els = (list) => [...list].filter((n) => n.nodeType === 1).length;
	const mo = new MutationObserver((recs) => {
		for (const r of recs) { added += els(r.addedNodes); removed += els(r.removedNodes); }
	});
	mo.observe(grid, { childList: true, subtree: true });
	const frame = () => new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
	const steps = 60, times = [];
	let missing = 0;
	for (let i = 1; i <= steps; i++) {
		const t0 = performance.now();
		sc.scrollLeft = (sc.scrollWidth - sc.clientWidth) * i / steps;
		sc.scrollTop = (sc.scrollHeight - sc.clientHeight) * i / steps;
		await frame();
		times.push(performance.now() - t0);
		most = Math.max(most, count());
		// The crosspoint in the middle of the view is drawn at every step.
		const r = sc.getBoundingClientRect();
		const hit = document.elementsFromPoint(r.left + r.width * 0.6, r.top + r.height * 0.6);
		if (!hit.some((n) => n.classList?.contains('sode-gridpanel__xp'))) missing++;
	}
	mo.takeRecords().forEach((r) => { added += els(r.addedNodes); removed += els(r.removedNodes); });
	mo.disconnect();
	times.sort((x, y) => x - y);
	return { added, removed, most, missing, median: times[times.length >> 1], worst: times[times.length - 1] };
})()`

// bandSweep wheels the KIND band ('dsts' or 'srcs') one pin per step to its end and back, each step until it shows. It
// counts the elements added and removed and the steps, and gives the median and worst step time.
const bandSweep = `(async () => {
	const grid = document.getElementById('panel-grid');
	const sc = grid.querySelector('.sode-gridpanel__scroll');
	const y = 'KIND' === 'dsts';
	const pinned = () => [...grid.querySelectorAll('.sode-gridpanel__corner .sode-gridpanel__' + (y ? 'dst' : 'src') + ':not([hidden])')];
	const first = () => Math.min(...pinned().map((n) => Number(y ? n.dataset.dst : n.dataset.src)));
	const xp = document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width;
	// The band's first pin: its pools are in no order, and the part pin past the band is drawn too.
	const r = pinned().map((n) => n.getBoundingClientRect()).sort((p, q) => (y ? p.top - q.top : p.left - q.left))[0];
	const at = y ? { clientX: r.right + 200, clientY: r.bottom - 2 } : { clientX: r.right - 2, clientY: r.bottom + 200 };
	const top = sc.scrollTop, left = sc.scrollLeft, was = first();
	let added = 0, removed = 0, moved = false;
	const els = (list) => [...list].filter((n) => n.nodeType === 1).length;
	const mo = new MutationObserver((recs) => { for (const q of recs) { added += els(q.addedNodes); removed += els(q.removedNodes); } });
	mo.observe(grid, { childList: true, subtree: true });
	const frame = () => new Promise((res) => requestAnimationFrame(() => requestAnimationFrame(res)));
	const times = [];
	for (const dir of [1, -1]) {
		for (let k = 0; k < 60; k++) {
			const before = first();
			const t0 = performance.now();
			sc.dispatchEvent(new WheelEvent('wheel', { ...at, deltaY: y ? dir * xp : 0, deltaX: y ? 0 : dir * xp, bubbles: true, cancelable: true }));
			// A step of one pin glides (gridpanelpins.js), so it waits for the band to reach the next pin.
			for (let f = 0; f < 15 && first() === before; f++) await frame();
			if (first() === before) break;
			moved = true;
			times.push(performance.now() - t0);
		}
	}
	mo.takeRecords().forEach((q) => { added += els(q.addedNodes); removed += els(q.removedNodes); });
	mo.disconnect();
	times.sort((p, q) => p - q);
	return { added, removed, moved, back: first() === was, still: sc.scrollTop === top && sc.scrollLeft === left, steps: times.length, median: times[times.length >> 1], worst: times[times.length - 1] };
})()`

// A Grid panel on a 2500 by 750 router. The page holds a bounded number of nodes and adds or removes none while
// it scrolls corner to corner: nodes are moved, never made again. Pinned names stay at the edge. The open time
// and the time for each scroll step are reported, not asserted.
func TestChromeGridPanelScale(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t, "SIM_SOURCES=2500", "SIM_DESTS=750")
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	const pinSrc, pinDst = 1200, 400
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Big grid","type":"grid","settings":{"mode":"take","levels":[1]},
		"sources":{"members":"all","items":{"%d":{"pinned":true}}},"destinations":{"members":"all","items":{"%d":{"pinned":true}}}}`, pinSrc, pinDst))
	waitNamesAndTally(t, a, ana)
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(100 * time.Millisecond) {
		body := apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String()
		if strings.Contains(body, `"SRC 2500"`) && strings.Contains(body, `"MV 737"`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read all 2500 by 750 names")
		}
	}

	start := time.Now()
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", `document.querySelector('.sode-gridpanel__src[data-src="0"]')?.textContent === 'Black'`)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	t.Logf("open to first grid: %v (includes the sign-in)", time.Since(start).Round(time.Millisecond))
	b.shot("scale")

	// Pinned items come first; the scroller is as large as the whole grid.
	b.waitFor("the pinned source and destination lead", `(() => {
		const s = document.querySelector('.sode-gridpanel__src[data-src="`+fmt.Sprint(pinSrc)+`"]');
		const d = document.querySelector('.sode-gridpanel__dst[data-dst="`+fmt.Sprint(pinDst)+`"]');
		const c = document.querySelector('.sode-gridpanel__src[data-src="0"]');
		return s && d && s.getBoundingClientRect().left < c.getBoundingClientRect().left;
	})()`)
	b.waitFor("a line marks the end of each pinned band", `(() => {
		const after = (n) => getComputedStyle(document.querySelector(n), '::after');
		return parseFloat(after('.sode-gridpanel__top').height) > 2 && parseFloat(after('.sode-gridpanel__side').width) > 2;
	})()`)
	b.waitFor("the scroller spans the whole grid", `(() => {
		const sc = document.querySelector('.sode-gridpanel__scroll');
		const xp = document.querySelector('.sode-gridpanel__src[data-src="0"]').getBoundingClientRect().width;
		return sc.scrollWidth >= 2500 * xp && sc.scrollHeight >= 750 * xp;
	})()`)

	sweep, ok := b.eval(gridSweep).(map[string]any)
	if !ok {
		t.Fatal("the sweep returned nothing")
	}
	t.Logf("scroll sweep: median step %.1f ms, worst %.1f ms (two frames each), most nodes %v", sweep["median"], sweep["worst"], sweep["most"])
	if sweep["added"].(float64) != 0 || sweep["removed"].(float64) != 0 {
		t.Errorf("scrolling added %v and removed %v elements, want nodes moved and none made again", sweep["added"], sweep["removed"])
	}
	if sweep["most"].(float64) > gridNodesMax {
		t.Errorf("the grid held %v nodes, want at most %d", sweep["most"], gridNodesMax)
	}
	if sweep["missing"].(float64) != 0 {
		t.Errorf("%v scroll steps showed no crosspoint mid-view", sweep["missing"])
	}

	// At the far corner: the last crosspoint is drawn with its names, and the pinned names stay at the edges.
	b.waitFor("the last crosspoint", gridXp(2499, 749)+` !== null && document.querySelector('.sode-gridpanel__src[data-src="2499"]').textContent === 'SRC 2500' && document.querySelector('.sode-gridpanel__dst[data-dst="749"]').textContent === 'MV 737'`)
	b.waitFor("pinned names stay at the edges", `(() => {
		const sc = document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect();
		const s = document.querySelector('.sode-gridpanel__src[data-src="`+fmt.Sprint(pinSrc)+`"]').getBoundingClientRect();
		const d = document.querySelector('.sode-gridpanel__dst[data-dst="`+fmt.Sprint(pinDst)+`"]').getBoundingClientRect();
		return s.top >= sc.top - 1 && s.left < sc.left + sc.width / 3 && d.left >= sc.left - 1 && d.top < sc.top + sc.height / 3;
	})()`)
	b.shot("scale-corner")

	// Idle: the polls get 304 and the tick runs each second, but nothing changed, so nothing in the grid is written.
	b.call("Performance.enable", nil)
	i0 := b.metrics()
	idle := b.eval(`new Promise((done) => {
		const grid = document.getElementById('panel-grid');
		let n = 0;
		const mo = new MutationObserver((recs) => { n += recs.length; });
		mo.observe(grid, { subtree: true, childList: true, attributes: true, characterData: true });
		setTimeout(() => { n += mo.takeRecords().length; mo.disconnect(); done(n); }, 5000);
	})`)
	i1 := b.metrics()
	t.Logf("5 s idle: tasks %.0f ms", (i1["TaskDuration"]-i0["TaskDuration"])*1000)
	if idle != 0.0 {
		t.Errorf("5 s idle wrote the grid %v times, want none", idle)
	}

	// Find glides across the whole Source bar to SRC 2400: nodes are moved, never made, and it ends with the match in
	// view. The glide time and its slowest frame are reported, not asserted.
	b.eval(`document.querySelector('.sode-gridpanel__scroll').scrollLeft = 0; true`)
	glide, ok := b.eval(`(async () => {
		const grid = document.getElementById('panel-grid');
		const sc = grid.querySelector('.sode-gridpanel__scroll');
		let added = 0, removed = 0;
		const els = (list) => [...list].filter((n) => n.nodeType === 1).length;
		const mo = new MutationObserver((recs) => {
			for (const r of recs) { added += els(r.addedNodes); removed += els(r.removedNodes); }
		});
		mo.observe(grid, { childList: true, subtree: true });
		await new Promise((r) => requestAnimationFrame(r));
		const input = document.querySelector('.sode-gridpanel__find[data-kind="srcs"] input');
		input.focus();
		const t0 = performance.now();
		const ended = new Promise((r) => sc.addEventListener('scrollend', r, { once: true }));
		document.execCommand('insertText', false, 'src 2400');
		let last = t0, worst = 0, going = true;
		const tick = () => { const now = performance.now(); worst = Math.max(worst, now - last); last = now; if (going) requestAnimationFrame(tick); };
		requestAnimationFrame(tick);
		await Promise.race([ended, new Promise((r) => setTimeout(r, 5000))]);
		going = false;
		const ms = performance.now() - t0;
		mo.takeRecords().forEach((r) => { added += els(r.addedNodes); removed += els(r.removedNodes); });
		mo.disconnect();
		const n = grid.querySelector('.sode-gridpanel__src[data-src="2399"]:not([hidden])');
		const r = sc.getBoundingClientRect();
		const shown = !!n && n.getBoundingClientRect().left >= r.left && n.getBoundingClientRect().right <= r.right;
		return { added, removed, ms, worst, shown };
	})()`).(map[string]any)
	if !ok {
		t.Fatal("the glide returned nothing")
	}
	t.Logf("Find glide to SRC 2400: %.0f ms, slowest frame %.1f ms", glide["ms"], glide["worst"])
	if glide["added"].(float64) != 0 || glide["removed"].(float64) != 0 {
		t.Errorf("the glide added %v and removed %v elements, want nodes moved and none made again", glide["added"], glide["removed"])
	}
	if glide["shown"] != true {
		t.Error("after the glide SRC 2400 is not in view")
	}
	realKey(b, "Escape", "Escape", 27)
	b.eval(`(() => { const sc = document.querySelector('.sode-gridpanel__scroll'); sc.scrollLeft = sc.scrollWidth; sc.scrollTop = sc.scrollHeight; return true; })()`)
	b.waitFor("back at the far corner", gridXp(2499, 749)+` !== null`)

	// A press far from the start still stages.
	b.eval(gridXp(2499, 749) + `.click()`)
	b.waitFor("the press stages", gridXp(2499, 749)+`.dataset.s === 'staged'`)

	// A staged route pulses, and only the staged crosspoints run the animation. An inherited property animated above
	// the grid restyles every node under it each frame. At this size that took 4.7 s of every 10 s.
	b.eval(gridXp(2498, 748) + `.click(); ` + gridXp(2497, 747) + `.click(); true`)
	b.waitFor("three staged", `document.querySelectorAll('.sode-gridpanel__xp[data-s="staged"]').length === 3`)
	b.waitFor("the staged crosspoints pulse", `document.getAnimations().filter((a) => a.effect?.target?.dataset?.s === 'staged').length === 3`)
	if big := b.eval(`document.getAnimations().map((a) => a.effect.target).filter((n) => n.getElementsByTagName('*').length > 20).map((n) => n.className).join(' ')`); big != "" {
		t.Errorf("an animation runs on an element with many nodes under it: %v", big)
	}
	b.waitFor("the staged crosspoints pulse in step", `(() => {
		const t = document.getAnimations().filter((a) => a.effect?.target?.dataset?.s === 'staged').map((a) => (a.currentTime - a.effect.getTiming().delay) % 2000);
		return Math.max(...t) - Math.min(...t) < 100;
	})()`)
	m0 := b.metrics()
	time.Sleep(3 * time.Second)
	m1 := b.metrics()
	style := (m1["RecalcStyleDuration"] - m0["RecalcStyleDuration"]) * 1000
	t.Logf("3 s with three staged: style recalc %.0f ms, tasks %.0f ms", style, (m1["TaskDuration"]-m0["TaskDuration"])*1000)
	// About 60 ms here, and about 1400 ms when the whole grid restyled each frame.
	if style > 600 {
		t.Errorf("3 s with three staged spent %.0f ms on style recalc, want under 600", style)
	}

	// 50 pins on each edge, with no limit. The pinned bands draw only what half the room holds, and
	// each band scrolls on its own. The grid keeps its bounds.
	pins := func(from int) string {
		p := make([]string, 50)
		for i := range p {
			p[i] = fmt.Sprintf(`"%d":{"pinned":true}`, from+i)
		}
		return strings.Join(p, ",")
	}
	many := makePanel(t, a, ana, `{"name":"Many pins","type":"grid","settings":{"mode":"take","levels":[1]},`+
		`"sources":{"members":"all","items":{`+pins(100)+`}},"destinations":{"members":"all","items":{`+pins(100)+`}}}`)
	b.open(srv.URL + "/panels/" + many + "?instance=Desk")
	b.waitFor("the 50 pinned lead", `document.querySelector('.sode-gridpanel__src[data-src="100"]')?.textContent === 'SRC 101'`)
	b.waitFor("each pinned band holds fewer than half the view", `(() => {
		const f = document.querySelector('.sode-gridpanel__field'), sc = document.querySelector('.sode-gridpanel__scroll');
		const v = (k) => Number(getComputedStyle(f).getPropertyValue(k).trim());
		const xp = document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width;
		const rows = v('--sode-row-pins'), cols = v('--sode-col-pins');
		return rows >= 1 && rows < 50 && cols >= 1 && cols < 50 && rows * xp <= sc.clientHeight / 2 && cols * xp <= sc.clientWidth / 2 &&
			v('--sode-cols') === 2500 - 50 + cols && v('--sode-rows') === 750 - 50 + rows;
	})()`)
	pinSweep, ok := b.eval(gridSweep).(map[string]any)
	if !ok {
		t.Fatal("the sweep with 50 pins returned nothing")
	}
	t.Logf("50 pins, scroll sweep: median step %.1f ms, worst %.1f ms, most nodes %v", pinSweep["median"], pinSweep["worst"], pinSweep["most"])
	if pinSweep["added"].(float64) != 0 || pinSweep["removed"].(float64) != 0 || pinSweep["most"].(float64) > gridNodesMax || pinSweep["missing"].(float64) != 0 {
		t.Errorf("50 pins, sweep: %v", pinSweep)
	}
	// A wheel over the Destination band scrolls the band alone: the grid does not move, and no node is made.
	band, ok := b.eval(`(async () => {
		const sc = document.querySelector('.sode-gridpanel__scroll'), grid = document.querySelector('.sode-gridpanel__field');
		const first = () => [...document.querySelectorAll('.sode-gridpanel__corner .sode-gridpanel__dst:not([hidden])')].map((n) => Number(n.dataset.dst)).sort((x, y) => x - y)[0];
		const was = first(), top = sc.scrollTop, left = sc.scrollLeft;
		let made = 0;
		const mo = new MutationObserver((recs) => recs.forEach((r) => { made += r.addedNodes.length + r.removedNodes.length; }));
		mo.observe(grid, { childList: true, subtree: true });
		const r = [...document.querySelectorAll('.sode-gridpanel__corner .sode-gridpanel__dst:not([hidden])')].map((n) => n.getBoundingClientRect()).sort((p, q) => p.top - q.top)[0];
		sc.dispatchEvent(new WheelEvent('wheel', { deltaY: 600, clientX: r.right + 200, clientY: r.top + 10, bubbles: true, cancelable: true }));
		for (let k = 0; k < 20 && first() === was; k++) await new Promise((res) => requestAnimationFrame(res));
		mo.disconnect();
		const bar = document.querySelector('.sode-gridpanel__pins--y + .sode-bigscroll--little');
		return { was, now: first(), still: sc.scrollTop === top && sc.scrollLeft === left, made, bar: !!bar && !bar.hidden };
	})()`).(map[string]any)
	if !ok || band["now"] == band["was"] || band["still"] != true || band["made"].(float64) != 0 || band["bar"] != true {
		t.Errorf("a wheel over the Destination band: %v, want the band moved, the grid still, no node made and the band's bar shown", band)
	}
	// Each band scrolled one pin per wheel step, end to end and back. Nodes are moved, never made, and the grid's
	// scroll stays. The step times and the work are reported, not asserted.
	b.call("Performance.enable", nil)
	for _, kind := range []string{"dsts", "srcs"} {
		b0 := b.metrics()
		bs, ok := b.eval(strings.ReplaceAll(bandSweep, "KIND", kind)).(map[string]any)
		b1 := b.metrics()
		if !ok {
			t.Fatalf("the %s band sweep returned nothing", kind)
		}
		ms := func(k string) float64 { return (b1[k] - b0[k]) * 1000 }
		t.Logf("%s band sweep, %v steps: median step %.1f ms, worst %.1f ms; style %.0f ms, layout %.0f ms, script %.0f ms, tasks %.0f ms",
			kind, bs["steps"], bs["median"], bs["worst"], ms("RecalcStyleDuration"), ms("LayoutDuration"), ms("ScriptDuration"), ms("TaskDuration"))
		if bs["added"].(float64) != 0 || bs["removed"].(float64) != 0 || bs["still"] != true || bs["steps"].(float64) < 10 || bs["moved"] != true {
			t.Errorf("the %s band sweep: %v, want the band moved, nodes moved and none made, and the grid still", kind, bs)
		}
	}
	// A wheel of a few px moves the band by those px, as it moves the grid. The band does not wait for a whole pin.
	smooth, ok := b.eval(`(async () => {
		const sc = document.querySelector('.sode-gridpanel__scroll');
		const xp = document.querySelector('.sode-gridpanel__probe-xp').getBoundingClientRect().width;
		const pins = () => [...document.querySelectorAll('.sode-gridpanel__corner .sode-gridpanel__dst:not([hidden])')];
		const r = pins().map((n) => n.getBoundingClientRect()).sort((p, q) => p.top - q.top)[0];
		const d = Math.min(...pins().map((n) => Number(n.dataset.dst))) + 1;
		const topOf = () => document.querySelector('.sode-gridpanel__corner .sode-gridpanel__dst[data-dst="' + d + '"]:not([hidden])').getBoundingClientRect().top;
		const step = Math.round(xp / 9), moves = [];
		for (let k = 0; k < 6; k++) {
			const was = topOf();
			sc.dispatchEvent(new WheelEvent('wheel', { clientX: r.right + 200, clientY: r.bottom - 2, deltaY: step, bubbles: true, cancelable: true }));
			await new Promise((res) => requestAnimationFrame(() => requestAnimationFrame(res)));
			moves.push(Math.round((was - topOf()) * 10) / 10);
		}
		return { step, moves };
	})()`).(map[string]any)
	if !ok {
		t.Fatal("the small wheel steps returned nothing")
	}
	for _, m := range smooth["moves"].([]any) {
		if d := m.(float64) - smooth["step"].(float64); d > 1 || d < -1 {
			t.Errorf("wheel steps of %v px over the Destination band moved it %v px, want each step's px", smooth["step"], smooth["moves"])
			break
		}
	}
	// A key move in the body leaves the band where the wheel put it, part of a pin and all.
	mid := b.eval(`(() => { const r = document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect(); return [r.left + r.width * 0.7, r.top + r.height * 0.7].join(); })()`).(string)
	var mx, my float64
	fmt.Sscanf(mid, "%f,%f", &mx, &my)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": mx, "y": my})
	bandTop := `[...document.querySelectorAll('.sode-gridpanel__corner .sode-gridpanel__dst:not([hidden])')].map((n) => n.getBoundingClientRect().top).sort((p, q) => p - q).join()`
	before := b.eval(bandTop)
	gridKey(b, "l")
	gridKey(b, "j")
	b.eval(`new Promise((res) => requestAnimationFrame(() => requestAnimationFrame(res)))`)
	if after := b.eval(bandTop); after != before {
		t.Errorf("a key move in the body moved the Destination band: pins at %v, then %v", before, after)
	}
	// The 40th pin is past the Source band's slots, so Find moves the band to it and the grid stays where it is.
	b.eval(`(() => { const sc = document.querySelector('.sode-gridpanel__scroll'); sc.scrollLeft = sc.scrollWidth; window.__left = sc.scrollLeft; return true; })()`)
	b.eval(`(() => { const i = document.querySelector('.sode-gridpanel__find[data-kind="srcs"] input'); i.focus(); document.execCommand('insertText', false, 'src 140'); return true; })()`)
	b.waitFor("Find brings the 40th pinned source into view", `(() => {
		const n = document.querySelector('.sode-gridpanel__src[data-src="139"]:not([hidden])'), r = document.querySelector('.sode-gridpanel__scroll').getBoundingClientRect();
		return !!n && !!n.closest('.sode-gridpanel__corner') && n.getBoundingClientRect().left >= r.left && n.getBoundingClientRect().right <= r.right &&
			document.querySelector('.sode-gridpanel__scroll').scrollLeft === window.__left;
	})()`)
	b.shot("scale-pins")
	b.check()
}
