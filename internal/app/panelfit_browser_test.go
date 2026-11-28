package app

import (
	"fmt"
	"testing"
)

// panelFitSizes are the desktop windows a Buttons panel page must fill with no page scroll.
var panelFitSizes = [][2]int{{1920, 1080}, {2560, 1440}, {3840, 2160}}

// A Buttons panel page is full width when nothing is stored. The grid is never higher than the window. A wide panel
// fills the width. A square panel is as high as the window and centred. Cell text grows with the cell. A kiosk and a
// panel with no title bar are full width, also when Normal width is stored.
func TestChromePanelFullWidthFits(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	cells := fmt.Sprintf(`[{"r":1,"c":1,"kind":"destination","n":%d},{"r":1,"c":2,"kind":"source","n":%d},{"r":2,"c":1,"kind":"take"}]`, simDst, simSrc)
	wide := makePanel(t, a, ana, `{"name":"Wide panel","rows":3,"cols":8,"cells":`+cells+`}`)
	square := makePanel(t, a, ana, `{"name":"Square panel","rows":4,"cols":4,"cells":`+cells+`}`)
	kiosk := makePanel(t, a, ana, `{"name":"Kiosk panel","rows":4,"cols":4,"cells":`+cells+`,"settings":{"kiosk":true}}`)
	bare := makePanel(t, a, ana, `{"name":"Bare panel","rows":4,"cols":4,"cells":`+cells+`,"settings":{"titleBar":false}}`)
	waitNamesAndTally(t, a, ana)

	sizeTo(b, 1920, 1080)
	b.open(srv.URL + "/panels/" + wide)
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the grid is drawn", `document.querySelector('.sode-panel__cell--destination span')?.textContent === 'DSM1'`)
	b.waitFor("nothing stored", `localStorage.getItem('sode-panel-full') === null`)

	settled := `document.getAnimations().every((a) => a.playState !== 'running' || a.animationName === 'sode-panel-pulse')`
	noScroll := `document.documentElement.scrollHeight <= innerHeight && document.documentElement.scrollWidth <= innerWidth`
	// The content box of the page, and the grid's box in it.
	frame := `(() => {
		const m = document.querySelector('.sode-panels'), c = getComputedStyle(m), r = m.getBoundingClientRect();
		const g = document.getElementById('panel-grid').getBoundingClientRect();
		return { l: r.left + parseFloat(c.paddingLeft), r: r.right - parseFloat(c.paddingRight), g };
	})()`
	textPx := `parseFloat(getComputedStyle(document.querySelector('.sode-panel__cell--destination .sode-panel__text')).fontSize)`

	var first float64
	for _, s := range panelFitSizes {
		sizeTo(b, s[0], s[1])
		at := fmt.Sprintf("wide panel at %dx%d", s[0], s[1])
		b.waitFor(at+": settled", settled)
		b.shot(fmt.Sprintf("wide-%dx%d", s[0], s[1]))
		b.waitFor(at+": full width by default", `!!document.querySelector('.sode-panels--full') && document.getElementById('panel-full').getAttribute('aria-pressed') === 'true'`)
		b.waitFor(at+": no page scroll", noScroll)
		b.waitFor(at+": the grid fills the page's width", `(() => { const f = `+frame+`; return Math.abs(f.g.left - f.l) < 1 && Math.abs(f.g.right - f.r) < 1; })()`)
		px, _ := b.eval(textPx).(float64)
		if first == 0 {
			first = px
		} else if px <= first {
			t.Errorf("%s: cell text is %.2f px, no larger than %.2f px at 1920x1080", at, px, first)
		}
	}

	b.open(srv.URL + "/panels/" + square)
	b.waitFor("the square grid is drawn", `document.querySelector('.sode-panel__cell--destination span')?.textContent === 'DSM1'`)
	for _, s := range panelFitSizes {
		sizeTo(b, s[0], s[1])
		at := fmt.Sprintf("square panel at %dx%d", s[0], s[1])
		b.waitFor(at+": settled", settled)
		b.shot(fmt.Sprintf("square-%dx%d", s[0], s[1]))
		b.waitFor(at+": no page scroll", noScroll)
		b.waitFor(at+": the grid is centred and narrower than the page", `(() => { const f = `+frame+`; return f.g.right - f.g.left < f.r - f.l - 10 && Math.abs((f.g.left - f.l) - (f.r - f.g.right)) < 1; })()`)
	}

	sizeTo(b, 1920, 1080)
	b.eval(`localStorage.setItem('sode-panel-full', '0'); true`)
	b.open(srv.URL + "/panels/" + square)
	b.waitFor("Normal width stored: capped", `!!document.querySelector('.sode-panel__cell') && !document.querySelector('.sode-panels--full') && getComputedStyle(document.querySelector('.sode-panels')).maxWidth !== 'none'`)
	b.waitFor("Normal width stored: no page scroll", noScroll)
	for _, p := range []struct{ what, id string }{{"kiosk", kiosk}, {"no title bar", bare}} {
		b.open(srv.URL + "/panels/" + p.id)
		b.waitFor(p.what+": drawn", `!!document.querySelector('.sode-panel__cell--destination span')`)
		b.waitFor(p.what+": full width with Normal width stored", `!!document.querySelector('.sode-panels--full') && getComputedStyle(document.querySelector('.sode-panels')).maxWidth === 'none'`)
		b.waitFor(p.what+": no page scroll", noScroll)
	}
	b.check()
}

