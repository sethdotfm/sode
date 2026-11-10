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

// A Trusted user renames and deletes from a tile under Manage. Delete asks the churning-letter Confirm with the use
// count; after it the cells that used the icon draw no Icon A, on the editor canvas and on the panel page, and keep
// their Icon B and text.
func TestChromeIconLibraryRenameAndDelete(t *testing.T) {
	var ic LibIcon
	b, a, _, id := editorTabPrep(t, `{"name":"Lib","rows":2,"cols":2}`, func(b *tab, a *App, ana, id string) {
		var err error
		if ic, err = a.icons.add("Logo", imgBytes("png", 16, 16), "image/png"); err != nil {
			t.Fatal(err)
		}
		lib := "lib:" + ic.ID
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Lib","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"source","n":1,"art":"`+lib+`","artB":"2"},{"r":1,"c":2,"kind":"source","n":2,"art":"`+lib+`"}]}`); rec.Code != 200 {
			t.Fatalf("art cells: %d %s", rec.Code, rec.Body.String())
		}
	})
	sq := `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]')`
	drawn := func(at string) string {
		return `(() => { const i = ` + at + `?.querySelector('.sode-panel__art-a'); return !!i && !i.hidden && i.complete && i.naturalWidth > 0; })()`
	}
	b.waitFor("the canvas draws the library icon", drawn(sq))
	b.eval(`(() => { const s = ` + sq + `; const r = s.getBoundingClientRect(); s.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); })()`)
	b.waitFor("the Cell tab", `document.querySelector('#ed-side .pb-tab[aria-selected="true"]')?.textContent === 'Cell'`)
	b.eval(`[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Choose icon').click(); true`)
	b.waitFor("Manage, for a Trusted user", `!!document.getElementById('ed-lib-manage')?.offsetParent && !!document.querySelector('#ed-lib-icons [data-lib]')`)

	b.eval(`document.getElementById('ed-lib-manage').click(); document.querySelector('#ed-lib-icons [data-lib]').click(); true`)
	b.waitFor("the tile is chosen, not picked", `!!document.getElementById('ed-lib-delete')?.offsetParent && document.getElementById('ed-lib-rename-name').value === 'Logo'`)
	b.eval(`document.getElementById('ed-lib-rename-name').value = 'Station'; document.getElementById('ed-lib-rename').click(); true`)
	b.waitFor("renamed", `document.querySelector('#ed-lib-icons [data-lib]')?.getAttribute('aria-label') === 'Station'`)
	if got := a.icons.list(); got[0].Name != "Station" || got[0].ID != ic.ID {
		t.Fatalf("after rename %+v", got)
	}
	b.waitFor("the renamed icon still draws", drawn(sq))

	b.eval(`document.querySelector('#ed-lib-icons [data-lib]').click(); document.getElementById('ed-lib-delete').click(); true`)
	b.waitFor("the Confirm with the count", `/2 cells on 1 panel use it/.test(document.getElementById('ed-lib-confirm')?.textContent || '')`)
	if b.eval(`document.getElementById('ed-lib-delete-ok').disabled`) != true {
		t.Error("the Confirm is not locked out at first")
	}
	b.waitFor("the lockout ends", `!document.getElementById('ed-lib-delete-ok').disabled`)
	b.eval(`(() => { const k = /\((.)\)/.exec(document.getElementById('ed-lib-delete-ok').textContent)[1].toLowerCase(); document.getElementById('ed-lib-confirm').dispatchEvent(new KeyboardEvent('keydown', { key: k, bubbles: true })); return true; })()`)
	b.waitFor("deleted", `/Deleted Station/.test(document.querySelector('.sode-side__lib-sec [role=status]').textContent)`)
	if n := len(a.icons.list()); n != 0 {
		t.Fatalf("library still has %d", n)
	}
	b.waitFor("the canvas draws no Icon A, Icon B and the text stay", `(() => { const s = `+sq+`; const i = s.querySelector('.sode-panel__art-a'); return (!i || i.hidden) && s.querySelector('.sode-panel__art-b')?.textContent === '2' && s.querySelector('.sode-ed__text').textContent !== ''; })()`)
	b.waitFor("the sidebar preview draws none either", `(() => { const i = document.querySelector('#panel-ed-cell .sode-panel__art-a'); return !i || i.hidden; })()`)

	b.open(b.eval(`location.origin`).(string) + "/panels/" + id)
	b.waitFor("the panel or its PIN pad", `!!document.querySelector('.sode-panel__cell--source') || !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	if b.eval(`!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`) == true {
		for _, d := range "1234" {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	}
	cell := `document.querySelector('.sode-panel__cell--source')`
	b.waitFor("the panel page draws Icon B", cell+`?.querySelector('.sode-panel__art-b')?.textContent === '2'`)
	if b.eval(drawn(cell)) == true {
		t.Error("the panel page still draws the deleted icon")
	}
	// The deleted icon's 404 is how a cell learns to draw no Icon A; anything else is a real error.
	for _, e := range b.problems() {
		if !strings.Contains(e, "404") || !strings.Contains(e, "/icon-library/"+ic.ID) {
			t.Errorf("page error: %s", e)
		}
	}
}

// A library SVG draws as a mask in the art row's text colour, so a red and blue file shows one colour that follows the
// theme. It is never an <img> with its own colours, and the editor's canvas and picker tile draw it the same way.
func TestChromeIconLibrarySVGTakesTheTextColour(t *testing.T) {
	b, _, _, id := editorTabPrep(t, `{"name":"Lib","rows":2,"cols":2}`, func(b *tab, a *App, ana, id string) {
		svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><rect width="5" height="10" fill="#f00"/><rect x="5" width="5" height="10" fill="#00f"/></svg>`
		ic, err := a.icons.add("Two colours", []byte(svg), "image/svg+xml")
		if err != nil {
			t.Fatal(err)
		}
		if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Lib","rows":2,"cols":2,"cells":[{"r":1,"c":1,"kind":"source","n":1,"art":"lib:`+ic.ID+`","artB":"2"}]}`); rec.Code != 200 {
			t.Fatalf("art cell: %d %s", rec.Code, rec.Body.String())
		}
	})
	// The colour when Icon A is a mask over the blob URL in its row's text colour, else ''.
	tinted := func(at string) string {
		return `(() => { const a = ` + at + `?.querySelector('.sode-panel__art-a'); if (!a || a.hidden || a.tagName === 'IMG' || !a.offsetWidth || !a.offsetHeight) return ''; const s = getComputedStyle(a); return s.maskImage.startsWith('url("blob:') && s.backgroundColor === getComputedStyle(a.parentElement).color ? s.backgroundColor : ''; })()`
	}
	sq := `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]')`
	b.waitFor("the editor canvas draws the SVG as a tinted mask", tinted(sq)+` !== ''`)
	b.eval(`(() => { const s = ` + sq + `; const r = s.getBoundingClientRect(); s.dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 })); })()`)
	b.waitFor("the Cell tab", `document.querySelector('#ed-side .pb-tab[aria-selected="true"]')?.textContent === 'Cell'`)
	b.eval(`[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Choose icon').click(); true`)
	b.waitFor("the picker tile is a mask too", `(() => { const m = document.querySelector('#ed-lib-icons [data-lib] .sode-side__lib-img'); return !!m && !m.hidden && m.tagName !== 'IMG' && m.offsetWidth > 0 && getComputedStyle(m).maskImage.startsWith('url("blob:'); })()`)

	b.open(b.eval(`location.origin`).(string) + "/panels/" + id)
	b.waitFor("the panel or its PIN pad", `!!document.querySelector('.sode-panel__cell--source') || !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	if b.eval(`!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`) == true {
		for _, d := range "1234" {
			b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
		}
		b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	}
	cell := `document.querySelector('.sode-panel__cell--source')`
	b.eval(`document.documentElement.dataset.theme = 'light'; true`)
	b.waitFor("the panel page draws the SVG as a tinted mask", tinted(cell)+` !== ''`)
	light := b.eval(tinted(cell)).(string)
	if got := b.eval(cell + `.querySelector('.sode-panel__art-b').className`).(string); !strings.Contains(got, "--badge") {
		t.Errorf("Icon B beside a library SVG is %q, want the corner badge", got)
	}
	b.eval(`document.documentElement.dataset.theme = 'dark'; true`)
	b.waitFor("the colour follows the theme", fmt.Sprintf(`((c) => c !== '' && c !== %q)(%s)`, light, tinted(cell)))
	b.check()
}
