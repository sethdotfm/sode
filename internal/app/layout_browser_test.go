package app

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

// Layout checks for browser checks: shared edges, centre lines, heights, token gaps and clipped text, each within
// layoutTol. Every selector takes all the elements it matches, in document order. An element with no box (hidden)
// is left out. A check that finds too few elements fails, so it never passes on nothing.
// docs/agents/browser-checks.md says how to call them.

// layoutTol is half a CSS pixel: less than that is rounding, more is a step a person can see at 2x.
const layoutTol = 0.5

// box is one element's border box in CSS px, with a short name for the failure text.
type box struct {
	Name                     string
	Left, Top, Right, Bottom float64
}

func (x box) height() float64  { return x.Bottom - x.Top }
func (x box) middleY() float64 { return (x.Top + x.Bottom) / 2 }

// layoutJS gives the boxes of every shown element the selectors match. clip adds the text overflow test. It finds
// content that is wider or taller than its box. It also finds text past the padding box of the element or of an
// ancestor that clips (overflow not visible). Text is tested sideways by its range. Up and down, it is tested by the
// middle of each line. A glyph box can be taller than its line box and still show whole.
const layoutJS = `((sels, clip) => {
	const name = (e) => e.tagName.toLowerCase() + (e.id ? '#' + e.id : '') + [...e.classList].map((c) => '.' + c).join('')
		+ ' "' + (e.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 24) + '"';
	const pad = (e) => {
		const r = e.getBoundingClientRect(), c = getComputedStyle(e);
		const n = (p) => parseFloat(c[p]) || 0;
		return { l: r.left + n('borderLeftWidth'), r: r.right - n('borderRightWidth'), t: r.top + n('borderTopWidth'), b: r.bottom - n('borderBottomWidth') };
	};
	const out = [];
	for (const sel of sels) for (const e of document.querySelectorAll(sel)) {
		if (!e.getClientRects().length) continue;
		const r = e.getBoundingClientRect();
		const x = { name: name(e), left: r.left, top: r.top, right: r.right, bottom: r.bottom };
		if (clip) {
			const why = [];
			const c = getComputedStyle(e);
			if (c.display !== 'inline' && (e.scrollWidth > e.clientWidth || e.scrollHeight > e.clientHeight))
				why.push('content ' + e.scrollWidth + 'x' + e.scrollHeight + ' in ' + e.clientWidth + 'x' + e.clientHeight);
			if (c.textOverflow === 'ellipsis' && e.scrollWidth > e.clientWidth) why.push('cut with an ellipsis');
			const rg = document.createRange();
			rg.selectNodeContents(e);
			const lines = [...rg.getClientRects()].filter((q) => q.width > 0);
			const boxes = [];
			if (c.display !== 'inline') boxes.push([e, pad(e)]);
			for (let a = e.parentElement; a; a = a.parentElement) {
				const s = getComputedStyle(a);
				if (s.overflowX !== 'visible' || s.overflowY !== 'visible') boxes.push([a, pad(a)]);
			}
			for (const q of lines) for (const [a, p] of boxes) {
				const m = (q.top + q.bottom) / 2;
				if (q.left < p.l - ` + "%[1]g" + ` || q.right > p.r + ` + "%[1]g" + ` || m < p.t || m > p.b)
					why.push('text ' + [q.left, q.right, q.top, q.bottom].map((v) => v.toFixed(1)).join(',') + ' past ' + name(a));
			}
			x.clip = why.join('; ');
		}
		out.push(x);
	}
	return out;
})`

type clipBox struct {
	box
	Clip string
}

func (b *tab) layoutBoxes(sels []string, clip bool) []clipBox {
	b.t.Helper()
	arg, _ := json.Marshal(sels)
	raw, _ := json.Marshal(b.eval(fmt.Sprintf(layoutJS, layoutTol) + `(` + string(arg) + `, ` + fmt.Sprint(clip) + `)`))
	var out []clipBox
	if err := json.Unmarshal(raw, &out); err != nil {
		b.t.Fatalf("layout boxes %v: %v", sels, err)
	}
	return out
}

