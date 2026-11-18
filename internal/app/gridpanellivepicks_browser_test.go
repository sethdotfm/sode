package app

import (
	"regexp"
	"testing"
)

// liveGridPanel makes and saves a Grid panel that holds every router item, and answers its id.
func liveGridPanel(t *testing.T, b *tab, srvURL string) string {
	t.Helper()
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	b.open(srvURL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	asGrid(b, true)
	b.eval(`document.getElementById('ed-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved' && location.search.startsWith('?panel=')`)
	return regexp.MustCompile(`panel=([\w-]+)`).FindStringSubmatch(b.eval(`location.search`).(string))[1]
}

// The Live Editor: names picked on the grid (click, Shift-click, Cmd-click) are the list's selection, and the drawer's
// Move, Pin and Artwork act on them. The Preview bar keeps its copy of those buttons hidden. The kind picked last owns
// the selection. The panel page gets none of this.
func TestChromeGridPanelLivePicks(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	id := liveGridPanel(t, b, srv.URL)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 800, "deviceScaleFactor": 1, "mobile": false})

	b.eval(`
		window.gridTop = () => Math.round(document.querySelector('#ed-live-view .sode-gridpanel__scroll').getBoundingClientRect().top);
		window.first = (k, side) => [...document.querySelectorAll('#ed-live-view .sode-gridpanel__' + k + '[data-' + k + ']')].map((x) => [x.dataset[k], x.getBoundingClientRect()]).filter(([, r]) => r.width > 0 && r.height > 0).sort((x, y) => x[1][side] - y[1][side]).map(([n]) => n);
		window.nameOf = (k, n) => document.querySelector('#ed-live-view .sode-gridpanel__' + k + '[data-' + k + '="' + n + '"]:not([hidden])');
		window.press = (n, mods = {}) => n.dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true, ...mods }));
		document.getElementById('ed-live').click(); true`)
	b.waitFor("the live grid shows the saved order", `first('src', 'left')[0] === '0' && first('dst', 'top')[0] === '0'`)
	b.waitFor("Back to editing is left of Edit lists, which is last and not pressed", `(() => {
		const bar = document.getElementById('ed-live-bar'), back = document.getElementById('ed-live-back'), open = document.getElementById('ed-live-lists');
		return bar.lastElementChild === open && open.previousElementSibling === back && open.getAttribute('aria-pressed') === 'false' && back.getBoundingClientRect().right <= open.getBoundingClientRect().left;
	})()`)
	b.eval(`document.getElementById('ed-live-lists').click(); true`)
	b.waitFor("Edit lists shows it is on while the drawer is open", `document.getElementById('ed-live-lists').getAttribute('aria-pressed') === 'true' && document.getElementById('ed-live-drawer').classList.contains('pb-drawer--open')`)
	b.eval(`document.getElementById('ed-live-lists').click(); true`)
	b.waitFor("a second press closes it", `document.getElementById('ed-live-lists').getAttribute('aria-pressed') === 'false' && !document.getElementById('ed-live-drawer').classList.contains('pb-drawer--open')`)

	b.waitFor("the bar says Panel Live Editor, and the run has no Settings link and no take note", `document.getElementById('ed-live-title').textContent === 'Panel Live Editor' && document.getElementById('ed-live-bar').getAttribute('aria-labelledby') === 'ed-live-title'
		&& getComputedStyle(document.querySelector('#ed-live-view .sode-gridpanel__settings')).display === 'none' && !document.querySelector('.sode-live__what')`)
	b.waitFor("the panel title is at the left of the band, not cleared for the Lock bar", `(() => {
		const t = document.querySelector('#ed-live-view .sode-panels__title').getBoundingClientRect(), d = document.querySelector('#ed-live-view .sode-gridpanel__deck').getBoundingClientRect();
		return t.width > 0 && t.left - d.left < 40;
	})()`)
	b.eval(`window.topWas = gridTop(); press(nameOf('src', 3)); press(nameOf('src', 5), { shiftKey: true }); true`)
	b.waitFor("a click and a Shift-click on the grid select those sources in the list; the Preview bar keeps its edit buttons hidden", `document.getElementById('ed-sources-sel').textContent === '3 selected' && getComputedStyle(document.getElementById('ed-live-picked')).display === 'none' && !document.getElementById('ed-sources-sel-top').offsetParent`)
	b.shot("picked")
	b.waitFor("the grid does not move", `gridTop() === topWas`)
	b.eval(`document.getElementById('ed-live-lists').click(); true`)
	b.waitFor("the drawer holds the list's own bar with the picks", `document.getElementById('ed-sources-col').contains(document.getElementById('ed-sources-acts')) && !!document.getElementById('ed-sources-sel-top').offsetParent`)
	b.eval(`document.getElementById('ed-sources-sel-top').click(); true`)
	b.waitFor("Move to the top shows on the live grid, the picks stay, and Save shows", `first('src', 'left').slice(0, 3).join() === '3,4,5' && document.getElementById('ed-sources-sel').textContent === '3 selected' && !document.getElementById('ed-live-save').hidden`)
	b.eval(`press(nameOf('src', 4), { metaKey: true }); true`)
	b.waitFor("Cmd-click takes one out", `document.getElementById('ed-sources-sel').textContent === '2 selected'`)
	b.eval(`document.getElementById('ed-sources-sel-pin').click(); true`)
	b.waitFor("Pin pins them", `document.querySelector('#ed-sources-list li[data-id="3"] [data-act="pin"]')?.getAttribute('aria-pressed') === 'true' && document.querySelector('#ed-sources-list li[data-id="5"] [data-act="pin"]')?.getAttribute('aria-pressed') === 'true'`)

	b.eval(`press(nameOf('dst', 2), { metaKey: true }); true`)
	b.waitFor("a destination picked after them takes the selection", `document.getElementById('ed-destinations-sel').textContent === '1 selected' && document.getElementById('ed-sources-sel').textContent === '0 selected'`)
	b.eval(`{ const x = document.getElementById('ed-destinations-sel-art'); x.focus(); x.click(); } true`)
	b.waitFor("Artwork opens the Cell tab's artwork over the drawer for that destination", `(() => {
		const p = document.getElementById('ed-live-art');
		return !p.hidden && document.getElementById('ed-live-drawer').classList.contains('pb-drawer--open') && p.contains(document.getElementById('ed-cell-kind'))
			&& document.getElementById('ed-cell-kind').textContent.startsWith('Destination 3');
	})()`)
	b.eval(`document.querySelector('#ed-live-art .sode-side__icons[aria-label="Icon A"] [aria-label="camera"]').click(); true`)
	b.waitFor("the live grid shows the camera on that destination", `!!nameOf('dst', 2)?.querySelector('.sode-gridpanel__art:not([hidden]) svg.sode-gridpanel__art-a:not([hidden])')`)
	b.eval(`document.getElementById('ed-live-art-done').click(); true`)
	b.waitFor("Done closes the artwork and puts the Cell tab back", `document.getElementById('ed-live-art').hidden && document.getElementById('panel-ed-cell').contains(document.getElementById('ed-cell-kind'))`)

	b.eval(`document.getElementById('ed-live-lists').click(); document.getElementById('panel-clear').click(); true`)
	b.waitFor("Clear drops the picks", `!document.getElementById('ed-live-picked').children.length && document.getElementById('ed-destinations-sel').textContent === '0 selected'`)

	b.eval(`document.getElementById('ed-live-save').click(); true`)
	b.waitFor("saved", `document.getElementById('ed-live-save').hidden`)
	p := readPanel(t, a, ana, id)
	srcs, _ := p["sources"].(map[string]any)
	members, _ := srcs["members"].([]any)
	items, _ := srcs["items"].(map[string]any)
	if len(members) < 3 || members[0] != float64(3) || members[1] != float64(5) || members[2] != float64(4) {
		t.Errorf("saved sources %v, want 3 and 5 (pinned), then 4", members)
	}
	for _, n := range []string{"3", "5"} {
		if it, _ := items[n].(map[string]any); it["pinned"] != true {
			t.Errorf("source %s saved %v, want pinned", n, it)
		}
	}
	dsts, _ := p["destinations"].(map[string]any)
	ditems, _ := dsts["items"].(map[string]any)
	if it, _ := ditems["2"].(map[string]any); it["art"] != "icon:camera" {
		t.Errorf("destination 2 saved %v, want the camera", it)
	}
	b.eval(`document.getElementById('ed-live-back').click(); true`)
	b.waitFor("Back to editing puts both bars back in their lists", `document.getElementById('ed-sources-col').contains(document.getElementById('ed-sources-acts')) && document.getElementById('ed-destinations-col').contains(document.getElementById('ed-destinations-acts'))`)

	rev := p["rev"]
	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the panel page shows", `!!document.querySelector('.sode-gridpanel__src[data-src="3"]:not([hidden])')`)
	b.eval(`document.querySelector('.sode-gridpanel__src[data-src="3"]:not([hidden])').dispatchEvent(new MouseEvent('click', { bubbles: true, cancelable: true })); true`)
	b.waitFor("a click on the panel page picks the name, with no edit actions", `!!document.querySelector('.sode-gridpanel__src[data-src="3"][data-picked]')
		&& !document.getElementById('ed-live-picked') && !document.querySelector('[id$="-sel-pin"], [id$="-sel-art"], .sode-edge__acts')`)
	if got := readPanel(t, a, ana, id)["rev"]; got != rev {
		t.Errorf("a pick on the panel page changed the panel: rev %v, was %v", got, rev)
	}
	b.check()
}
