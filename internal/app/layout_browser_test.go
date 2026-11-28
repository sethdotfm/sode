package app

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

// Layout checks for browser checks: shared edges, centre lines, heights, token gaps and clipped text, each within
// layoutTol. Every selector takes all the elements it matches, in document order. An element that does not show is
// left out: one with no box (display: none, hidden) and one with visibility: hidden. A selector that finds no shown
// element fails, and so does a check that finds too few in all, so it never passes on nothing.
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
	const out = [], empty = [];
	for (const sel of sels) {
	let shown = 0;
	for (const e of document.querySelectorAll(sel)) {
		if (!e.getClientRects().length || !e.checkVisibility({ visibilityProperty: true })) continue;
		shown++;
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
	if (!shown) empty.push(sel);
	}
	return { boxes: out, empty };
})`

type clipBox struct {
	box
	Clip string
}

// layoutBoxes gives the boxes and, as the second value, a failure that names each selector with no shown element.
// One selector that finds nothing must fail even when the others find enough, else a check passes on a typo.
func (b *tab) layoutBoxes(sels []string, clip bool) ([]clipBox, string) {
	b.t.Helper()
	arg, _ := json.Marshal(sels)
	raw, _ := json.Marshal(b.eval(fmt.Sprintf(layoutJS, layoutTol) + `(` + string(arg) + `, ` + fmt.Sprint(clip) + `)`))
	var out struct {
		Boxes []clipBox
		Empty []string
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		b.t.Fatalf("layout boxes %v: %v", sels, err)
	}
	if len(out.Empty) > 0 {
		return out.Boxes, "no shown element for " + strings.Join(out.Empty, ", ")
	}
	return out.Boxes, ""
}

// shown gives the border box of every shown element that the selectors match, in selector then document order,
// and the failure from layoutBoxes.
func (b *tab) shown(sels ...string) ([]box, string) {
	b.t.Helper()
	xs, msg := b.layoutBoxes(sels, false)
	var out []box
	for _, x := range xs {
		out = append(out, x.box)
	}
	return out, msg
}

// boxes is shown, and stops the test when a selector has no shown element.
func (b *tab) boxes(sels ...string) []box {
	b.t.Helper()
	out, msg := b.shown(sels...)
	if msg != "" {
		b.shot("layout")
		b.t.Fatalf("boxes %v: %s", sels, msg)
	}
	return out
}

// tokenPx resolves a length in the page: a custom property name ("--pb-space-3") or any CSS length ("2rem"). It
// measures a probe in the parent of the first element sel finds, so em and a token set on a subtree resolve there.
// It stops the test when sel finds nothing or the property is not set there. An unset property gives a width of 0,
// and then a gap check passes on elements that touch.
func (b *tab) tokenPx(sel, token string) float64 {
	b.t.Helper()
	msg, px := b.tokenPxErr(sel, token)
	if msg != "" {
		b.t.Fatal(msg)
	}
	return px
}

// tokenPxErr is tokenPx with the failure as a value.
func (b *tab) tokenPxErr(sel, token string) (string, float64) {
	b.t.Helper()
	s, _ := json.Marshal(sel)
	n, _ := json.Marshal(token)
	v := token
	if strings.HasPrefix(token, "--") {
		v = "var(" + token + ")"
	}
	vj, _ := json.Marshal(v)
	raw, _ := json.Marshal(b.eval(`(() => {
		const e = document.querySelector(` + string(s) + `);
		if (!e) return { why: 'no element' };
		const host = e.parentElement || document.body;
		const name = ` + string(n) + `;
		if (name.startsWith('--') && getComputedStyle(host).getPropertyValue(name).trim() === '') return { why: 'not set' };
		const p = document.createElement('div');
		p.style.cssText = 'position:absolute;visibility:hidden;height:0;padding:0;border:0;width:' + ` + string(vj) + `;
		host.append(p);
		const w = p.getBoundingClientRect().width;
		p.remove();
		return { px: w };
	})()`))
	var out struct {
		Why string
		Px  float64
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return fmt.Sprintf("token %s at %s did not resolve: %v", token, sel, err), 0
	}
	switch out.Why {
	case "no element":
		return fmt.Sprintf("token %s: no element for %s", token, sel), 0
	case "not set":
		return fmt.Sprintf("token %s is not set at %s", token, sel), 0
	}
	return "", out.Px
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

// boxCheck runs check on the boxes of sels, or fails with the name of a selector that has no shown element.
func (b *tab) boxCheck(what string, sels []string, check func([]box) string) bool {
	b.t.Helper()
	xs, msg := b.shown(sels...)
	if msg != "" {
		return b.layoutFail(what + ": " + msg)
	}
	return b.layoutFail(check(xs))
}

// sameLeft fails the test unless every element the selectors match has one left edge.
func (b *tab) sameLeft(what string, sels ...string) bool {
	b.t.Helper()
	return b.boxCheck(what, sels, func(xs []box) string { return sameLeftErr(what, xs) })
}

// sameMiddle fails the test unless every element the selectors match has one horizontal centre line (one row).
func (b *tab) sameMiddle(what string, sels ...string) bool {
	b.t.Helper()
	return b.boxCheck(what, sels, func(xs []box) string { return sameMiddleErr(what, xs) })
}

// sameHeight fails the test unless every element the selectors match has one height.
func (b *tab) sameHeight(what string, sels ...string) bool {
	b.t.Helper()
	return b.boxCheck(what, sels, func(xs []box) string { return sameHeightErr(what, xs) })
}

// gapX fails the test unless the space from each element to the next, left to right in match order, is token.
// The token is a custom property name such as "--pb-space-3", or a CSS length.
func (b *tab) gapX(what, token string, sels ...string) bool {
	b.t.Helper()
	return b.boxCheck(what, sels, func(xs []box) string { return gapErr(what, xs, 'x', b.tokenPx(sels[0], token)) })
}

// gapY is gapX from top to bottom.
func (b *tab) gapY(what, token string, sels ...string) bool {
	b.t.Helper()
	return b.boxCheck(what, sels, func(xs []box) string { return gapErr(what, xs, 'y', b.tokenPx(sels[0], token)) })
}

// noClip fails the test when an element's text overflows its box. It also fails when the box or an ancestor that
// clips cuts the text.
func (b *tab) noClip(what string, sels ...string) bool {
	b.t.Helper()
	xs, msg := b.layoutBoxes(sels, true)
	if msg != "" {
		return b.layoutFail(what + ": " + msg)
	}
	return b.layoutFail(clipErr(what, xs))
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
	<p class="cut">A long name that does not fit</p><p class="fits">Short</p><p hidden class="a">Hidden</p>
	<p class="ghost" style="visibility: hidden">Ghost</p>`)
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
	clipped := func(sels ...string) []clipBox {
		t.Helper()
		xs, msg := b.layoutBoxes(sels, true)
		ok(msg)
		return xs
	}
	ok(clipErr("short text", clipped(".fits", ".row > *")))
	bad(clipErr("long text", clipped(".cut")), `p.cut "A long name`)
	// One selector that finds nothing fails, also when the others find two or more. A hidden element is not shown.
	_, msg := b.shown(".nothing", ".row > .a")
	bad(msg, "no shown element for .nothing")
	_, msg = b.shown(".fits", "[hidden]")
	bad(msg, "no shown element for [hidden]")
	_, msg = b.shown(".fits", ".ghost")
	bad(msg, "no shown element for .ghost")
	_, msg = b.layoutBoxes([]string{".fits", ".nothing"}, true)
	bad(msg, ".nothing")
	// An unset property and a selector that finds nothing do not resolve to 0 px.
	msg, _ = b.tokenPxErr(".row > .a", "--unset")
	bad(msg, "--unset is not set")
	msg, _ = b.tokenPxErr(".nothing", "--gap")
	bad(msg, "no element for .nothing")
	b.check()
}