// The panel title is on the Lock chip's centre line. In Full width it clears the chip by one space-4. In Normal width it
// starts on the grid's left edge when the chip is outside the page's column.
func TestChromePanelTitleLine(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Title panel","rows":3,"cols":8,"cells":[{"r":1,"c":1,"kind":"destination","n":%d}]}`, simDst))
	waitNamesAndTally(t, a, ana)

	sizeTo(b, 1920, 1080)
	b.open(srv.URL + "/panels/" + id)
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the grid is drawn", `document.querySelector('.sode-panel__cell--destination span')?.textContent === 'DSM1'`)
	settled := `document.getAnimations().every((a) => a.playState !== 'running')`

	b.waitFor("full width", `!!document.querySelector('.sode-panels--full') && !!document.querySelector('.sode-lock__who')?.offsetWidth && `+settled)
	b.shot("full")
	b.sameMiddle("the title is on the chip's centre line", ".sode-lock__who-bar", "#panel-title")
	// --sode-who-w is the chip's width rounded up, so the gap is space-4 plus less than 1 px.
	x := b.boxes(".sode-lock__who", "#panel-title")
	if gap, want := x[1].Left-x[0].Right, b.tokenPx("#panel-title", "--pb-space-4"); gap < want-layoutTol || gap > want+1 {
		t.Errorf("the title is %.2f px from the chip, want %.2f", gap, want)
	}

	b.eval(`document.getElementById('panel-full').click(); true`)
	b.waitFor("normal width", `!document.querySelector('.sode-panels--full') && `+settled)
	b.shot("normal")
	b.sameMiddle("the title is on the chip's centre line in Normal width", ".sode-lock__who-bar", "#panel-title")
	// At 1920 px the column starts 1 px right of the chip, so the title moves only by what space-4 from the chip needs.
	x = b.boxes(".sode-lock__who", "#panel-title", "#panel-grid")
	if x[1].Left < x[0].Right+b.tokenPx("#panel-title", "--pb-space-4")-layoutTol || x[1].Left > x[2].Left+b.tokenPx("#panel-title", "--pb-space-4") {
		t.Errorf("Normal width at 1920: the title starts at %.2f, the chip ends at %.2f and the grid starts at %.2f", x[1].Left, x[0].Right, x[2].Left)
	}
	sizeTo(b, 2560, 1440)
	b.waitFor("2560x1440", `innerWidth === 2560 && `+settled)
	b.sameLeft("the title, the instance name and the grid start on one edge", "#panel-title", "#panel-device", "#panel-grid")
	b.check()
}
