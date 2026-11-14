package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Fixed destination from the editor to the router: the Panel tab's picker sets the list, which sets aside the
// Destination cell, greys Destination in Type and turns off Stage several Destinations; clearing the list brings
// them back. Saved, the panel page patches a Source press and a Preset press to the fixed destinations only.
func TestChromeEditorFixedDests(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	// The simulator starts with d routed from d, so the Preset's route to elsewhere is seen not to happen.
	const other, elsewhere, presetSrc = 5, 8, 3
	a := simApp(t, sim, func(c *config.Config) {
		c.Presets = append(c.Presets, config.Preset{Name: "Pair", Items: []config.PresetItem{{Src: presetSrc, Dsts: []int{simDst, elsewhere}}, {Src: presetSrc, Dsts: []int{other}}}})
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Wall","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"source","n":%d},
		{"r":1,"c":3,"kind":"preset","scope":"everyone","name":"Pair"},
		{"r":1,"c":4,"kind":"take"}],"settings":{"mode":"take"}}`, elsewhere, simSrc))
	var names struct {
		Destinations []struct {
			N    int    `json:"n"`
			Name string `json:"name"`
		} `json:"destinations"`
	}
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"routes":[{`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}
	if err := json.Unmarshal(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.Bytes(), &names); err != nil {
		t.Fatal(err)
	}
	nameOf := map[int]string{}
	for _, d := range names.Destinations {
		nameOf[d.N] = d.Name
	}
	if nameOf[other] == "" {
		t.Fatalf("destination %d has no router name: %v", other, names.Destinations)
	}

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Wall'`)
	b.eval(`document.getElementById('tab-ed-panel').click()`)

	b.eval(`window.pick = (text) => { const i = document.getElementById('ed-fixed-find'); i.focus(); i.value = text; i.dispatchEvent(new Event('input', { bubbles: true })); };
		window.row = () => document.querySelector('#ed-fixed-results .sode-fixed__row:not([aria-disabled="true"])');
		window.chosen = () => [...document.querySelectorAll('#ed-fixed-list > li')].map((li) => li.dataset.n).join();
		window.dstSquare = () => document.querySelector('#ed-grid .sode-ed__sq--destination');
		window.multi = () => document.getElementById('ed-set-multi');
		true`)
	b.waitFor("the Destination cell is drawn while the list is empty", `!!dstSquare() && !multi().disabled`)
	b.eval(`pick('DSM1'); true`)
	b.waitFor("the picker lists DSM1", `row()?.textContent.includes('DSM1')`)
	b.eval(`row().click(); true`)
	b.waitFor("DSM1 chosen, by name", `chosen() === '`+fmt.Sprint(simDst)+`' && document.querySelector('#ed-fixed-list > li').textContent.includes('DSM1')`)
	b.waitFor("the Destination cell is set aside and Stage several Destinations is off with a note", `!dstSquare() && multi().disabled && document.getElementById('ed-multi-note').textContent !== ''`)
	b.waitFor("unsaved", `document.getElementById('ed-state').textContent === 'Unsaved changes'`)
	b.eval(`const sq = document.querySelector('.sode-ed__sq[data-r="2"][data-c="1"]'); const r = sq.getBoundingClientRect();
		sq.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); true`)
	b.waitFor("Type offers Destination greyed", `[...document.querySelectorAll('.sode-ed__menu .pb-menu__sub .pb-menu__item')].some((x) => x.querySelector('span')?.textContent === 'Destination' && x.disabled)`)
	b.eval(`document.getElementById('ed-grid').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true`)

	b.eval(`document.querySelector('#ed-fixed-list button').click(); true`)
	b.waitFor("an empty list brings the Destination cell and Stage several Destinations back", `chosen() === '' && !!dstSquare() && !multi().disabled`)
	b.waitSaved("and nothing is unsaved")

	for _, text := range []string{"DSM1", nameOf[other]} {
		b.eval(fmt.Sprintf(`pick(%q); true`, text))
		b.waitFor("the picker lists "+text, fmt.Sprintf(`row()?.textContent.includes(%q)`, text))
		b.eval(`row().click(); true`)
	}
	b.waitFor("both chosen in order", `chosen() === '`+fmt.Sprintf("%d,%d", simDst, other)+`'`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitSaved("saved")
	var stored struct {
		Pages []struct {
			Cells []struct {
				Kind string `json:"kind"`
				N    *int   `json:"n"`
			} `json:"cells"`
		} `json:"pages"`
		Settings struct {
			FixedDests []int `json:"fixedDests"`
		} `json:"settings"`
	}
	raw := apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Body.Bytes()
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(stored.Settings.FixedDests) != fmt.Sprint([]int{simDst, other}) {
		t.Errorf("fixedDests not stored in order: %s", raw)
	}
	kept := false
	for _, c := range stored.Pages[0].Cells {
		kept = kept || (c.Kind == "destination" && c.N != nil && *c.N == elsewhere)
	}
	if !kept || len(stored.Pages[0].Cells) != 4 {
		t.Errorf("the set-aside Destination cell was not kept: %s", raw)
	}

	b.eval(`document.getElementById('ed-live').click()`)
	b.waitFor("Test it live draws the panel without Destination cells", `!!document.querySelector('#ed-live-view .sode-panel__cell--source') && !document.querySelector('#ed-live-view .sode-panel__cell--destination')`)
	b.eval(`document.getElementById('ed-live').click()`)

	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the panel or its PIN pad", `!!document.querySelector('.sode-panel__cell--source') || !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	if b.eval(`!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`) == "true" {
		for _, d := range "1234" {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	}
	b.waitFor("the panel is drawn", `!!document.querySelector('.sode-panel__cell--source')`)
	take := `document.querySelector('.sode-panel__take')`

	b.eval(`document.querySelector('.sode-panel__cell--source').click()`)
	b.waitFor("Take armed for the Source", `!`+take+`.disabled`)
	b.eval(take + `.click()`)
	for _, d := range []int{simDst, other} {
		waitRouted(t, a, ana, d, simSrc)
	}

	b.eval(`document.querySelector('.sode-panel__cell--preset').click()`)
	b.waitFor("Take armed for the Preset", `!`+take+`.disabled`)
	b.eval(take + `.click()`)
	for _, d := range []int{simDst, other} {
		waitRouted(t, a, ana, d, presetSrc)
	}
	waitRouted(t, a, ana, elsewhere, elsewhere)
	b.check()
}