// boxes gives the border box of every shown element that the selectors match, in selector then document order.
func (b *tab) boxes(sels ...string) []box {
	b.t.Helper()
	var out []box
	for _, x := range b.layoutBoxes(sels, false) {
		out = append(out, x.box)
	}
	return out
}

// tokenPx resolves a length in the page: a custom property name ("--pb-space-3") or any CSS length ("2rem"). It
// measures a probe in the parent of the first element sel finds, so em and a token set on a subtree resolve there.
func (b *tab) tokenPx(sel, token string) float64 {
	b.t.Helper()
	if strings.HasPrefix(token, "--") {
		token = "var(" + token + ")"
	}
	s, _ := json.Marshal(sel)
	v, _ := json.Marshal(token)
	px, ok := b.eval(`(() => {
		const host = document.querySelector(` + string(s) + `)?.parentElement || document.body;
		const p = document.createElement('div');
		p.style.cssText = 'position:absolute;visibility:hidden;height:0;padding:0;border:0;width:' + ` + string(v) + `;
		host.append(p);
		const w = p.getBoundingClientRect().width;
		p.remove();
		return w;
	})()`).(float64)
	if !ok {
		b.t.Fatalf("token %s did not resolve", token)
	}
	return px
}

// spread fails when fewer than two boxes are shown or when the measure of any box is more than layoutTol from the
// first box's.
func spread(what string, xs []box, measure string, f func(box) float64) string {
	if len(xs) < 2 {
		return fmt.Sprintf("%s: %d shown elements, want 2 or more", what, len(xs))
	}
	var bad []string
	for _, x := range xs[1:] {
		if math.Abs(f(x)-f(xs[0])) > layoutTol {
			bad = append(bad, fmt.Sprintf("%s %s %.2f", x.Name, measure, f(x)))
		}
	}
	if bad == nil {
		return ""
	}
	return fmt.Sprintf("%s: %s %s %.2f, but %s", what, xs[0].Name, measure, f(xs[0]), strings.Join(bad, "; "))
}

func sameLeftErr(what string, xs []box) string {
	return spread(what, xs, "left", func(x box) float64 { return x.Left })
}

func sameMiddleErr(what string, xs []box) string {
	return spread(what, xs, "middle", box.middleY)
}

func sameHeightErr(what string, xs []box) string {
	return spread(what, xs, "height", box.height)
}

// gapErr checks each pair of boxes next to each other in order. On x, the gap is from one's right edge to the next
// one's left. On y, it is from one's bottom to the next one's top.
func gapErr(what string, xs []box, axis byte, want float64) string {
	if len(xs) < 2 {
		return fmt.Sprintf("%s: %d shown elements, want 2 or more", what, len(xs))
	}
	var bad []string
	for i := 1; i < len(xs); i++ {
		g := xs[i].Left - xs[i-1].Right
		if axis == 'y' {
			g = xs[i].Top - xs[i-1].Bottom
		}
		if math.Abs(g-want) > layoutTol {
			bad = append(bad, fmt.Sprintf("%s to %s: %.2f", xs[i-1].Name, xs[i].Name, g))
		}
	}
	if bad == nil {
		return ""
	}
	return fmt.Sprintf("%s: want a gap of %.2f px, got %s", what, want, strings.Join(bad, "; "))
}

func clipErr(what string, xs []clipBox) string {
	if len(xs) == 0 {
		return what + ": no shown element"
	}
	var bad []string
	for _, x := range xs {
		if x.Clip != "" {
			bad = append(bad, x.Name+": "+x.Clip)
		}
	}
	if bad == nil {
		return ""
	}
	return what + ": " + strings.Join(bad, "; ")
}

func (b *tab) layoutFail(msg string) bool {
	b.t.Helper()
	if msg == "" {
		return true
	}
	b.shot("layout")
	b.t.Error(msg)
	return false
}

// sameLeft fails the test unless every element the selectors match has one left edge.
func (b *tab) sameLeft(what string, sels ...string) bool {
	b.t.Helper()
	return b.layoutFail(sameLeftErr(what, b.boxes(sels...)))
}

