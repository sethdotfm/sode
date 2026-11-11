package app

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"
)

// leakProbe runs before the page's own scripts and counts what a leak leaves behind: live intervals,
// listeners on the document and window, object URLs and fetches, so a round of use can be compared with the last.
const leakProbe = `(() => {
  const m = (window.__leak = { intervals: new Set(), docListeners: {}, urls: 0, revoked: 0, fetches: 0 });
  const si = window.setInterval, ci = window.clearInterval;
  window.setInterval = function (...a) { const id = si.apply(window, a); m.intervals.add(id); return id; };
  window.clearInterval = function (id) { m.intervals.delete(id); return ci.call(window, id); };
  const top = (t) => t === window || t === document || t === document.documentElement || t === document.body;
  const addL = EventTarget.prototype.addEventListener, remL = EventTarget.prototype.removeEventListener;
  EventTarget.prototype.addEventListener = function (type, fn, opts) {
    if (top(this)) m.docListeners[type] = (m.docListeners[type] || 0) + 1;
    return addL.call(this, type, fn, opts);
  };
  EventTarget.prototype.removeEventListener = function (type, fn, opts) {
    if (top(this)) m.docListeners[type] = (m.docListeners[type] || 0) - 1;
    return remL.call(this, type, fn, opts);
  };
  const cu = URL.createObjectURL, ru = URL.revokeObjectURL;
  URL.createObjectURL = function (b) { m.urls++; return cu.call(URL, b); };
  URL.revokeObjectURL = function (u) { m.revoked++; return ru.call(URL, u); };
  const f = window.fetch;
  window.fetch = function (...a) { m.fetches++; return f.apply(window, a); };
})();`

// The running CSS animations (not transitions, which end by themselves), each as name@target class, or ” for none.
const animList = `document.getAnimations().filter((a) => a instanceof CSSAnimation).map((a) => (a.animationName || a.transitionProperty || 'animation') + '@' + String(a.effect?.target?.getAttribute?.('class') || a.effect?.target?.tagName)).join(' ')`

// The decoded size of the artwork shown, which the heap and DOM counters leave out: 4 bytes a pixel.
const artDecoded = `[...document.querySelectorAll('img.sode-panel__art-a')].filter((i) => i.complete && i.naturalWidth).map((i) => i.naturalWidth + 'x' + i.naturalHeight + ', ' + (i.naturalWidth * i.naturalHeight * 4 / 1e6).toFixed(1) + ' MB decoded').join('; ')`

type leakSample struct {
	heap      float64 // bytes after a forced GC
	documents float64 // documents alive in the renderer, the page and resource documents such as the icon sprite
	nodes     float64 // every DOM node alive, attached or not
	attached  float64 // elements in the page's own tree
	listeners float64 // every JS event listener alive
	intervals float64
	docL      float64 // listeners on document and window
	urls      float64 // object URLs made and not revoked
	fetches   float64
}

func (s leakSample) String() string {
	return fmt.Sprintf("heap %.1f MB, docs %.0f, nodes %.0f (attached %.0f), listeners %.0f, intervals %.0f, doc listeners %.0f, blob urls %.0f, fetches %.0f",
		s.heap/1e6, s.documents, s.nodes, s.attached, s.listeners, s.intervals, s.docL, s.urls, s.fetches)
}

// measure forces a GC, then reads the heap, the DOM counters and the probe.
func (b *tab) measure() leakSample {
	b.t.Helper()
	// A loaded runner needs more passes and a pause before detached nodes are really gone.
	for i := 0; i < 4; i++ {
		b.call("HeapProfiler.collectGarbage", nil)
		time.Sleep(100 * time.Millisecond)
	}
	var heap struct{ UsedSize float64 }
	_ = json.Unmarshal(b.call("Runtime.getHeapUsage", nil), &heap)
	var dom struct{ Documents, Nodes, JsEventListeners float64 }
	_ = json.Unmarshal(b.call("Memory.getDOMCounters", nil), &dom)
	probe, _ := b.eval(`(() => { const m = window.__leak; return { attached: document.getElementsByTagName('*').length, intervals: m.intervals.size, docL: Object.values(m.docListeners).reduce((a, b) => a + b, 0), urls: m.urls - m.revoked, fetches: m.fetches }; })()`).(map[string]any)
	num := func(k string) float64 { v, _ := probe[k].(float64); return v }
	return leakSample{heap: heap.UsedSize, documents: dom.Documents, attached: num("attached"), nodes: dom.Nodes, listeners: dom.JsEventListeners, intervals: num("intervals"), docL: num("docL"), urls: num("urls"), fetches: num("fetches")}
}

