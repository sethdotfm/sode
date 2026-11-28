package app

import (
	"math"
	"testing"
)

// The Grid panel page at 1920 by 1080 against the prototype's sizes and the design review's marks.
func TestChromeGridPanelDesign(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Studio grid","type":"grid","settings":{"mode":"takeOnly","levels":[1,2]}}`)
	waitNamesAndTally(t, a, ana)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the grid", `!document.querySelector('.sode-lock.is-on') && !!`+gridXp(3, 3))

	near := func(what string, got, want float64) {
		t.Helper()
		if math.Abs(got-want) > 0.5 {
			t.Errorf("%s: %.2f px, want %.2f", what, got, want)
		}
	}
	px := func(js string) float64 {
		t.Helper()
		v, ok := b.eval(js).(float64)
		if !ok {
			t.Fatalf("%s gave no number", js)
		}
		return v
	}
	font := func(sel string) float64 {
		t.Helper()
		return px(`parseFloat(getComputedStyle(document.querySelector('` + sel + `')).fontSize)`)
	}
	rem := px(`parseFloat(getComputedStyle(document.documentElement).fontSize)`)

	// The prototype's sizes (C2-44): crosspoints 4.5rem, the Destination bar 13rem wide, the Source bar 7rem high.
	probe := b.boxes(".sode-gridpanel__probe")
	near("the Destination bar's width", probe[0].Right-probe[0].Left, 13*rem)
	near("the Source bar's height", probe[0].height(), 7*rem)
	xp := b.boxes(".sode-gridpanel__probe-xp")
	near("a crosspoint", xp[0].Right-xp[0].Left, 4.5*rem)
	sm := b.tokenPx(".sode-gridpanel__deck", "--pb-text-sm")
	for _, sel := range []string{".sode-gridpanel__find-input", "#panel-lvl-all", ".sode-gridpanel__chip", "#panel-levels .sode__box-head", ".sode-gridpanel__label"} {
		near(sel+" text", font(sel), sm)
	}
	near("Take's text", font("#panel-take"), b.tokenPx(".sode-gridpanel__deck", "--pb-text-xl"))
	take := b.boxes("#panel-take")
	near("Take's width", take[0].Right-take[0].Left, 12*rem)
	pick := b.boxes("#panel-lvl-pick > .sode__pick:nth-child(2)")
	near("a level pick", pick[0].height(), 2.6*rem)
	if b.eval(`getComputedStyle(document.querySelector('#panel-lvl-pick > .sode__pick')).fontFamily === (() => { const p = document.createElement('i'); p.style.fontFamily = 'var(--pb-font-num)'; document.body.append(p); const f = getComputedStyle(p).fontFamily; p.remove(); return f; })()`) != true {
		t.Error("the level picks are not in the number font")
	}
	b.noClip("the deck's words are whole", "#panel-lvl-all", "#panel-clear", "#panel-take", ".sode-gridpanel__find-mode")

	b.check()
}
