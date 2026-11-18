package app

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Sode's own big scroll bars (bigscroll.js) on the Source match list and on the main page. The
// thumb is in scale with the view. A real mouse drag on it scrolls the list and keeps it open. A press
// on the track pages, and the native bar takes no room.
func TestChromeBigScroll(t *testing.T) {
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
	b.waitFor("a long Source match list", `(() => { const t = document.getElementById('src'); t.focus(); t.value = 's'; t.dispatchEvent(new Event('input', {bubbles: true})); const m = document.getElementById('src-menu'); return !m.hidden && m.scrollHeight > 2 * m.clientHeight; })()`)
	menu := `document.getElementById('src-menu')`
	bar := menu + `.nextElementSibling`
	b.waitFor("the list's big bar shows", bar+`?.classList.contains('sode-bigscroll') && !`+bar+`.hidden && `+bar+`.offsetHeight > 0`)
	b.waitFor("no native bar beside it", menu+`.offsetWidth - `+menu+`.clientWidth === `+menu+`.clientLeft * 2`)

	num := func(expr string) float64 { return b.eval(expr).(float64) }
	// The bar is the list's height and on its right inside edge, 1.4rem (28 px at 125%) wide.
	geo := b.eval(`(() => { const m = ` + menu + `, r = m.getBoundingClientRect(), g = ` + bar + `.getBoundingClientRect(); return [g.width, g.height, m.clientHeight, Math.round(r.right - m.clientLeft - g.right), Math.round(g.top - r.top - m.clientTop)]; })()`).([]any)
	if geo[0].(float64) != 28 || geo[1] != geo[2] || geo[3].(float64) != 0 || geo[4].(float64) != 0 {
		t.Fatalf("bar width, height, list height, right gap, top gap = %v, want 28 px, the list's height, 0 and 0", geo)
	}
	thumb := bar + `.firstElementChild`
	track := num(bar + `.clientHeight`)
	view, content := num(menu+`.clientHeight`), num(menu+`.scrollHeight`)
	want := math.Max(60, math.Round(track*view/content)) // min-height 3rem
	if got := num(thumb + `.offsetHeight`); math.Abs(got-want) > 1 {
		t.Fatalf("thumb %v px, want %v (track %v, view %v of %v)", got, want, track, view, content)
	}

	mouse := func(kind string, x, y float64) {
		b.call("Input.dispatchMouseEvent", map[string]any{"type": kind, "x": x, "y": y, "button": "left", "buttons": 1, "clickCount": 1})
	}
	at := func(expr string) (float64, float64) {
		v := b.eval(`(() => { const r = ` + expr + `.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`).([]any)
		return v[0].(float64), v[1].(float64)
	}
	x, y := at(thumb)
	mouse("mousePressed", x, y)
	mouse("mouseMoved", x, y+40)
	mouse("mouseMoved", x, y+80)
	mouse("mouseReleased", x, y+80)
	travel := track - num(thumb+`.offsetHeight`)
	moved := 80 * (content - view) / travel
	b.waitFor("the drag scrolls the list in step", fmt.Sprintf(`Math.abs(%s.scrollTop - %v) < 3`, menu, moved))
	b.waitFor("the list stays open with focus in Source", menu+`.hidden === false && document.activeElement.id === 'src'`)
	b.waitFor("the thumb follows", fmt.Sprintf(`Math.abs(new DOMMatrix(getComputedStyle(%s).transform).m42 - 80) <= 1`, thumb))

	// A press on the track below the thumb pages down by most of the view.
	from := num(menu + `.scrollTop`)
	r := b.eval(`(() => { const r = ` + bar + `.getBoundingClientRect(); return [r.left + r.width / 2, r.bottom - 4]; })()`).([]any)
	mouse("mousePressed", r[0].(float64), r[1].(float64))
	mouse("mouseReleased", r[0].(float64), r[1].(float64))
	b.waitFor("a track press pages down", fmt.Sprintf(`Math.abs(%s.scrollTop - %v) < 3`, menu, from+math.Round(view*0.9)))
	b.waitFor("the list is still open", menu+`.hidden === false && document.activeElement.id === 'src'`)

	// A little bar (History filter lists) is half as wide, and the list's padding makes room for it.
	b.eval(`document.getElementById('src').blur(); true`)
	b.waitFor("a long History Source filter list", `(() => { const t = document.querySelector('#history-search [data-search-src]'); t.focus(); t.value = 's'; t.dispatchEvent(new Event('input', {bubbles: true})); const m = t.parentElement.querySelector('.sode__menu'); return !m.hidden && m.scrollHeight > 2 * m.clientHeight; })()`)
	little := `document.querySelector('#history-search .sode__menu')`
	lbar := little + `.nextElementSibling`
	b.waitFor("its little bar shows", lbar+`.classList.contains('sode-bigscroll--little') && !`+lbar+`.hidden`)
	if w := num(lbar + `.getBoundingClientRect().width`); w != 14 {
		t.Fatalf("little bar is %v px wide, want 14 (0.7rem at 125%%)", w)
	}
	b.eval(`document.querySelector('#history-search [data-search-src]').blur(); true`)

	// In a short window the main page scrolls: its bar is fixed at the window's right edge and drags the page.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 400, "deviceScaleFactor": 1, "mobile": false})
	b.eval(`document.getElementById('src').blur(); true`)
	page := `document.querySelector('.sode-bigscroll--page')`
	b.waitFor("the page's big bar shows", page+` && !`+page+`.hidden && document.documentElement.hasAttribute('data-bigscroll-on')`)
	b.waitFor("it sits at the right edge, full height", page+`.getBoundingClientRect().right === innerWidth && `+page+`.offsetHeight === innerHeight`)
	b.waitFor("no native bar on the page", `innerWidth === document.documentElement.clientWidth`)
	x, y = at(page + `.firstElementChild`)
	mouse("mousePressed", x, y)
	mouse("mouseMoved", x, y+100)
	mouse("mouseReleased", x, y+100)
	b.waitFor("the drag scrolls the page", `scrollY > 100`)

	// Every page that runs bigScrollAll draws its page bar: /log and the panel editor too. It shows exactly when the
	// page is taller than the window, so a page that fits cannot pass with the bar broken.
	for _, path := range []string{"/log", "/panels/editor"} {
		b.open(srv.URL + path)
		b.waitFor(path+" draws its page bar", `(() => { const p = document.querySelector('.sode-bigscroll--page'); return !!p && (document.scrollingElement.scrollHeight > innerHeight) === !p.hidden; })()`)
	}
	b.check()
}
