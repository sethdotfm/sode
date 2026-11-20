package app

import (
	"testing"
)

// The Live Editor's Smart menu beside picked names on the live grid. It checks where the menu opens, one action of each
// kind, where Insert puts names, the keys, Follow the router and the phone layout.
func TestChromeGridPanelLiveMenu(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := liveGridPanel(t, b, srv.URL)

	b.eval(`
		window.names = (k) => [...document.querySelectorAll('#ed-live-view .sode-gridpanel__' + k + '[data-' + k + ']')].map((x) => [x.dataset[k], x.getBoundingClientRect()]).filter(([, r]) => r.width > 0).sort((x, y) => (k === 'src' ? x[1].left - y[1].left : x[1].top - y[1].top)).map(([n]) => n).slice(0, 6).join();
		window.nameOf = (k, n) => document.querySelector('#ed-live-view .sode-gridpanel__' + k + '[data-' + k + '="' + n + '"]:not([hidden])');
		window.press = (k, n, mods = {}) => nameOf(k, n).dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, ...mods }));
		window.menu = () => document.getElementById('ed-live-menu');
		window.shown = () => !!menu() && !menu().hidden && menu().style.visibility !== 'hidden';
		window.labels = () => [...menu().querySelectorAll(':scope > li')].map((x) => x.classList.contains('pb-menu__divider') ? '|' : x.querySelector('.pb-menu__item > span').textContent).join();
		window.item = (label) => [...menu().querySelectorAll('.pb-menu__item')].find((x) => x.querySelector('span')?.textContent === label);
		window.toGrid = () => document.querySelector('#ed-live-view .sode-gridpanel__scroll').focus();
		window.picked = (k) => [...new Set([...document.querySelectorAll('#ed-live-view .sode-gridpanel__' + k + '[data-picked]:not([hidden])')].map((x) => x.dataset[k]))].sort((x, y) => x - y).join();
		document.getElementById('ed-live').click(); true`)
	b.waitFor("the live grid shows", `names('src') === '0,1,2,3,4,5'`)
	b.waitFor("no menu while nothing is picked", `!shown()`)

	b.eval(`press('src', 2); true`)
	b.waitFor("a picked source opens the menu above its name, with the items in order", `shown() && labels() === 'Artwork,Pin,Move,Insert,|,Remove from the panel'
		&& menu().getBoundingClientRect().bottom <= nameOf('src', 2).getBoundingClientRect().top + 1`)
	b.waitFor("Pin and Insert show their chords, Remove its key", `/P$/.test(item('Pin').textContent) && /I$/.test(item('Insert').textContent) && item('Remove from the panel').textContent.endsWith('Delete')`)
	b.shot("menu-source")
	b.eval(`press('src', 4, { shiftKey: true }); true`)
	b.waitFor("Shift-click picks a range, and the list selection follows", `picked('src') === '2,3,4' && document.getElementById('ed-sources-sel').textContent === '3 selected'`)
	b.eval(`item('Pin').click(); true`)
	b.waitFor("Pin pins every picked name, and the item turns to Unpin", `names('src').startsWith('2,3,4') && labels().startsWith('Artwork,Unpin')`)
	b.eval(`item('Unpin').click(); true`)
	b.waitFor("Unpin leaves them at the top of the rest", `names('src') === '2,3,4,0,1,5' && labels().startsWith('Artwork,Pin')`)
	b.eval(`[...item('Move').parentElement.querySelectorAll('.pb-menu__sub .pb-menu__item')].find((x) => x.textContent.startsWith('Down')).click(); true`)
	b.waitFor("Move Down moves the picked block, and the picks follow", `names('src') === '0,2,3,4,1,5' && picked('src') === '2,3,4'`)
	b.eval(`item('Artwork').click(); true`)
	b.waitFor("Artwork opens the Artwork panel for the picks", `!document.getElementById('ed-live-art').hidden && document.getElementById('ed-cell-kind').textContent === '3 sources selected'`)
	b.eval(`document.getElementById('ed-live-art-done').click(); true`)
	b.waitFor("Done closes it", `document.getElementById('ed-live-art').hidden`)

	b.eval(`press('dst', 3); true`)
	b.waitFor("a picked destination takes the menu to the right of its name", `shown() && Math.abs(menu().getBoundingClientRect().left - nameOf('dst', 3).getBoundingClientRect().right) < 20`)
	b.eval(`item('Remove from the panel').click(); true`)
	b.waitFor("Remove takes it out, and the pick goes to the name before", `names('dst') === '0,1,2,4,5,6' && picked('dst') === '2' && shown()`)

	// The keys: the cursor on the Source bar, then Space picks, Delete removes, Ctrl+I inserts, Enter and Esc.
	b.eval(`toGrid(); true`)
	realKey(b, "Home", "Home", 36)
	for range 2 {
		realKey(b, "l", "KeyL", 76)
	}
	realKey(b, " ", "Space", 32)
	b.waitFor("Space picks the source at the cursor", `picked('src') === '2' && shown()`)
	realKey(b, "Delete", "Delete", 46)
	b.waitFor("Delete removes it, and the pick goes to the name before", `names('src') === '0,3,4,1,5,6' && picked('src') === '0'`)
	realKeyMods(b, "i", "KeyI", 73, modCtrl)
	b.waitFor("Ctrl+I opens Insert with its Find box", `document.activeElement === document.getElementById('ed-sources-ins-find')`)
	b.waitFor("Insert lists the one source the panel does not have", `document.getElementById('ed-sources-ins-shown').textContent === '1'`)
	realKeyMods(b, "Enter", "Enter", 13, 0)
	b.waitFor("Enter inserts it after the pick, and it becomes the pick", `names('src') === '0,2,3,4,1,5' && picked('src') === '2'`)
	b.eval(`toGrid(); true`)
	realKeyMods(b, "Enter", "Enter", 13, 0)
	b.waitFor("Enter on the bar moves focus into the menu", `menu().contains(document.activeElement)`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc in the menu goes back to the grid, picks kept", `document.activeElement === document.querySelector('#ed-live-view .sode-gridpanel__scroll') && picked('src') === '2'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc on the grid drops the picks and the menu", `picked('src') === '' && !shown()`)

	// Follow the router: only Artwork and Pin.
	b.eval(`document.getElementById('ed-destinations-all').click(); true`)
	b.waitFor("the destinations follow the router", `document.getElementById('ed-destinations-all').checked && names('dst') === '0,1,2,3,4,5'`)
	b.eval(`press('dst', 4); true`)
	b.waitFor("with Follow the router on, the menu holds only Artwork and Pin", `shown() && labels() === 'Artwork,Pin'`)
	realKey(b, "Escape", "Escape", 27)

	// The phone layout has no Smart menu.
	b.eval(`document.documentElement.dataset.layout = 'phone'; press('src', 3); true`)
	b.eval(`new Promise((ok) => requestAnimationFrame(() => requestAnimationFrame(() => ok(true))))`)
	b.waitFor("no menu on the phone layout", `picked('src') === '3' && !shown()`)
	b.eval(`delete document.documentElement.dataset.layout; true`)

	b.eval(`document.getElementById('ed-live-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-live-save').hidden`)
	srcs := gridEdgeNums(t, readPanel(t, a, ana, id), "sources")
	members, _ := srcs["members"].([]any)
	if len(members) < 6 || members[0] != float64(0) || members[1] != float64(2) || members[2] != float64(3) || members[4] != float64(1) {
		t.Errorf("saved sources start %v, want 0, 2, 3, 4, 1", members[:min(6, len(members))])
	}
	b.check()
}
