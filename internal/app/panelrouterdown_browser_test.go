package app

import (
	"fmt"
	"testing"
)

// A Buttons panel whose cells are on router B, with router A up and router B's simulator stopped (C2-41). Before the
// fix the page looked idle: no mark, live cells, Take ready. Now the cells have the down mark and aria-disabled, a press
// does nothing, and the status line names the router.
func TestChromePanelCellsOnStoppedRouter(t *testing.T) {
	b := startBrowser(t)
	simA, _ := startSimStoppable(t)
	simB, stopB := startSimStoppable(t)
	a := simRoutersApp(t, []string{simA, simB})
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "DSM1")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"B panel","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"destination","router":%[1]q,"n":%[2]d},
		{"r":1,"c":2,"kind":"source","router":%[1]q,"n":%[3]d},
		{"r":1,"c":3,"kind":"take"}],"settings":{"mode":"take"}}`, uids[1], simDst, simSrc))

	sizeTo(b, 1920, 1080)
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("router B's names", `document.querySelector('.sode-panel__cell--destination span')?.textContent === 'DSM1'`)
	stopB()
	b.waitFor("the status line names router B", `document.getElementById('panel-status').textContent === 'Router B is down. Its cells do nothing.' && !document.getElementById('panel-status').hidden`)
	b.waitFor("its cells have the mark and aria-disabled", `(() => {
		const cs = [...document.querySelectorAll('.sode-panel__cell--destination, .sode-panel__cell--source')];
		return cs.every((c) => c.getAttribute('aria-disabled') === 'true' && c.querySelector('.sode-panel__status-mark--down[aria-label="Router down"]'));
	})()`)
	b.waitFor("the page is not Offline, since router A is up", `document.getElementById('panel-offline').hidden`)
	b.shot("router-b-down")
	b.eval(`document.querySelector('.sode-panel__cell--destination').click(); true`)
	b.waitFor("a press does nothing", `document.querySelector('.sode-panel__cell--destination').dataset.s === 'idle'`)
	b.check()
}
