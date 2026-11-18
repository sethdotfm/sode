package app

import (
	"fmt"
	"strings"
	"testing"
)

// A Drawn line on a Grid panel in Take mode, with real mouse events. A drag down one column numbers its targets and
// skips the protected row; release stages them and the line goes, and Enter takes them. A press that barely moves
// stays a click. Clear and Esc leave no line. v draws a line from the keys, Esc drops it, and Space stages it.
func TestChromeGridPanelLine(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Line grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	waitNamesAndTally(t, a, ana)
	a.mu.Lock()
	a.router.protect[3] = map[int]int{1: 9}
	a.mu.Unlock()

	// Tall enough that destination 4 is clear of the edge zone, where a drag scrolls the grid.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", gridXp(7, 4)+` !== null`)
	b.waitFor("the lock screen goes", `document.querySelector('.sode-lock').hidden`)
	b.waitFor("destination 3 shows protected", `document.querySelector('.sode-gridpanel__dst[data-dst="3"]').classList.contains('pb-xpt--protected')`)

	at := func(src, dst int, dy float64) (float64, float64) {
		r := b.eval(`(() => { const r = ` + gridXp(src, dst) + `.getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`).([]any)
		return r[0].(float64), r[1].(float64) + dy
	}
	mouse := func(typ string, x, y float64) {
		ev := map[string]any{"type": typ, "x": x, "y": y, "button": "left", "buttons": 1, "clickCount": 1}
		if typ == "mouseReleased" {
			ev["buttons"] = 0
		}
		b.call("Input.dispatchMouseEvent", ev)
	}
	staged := func(src, dst int) string { return gridXp(src, dst) + `.dataset.s === 'staged'` }
	// The layer's hidden must be the attribute: an svg element has no hidden property, so only the attribute hides it.
	const lineGone = `getComputedStyle(document.querySelector('.sode-gridpanel__lines')).display === 'none' && !document.querySelector('[data-n], [data-skip]')`
	const lineShows = `getComputedStyle(document.querySelector('.sode-gridpanel__lines')).display !== 'none'`
	drag := func(x0, y0, x1, y1 float64) {
		mouse("mousePressed", x0, y0)
		for k := 1; k <= 12; k++ {
			mouse("mouseMoved", x0+(x1-x0)*float64(k)/12, y0+(y1-y0)*float64(k)/12)
		}
	}

	// A press that moves less than half a crosspoint is a click.
	x, y := at(9, 0, 0)
	mouse("mousePressed", x, y)
	mouse("mouseMoved", x+4, y+3)
	mouse("mouseReleased", x+4, y+3)
	b.waitFor("a small move stays a click and stages", staged(9, 0))
	gridKey(b, "Escape")
	b.waitFor("Esc drops it", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)

	// A line down source 7 from destination 1 to destination 4, drawn a little off vertical: it snaps to vertical.
	x0, y0 := at(7, 1, 0)
	_, y1 := at(7, 4, 0)
	mouse("mousePressed", x0, y0)
	for k := 1; k <= 12; k++ {
		mouse("mouseMoved", x0+8*float64(k)/12, y0+(y1-y0)*float64(k)/12)
	}
	b.waitFor("the line snaps to vertical with numbered targets and the protected row skipped", `(() => {
		const n = (d) => `+gridXp(7, 0)+`.parentNode.querySelector('.sode-gridpanel__xp[data-src="7"][data-dst="' + d + '"]').dataset.n;
		const l = document.querySelector('.sode-gridpanel__line');
		return `+lineShows+` && l.getAttribute('x1') === l.getAttribute('x2') && n(1) === '1' && n(2) === '2' && n(4) === '3' &&
			`+gridXp(7, 3)+`.hasAttribute('data-skip');
	})()`)
	b.shot("drawing")
	mouse("mouseReleased", x0+8, y1)
	b.waitFor("release stages three routes, takes nothing and the line goes", staged(7, 1)+` && `+staged(7, 2)+` && `+staged(7, 4)+` && !(`+staged(7, 3)+`) && `+lineGone)
	b.shot("staged")
	b.waitFor("no square cursor after the release: no focused crosspoint and no box on the cursor or any crosspoint", `(() => {
		const boxed = (n) => { const c = getComputedStyle(n); return c.outlineStyle !== 'none' && parseFloat(c.outlineWidth) > 0; };
		const xps = [...document.querySelectorAll('.sode-gridpanel__xp')];
		return !document.activeElement.closest('.sode-gridpanel__xp') && !boxed(document.querySelector('.sode-gridpanel__cursor')) && !xps.some(boxed);
	})()`)
	if !b.eval(`document.getElementById('confirm').hidden`).(bool) {
		t.Fatal("release opened the confirm in Take mode")
	}
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter opens the confirm for three destinations", `document.getElementById('confirm-body').textContent.includes('3 destinations')`)
	gridConfirm(b)
	for _, d := range []int{1, 2, 4} {
		tallyHas(t, a, ana, d, 7)
	}
	if strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"dst":3,"level":1,"src":7`) {
		t.Error("the protected row was taken")
	}

	// A staged line and Clear, then a staged line and Esc: no line is left.
	x0, y0 = at(9, 1, 0)
	_, y1 = at(9, 2, 0)
	drag(x0, y0, x0, y1)
	mouse("mouseReleased", x0, y1)
	b.waitFor("a second line stages two routes", staged(9, 1)+` && `+staged(9, 2)+` && `+lineGone)
	b.eval(`document.getElementById('panel-clear').click()`)
	b.waitFor("Clear drops them and no line shows", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]') && `+lineGone)
	drag(x0, y0, x0, y1)
	b.waitFor("a line shows while it draws", lineShows)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc during the drag drops the line", lineGone)
	mouse("mouseReleased", x0, y1)
	b.waitFor("the release after Esc stages nothing", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]') && `+lineGone)

	// From the keys: the pointer places the cursor, v starts a line there, Esc drops it, and Space stages one.
	px, py := at(5, 0, 0)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": px, "y": py})
	gridCursorOn(b, "the pointer places the cursor", 0, 5)
	realKey(b, "v", "KeyV", 86)
	realKey(b, "j", "KeyJ", 74)
	b.waitFor("v and j draw a line of two, and Clear can drop it", lineShows+` && `+gridXp(5, 1)+`.dataset.n === '2' && !document.getElementById('panel-clear').disabled`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc drops the line and stages nothing", lineGone+` && !document.querySelector('[data-s="staged"]')`)
	realKey(b, "v", "KeyV", 86)
	realKey(b, "j", "KeyJ", 74)
	realKey(b, "l", "KeyL", 76)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter does nothing while a line is drawn", `document.getElementById('confirm').hidden`)
	realKey(b, " ", "Space", 32)
	b.waitFor("Space stages the line and the line goes", staged(5, 1)+` && `+staged(6, 2)+` && `+lineGone+` && document.getElementById('confirm').hidden`)
	realKey(b, "Enter", "Enter", 13)
	b.waitFor("Enter opens the confirm for two destinations", `document.getElementById('confirm-body').textContent.includes('2 destinations')`)
	gridConfirm(b)
	tallyHas(t, a, ana, 1, 5)
	tallyHas(t, a, ana, 2, 6)
	// The router can confirm before the page has the POST's answer.
	b.waitFor("nothing staged after the take", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)

	// A drag across names picks them: along the Source bar from 2 to 5 and back to 4, then down the Destination bar.
	name := func(kind string, n int) (float64, float64) {
		r := b.eval(fmt.Sprintf(`(() => { const r = document.querySelector('.sode-gridpanel__%s[data-%s="%d"]:not([hidden])').getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2]; })()`, kind, kind, n)).([]any)
		return r[0].(float64), r[1].(float64)
	}
	picked := func(kind string) string {
		return fmt.Sprintf(`[...document.querySelectorAll('.sode-gridpanel__%s[data-picked]:not([hidden])')].map((n) => +n.dataset.%s).sort((a, b) => a - b).join(',')`, kind, kind)
	}
	x0, y0 = name("src", 2)
	x1, _ := name("src", 5)
	x2, _ := name("src", 4)
	drag(x0, y0, x1, y0)
	b.waitFor("a drag along the Source bar picks the names it passes", picked("src")+` === '2,3,4,5'`)
	mouse("mouseMoved", x2, y0+20)
	b.waitFor("back to source 4, off the bar, unpicks source 5", picked("src")+` === '2,3,4'`)
	mouse("mouseReleased", x2, y0+20)
	b.shot("name drag")
	if got := b.eval(picked("src")).(string); got != "2,3,4" {
		t.Fatalf("the release changed the picks to %q", got)
	}
	if sel := b.eval(`getSelection().toString()`).(string); sel != "" {
		t.Errorf("the drag selected text %q", sel)
	}
	x0, y0 = name("dst", 0)
	_, y1 = name("dst", 2)
	drag(x0, y0, x0, y1)
	mouse("mouseReleased", x0, y1)
	b.waitFor("a drag down the Destination bar picks those, and keeps the source picks", picked("dst")+` === '0,1,2' && `+picked("src")+` === '2,3,4'`)
	x0, y0 = name("src", 2)
	mouse("mousePressed", x0, y0)
	mouse("mouseReleased", x0, y0)
	b.waitFor("a plain click on a name still picks only it", picked("src")+` === '2'`)
	b.check()
}