// sameMiddle fails the test unless every element the selectors match has one horizontal centre line (one row).
func (b *tab) sameMiddle(what string, sels ...string) bool {
	b.t.Helper()
	return b.layoutFail(sameMiddleErr(what, b.boxes(sels...)))
}

// sameHeight fails the test unless every element the selectors match has one height.
func (b *tab) sameHeight(what string, sels ...string) bool {
	b.t.Helper()
	return b.layoutFail(sameHeightErr(what, b.boxes(sels...)))
}

// gapX fails the test unless the space from each element to the next, left to right in match order, is token.
// The token is a custom property name such as "--pb-space-3", or a CSS length.
func (b *tab) gapX(what, token string, sels ...string) bool {
	b.t.Helper()
	return b.layoutFail(gapErr(what, b.boxes(sels...), 'x', b.tokenPx(sels[0], token)))
}

// gapY is gapX from top to bottom.
func (b *tab) gapY(what, token string, sels ...string) bool {
	b.t.Helper()
	return b.layoutFail(gapErr(what, b.boxes(sels...), 'y', b.tokenPx(sels[0], token)))
}

// noClip fails the test when an element's text overflows its box. It also fails when the box or an ancestor that
// clips cuts the text.
func (b *tab) noClip(what string, sels ...string) bool {
	b.t.Helper()
	return b.layoutFail(clipErr(what, b.layoutBoxes(sels, true)))
}

// The helper on a fixed page: each check passes on the good elements and names the bad one.
func TestChromeLayoutHelper(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 800, "height": 600, "deviceScaleFactor": 1, "mobile": false})
	b.open(`data:text/html,<!doctype html><style>
		body { margin: 0; font: 16px/1.5 sans-serif; --gap: 12px; }
		.row { display: flex; align-items: center; gap: var(--gap); padding: 8px; }
		.row > * { margin: 0; }
		.a { height: 32px; } .b { height: 32px; } .c { height: 40px; margin-left: 1px; }
		.cut { width: 4rem; overflow: hidden; white-space: nowrap; }
		.fits { width: 20rem; }
	</style>
	<div class="row"><span class="a">One</span><span class="b">Two</span><span class="c">Three</span></div>
	<div class="row"><span class="a">Left</span></div>
	<p class="cut">A long name that does not fit</p><p class="fits">Short</p><p hidden class="a">Hidden</p>`)
	b.waitFor("the page", `document.querySelector('.fits') !== null`)

	ok := func(msg string) {
		t.Helper()
		if msg != "" {
			t.Error(msg)
		}
	}
	bad := func(msg, want string) {
		t.Helper()
		if !strings.Contains(msg, want) {
			t.Errorf("want a failure that names %q, got %q", want, msg)
		}
	}
	ok(sameLeftErr("first boxes", b.boxes(".row > :first-child")))
	bad(sameLeftErr("first and second", b.boxes(".row > .a", ".row > .b")), `span.b "Two"`)
	ok(sameMiddleErr("one centre line", b.boxes(".row:first-child > *")))
	b.sameHeight("a and b", ".row:first-child > .a", ".row:first-child > .b")
	bad(sameHeightErr("a and c", b.boxes(".row:first-child > .a", ".row:first-child > .c")), `span.c "Three"`)
	b.gapX("a to b", "--gap", ".row:first-child > .a", ".row:first-child > .b")
	b.gapY("the rows touch", "0px", ".row")
	bad(gapErr("b to c", b.boxes(".row:first-child > .b", ".row:first-child > .c"), 'x', b.tokenPx(".row > .a", "--gap")), "13.00")
	if px := b.tokenPx("p", "2rem"); px != 32 {
		t.Errorf("2rem resolves to %v px, want 32", px)
	}
	ok(clipErr("short text", b.layoutBoxes([]string{".fits", ".row > *"}, true)))
	bad(clipErr("long text", b.layoutBoxes([]string{".cut"}, true)), `p.cut "A long name`)
	bad(sameLeftErr("hidden is left out", b.boxes("[hidden]", ".fits")), "1 shown elements")
	b.check()
}
