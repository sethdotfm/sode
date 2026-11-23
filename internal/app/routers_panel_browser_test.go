package app

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// twoLevelRouterB: router B has 2 levels, router A the simulator's 17.
func twoLevelRouterB(c *config.Config) { c.MoreRouters[0].Levels, c.MoreRouters[0].DefaultLevels = 2, "" }

// lastTakeLevels gives the levels of the newest History entry, which must be on router uid.
func lastTakeLevels(t *testing.T, a *App, key, uid string) []int {
	t.Helper()
	var hist struct {
		Entries []struct {
			Router string `json:"router"`
			Levels []int  `json:"levels"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(apiDo(a, "GET", "/api/v1/history", key, nil).Body.Bytes(), &hist); err != nil || len(hist.Entries) == 0 {
		t.Fatalf("history: %v %+v", err, hist)
	}
	if e := hist.Entries[0]; e.Router != uid {
		t.Errorf("the newest entry is on router %q, want %q", e.Router, uid)
	}
	return hist.Entries[0].Levels
}

// itemSpan is SPAN in web/routerids.js: an item number of router B is itemSpan plus its router number.
const itemSpan = 0x10000

// A Grid panel holds sources and destinations of two routers. The page reads each router's names and tally and shows
// each item with its prefix. It takes on router B and refuses a route across the two routers. The editor lists the
// members of both routers, and its Add from the router lists router B's items with their prefix.
func TestChromeGridPanelTwoRouters(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 Stage Cam", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB}, twoLevelRouterB)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Two routers","type":"grid","settings":{"mode":"take","levels":{%[1]q:[1,2,3]}},
		"sources":{"members":[{"router":%[1]q,"n":10},{"router":%[2]q,"n":11}]},
		"destinations":{"members":[{"router":%[1]q,"n":2},{"router":%[2]q,"n":3}]}}`, uids[0], uids[1]))
	bSrc, bDst := itemSpan+11, itemSpan+3

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	b.waitFor("the lock screen goes", `!document.querySelector('.sode-lock.is-on')`)
	b.waitFor("each router's names, with the router prefix", fmt.Sprintf(`(() => {
		const d = document.querySelector('.sode-gridpanel__dst[data-dst="%d"]');
		const a = document.querySelector('.sode-gridpanel__dst[data-dst="2"]');
		return d?.textContent.startsWith('Stage Mon') && d.dataset.num === 'B.4' && a?.textContent.startsWith('DSM1') && a.dataset.num === 'A.3';
	})()`, bDst))
	// Each destination shows the source it has now that the panel does not show: router B's comes from router B's tally.
	b.waitFor("router B's tally names its own source", fmt.Sprintf(`document.querySelector('.sode-gridpanel__dst[data-dst="%d"]').textContent === 'Stage MonCG1 Key'`, bDst))

	// A route across the two routers is refused at the press.
	b.eval(gridXp(10, bDst) + `.click(); true`)
	b.waitFor("a route across routers is refused", gridMessage+`.includes('are on two routers')`)
	b.waitFor("nothing is staged", `!document.querySelector('.sode-gridpanel__xp[data-s="staged"]')`)

	// A route on router B goes to router B, and its tally comes back to the crosspoint.
	staged := func(src, dst int) string { return gridXp(src, dst) + `.dataset.s === 'staged'` }
	b.eval(gridXp(bSrc, bDst) + `.click(); true`)
	b.waitFor("a press on router B stages", staged(bSrc, bDst))
	b.eval(`document.getElementById('panel-take').click(); true`)
	gridConfirm(b)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	routerTallyHas(t, a, ana, uids[0], 3, 3)
	// The Levels box holds router A's 1-3; router B has 2 levels, so the route on B takes 1 and 2 only.
	if got := lastTakeLevels(t, a, ana, uids[1]); !slices.Equal(got, []int{1, 2}) {
		t.Errorf("the take on router B sent levels %v, want [1 2]", got)
	}
	b.waitFor("router B's tally reaches the crosspoint", gridXp(bSrc, bDst)+`.dataset.on?.startsWith('1')`)

	// The editor lists the members of both routers, and Add from the router has router B's items.
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the lock goes in the editor", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the destination list has both routers, with prefixes", fmt.Sprintf(`(() => {
		const row = (n) => document.querySelector('#ed-destinations-list li[data-id="' + n + '"]')?.textContent || '';
		return row(2).includes('A.3') && row(2).includes('DSM1') && row(%d).includes('B.4') && row(%d).includes('Stage Mon');
	})()`, bDst, bDst))
	// Add from the router lists router B's items too, and a typed prefix finds only that router's.
	b.waitFor("Add from the router finds router B's items by prefix", fmt.Sprintf(`(() => {
		const f = document.getElementById('ed-destinations-find');
		if (f.value !== 'B.5') { f.value = 'B.5'; f.dispatchEvent(new Event('input', {bubbles: true})); }
		const rows = [...document.querySelectorAll('#ed-destinations-router .sode-rlist__row:not([hidden])')];
		return rows.some((r) => r.id === 'ed-destinations-opt-%d') && rows.every((r) => r.textContent.startsWith('B.5'));
	})()`, itemSpan+4))
	b.check()
}

// A Buttons panel with cells on two routers: each cell shows its own router's names and tally. A take goes to the
// router of its cells. A source on a router that is not the destination's is refused at the press.
func TestChromeButtonsPanelTwoRouters(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB := startSim(t, simNames(t, "src 11 Stage Cam", "dst 3 Stage Mon"))
	a := simRoutersApp(t, []string{simA, simB}, twoLevelRouterB)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "Stage Mon")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Two routers","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","router":%[2]q,"n":3},
		{"r":1,"c":2,"kind":"source","router":%[2]q,"n":11},
		{"r":1,"c":3,"kind":"source","router":%[1]q,"n":%[3]d},
		{"r":1,"c":4,"kind":"take"}],"settings":{"mode":"take"}}`, uids[0], uids[1], simSrc))

	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	signInOnPad(b)
	cell := func(r, c int) string {
		return fmt.Sprintf(`document.querySelector('.sode-panel__cell[style*="grid-area: %d / %d"]')`, r, c)
	}
	b.waitFor("the cells", `document.querySelectorAll('.sode-panel__cell').length === 4`)
	b.waitFor("router B's names and tally on its destination", cell(1, 1)+`?.querySelector('span')?.textContent === 'Stage Mon' && `+cell(1, 1)+`.querySelector('small')?.textContent === 'CG1 Key'`)
	b.waitFor("router B's source name", cell(1, 2)+`?.querySelector('span')?.textContent === 'Stage Cam'`)

	b.eval(cell(1, 1) + `.click()`)
	b.eval(cell(1, 3) + `.click()`)
	b.waitFor("a source on router A is refused for router B's destination", `document.getElementById('panel-status').textContent.includes('are on two routers')`)
	b.eval(cell(1, 2) + `.click()`)
	b.waitFor("Take armed", `!`+cell(1, 4)+`.disabled`)
	b.eval(cell(1, 4) + `.click()`)
	routerTallyHas(t, a, ana, uids[1], 3, 11)
	routerTallyHas(t, a, ana, uids[0], 3, 3)
	// Router A has 17 levels and router B 2: the take on B sends B's two levels.
	if got := lastTakeLevels(t, a, ana, uids[1]); !slices.Equal(got, []int{1, 2}) {
		t.Errorf("the take on router B sent levels %v, want [1 2]", got)
	}
	b.waitFor("router B's new source under the destination", cell(1, 1)+`.querySelector('small')?.textContent === 'Stage Cam'`)
	b.check()
}
