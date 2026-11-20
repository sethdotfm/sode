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

// The desktop main page's boxes. Show all sources is in the place of Show all destinations. Levels shrinks with Source
// and Destination. Its picks are squares an action high in two rows, and only an odd number of levels makes the first
// pick two rows high. No router with more than one level gives no Levels box.
func TestChromeMainBoxesLayout(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
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

	// widths gives Source, Destination and Levels box widths and whether every pick and Select All are inside the
	// Levels box.
	widths := func() (src, dst, lvl float64, inside bool) {
		r := b.eval(`(() => {
			const w = (s) => document.querySelector(s).getBoundingClientRect();
			const box = w('.sode__box--levels');
			const parts = [...document.querySelectorAll('#lvl-pick > button, #lvl-all')].map((n) => n.getBoundingClientRect());
			const inside = parts.every((p) => p.left >= box.left - 0.5 && p.right <= box.right + 0.5);
			return [w('.sode__box--src').width, w('.sode__box--dst').width, box.width, inside];
		})()`).([]any)
		return r[0].(float64), r[1].(float64), r[2].(float64), r[3].(bool)
	}
	rows := func() int {
		return int(b.eval(`new Set([...document.querySelectorAll('#lvl-pick > button')].map((n) => Math.round(n.getBoundingClientRect().top))).size`).(float64))
	}
	wideSrc, _, wideLvl, inside := widths()
	t.Logf("at 1600 px: Source %.1f px, Levels %.1f px", wideSrc, wideLvl)
	if !inside {
		t.Fatal("at 1600 px a level pick or Select All is outside the Levels box")
	}
	if n := rows(); n < 2 {
		t.Fatalf("at 1600 px the 17 picks are on %d rows, want 2 or more", n)
	}
	if msg, _ := pickShape(b); msg != "" {
		t.Fatalf("at 1600 px with 17 levels: %s", msg)
	}
	size(850)
	b.waitFor("the boxes shrink", `document.querySelector('.sode__box--src').getBoundingClientRect().width < `+strconv.FormatFloat(wideSrc-50, 'f', 1, 64))
	src, dst, lvl, inside := widths()
	if !inside {
		t.Fatal("at 850 px a level pick or Select All is outside the Levels box")
	}
	// A 64th of a px apart is layout rounding.
	if math.Abs(src-dst) > 0.5 {
		t.Fatalf("at 850 px Source is %v px and Destination %v px, want the same", src, dst)
	}
	// The margin before Levels is not part of its box, so compare the shares with it in.
	margin := b.eval(`parseFloat(getComputedStyle(document.querySelector('.sode__box--levels')).marginLeft)`).(float64)
	wide, narrow := wideSrc/(wideLvl+margin), src/(lvl+margin)
	if math.Abs(wide-narrow) > 0.02 {
		t.Fatalf("Source to Levels is %.3f at 1600 px and %.3f at 850 px, want the same ratio", wide, narrow)
	}
	if rows() <= 2 {
		t.Fatal("at 850 px the picks did not wrap onto more rows")
	}
	if msg, _ := pickShape(b); msg != "" {
		t.Fatalf("at 850 px with 17 levels: %s", msg)
	}
	if b.eval(`document.documentElement.scrollWidth <= document.documentElement.clientWidth`) != true {
		t.Fatal("at 850 px the main page scrolls sideways")
	}
	size(1600)

	// One level on every router: no Levels box, Tab skips it, f in keyboard mode does nothing, and a take is level 1.
	setLevels := func(n int, def string) {
		if err := a.applyConfig(func(c *config.Config) error { c.Levels, c.DefaultLevels = n, def; return nil }); err != nil {
			t.Fatal(err)
		}
	}
	// An even number of levels gives two rows of squares an action high, and an odd number a first pick two rows high.
	for _, n := range []int{4, 5, 11, 12} {
		setLevels(n, "1-"+strconv.Itoa(n))
		b.waitFor(strconv.Itoa(n)+" level picks", `document.querySelectorAll('#lvl-pick > button').length === `+strconv.Itoa(n))
		msg, side := pickShape(b)
		if msg != "" {
			t.Fatalf("at 1600 px with %d levels: %s", n, msg)
		}
		if math.Abs(side-40) > 0.5 {
			t.Fatalf("at 1600 px with %d levels a pick is %v px, want an action high (40 px)", n, side)
		}
		if got := rows(); got != 2 {
			t.Fatalf("at 1600 px the %d picks are on %d rows, want 2", n, got)
		}
		if _, _, _, inside := widths(); !inside {
			t.Fatalf("at 1600 px with %d levels a pick or Select All is outside the Levels box", n)
		}
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
	b.check()
}

// pickShape says what is wrong with the level picks, or "", and the side of a pick in px. Each pick is a square, an
// action high at most and never under the old 1.375rem. The first is two rows high (two squares and the gap) only when
// the number of picks is odd. Select All beside the picks is two rows high.
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
		if (near(all.top, picks[0].top) && !near(all.height, 2 * side + gap)) return ['Select All is ' + all.height + ' px high, want ' + (2 * side + gap), side];
		return ['', side];
	})()`).([]any)
	return r[0].(string), r[1].(float64)
}
