package app

import (
	"context"
	"math"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The desktop main page's boxes. Show all sources is in the place of Show all destinations. The three groups (Source,
// Destination with +, Levels with Take and Clear) have one width. On a narrow window the last group keeps the room of
// its controls. The picks are the old small squares in two rows, and only an odd number of levels makes the first
// pick two rows high. 17 levels fit with no scroll from 850 to 1600 px. More columns than fit scroll inside the picks,
// and Select All stays in its place. A click from All selects only that level. No router with more than one level
// gives no Levels box. The arrow, +, Levels, Take and Clear line up with the Source and Destination boxes. They also
// line up with a second router, whose router buttons make the heads an action high.
func TestChromeMainBoxesLayout(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	simB := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)

	size := func(w int) {
		b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	}
	size(1600)
	b.open(srv.URL + "/")
	b.waitFor("17 level picks", `document.querySelectorAll('#lvl-pick > button').length === 17`)
	b.waitFor("fonts loaded", `document.fonts.status === 'loaded'`)

	// lined checks that the arrow, +, Levels, Take and Clear have the top and the bottom of the Source and Destination
	// boxes, as on main.
	lined := func(when string) {
		t.Helper()
		if got := b.eval(`(() => {
			const y = (s) => { const r = document.querySelector(s).getBoundingClientRect(); return [r.top, r.bottom]; };
			const [top, bottom] = y('#dst');
			const off = ['#src', '#flip', '#dst-add', '#lvl', '#patch', '#clear-all'].filter((s) => document.querySelector(s).offsetParent)
				.map((s) => [s, y(s)]).filter(([, [t, b]]) => Math.abs(t - top) > 0.5 || Math.abs(b - bottom) > 0.5);
			return off.length === 0 || off.map(([s, [t, b]]) => s + ' ' + t + '-' + b).join(', ') + ' against #dst ' + top + '-' + bottom;
		})()`); got != true {
			t.Fatalf("%s: the patch row does not line up: %v", when, got)
		}
	}
	lined("one router, 17 levels")

	// Show all sources hangs off the Source box as Show all destinations does off the Destination box.
	place := func(k string) []any {
		b.waitFor("Show all "+k+" shown", `(() => { const t = document.getElementById('`+k+`'); t.blur(); t.focus(); return !document.getElementById('`+k+`-all').hidden && !!document.querySelector('#`+k+`-all button').offsetParent; })()`)
		return b.eval(`(() => {
			const all = document.querySelector('#` + k + `-all button').getBoundingClientRect();
			const box = document.querySelector('.sode__box--` + k + `').getBoundingClientRect();
			const input = document.getElementById('` + k + `').getBoundingClientRect();
			return [Math.round(box.right - all.right), Math.round(all.top - input.bottom), Math.round(all.width > 0 ? all.height : -1)];
		})()`).([]any)
	}
	if src, dst := place("src"), place("dst"); src[0] != dst[0] || src[1] != dst[1] || src[2] != dst[2] {
		t.Fatalf("Show all sources at %v (right gap, gap below the input, height), want %v as Show all destinations", src, dst)
	}
	b.eval(`document.getElementById('src').focus(); document.querySelector('#src-all button').click(); true`)
	b.waitFor("Show all sources lists every source", `!document.getElementById('src-menu').hidden && document.querySelectorAll('#src-menu [role=option]').length >= 12`)
	b.eval(`document.getElementById('src').blur(); true`)

	// widths gives Source, Destination and Levels box widths and whether the picks' scroller and Select All are inside
	// the Levels box.
	widths := func() (src, dst, lvl float64, inside bool) {
		r := b.eval(`(() => {
			const w = (s) => document.querySelector(s).getBoundingClientRect();
			const box = w('.sode__box--levels');
			const parts = [w('#lvl-pick'), w('#lvl-all')];
			const inside = parts.every((p) => p.left >= box.left - 0.5 && p.right <= box.right + 0.5);
			return [w('.sode__box--src').width, w('.sode__box--dst').width, box.width, inside];
		})()`).([]any)
		return r[0].(float64), r[1].(float64), r[2].(float64), r[3].(bool)
	}
	// groups gives the three groups' widths: Source, Destination to +, and Levels to Clear.
	groups := func() [3]float64 {
		r := b.eval(`(() => {
			const w = (s) => document.querySelector(s).getBoundingClientRect();
			return [w('.sode__box--src').width, w('.sode__box--add').right - w('.sode__box--dst').left, w('#clear-all').right - w('.sode__box--levels').left];
		})()`).([]any)
		return [3]float64{r[0].(float64), r[1].(float64), r[2].(float64)}
	}
	sameGroups := func(w int) {
		t.Helper()
		// A 64th of a px apart is layout rounding.
		if g := groups(); math.Abs(g[0]-g[1]) > 0.5 || math.Abs(g[0]-g[2]) > 0.5 {
			t.Fatalf("at %d px the groups are %v px wide, want one width", w, g)
		}
	}
	rows := func() int {
		return int(b.eval(`new Set([...document.querySelectorAll('#lvl-pick > button')].map((n) => Math.round(n.getBoundingClientRect().top))).size`).(float64))
	}
	// scrolls: the picks scroll sideways. barShown and barHidden: their big bar.
	scrolls := func() bool {
		return b.eval(`(() => { const p = document.getElementById('lvl-pick'); return p.scrollWidth > p.clientWidth + 1; })()`).(bool)
	}
	barShown := `(() => { const bar = document.getElementById('lvl-pick').nextElementSibling; return !!bar && bar.classList.contains('sode-bigscroll') && !bar.hidden && bar.offsetHeight > 0; })()`
	barHidden := `(() => { const bar = document.getElementById('lvl-pick').nextElementSibling; return !bar?.classList.contains('sode-bigscroll') || bar.hidden; })()`
	// wholeCols: the scroller shows only whole columns of picks.
	wholeCols := func() bool {
		return b.eval(`(() => {
			const p = document.getElementById('lvl-pick'), [a, b] = p.children;
			const step = b.getBoundingClientRect().left - a.getBoundingClientRect().left;
			const cols = (p.clientWidth + parseFloat(getComputedStyle(p).columnGap)) / step;
			return Math.abs(cols - Math.round(cols)) < 0.05;
		})()`).(bool)
	}
	allLeft := func() float64 {
		return b.eval(`document.getElementById('lvl-all').getBoundingClientRect().left`).(float64)
	}
	width := func(w int) {
		size(w)
		b.waitFor("the page is "+strconv.Itoa(w)+" px", `innerWidth === `+strconv.Itoa(w))
	}
	// fits checks n levels at the width: two rows of picks 1.375rem (22 px) high, all inside the box, no scroll.
	fits := func(w, n int) {
		t.Helper()
		at := strconv.Itoa(w) + " px with " + strconv.Itoa(n) + " levels"
		msg, side := pickShape(b)
		if msg != "" {
			t.Fatalf("at %s: %s", at, msg)
		}
		if math.Abs(side-22) > 0.5 {
			t.Fatalf("at %s a pick is %v px, want the old 1.375rem (22 px)", at, side)
		}
		if got := rows(); got != 2 {
			t.Fatalf("at %s the picks are on %d rows, want 2", at, got)
		}
		if _, _, _, inside := widths(); !inside {
			t.Fatalf("at %s the picks or Select All are outside the Levels box", at)
		}
		if scrolls() {
			t.Fatalf("at %s the picks scroll, want every pick in view", at)
		}
		b.waitFor("no bar at "+at, barHidden)
	}
	sameGroups(1600)
	t.Logf("at 1600 px the groups are %v px wide", groups())
	fits(1600, 17)
	all17 := allLeft()

	// A click from All selects only that level, as typing "2" does. Further clicks add or remove; Select All is All again.
	pickLevel := func(ui int) {
		b.eval(`document.querySelector('#lvl-pick > button[data-ui="` + strconv.Itoa(ui) + `"]').click(); true`)
	}
	lvlIs := func(what, want string) { b.waitFor(what, `document.getElementById('lvl').value === '`+want+`'`) }
	pickLevel(2)
	lvlIs("a click from All selects only level 2", "2")
	pickLevel(3)
	lvlIs("a second click adds level 3", "2-3")
	pickLevel(2)
	lvlIs("a click on a selected level removes it", "3")
	b.eval(`document.getElementById('lvl-all').click(); true`)
	lvlIs("Select All selects every level", "1-17")
	pickLevel(5)
	lvlIs("a click after Select All selects only level 5", "5")
	b.eval(`(() => { const i = document.getElementById('lvl'); i.value = ''; i.dispatchEvent(new Event('input', { bubbles: true })); })(); true`)
	lvlIs("the box is empty again", "")

	for _, w := range []int{1440, 1280} {
		width(w)
		sameGroups(w)
		fits(w, 17)
	}

	// Narrow: Source and Destination keep one width, and the Levels group keeps the room of its text box, Take, Clear,
	// 17 picks and Select All.
	width(850)
	if g := groups(); math.Abs(g[0]-g[1]) > 0.5 || g[2] < g[0] {
		t.Fatalf("at 850 px the groups are %v px wide, want the first two the same and the last no narrower", g)
	}
	fits(850, 17)
	if b.eval(`document.documentElement.scrollWidth <= document.documentElement.clientWidth`) != true {
		t.Fatal("at 850 px the main page scrolls sideways")
	}
	width(1600)

	// One level on every router: no Levels box, Tab skips it, f in keyboard mode does nothing, and a take is level 1.
	setLevels := func(n int, def string) {
		if err := a.applyConfig(func(c *config.Config) error { c.Routers[0].Levels, c.Routers[0].DefaultLevels = n, def; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	// An even number of levels gives two rows of squares, and an odd number a first pick two rows high. Up to 17 levels
	// every pick shows at 1440 and 1600 px.
	for _, n := range []int{4, 5, 11, 12} {
		setLevels(n, "1-"+strconv.Itoa(n))
		b.waitFor(strconv.Itoa(n)+" level picks", `document.querySelectorAll('#lvl-pick > button').length === `+strconv.Itoa(n))
		for _, w := range []int{1600, 1440} {
			width(w)
			fits(w, n)
		}
		lined(strconv.Itoa(n) + " levels")
	}
	// 24 levels: the picks scroll sideways in two rows with a big bar, and Select All stays where it is with 17.
	setLevels(24, "1-24")
	b.waitFor("24 level picks", `document.querySelectorAll('#lvl-pick > button').length === 24`)
	for _, w := range []int{1440, 1600} {
		width(w)
		if rows() != 2 || !scrolls() || !wholeCols() {
			t.Fatalf("at %d px the 24 picks are on %d rows (scroll %v, whole columns %v), want 2 rows that scroll and show whole columns", w, rows(), scrolls(), wholeCols())
		}
		b.waitFor("the picks' big bar shows", barShown)
		if _, _, _, inside := widths(); !inside {
			t.Fatalf("at %d px with 24 levels the picks or Select All are outside the Levels box", w)
		}
		if msg, _ := pickShape(b); msg != "" {
			t.Fatalf("at %d px with 24 levels: %s", w, msg)
		}
	}
	if l := allLeft(); math.Abs(l-all17) > 0.5 {
		t.Fatalf("with 24 levels Select All is at %v px, want %v px as with 17", l, all17)
	}
	setLevels(1, "1")
	b.waitFor("the Levels box hides", `document.querySelector('.sode__box--levels').hidden && !document.getElementById('lvl').offsetParent`)
	if b.eval(`(() => { const take = document.getElementById('patch').getBoundingClientRect(), dst = document.querySelector('.sode__box--add').getBoundingClientRect(); return take.left > dst.right && take.left - dst.right < 40; })()`) != true {
		t.Fatal("with no Levels box, Take does not follow the + box")
	}
	waitNamesAndTally(t, a, ana)
	b.eval(`document.getElementById('dst').focus(); true`)
	realKey(b, "Tab", "Tab", 9)
	b.waitFor("Tab from Destination goes back to Source", `document.activeElement.id === 'src'`)
	b.eval(`document.getElementById('src').blur(); document.getElementById('history').focus(); true`)
	b.eval(`document.getElementById('history').dispatchEvent(new KeyboardEvent('keydown', {key: 'f', code: 'KeyF', bubbles: true, cancelable: true})); true`)
	if b.eval(`document.activeElement.id`) == "lvl" {
		t.Fatal("f moved focus into the hidden Levels box")
	}
	pick := func(id, text string) {
		b.waitFor(id+" match list", `(() => { const t = document.getElementById('`+id+`'); t.focus(); t.value = '`+text+`'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('`+id+`-opt-0'); })()`)
		b.eval(`document.getElementById('` + id + `').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
		b.waitFor(id+" picked", `document.activeElement.id !== '`+id+`'`)
	}
	pick("src", simSrcName)
	pick("dst", "DSM1")
	b.eval(`document.getElementById('dst').dispatchEvent(new KeyboardEvent('keydown', {key: 'Enter', code: 'Enter', bubbles: true, cancelable: true})); true`)
	b.waitFor("confirm unlocks", `!document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	tallyHas(t, a, ana, simDst, simSrc)

	setLevels(17, "1-17")
	b.waitFor("the Levels box shows again", `!document.querySelector('.sode__box--levels').hidden && document.querySelectorAll('#lvl-pick > button').length === 17`)
	b.waitFor("Last shows", `!!document.querySelector('[data-last="dst"] .sode__last')`)
	lined("one router, Last showing")

	// A second router shows the router button in the Source and Destination heads, which is an action high.
	if err := a.applyConfig(func(c *config.Config) error {
		rc := c.DefaultRouter()
		rc.UID, rc.Prefix, rc.Title, rc.Address = "", "B", "Router B", simB
		c.Routers = append(c.Routers, rc)
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	b.waitFor("the router buttons show", `!document.getElementById('dst-router').hidden && !!document.getElementById('dst-router').offsetParent`)
	lined("two routers, Last showing")
	b.check()
}

// pickShape says what is wrong with the level picks, or "", and the side of a pick in px. Each pick is a square, an
// action high at most and never under the old 1.375rem. The first is two rows high (two squares and the gap) only when
// the number of picks is odd. Select All is beside the picks, at their top, and two rows high.
func pickShape(b *tab) (string, float64) {
	r := b.eval(`(() => {
		const picks = [...document.querySelectorAll('#lvl-pick > button')].map((n) => n.getBoundingClientRect());
		const rem = parseFloat(getComputedStyle(document.documentElement).fontSize);
		const gap = parseFloat(getComputedStyle(document.getElementById('lvl-pick')).rowGap);
		const near = (a, b) => Math.abs(a - b) < 1;
		const odd = picks.length % 2 === 1;
		const side = picks[picks.length - 1].width;
		if (side > 2.5 * rem + 0.5 || side < 1.375 * rem - 0.5) return ['a pick is ' + side + ' px wide, want 22 to 40', side];
		for (const [i, r] of picks.entries()) {
			const high = odd && i === 0 ? 2 * side + gap : side;
			if (!near(r.width, side) || !near(r.height, high)) return ['pick ' + (i + 1) + ' is ' + r.width + ' by ' + r.height + ' px, want ' + side + ' by ' + high, side];
		}
		const all = document.getElementById('lvl-all').getBoundingClientRect();
		const box = document.getElementById('lvl-pick').getBoundingClientRect();
		if (!near(all.top, picks[0].top) || all.left < box.right) return ['Select All is not beside the picks', side];
		if (!near(all.height, 2 * side + gap)) return ['Select All is ' + all.height + ' px high, want ' + (2 * side + gap), side];
		return ['', side];
	})()`).([]any)
	return r[0].(string), r[1].(float64)
}

// levelPicksAt gives the rows of level picks, whether they scroll sideways, and whether the picks' scroller and Select
// All are inside the Levels box.
func levelPicksAt(b *tab) (rows int, scrolls, inside bool) {
	r := b.eval(`(() => {
		const p = document.getElementById('lvl-pick');
		const rows = new Set([...p.children].map((n) => Math.round(n.getBoundingClientRect().top))).size;
		const box = document.querySelector('.sode__box--levels').getBoundingClientRect();
		const inside = [p, document.getElementById('lvl-all')].map((n) => n.getBoundingClientRect()).every((r) => r.left >= box.left - 0.5 && r.right <= box.right + 0.5);
		return [rows, p.scrollWidth > p.clientWidth + 1, inside];
	})()`).([]any)
	return int(r[0].(float64)), r[1].(bool), r[2].(bool)
}
