package app

import (
	"testing"
)

// A Grid panel's lists in the editor at 1920x1080: the list bar's Pin, Move up, Move down and Remove sit over the
// row's own, in a list that scrolls (Sources) and in one that does not (Destinations). Names start at one edge
// whatever the number's length, in the panel's list and in Add from the router. While a list follows the router,
// its rows have only Pin, still under the bar's Pin, and the heading does not move when its note shows.
func TestChromeGridPanelEditorListLines(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t, "SIM_SOURCES=2500", "SIM_DESTS=5")
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	asGrid(b, true)
	b.waitFor("the lists fill", `!!document.querySelector('#ed-sources-list li[data-id="20"]') && !!document.querySelector('#ed-destinations-list li[data-id="4"]')`)
	b.waitFor("Sources scrolls and Destinations does not", `(() => {
		const s = document.getElementById('ed-sources-list'), d = document.getElementById('ed-destinations-list');
		return s.scrollHeight > s.clientHeight && d.scrollHeight <= d.clientHeight;
	})()`)
	headTop := b.boxes("#ed-sources-col .sode-edge__h")[0].Top

	tools := [][2]string{{"pin", "pin"}, {"up", "up"}, {"down", "down"}, {"remove", "delete"}}
	for _, k := range []string{"sources", "destinations"} {
		for _, x := range tools {
			b.sameLeft(k+": the bar's "+x[0]+" is over the rows' "+x[0], "#ed-"+k+"-sel-"+x[0], "#ed-"+k+`-list li [data-act="`+x[1]+`"]`)
		}
	}
	b.eval(`(() => {
		const l = document.getElementById('ed-sources-list'), r = l.querySelectorAll('li');
		l.scrollTop = (r[1].offsetTop - r[0].offsetTop) * 990;
		l.dispatchEvent(new Event('scroll'));
		return true;
	})()`)
	b.waitFor("the list draws the rows round 1000", `!!document.querySelector('#ed-sources-list li[data-id="995"]') && !!document.querySelector('#ed-sources-list li[data-id="1005"]')`)
	b.sameLeft("source names on one edge from 3 to 4 digits", "#ed-sources-list li .sode-pages__name")

	b.eval(`document.getElementById('ed-sources-clear').click(); true`)
	b.waitFor("Clear empties Sources", `!document.querySelector('#ed-sources-list li')`)
	b.eval(`const f = document.getElementById('ed-sources-find'); f.value = '995-1005'; f.dispatchEvent(new Event('input', { bubbles: true })); true`)
	b.waitFor("Add from the router lists 995 to 1005", `document.querySelectorAll('#ed-sources-router .sode-rlist__row:not([hidden])').length === 11`)
	b.sameLeft("router list names on one edge from 995 to 1005", "#ed-sources-router .sode-rlist__row:not([hidden]) .sode-pages__name")

	b.eval(`document.getElementById('ed-sources-all').click(); true`)
	b.waitFor("Sources follows the router", `document.getElementById('ed-sources-all').checked && !!document.querySelector('#ed-sources-list li[data-id="20"]')`)
	b.waitFor("its rows have only Pin, and the bar shows no Move or Remove", `(() => {
		const row = document.querySelector('#ed-sources-list li[data-id="3"]');
		const none = ['up', 'down', 'delete'].every((a) => !row.querySelector('[data-act="' + a + '"]')) && !!row.querySelector('[data-act="pin"]');
		const gone = (id) => { const e = document.getElementById('ed-sources-sel-' + id); return e.hidden || getComputedStyle(e).visibility === 'hidden'; };
		return none && ['top', 'up', 'down', 'end', 'remove'].every(gone) && !gone('pin');
	})()`)
	b.sameLeft("following: the bar's Pin is over the rows' Pin", "#ed-sources-sel-pin", `#ed-sources-list li [data-act="pin"]`)
	b.waitFor("the Sources note is under its heading, in the list's column", `(() => {
		const n = document.getElementById('ed-sources-following');
		return !!n && !n.hidden && document.getElementById('ed-sources-col').contains(n) && n.textContent === 'Follows the router, in router order. Pins and artwork still apply.'
			&& document.getElementById('ed-destinations-following').hidden;
	})()`)
	if top := b.boxes("#ed-sources-col .sode-edge__h")[0].Top; top != headTop {
		t.Errorf("the Sources heading moved from %v to %v when its note showed", headTop, top)
	}
	b.shot("lines")
}
