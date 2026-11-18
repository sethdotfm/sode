package app

import (
	"fmt"
	"testing"
)

// The grid's scroll bars are Sode's own big ones (bigscroll.js) on both axes, with the same thickness. A corner
// square is where they meet, and native bars take no room. Nothing in the grid paints over them. A wheel turned over
// the horizontal one scrolls the grid across, not down; over the crosspoints the same wheel scrolls down. A press on
// a scroll bar stages nothing, and a thumb drag scrolls.
func TestChromeGridPanelScrollBars(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wheel grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden && !!`+gridXp(3, 3))
	sc := `document.querySelector('.sode-gridpanel__scroll')`
	xb := `document.querySelector('.sode-gridpanel__frame > .sode-bigscroll--x')`
	yb := `document.querySelector('.sode-gridpanel__frame > .sode-bigscroll--y')`
	corner := `document.querySelector('.sode-gridpanel__frame > .sode-bigscroll__corner')`
	b.waitFor("both scroll bars and the corner show", `!!`+xb+` && !`+xb+`.hidden && !!`+yb+` && !`+yb+`.hidden && !`+corner+`.hidden`)
	b.waitFor("the native bars take no room", sc+`.offsetHeight === `+sc+`.clientHeight && `+sc+`.offsetWidth === `+sc+`.clientWidth`)
	b.waitFor("both scroll bars are 25 px thick (1.4rem at the Grid page's 18 px root)", xb+`.offsetHeight === 25 && `+yb+`.offsetWidth === 25`)
	b.waitFor("they sit on the scroller's bottom and right edges and meet at the corner", `(() => {
		const s = `+sc+`.getBoundingClientRect(), x = `+xb+`.getBoundingClientRect(), y = `+yb+`.getBoundingClientRect();
		const c = `+corner+`.getBoundingClientRect();
		const near = (p, q) => Math.abs(p - q) < 1;
		return near(x.bottom, s.bottom) && near(x.left, s.left) && near(x.right, c.left) &&
			near(y.right, s.right) && near(y.top, s.top) && near(y.bottom, c.top) && near(c.right, s.right) && near(c.bottom, s.bottom);
	})()`)
	// Each end of each bar, past the sticky name bars and pinned bands, finds the bar on top.
	b.waitFor("nothing in the grid paints over the scroll bars", `(() => {
		const on = (bar, x, y) => document.elementFromPoint(x, y)?.closest('.sode-bigscroll') === bar;
		const x = `+xb+`, y = `+yb+`, xr = x.getBoundingClientRect(), yr = y.getBoundingClientRect();
		return on(x, xr.left + 4, xr.top + xr.height / 2) && on(x, xr.right - 4, xr.top + xr.height / 2) &&
			on(y, yr.left + yr.width / 2, yr.top + 4) && on(y, yr.left + yr.width / 2, yr.bottom - 4);
	})()`)

	pos := func() (float64, float64) {
		v := b.eval(`[` + sc + `.scrollLeft, ` + sc + `.scrollTop]`).([]any)
		return v[0].(float64), v[1].(float64)
	}
	mid := func(bar string) (float64, float64) {
		v := b.eval(`(() => { const r = ` + bar + `.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`).([]any)
		return v[0].(float64), v[1].(float64)
	}
	mouse := func(typ string, x, y float64) {
		ev := map[string]any{"type": typ, "x": x, "y": y, "button": "left", "buttons": 1, "clickCount": 1}
		if typ == "mouseReleased" {
			ev["buttons"] = 0
		}
		b.call("Input.dispatchMouseEvent", ev)
	}

	// A press on each scroll bar's track pages the grid but stages nothing.
	bx, by := mid(xb)
	mouse("mousePressed", bx+200, by)
	mouse("mouseReleased", bx+200, by)
	yx, yy := mid(yb)
	mouse("mousePressed", yx, yy+150)
	mouse("mouseReleased", yx, yy+150)
	b.waitFor("track presses page the grid", sc+`.scrollLeft > 0 && `+sc+`.scrollTop > 0`)
	b.waitFor("a press on a scroll bar stages nothing", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)
	b.eval(sc + `.scrollLeft = 0; ` + sc + `.scrollTop = 0; true`)
	b.waitFor("back at the start", sc+`.scrollLeft === 0 && `+sc+`.scrollTop === 0`)

	// P10: a wheel over the horizontal bar scrolls across.
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": bx, "y": by})
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": bx, "y": by, "deltaX": 0, "deltaY": 240})
	b.waitFor("a wheel over the horizontal bar scrolls across", sc+`.scrollLeft > 0 && `+sc+`.scrollTop === 0`)
	left, _ := pos()

	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": bx, "y": 400})
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseWheel", "x": bx, "y": 400, "deltaX": 0, "deltaY": 240})
	b.waitFor("over the crosspoints it scrolls down", fmt.Sprintf(`%s.scrollTop > 0 && %s.scrollLeft === %v`, sc, sc, left))

	// A drag on the vertical thumb scrolls down.
	_, top := pos()
	th := yb + `.firstElementChild`
	tx, ty := mid(th)
	mouse("mousePressed", tx, ty)
	for k := 1; k <= 6; k++ {
		mouse("mouseMoved", tx, ty+float64(k)*20)
	}
	mouse("mouseReleased", tx, ty+120)
	b.waitFor("a thumb drag scrolls down", fmt.Sprintf(`%s.scrollTop > %v + 100`, sc, top))
	b.waitFor("still nothing staged", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)

	// The gutters: at the far end the last crosspoint sits clear of both bars.
	b.eval(sc + `.scrollLeft = 1e9; ` + sc + `.scrollTop = 1e9; true`)
	b.waitFor("the last crosspoint is clear of both scroll bars", `(() => {
		const n = `+gridXp(199, 119)+`;
		if (!n) return false;
		const r = n.getBoundingClientRect();
		return r.right <= `+yb+`.getBoundingClientRect().left + 0.5 && r.bottom <= `+xb+`.getBoundingClientRect().top + 0.5;
	})()`)
	b.check()
}
