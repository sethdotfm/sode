package app

import (
	"fmt"
	"strings"
	"testing"
)

// The Icon library end to end: Upload in the Choose icon picker's Uploaded section adds a PNG, its tile picks it for
// a cell, the save stores `lib:<id>`, and the panel page draws it as a picture with Icon B as its corner badge.
func TestChromeIconLibraryUploadPickAndDraw(t *testing.T) {
	b, a, _, id := editorTabPrep(t, `{"name":"Lib","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"source","n":1,"artB":"2"}]}`, nil)
	sq := `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]')`
	b.eval(`(() => { const s = ` + sq + `; const r = s.getBoundingClientRect(); s.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); })()`)
	b.waitFor("the Cell tab", `document.querySelector('#ed-side .pb-tab[aria-selected="true"]')?.textContent === 'Cell'`)
	b.eval(`[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Choose icon').click(); true`)
	b.waitFor("the Uploaded section, empty", `!!document.getElementById('ed-lib-upload')?.offsetParent && document.getElementById('ed-lib-icons').hidden`)

	// The name is typed; the file is a 1024 by 1024 PNG drawn on a canvas.
	b.eval(`document.getElementById('ed-lib-name').value = 'Station logo'; true`)
	b.eval(`(async () => { const c = document.createElement('canvas'); c.width = 1024; c.height = 1024; const x = c.getContext('2d'); x.fillStyle = '#c00'; x.fillRect(0, 0, 1024, 1024); const blob = await new Promise((r) => c.toBlob(r, 'image/png')); const dt = new DataTransfer(); dt.items.add(new File([blob], 'logo.png', { type: 'image/png' })); const f = document.querySelector('.sode-side__lib-sec input[type=file]'); f.files = dt.files; f.dispatchEvent(new Event('change')); return true; })()`)
	b.waitFor("the icon is added and its tile drawn", `(() => { const i = document.querySelector('#ed-lib-icons [data-lib] img'); return !!i && !i.hidden && i.complete && i.naturalWidth === 1024; })()`)
	icons := a.icons.list()
	if len(icons) != 1 || icons[0].Name != "Station logo" || icons[0].Type != "image/png" {
		t.Fatalf("library: %+v", icons)
	}
	lib := "lib:" + icons[0].ID

	// The upload picks it; picking None and then the tile shows the tile does it too.
	b.waitFor("the upload picked it for the cell", sq+`.querySelector('img.sode-panel__art-a') !== null`)
	b.eval(`[...document.querySelector('#panel-ed-cell .sode-side__col-top').querySelectorAll('.pb-btn')].find((x) => x.textContent === 'None').click(); true`)
	b.waitFor("None cleared it", sq+`.querySelector('img.sode-panel__art-a') === null`)
	b.eval(`document.querySelector('#ed-lib-icons [data-lib]').click(); true`)
	b.waitFor("the tile picks it, pressed, with Icon B as a badge", sq+`.querySelector('img.sode-panel__art-a') !== null && !!`+sq+`.querySelector('.sode-panel__art-b--badge') && document.querySelector('#ed-lib-icons [data-lib]').getAttribute('aria-pressed') === 'true'`)

	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("saved", `document.getElementById('ed-save').disabled`)
	if p, _ := a.panels.get(id); p.Cells[0]["art"] != lib {
		t.Fatalf("stored art %v, want %s", p.Cells[0]["art"], lib)
	}

	b.open(b.eval(`location.origin`).(string) + "/panels/" + id)
	b.waitFor("the panel or its PIN pad", `!!document.querySelector('.sode-panel__cell--source') || !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	if b.eval(`!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`) == true {
		for _, d := range "1234" {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	}
	cell := `document.querySelector('.sode-panel__cell--source')`
	b.waitFor("the panel page draws the library icon as a picture", `(() => { const i = `+cell+`?.querySelector('img.sode-panel__art-a'); return !!i && !i.hidden && i.complete && i.naturalWidth === 1024 && i.src.startsWith('blob:'); })()`)
	if got := b.eval(cell + `.querySelector('.sode-panel__art-b').className`).(string); !strings.Contains(got, "--badge") {
		t.Errorf("Icon B beside a library icon is %q, want the corner badge", got)
	}
	b.check()
}
