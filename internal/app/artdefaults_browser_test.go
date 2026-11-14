package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Default artwork end to end: the editor's Cell tab sets a cell's artwork as its Source's default and another cell
// of that Source draws it; a default set under another router name warns; on the panel page a changed and then a
// cleared default arrive through the update banner with no panel save.
func TestChromeArtDefaultEditorAndLiveUpdate(t *testing.T) {
	const moved = 11
	b, a, ana, id := editorTabPrep(t, fmt.Sprintf(`{"name":"Wall","rows":2,"cols":4,"cells":[
		{"r":1,"c":1,"kind":"source","n":%d,"art":"icon:camera"},
		{"r":1,"c":2,"kind":"source","n":%d},
		{"r":1,"c":3,"kind":"source","n":%d}]}`, simSrc, simSrc, moved), func(_ *tab, a *App, key, _ string) {
		for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", key, nil).Body.String(), "Zoom 1"); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("the app never read names from swp08sim")
			}
		}
		n := moved
		if _, err := a.artDefaults.set(ArtDefault{Kind: artKindSource, N: &n, Name: "OLD NAME", Art: "icon:mic"}, nil); err != nil {
			t.Fatal(err)
		}
	})
	sq := func(c int) string {
		return fmt.Sprintf(`document.querySelector('.sode-ed__sq[data-r="1"][data-c="%d"]')`, c)
	}
	b.eval(`window.sel = (s) => { const r = s.getBoundingClientRect(); s.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); }; true`)
	b.waitFor("grid drawn", sq(3)+` !== null`)
	b.waitFor("the moved source draws its default", sq(3)+`.querySelector('.sode-panel__art use')?.getAttribute('href') === '#pb-i-mic'`)
	b.eval(`sel(` + sq(3) + `); true`)
	b.waitFor("the Cell tab warns the default may be for another item", `(() => { const w = document.getElementById('ed-default-warn'); return !!w.offsetParent && w.textContent.includes('OLD NAME'); })()`)

	b.eval(`sel(` + sq(1) + `); true`)
	b.waitFor("Set as default is offered", `!document.getElementById('ed-default-set').disabled && document.getElementById('ed-default-warn').hidden`)
	b.eval(`document.getElementById('ed-default-set').click(); true`)
	b.waitFor("the other cell of the source draws the default", sq(2)+`.querySelector('.sode-panel__art use')?.getAttribute('href') === '#pb-i-camera'`)
	if p, _ := a.panels.get(id); p.Pages[0].Cells[1]["art"] != nil {
		t.Fatalf("the default was stored in the cell: %v", p.Pages[0].Cells[1])
	}

	b.open(b.eval(`location.origin`).(string) + "/panels/" + id)
	b.waitFor("the panel or its PIN pad", `!!document.querySelector('.sode-panel__cell--source') || !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	if b.eval(`!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`) == true {
		for _, d := range "1234" {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	}
	cell := `document.querySelectorAll('.sode-panel__cell--source')[1]`
	// The page repaints its cells when its first poll brings the router's names; read after that, not across it.
	b.waitFor("the page's first poll answered", cell+`?.querySelector('.sode-panel__text')?.textContent === 'Zoom 1'`)
	b.waitFor("the page draws the default", cell+`?.querySelector('.sode-panel__art use')?.getAttribute('href') === '#pb-i-camera'`)

	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", ana, fmt.Sprintf(`{"kind":"source","n":%d,"art":"icon:tv"}`, simSrc)); rec.Code != 200 {
		t.Fatalf("change: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("update banner for a changed default", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click(); true`)
	b.waitFor("the changed default is drawn", cell+`?.querySelector('.sode-panel__art use')?.getAttribute('href') === '#pb-i-tv'`)

	if rec := apiSend(a, "DELETE", "/api/v1/default-artwork", ana, fmt.Sprintf(`{"kind":"source","n":%d}`, simSrc)); rec.Code != 204 {
		t.Fatalf("clear: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("update banner for a cleared default", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click(); true`)
	b.waitFor("the cleared default is gone and the cell's own artwork stays", cell+` && !`+cell+`.querySelector('.sode-panel__art') && !!document.querySelector('.sode-panel__cell--source .sode-panel__art')`)
	b.check()
}
