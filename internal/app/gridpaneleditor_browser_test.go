package app

import (
	"encoding/json"
	"regexp"
	"testing"
)

// A new panel made as a Grid panel in the editor. It checks the type switch, the hidden Buttons settings, and the
// lists beside the same tabs as a Buttons panel. A new Grid panel has no sources and no destinations. Follow the router is
// in the Panel tab; while it is on, only Pin and artwork change a list. A pin on the Sources list is saved, the type
// locks, and the panel page shows the pinned source first.
func TestChromeGridPanelEditor(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)

	b.open(srv.URL + "/panels/editor?new")
	signInOnPad(b)
	b.waitFor("the lock goes", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("the sidebar is built", `!!document.getElementById('ed-set-type') && !!document.getElementById('ed-templates')`)
	tabs := `[...document.querySelectorAll('#ed-side .pb-tab')].map((x) => x.textContent).join()`
	b.waitFor("a new panel is a Buttons panel", tabs+` === 'Cell,Panel,Access' && document.getElementById('ed-set-type-buttons').checked && !document.getElementById('ed-set-type-grid').disabled`)

	b.eval(`document.getElementById('ed-set-type-grid').click(); true`)
	b.waitFor("a Grid panel keeps the Cell, Panel and Access tabs, and starts with no sources and no destinations", tabs+` === 'Cell,Panel,Access' && !document.getElementById('ed-sources-empty').hidden && !document.getElementById('ed-destinations-empty').hidden && document.getElementById('ed-sources-acts').hidden && !document.querySelector('#ed-sources-list li')`)
	shown := func(id string) string { return `!!document.getElementById('` + id + `')?.offsetParent` }
	b.waitFor("the Buttons settings, the cells and Templates are hidden", `!`+shown("ed-rows")+` && !`+shown("ed-set-guard")+` && !`+shown("ed-set-multi")+` && !`+shown("ed-set-wrapPages")+` && !`+shown("ed-grid")+` && !`+shown("ed-pages")+` && !`+shown("ed-templates"))
	b.waitFor("the Grid keeps Mode, levels and Kiosk mode", shown("ed-set-mode")+` && `+shown("ed-set-kiosk")+` && `+shown("ed-set-wake"))

	b.waitFor("the Follow the router switches are in the Panel tab", `document.getElementById('panel-ed-panel').contains(document.getElementById('ed-sources-all')) && !document.getElementById('ed-sources-col').contains(document.getElementById('ed-sources-all'))`)
	b.eval(`document.getElementById('ed-sources-all').click(); document.getElementById('ed-destinations-all').click(); true`)
	src := `document.querySelector('#ed-sources-list li[data-id="5"]')`
	b.waitFor("the Sources list follows the router", src+` !== null && document.getElementById('ed-sources-state').textContent === '200' && document.getElementById('ed-sources-all').checked && document.getElementById('ed-sources-empty').hidden`)
	b.eval(src + `.querySelector('[data-act="pin"]').click(); true`)
	b.waitFor("the source is pinned and goes to the top", src+`.querySelector('[data-act="pin"]').getAttribute('aria-pressed') === 'true' && document.getElementById('ed-sources-pins').textContent === '1 pinned' && document.querySelector('#ed-sources-list li').dataset.id === '5'`)
	b.waitFor("while following, Move, Remove, Clear and Add from the router are off, and the list says why", `(() => {
		const row = document.querySelector('#ed-sources-list li[data-id="0"]');
		const off = ['up', 'down', 'delete'].every((a) => row.querySelector('[data-act="' + a + '"]').disabled);
		return off && !row.querySelector('[data-act="pin"]').disabled && document.getElementById('ed-sources-clear').disabled && document.getElementById('ed-sources-find').disabled
			&& !document.getElementById('ed-sources-following').hidden && /Panel tab/.test(document.getElementById('ed-sources-following').textContent);
	})()`)
	b.shot("sources")

	b.eval(`document.getElementById('ed-name').value = 'Edited grid'; document.getElementById('ed-name').dispatchEvent(new Event('input', { bubbles: true })); document.getElementById('ed-save').click(); true`)
	b.waitFor("saved, and the type is locked", `document.getElementById('ed-state').textContent === 'Saved' && document.getElementById('ed-set-type-grid').disabled && location.search.startsWith('?panel=')`)
	id := regexp.MustCompile(`panel=([\w-]+)`).FindStringSubmatch(b.eval(`location.search`).(string))[1]
	p := readPanel(t, a, ana, id)
	if p["type"] != "grid" {
		t.Fatalf("type %v", p["type"])
	}
	srcs, _ := p["sources"].(map[string]any)
	hide, _ := json.Marshal(srcs["hide"])
	items, _ := json.Marshal(srcs["items"])
	if srcs["members"] != "all" || string(hide) != `null` || string(items) != `{"5":{"pinned":true}}` {
		t.Errorf("sources %v", srcs)
	}

	// Opened again from a fresh page: no unsaved changes, the type locked, and a second save is a PUT that keeps both.
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("the saved Grid panel opens in the editor", tabs+` === 'Cell,Panel,Access' && document.getElementById('ed-state').textContent === 'Saved' && document.getElementById('ed-set-type-grid').checked && document.getElementById('ed-set-type-grid').disabled`)
	b.waitFor("the pin loads", src+`?.querySelector('[data-act="pin"]').getAttribute('aria-pressed') === 'true'`)
	b.eval(`document.querySelector('#ed-sources-list li[data-id="7"] [data-act="pin"]').click(); true`)
	b.waitFor("a second pin is an unsaved change", `document.getElementById('ed-state').textContent === 'Unsaved changes'`)
	b.eval(`document.getElementById('ed-save').click(); true`)
	b.waitFor("saved again", `document.getElementById('ed-state').textContent === 'Saved'`)
	srcs, _ = readPanel(t, a, ana, id)["sources"].(map[string]any)
	hide, _ = json.Marshal(srcs["hide"])
	items, _ = json.Marshal(srcs["items"])
	if srcs["members"] != "all" || string(hide) != `null` || string(items) != `{"5":{"pinned":true},"7":{"pinned":true}}` {
		t.Errorf("after the PUT, sources %v", srcs)
	}

	b.eval(`document.getElementById('ed-live').click(); true`)
	b.waitFor("The Live Editor runs the Grid panel as its own page does: the header band, the Levels box and Take, with the editor's bars hidden", `!!document.querySelector('#ed-live-view .sode-gridpanel__xp') && !!document.querySelector('#ed-live-view .sode-gridpanel__deck') && !document.getElementById('panel-levels').hidden && !!document.getElementById('panel-take') && !!document.getElementById('confirm') && !document.querySelector('.sode-ed__bar').offsetParent`)
	b.waitFor("a bar over it says Preview and holds Back to editing and Edit lists, with no Save while nothing is unsaved", `(() => {
		const bar = document.getElementById('ed-live-bar');
		return !!bar?.offsetParent && /Preview/.test(bar.textContent) && bar.contains(document.getElementById('ed-live-back')) && bar.lastElementChild === document.getElementById('ed-live-lists') && document.getElementById('ed-live-lists').previousElementSibling === document.getElementById('ed-live-back')
			&& !document.querySelector('.sode-gridpanel__deck #ed-live-back') && !!document.getElementById('ed-live-lists').offsetParent && document.getElementById('ed-live-save').hidden;
	})()`)
	b.shot("live")
	b.eval(`document.getElementById('ed-live-back').click(); true`)
	b.waitFor("Back to editing puts the editor back, with both lists in their place", `!!document.querySelector('.sode-ed__bar').offsetParent && !document.getElementById('confirm') && !document.getElementById('panel-levels')
		&& [...document.getElementById('ed-lists').children].map((x) => x.id).join() === 'ed-sources-col,ed-destinations-col' && !!document.getElementById('ed-sources-list').offsetParent`)

	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the panel opens as a Grid panel", `!!document.querySelector('.sode-gridpanel__xp')`)
	b.waitFor("the pinned sources lead, in list order", `(() => {
		const x = (n) => document.querySelector('.sode-gridpanel__src[data-src="' + n + '"]')?.getBoundingClientRect().left;
		return x(5) < x(7) && x(7) < x(0) && x(0) < x(1);
	})()`)
	b.check()
}