// A Destination cell set aside by Fixed destination is dropped when the grid shrinks past it or a merge covers its
// square, and the status note counts it, so a save never loses a cell without a word.
func TestChromeEditorFixedDestsSaysWhatIsDropped(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Wall","rows":2,"cols":3,"cells":[
		{"r":1,"c":1,"kind":"destination","n":8},
		{"r":1,"c":2,"kind":"take"},
		{"r":2,"c":3,"kind":"destination","n":5}],"settings":{"fixedDests":[%d]}}`, simDst))

	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded with its Destination cells set aside", `document.getElementById('ed-name').value === 'Wall' && !document.querySelector('#ed-grid .sode-ed__sq--destination')`)
	status := `document.getElementById('ed-status').textContent`

	b.eval(`document.getElementById('tab-ed-panel').click(); const c = document.getElementById('ed-cols'); c.value = '2'; c.dispatchEvent(new Event('change', { bubbles: true })); true`)
	b.waitFor("shrinking past a set-aside cell counts it", status+` === 'Removed 1 cell that no longer fit.'`)

	b.eval(`const sq = document.querySelector('.sode-ed__sq[data-r="1"][data-c="2"]'); const r = sq.getBoundingClientRect();
		sq.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 }));
		const grid = document.getElementById('ed-grid');
		grid.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowLeft', shiftKey: true, bubbles: true }));
		grid.dispatchEvent(new KeyboardEvent('keydown', { key: 'm', code: 'KeyM', ctrlKey: true, bubbles: true })); true`)
	b.waitFor("a merge over a set-aside cell says it was removed", `/^Removed 1 Destination cell /.test(`+status+`)`)

	b.eval(`document.getElementById('ed-save').click()`)
	b.waitSaved("saved")
	var stored struct {
		Cols  int `json:"cols"`
		Pages []struct {
			Cells []struct {
				Kind string `json:"kind"`
				Cs   int    `json:"cs"`
			} `json:"cells"`
		} `json:"pages"`
	}
	raw := apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Body.Bytes()
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	if cells := stored.Pages[0].Cells; stored.Cols != 2 || len(cells) != 1 || cells[0].Kind != "take" || cells[0].Cs != 2 {
		t.Errorf("stored %s", raw)
	}

	// Undo brings back the set-aside cells the merge and the resize dropped, each once, still set aside.
	b.eval(`const g = document.getElementById('ed-grid'); g.focus(); g.dispatchEvent(new KeyboardEvent('keydown', { key: 'z', code: 'KeyZ', metaKey: true, bubbles: true, cancelable: true })); true`)
	b.waitFor("merge undone", status+` === 'Undid merge.'`)
	b.eval(`document.getElementById('ed-grid').dispatchEvent(new KeyboardEvent('keydown', { key: 'z', code: 'KeyZ', ctrlKey: true, bubbles: true, cancelable: true })); true`)
	b.waitFor("resize undone, Destination cells still set aside", status+` === 'Undid resize.' && document.getElementById('ed-cols').value === '3' && !document.querySelector('#ed-grid .sode-ed__sq--destination')`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitSaved("saved again")
	got := storedCells(t, a, ana, id)
	want := []string{"1,1 1x1 destination art= artB=", "1,2 1x1 take art= artB=", "2,3 1x1 destination art= artB="}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("after undo:\n got %q\nwant %q", got, want)
	}
	b.check()
}