// The editor must not grow while it sits open or is used: a round of selecting, the menu, Find, the modals, Save and
// Test it live, repeated, ends with the same intervals, listeners, nodes and object URLs as the round before.
func TestChromeEditorHoldsNoMemory(t *testing.T) {
	var art string
	b, _, _, _ := editorTabPrep(t, fmt.Sprintf(`{"name":"Wall","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"source","n":%d},
		{"r":1,"c":3,"kind":"take"}],"settings":{"mode":"take"}}`, simDst, simSrc), func(b *tab, a *App, ana, id string) {
		b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": leakProbe})
		// The largest artwork the server takes (1024 by 1024), so the counts include its decoded size.
		rec := apiSendRaw(a, "POST", "/api/v1/panels/"+id+"/artwork", ana, imgBytes("png", panelArtSide, panelArtSide))
		if rec.Code != 201 {
			t.Fatalf("upload: %d %s", rec.Code, rec.Body.String())
		}
		art = apiBody(t, rec)["id"].(string)
		// An Icon library SVG, drawn as a mask over its own blob URL, which the same revoke must free.
		ic, err := a.icons.add("Mark", []byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><circle cx="5" cy="5" r="4"/></svg>`), "image/svg+xml")
		if err != nil {
			t.Fatal(err)
		}
		rec = apiSend(a, "PUT", "/api/v1/panels/"+id, ana, fmt.Sprintf(`{"name":"Wall","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d,"art":%q},
		{"r":1,"c":2,"kind":"source","n":%d,"art":"lib:%s"},
		{"r":1,"c":3,"kind":"take"}],"settings":{"mode":"take"}}`, simDst, art, simSrc, ic.ID))
		if rec.Code != 200 {
			t.Fatalf("art cell: %d %s", rec.Code, rec.Body.String())
		}
	})
	b.waitFor("editor grid", `document.querySelectorAll('#ed-grid .sode-ed__sq').length === 32`)
	b.waitFor("the artwork decoded", artDecoded+` !== ''`)
	t.Logf("artwork: %v", b.eval(artDecoded))
	b.eval(`window.press = (el, init) => el.dispatchEvent(new KeyboardEvent('keydown', { bubbles: true, cancelable: true, ...init })); true`)
	b.waitFor("Find attached", `(document.querySelector('.is-sel') || press(document.getElementById('ed-grid'), { key: 'ArrowDown' }), !!document.querySelector('.sode-find'))`)
	b.eval(`press(document.getElementById('ed-grid'), { key: 'Escape' })`)
	// panels.css once ran the pulse on :root, which restyled the whole page every frame on every panel page.
	if n := b.eval(animList); n != "" {
		t.Errorf("the idle editor runs animations; it must run none: %v", n)
	}

	step := func(what string) {
		if n := b.measure(); testing.Verbose() {
			t.Logf("  after %s: %s", what, n)
		}
	}
	round := func(n int) {
		grid := `document.getElementById('ed-grid')`
		// Select, extend, the type submenu, clear.
		b.eval(`press(` + grid + `, { key: 'ArrowDown' }); press(` + grid + `, { key: 'ArrowDown' }); press(` + grid + `, { key: 'ArrowRight', shiftKey: true })`)
		b.waitFor("menu shown", `!document.querySelector('.sode-ed__menu').hidden`)
		b.eval(`document.querySelector('.sode-ed__menu .pb-menu__sub .pb-menu__item').click()`)
		b.waitFor("kind set", `document.querySelectorAll('#ed-grid .is-sel.has-cell').length === 2`)
		b.eval(`press(` + grid + `, { key: 'Delete' })`)
		step("select and menu")
		// Find: type, narrow, leave.
		b.eval(`press(` + grid + `, { key: 'z' })`)
		b.waitFor("Find open", `document.querySelector('.sode-find input') === document.activeElement`)
		b.eval(`{ const box = document.querySelector('.sode-find input'); box.value = 'zoom'; box.dispatchEvent(new Event('input')); }`)
		b.waitFor("results", `document.querySelectorAll('.sode-find__row').length > 0`)
		b.eval(`press(document.querySelector('.sode-find input'), { key: 'Escape' })`)
		step("find")
		// Templates and Share open and close.
		b.eval(`document.getElementById('ed-templates').click()`)
		b.waitFor("templates open", `!!document.querySelector('.sode-tpl')`)
		b.eval(`[...document.querySelectorAll('.sode-tpl .pb-modal__footer .pb-btn')].find((x) => x.textContent === 'Close').click()`)
		b.waitFor("templates closed", `!document.querySelector('.sode-tpl')`)
		step("templates")
		b.eval(`document.getElementById('ed-share').click()`)
		b.waitFor("share open", `!!document.querySelector('.sode-ov__share')`)
		b.eval(`[...document.querySelectorAll('.sode-ov__share .pb-btn')].find((x) => x.textContent === 'Cancel').click()`)
		b.waitFor("share closed", `!document.querySelector('.sode-ov__share')`)
		step("share")
		// A change and Save.
		b.eval(fmt.Sprintf(`{ const nm = document.getElementById('ed-name'); nm.value = 'Wall %d'; nm.dispatchEvent(new Event('input')); }`, n))
		b.waitFor("save enabled", `!document.getElementById('ed-save').disabled`)
		b.eval(`document.getElementById('ed-save').click()`)
		b.waitFor("saved", `document.getElementById('ed-status').textContent === 'Saved.'`)
		step("save")
		// Test it live, a poll and some ticks, Back to editing.
		b.eval(`document.getElementById('ed-live').click()`)
		b.waitFor("live cells", `document.querySelectorAll('#ed-live-view .sode-panel__cell').length === 3`)
		b.waitFor("live art shown", `!document.querySelector('#ed-live-view .sode-panel__art-a')?.hidden`)
		time.Sleep(2500 * time.Millisecond)
		step("live")
		b.eval(`document.getElementById('ed-live-back').click()`)
		b.waitFor("back to editing", `document.getElementById('ed-live-view').hidden && document.querySelectorAll('#ed-live-view .sode-panel__cell').length === 0`)
		b.eval(`press(` + grid + `, { key: 'Escape' })`)
	}

	const rounds = 6
	var samples []leakSample
	t.Logf("at start: %s", b.measure())
	for i := 1; i <= rounds; i++ {
		round(i)
		s := b.measure()
		samples = append(samples, s)
		t.Logf("round %d: %s", i, s)
	}
	// The first round warms caches (names, presets, the templates list); from the second on nothing may grow.
	base, last := samples[1], samples[rounds-1]
	var bad []string
	if last.intervals != base.intervals {
		bad = append(bad, fmt.Sprintf("intervals %.0f -> %.0f", base.intervals, last.intervals))
	}
	if last.docL != base.docL {
		bad = append(bad, fmt.Sprintf("document listeners %.0f -> %.0f", base.docL, last.docL))
	}
	if last.urls != base.urls {
		bad = append(bad, fmt.Sprintf("object URLs %.0f -> %.0f", base.urls, last.urls))
	}
	// Nodes still in the document are exact. Detached nodes and the listeners on them wait for a GC that a loaded
	// runner can delay by a few rounds (+374 nodes and +36 listeners seen, attached unchanged), so those compare the
	// lowest of the last two rounds with the lowest of the first two measured; a leaked modal or sprite copy is
	// thousands of nodes every round and lifts both.
	if last.attached != base.attached {
		bad = append(bad, fmt.Sprintf("attached DOM nodes %.0f -> %.0f", base.attached, last.attached))
	}
	lowNodes := func(a, b leakSample) float64 { return min(a.nodes, b.nodes) }
	lowListeners := func(a, b leakSample) float64 { return min(a.listeners, b.listeners) }
	if lo, hi := lowNodes(samples[1], samples[2]), lowNodes(samples[rounds-2], samples[rounds-1]); hi > lo+250 {
		bad = append(bad, fmt.Sprintf("DOM nodes %.0f -> %.0f", lo, hi))
	}
	if lo, hi := lowListeners(samples[1], samples[2]), lowListeners(samples[rounds-2], samples[rounds-1]); hi > lo+40 {
		bad = append(bad, fmt.Sprintf("listeners %.0f -> %.0f", lo, hi))
	}
	if last.heap > base.heap*1.15 {
		bad = append(bad, fmt.Sprintf("heap %.1f MB -> %.1f MB", base.heap/1e6, last.heap/1e6))
	}
	if len(bad) > 0 {
		t.Errorf("the editor grew over %d rounds: %s", rounds-2, strings.Join(bad, "; "))
	}

	// Back to editing before the panel has loaded: nothing started afterwards may stay behind.
	b.eval(`document.getElementById('ed-live').click(); document.getElementById('ed-live-back').click()`)
	time.Sleep(1500 * time.Millisecond)
	if s := b.measure(); s.intervals != base.intervals {
		t.Errorf("an early Back to editing left timers running: intervals %.0f -> %.0f", base.intervals, s.intervals)
	}

	// Idle in live mode: the poll and the ticks must not add anything either. A staged destination pulses, and only
	// then: the pulse is the grid's own animation, started by a pulsing cell.
	b.eval(`document.getElementById('ed-live').click()`)
	b.waitFor("live cells", `document.querySelectorAll('#ed-live-view .sode-panel__cell').length === 3`)
	b.waitFor("destination named", `document.querySelector('#ed-live-view .sode-panel__cell--destination span')?.textContent === 'DSM1'`)
	if n := b.eval(animList); n != "" {
		t.Errorf("a live panel with nothing staged runs animations: %v", n)
	}
	b.eval(`document.querySelector('#ed-live-view .sode-panel__cell--destination').click(); document.querySelector('#ed-live-view .sode-panel__cell--source').click()`)
	b.waitFor("the grid pulses once a destination is staged", `document.getAnimations().some((a) => a.animationName === 'sode-panel-pulse' && a.effect.target.classList.contains('sode-panel__grid'))`)
	time.Sleep(2 * time.Second)
	before := b.measure()
	time.Sleep(6 * time.Second)
	after := b.measure()
	t.Logf("live idle: %s -> %s", before, after)
	if after.nodes > before.nodes+20 || after.listeners > before.listeners+10 || after.intervals != before.intervals || after.urls != before.urls {
		t.Errorf("the live test grew while idle: %s -> %s", before, after)
	}
	b.eval(`document.getElementById('ed-live-back').click()`)
	b.waitFor("back to editing", `document.getElementById('ed-live-view').hidden`)
	if n := b.eval(animList); n != "" {
		t.Errorf("animations kept running after Back to editing: %v", n)
	}
	b.check()
}
