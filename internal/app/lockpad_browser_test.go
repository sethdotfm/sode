package app

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Twelve users with Ana, one with the longest name a user can have (24 characters).
var lockPadNames = []string{"Bo", "Cleo", "Dana", "Eli", "Fay", "Gus", "Hal", "Ivy", "Jo", "Kai", "Maximiliana Rosalind Abe"}

// The lock screen is one column. The Name box and the pad share one width, and the page's own site name hides under
// the overlay. The dots and the site name are centred on the pad. Name comes first, then the dots and the PIN lines,
// then the keys. The name list drops over the pad and moves nothing, and shows six and a half rows. The last key says
// Sign in with an icon, is as high as the digits and keeps patchbay's own disabled look. All of it fits 1366x768.
// The Approval popup shows the same list whole.
func TestChromeLockPadLayout(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		for _, n := range lockPadNames {
			c.Users = append(c.Users, mkUser(t, n, "5678"))
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	// Reduced motion: the Lock is at rest as soon as it is up.
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-reduced-motion", "value": "reduce"}}})
	size := func(w, h int) {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": h, "deviceScaleFactor": 1, "mobile": false})
		b.waitFor(fmt.Sprintf("the window is %dx%d", w, h), fmt.Sprintf(`innerWidth === %d && innerHeight === %d`, w, h))
	}
	size(1920, 1080)
	b.open(srv.URL + "/")
	b.waitFor("Home is open", `!!document.getElementById('lock-chip-signin')?.offsetParent`)
	b.eval(`document.getElementById('lock-chip-signin').click()`)
	const listOpen = `!document.getElementById('lock-name-list').hidden`
	b.waitFor("the pad with the name list open", `!!document.querySelector('.sode-lock.is-on') && `+listOpen)
	b.eval(`document.getElementById('lock-name').blur()`)
	b.waitFor("the list closed", `!`+listOpen)
	for _, d := range "12" {
		realKey(b, string(d), "Digit"+string(d), int(d))
	}
	b.waitFor("two dots", `document.querySelector('.sode-lock__dots').textContent === '●●'`)

	one := func(sel string) box {
		t.Helper()
		xs := b.boxes(sel)
		if len(xs) != 1 {
			t.Fatalf("%s: %d shown, want 1", sel, len(xs))
		}
		return xs[0]
	}
	near := func(what string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > layoutTol {
			t.Errorf("%s: %.2f, want %.2f", what, got, want)
		}
	}

	// The list's big scroll bar shows on top of the list, not under it.
	barOnTop := func(what, list string) {
		t.Helper()
		bar := fmt.Sprintf(`document.querySelector(%q).parentElement.querySelector(':scope > .sode-bigscroll')`, list)
		b.waitFor(what+": the list's scroll bar shows", `!(`+bar+`)?.hidden`)
		if got := b.eval(`(() => {
			const bar = ` + bar + `;
			const r = bar.getBoundingClientRect(), hit = document.elementFromPoint(r.left + r.width / 2, r.top + r.height / 2);
			return bar.contains(hit) ? '' : 'under ' + (hit?.className || hit?.tagName);
		})()`); got != "" {
			t.Errorf("%s: the list's scroll bar: %v", what, got)
		}
	}

	column := func(where string) {
		t.Helper()
		pad, name := one(".sode-lock__pad"), one(".sode-lock .sode-lock__name")
		near(where+": the Name box's left edge is the pad's", name.Left, pad.Left)
		near(where+": the Name box's right edge is the pad's", name.Right, pad.Right)
		lock := b.eval(`(() => { const l = document.querySelector('.sode-lock'); return [l.scrollHeight - l.clientHeight, l.scrollWidth - l.clientWidth]; })()`).([]any)
		if lock[0].(float64) > 0 || lock[1].(float64) > 0 {
			t.Errorf("%s: the Lock scrolls by %v", where, lock)
		}
		if h := b.eval(`innerHeight`).(float64); pad.Bottom > h {
			t.Errorf("%s: the pad ends at %.1f, below the window (%.0f)", where, pad.Bottom, h)
		}
		key := one(`.sode-lock__key[data-key="1"]`)
		if min := b.tokenPx(".sode-lock__pad", "--sode-h-action"); key.height() < min-layoutTol {
			t.Errorf("%s: keys %.1f high, under --sode-h-action (%.1f)", where, key.height(), min)
		}
	}

	// A1-02, C1-26: one width, and the longest name fits it.
	column("1920x1080")
	if w := b.eval(`(() => { const i = document.getElementById('lock-name'), v = i.value; i.value = 'Maximiliana Rosalind Abe'; const w = [i.scrollWidth, i.clientWidth]; i.value = v; return w; })()`).([]any); w[0].(float64) > w[1].(float64) {
		t.Errorf("a 24-character name needs %vpx in a Name box of %vpx", w[0], w[1])
	}
	// A1-03: the page's own site name does not show through the overlay.
	if v := b.eval(`getComputedStyle(document.querySelector('.sode__mark')).visibility`); v != "hidden" {
		t.Errorf("the page's site name under the Lock is %v, want hidden", v)
	}
	// A1-09: the dots and the site name are centred on the pad.
	// The glyphs' own centre: letter-spacing also follows the last glyph, and it is not part of the text.
	glyphMiddle := func(sel string) float64 {
		t.Helper()
		return b.eval(fmt.Sprintf(`(() => {
			const e = document.querySelector(%q), n = e.firstChild, r = document.createRange();
			r.setStart(n, 0); r.setEnd(n, 1);
			const left = r.getBoundingClientRect().left;
			r.setStart(n, n.length - 1); r.setEnd(n, n.length);
			const right = r.getBoundingClientRect().right - (parseFloat(getComputedStyle(e).letterSpacing) || 0);
			return (left + right) / 2;
		})()`, sel)).(float64)
	}
	pad := one(".sode-lock__pad")
	padMiddle := (pad.Left + pad.Right) / 2
	near("the dots' centre", glyphMiddle(".sode-lock__dots"), padMiddle)
	near("the site name's centre", glyphMiddle(".sode-lock__brand"), padMiddle)
	// A1-11: who, then the PIN lines, then the keys.
	order := []string{".sode-lock__namefield", ".sode-lock__dots", ".sode-lock__state", ".sode-lock__msg", ".sode-lock__pad"}
	for i := 1; i < len(order); i++ {
		if up, down := one(order[i-1]), one(order[i]); down.Top < up.Bottom-layoutTol {
			t.Errorf("%s (top %.1f) is not under %s (bottom %.1f)", order[i], down.Top, order[i-1], up.Bottom)
		}
	}
	b.gapY("the site name to the Name box is the column's gap", "--pb-space-7", ".sode-lock__brand", ".sode-lock__namefield")
	// A1-12: Sign in in words with the check icon, as high as a digit, patchbay's own disabled look.
	if got := b.eval(`(() => { const k = document.querySelector('.sode-lock__key[data-key="enter"]'); return [k.textContent.trim(), k.hasAttribute('aria-label'), !!k.querySelector('svg.pb-icon use[href="#pb-i-check"]'), k.disabled]; })()`).([]any); fmt.Sprint(got) != "[Sign in false true true]" {
		t.Errorf("the last key (text, its own aria-label, check icon, off) is %v", got)
	}
	b.sameHeight("the Sign in key is as high as a digit", `.sode-lock__key[data-key="1"]`, `.sode-lock__key[data-key="enter"]`)
	look := `(k) => { const s = getComputedStyle(k); return [s.opacity, s.color, s.backgroundColor, s.borderTopColor].join(' '); }`
	if got, want := b.eval(`(`+look+`)(document.querySelector('.sode-lock__key[data-key="enter"]'))`), b.eval(`(() => {
		const k = document.createElement('button');
		k.className = 'pb-btn pb-btn--outline';
		k.disabled = true;
		document.querySelector('.sode-lock__pad').append(k);
		const v = (`+look+`)(k);
		k.remove();
		return v;
	})()`); got != want {
		t.Errorf("the Sign in key off is %v, want patchbay's disabled outline button %v", got, want)
	}
	// A1-18: words in full, and no key cuts its word.
	if got := b.eval(`document.querySelector('.sode-lock__key[data-key="back"]').textContent`); got != "Delete" {
		t.Errorf("the back key says %v, want Delete", got)
	}
	b.noClip("no key cuts its word", ".sode-lock__key")

	// A1-10: the list drops over the pad and moves nothing. A1-28: six and a half rows of twelve.
	listDrops := func(where string, rows float64) {
		t.Helper()
		brand, pad := one(".sode-lock__brand"), one(".sode-lock__pad")
		b.eval(`document.getElementById('lock-name').focus()`)
		b.waitFor(where+": the list open", listOpen)
		near(where+": the site name's top with the list open", one(".sode-lock__brand").Top, brand.Top)
		near(where+": the pad's top with the list open", one(".sode-lock__pad").Top, pad.Top)
		list, item := one("#lock-name-list"), one("#lock-name-list-0")
		if list.Bottom <= pad.Top {
			t.Errorf("%s: the list ends at %.1f, above the pad (%.1f): it does not drop over it", where, list.Bottom, pad.Top)
		}
		if h := b.eval(`innerHeight`).(float64); list.Bottom > h {
			t.Errorf("%s: the list ends at %.1f, below the window (%.0f)", where, list.Bottom, h)
		}
		if shown := list.height() / item.height(); shown < rows {
			t.Errorf("%s: the list shows %.1f rows, want %.1f or more", where, shown, rows)
		}
		barOnTop(where, "#lock-name-list")
		if lock := b.eval(`(() => { const l = document.querySelector('.sode-lock'); return [l.scrollHeight - l.clientHeight, l.scrollWidth - l.clientWidth]; })()`).([]any); lock[0].(float64) > 0 || lock[1].(float64) > 0 {
			t.Errorf("%s: the Lock scrolls by %v with the list open", where, lock)
		}
		b.eval(`document.getElementById('lock-name').blur()`)
		b.waitFor(where+": the list closed", `!`+listOpen)
	}
	listDrops("1920x1080", 6.4)

	// The pad still fits 1366x768, with the list open too.
	size(1366, 768)
	column("1366x768")
	listDrops("1366x768", 5)
	// A long wait ("Try again in 4 minutes 30 seconds") sits on the message line and moves nothing.
	padTop, dotsH := one(".sode-lock__pad").Top, one(".sode-lock__dots").height()
	b.eval(`(() => { document.querySelector('.sode-lock__dots').textContent = 'Too many wrong PINs'; document.querySelector('.sode-lock__dots').className = 'sode-lock__dots is-text is-bad'; document.querySelector('.sode-lock__msg').textContent = 'Try again in 4 minutes 30 seconds'; })()`)
	near("1366x768: the pad's top with a wait shown", one(".sode-lock__pad").Top, padTop)
	near("1366x768: the dots line's height with a wait shown", one(".sode-lock__dots").height(), dotsH)
	column("1366x768 with a wait")

	// A2-07: the Approval popup's list is not cut by the dialog.
	size(1920, 1080)
	b.eval(`document.querySelector('.sode-lock__cancel').click()`)
	b.waitFor("the Lock is gone", `document.querySelector('.sode-lock').hidden`)
	b.eval(`import('/approval.js').then((m) => m.askApproval('Saving these changes', 'Administrator', () => true))`)
	b.waitFor("the Approval list is open", `!document.getElementById('approval-name-list')?.hidden && !!document.getElementById('approval-name-list').offsetParent`)
	nameRight := `document.getElementById('approval-name').getBoundingClientRect().right`
	b.waitFor("the Approval body has no scroll gutter with the list open", `!document.getElementById('approval-name').closest('.pb-modal__body').style.paddingRight`)
	open := b.eval(nameRight).(float64)
	b.eval(`document.getElementById('approval-name').blur()`)
	b.waitFor("the Approval list closed", `document.getElementById('approval-name-list').hidden`)
	// The big scroll bar decides on the next frames after the list closes; let it settle before measuring.
	b.eval(`new Promise((r) => setTimeout(r, 400))`)
	b.waitFor("the Approval body has no scroll gutter", `!document.getElementById('approval-name').closest('.pb-modal__body').style.paddingRight`)
	near("the Approval Name box's right edge, list open against closed", open, b.eval(nameRight).(float64))
	b.eval(`document.getElementById('approval-name').focus()`)
	b.waitFor("the Approval list open again", `!document.getElementById('approval-name-list').hidden`)
	if cut := b.eval(`(() => {
		const l = document.getElementById('approval-name-list'), r = l.getBoundingClientRect(), out = [];
		for (let a = l.parentElement; a; a = a.parentElement) {
			const s = getComputedStyle(a);
			if (s.overflowX === 'visible' && s.overflowY === 'visible') continue;
			const p = a.getBoundingClientRect();
			if (r.top < p.top - 0.5 || r.bottom > p.bottom + 0.5 || r.left < p.left - 0.5 || r.right > p.right + 0.5) out.push(a.className);
		}
		return out.join('; ');
	})()`); cut != "" {
		t.Errorf("the Approval name list is cut by %v", cut)
	}
	if n := b.eval(`document.getElementById('approval-name-list').children.length`); n != float64(12) {
		t.Errorf("the Approval list has %v names, want 12", n)
	}
	barOnTop("Approval", "#approval-name-list")
	// The body does not scroll for the list, so it shows no bar of its own.
	if n := b.eval(`[...document.getElementById('approval-name').closest('.pb-modal').querySelectorAll(':scope > .sode-bigscroll')].filter((x) => x.getClientRects().length).length`); n != float64(0) {
		t.Errorf("the Approval body shows %v scroll bars for the open list, want none", n)
	}

	// C1-26: the phone keeps the one width.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 375, "height": 812, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone' && document.documentElement.dataset.conn === 'up'`)
	b.waitFor("the phone pad", `!!document.querySelector('.sode-lock.is-on') && !!document.getElementById('lock-name')?.offsetParent || (import('/core.js').then((m) => m.emit('login')), false)`)
	pad, name := one(".sode-lock__pad"), one(".sode-lock .sode-lock__name")
	near("phone: the Name box's left edge is the pad's", name.Left, pad.Left)
	near("phone: the Name box's right edge is the pad's", name.Right, pad.Right)
	if sx := b.eval(`(() => { const l = document.querySelector('.sode-lock'); return l.scrollWidth - l.clientWidth; })()`); sx != float64(0) {
		t.Errorf("phone: the Lock scrolls sideways by %v", sx)
	}
	column("phone 375x812")
	b.check()
}
