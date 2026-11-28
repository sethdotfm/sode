package app

import (
	"fmt"
	"testing"
)

// settledJS is true when no animation or transition runs but the panel's own pulse.
const settledJS = `document.getAnimations().every((a) => a.playState !== 'running' || a.animationName === 'sode-panel-pulse')`

// Every cell has a frame: the idle Take gets a line as Clear has. Label cells (Info, Page Name) are not faded as a
// disabled control is (C2-38). A staged destination strikes through its present source and shows the staged one at
// text-sm (C2-40).
func TestChromePanelCellLook(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Look panel","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},{"r":1,"c":2,"kind":"source","n":%d},
		{"r":2,"c":1,"kind":"info"},{"r":2,"c":2,"kind":"pageName"},{"r":2,"c":3,"kind":"clear"},{"r":2,"c":4,"kind":"take"}]}`, simDst, simSrc))
	waitNamesAndTally(t, a, ana)

	sizeTo(b, 1920, 1080)
	b.open(srv.URL + "/panels/" + id)
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the grid is drawn", `document.querySelector('.sode-panel__cell--destination span')?.textContent === 'DSM1' && `+settledJS)
	b.shot("idle")
	b.waitFor("the idle Take has an inset line", `(() => { const t = document.querySelector('.sode-panel__take'); return t.disabled && getComputedStyle(t).boxShadow.includes('inset'); })()`)
	b.waitFor("Info and Page Name are not faded", `['info', 'pageName'].every((k) => getComputedStyle(document.querySelector('.sode-panel__cell--' + k)).opacity === '1')`)

	b.eval(`document.querySelector('.sode-panel__cell--destination').click(); document.querySelector('.sode-panel__cell--source').click(); true`)
	b.waitFor("staged", `!!document.querySelector('.sode-panel__staged')`)
	b.shot("staged")
	b.waitFor("the present source is struck through", `getComputedStyle(document.querySelector('.sode-panel__cell--destination small.sode-panel__text')).textDecorationLine === 'line-through'`)
	// The cell text grows with the cell, so the staged line is text-sm or more, and smaller than the name.
	sm := b.tokenPx(".sode-panel__staged", "--pb-text-sm")
	got, _ := b.eval(`parseFloat(getComputedStyle(document.querySelector('.sode-panel__staged')).fontSize)`).(float64)
	name, _ := b.eval(`parseFloat(getComputedStyle(document.querySelector('.sode-panel__cell--destination span .sode-panel__text')).fontSize)`).(float64)
	if got < sm-layoutTol || got >= name {
		t.Errorf("the staged line is %.2f px, want text-sm (%.2f) or more and less than the name (%.2f)", got, sm, name)
	}
	b.check()
}
