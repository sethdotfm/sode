package app

import "testing"

// bandCounts is how many of each band control show under root, as "name=n" joined by commas. A phone-only part
// (gridpanelphone.css) counts only when it has a box.
const bandCounts = `((root) => {
	const shown = (n) => n.getClientRects().length > 0 && getComputedStyle(n).visibility !== 'hidden';
	const n = (sel) => [...root.querySelectorAll(sel)].filter(shown).length;
	return [
		['switch', '.sode-gridpanel__find-mode'],
		['choice', '.sode-gridpanel__find-choice'],
		['srcArrow', '.sode-gridpanel__label--src > svg'],
		['dstArrow', '.sode-gridpanel__label--dst > svg'],
		['range', '.sode-gridpanel__range'],
		['pos', '.sode-gridpanel__pos'],
		['more', '.sode-gridpanel__more'],
		['levelWords', '.sode-gridpanel__lv'],
		['srcBox', '.sode-gridpanel__find[data-kind="srcs"]'],
		['dstBox', '.sode-gridpanel__find[data-kind="dsts"]'],
		['deck', '.sode-gridpanel__deck'],
	].map(([k, s]) => k + '=' + n(s)).join() + ',filler=' + /Find marks the matches/.test(root.textContent);
})`

// The Live Editor draws the band's controls as the panel page does. There is one Find/Filter switch, one arrow on each
// corner word and one box per bar. No part of the phone layout shows.
func TestChromeGridPanelLiveBand(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	id := liveGridPanel(t, b, srv.URL)

	const want = "switch=1,choice=0,srcArrow=1,dstArrow=1,range=0,pos=0,more=0,levelWords=0,srcBox=1,dstBox=1,deck=1,filler=false"
	b.eval(`document.getElementById('ed-live').click(); true`)
	b.waitFor("the live grid shows", `!!document.querySelector('#ed-live-view .sode-gridpanel__xp') && !!document.querySelector('#ed-live-view .sode-gridpanel__find-mode')`)
	b.shot("live")
	if got := b.eval(bandCounts + `(document.getElementById('ed-live-view'))`); got != want {
		t.Errorf("Live Editor band: %v, want %v", got, want)
	}

	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the panel page shows", `!!document.querySelector('.sode-gridpanel__xp') && !!document.querySelector('.sode-gridpanel__find-mode')`)
	if got := b.eval(bandCounts + `(document.body)`); got != want {
		t.Errorf("panel page band: %v, want %v", got, want)
	}
	b.check()
}
