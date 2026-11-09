package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// Templates in the panel editor: a layout saved to Me comes back through REST without cell contents, applying it
// leaves the cells unassigned and the panel unsaved, and Everyone asks a user who is not Trusted for the PIN.
func TestChromeEditorTemplates(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Show panel","rows":2,"cols":2,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":1},
		{"r":1,"c":2,"rs":1,"cs":1,"kind":"destination","n":2},
		{"r":2,"c":1,"rs":1,"cs":2,"kind":"take"}]}`)
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Show panel' && !!document.getElementById('ed-templates')`)

	b.eval(`document.getElementById('ed-templates').click()`)
	b.waitFor("dialog", `!!document.getElementById('tpl-save') && document.body.textContent.includes('No templates yet.')`)
	b.eval(`document.getElementById('tpl-name').value = 'Show layout'; document.getElementById('tpl-save').click()`)
	b.waitFor("saved to Me", `document.querySelector('.sode-tpl .pb-modal__footer')?.textContent.includes('Saved Show layout to Me.')`)
	body := apiDo(a, "GET", "/api/v1/panel-templates", ana, nil).Body.String()
	if !strings.Contains(body, `"name":"Show layout"`) || strings.Contains(body, `"n":`) || !strings.Contains(body, `"kind":"source"`) {
		t.Fatalf("the template kept cell contents or lost its layout: %s", body)
	}

	// Everyone is for a Trusted user or the PIN; Ana is neither, so the dialog asks for the PIN.
	b.eval(`document.getElementById('tpl-name').value = 'For all'; document.getElementById('tpl-scope-everyone').click(); document.getElementById('tpl-save').click()`)
	b.waitFor("PIN asked", `!document.querySelector('.sode-tpl__pin').hidden && document.querySelector('.sode-tpl .pb-modal__footer').textContent.includes('Trusted user')`)
	b.shot("pin")

	b.eval(`document.querySelector('[data-apply]').click()`)
	b.waitFor("applied with the cells unassigned", `!document.getElementById('tpl-save') && document.getElementById('ed-todo').textContent === '2 unassigned squares' && document.getElementById('ed-state').textContent === 'Unsaved changes'`)
	// The refused Everyone save is the one expected failed load.
	for _, e := range b.problems() {
		if !strings.Contains(e, "403") || !strings.Contains(e, "/api/v1/panel-templates") {
			t.Errorf("page error: %s", e)
		}
	}
}

// A template stored with Sources keeps the Sources and leaves the Destinations unassigned when applied to another panel,
// storing with both brings both back, and the toolbar counts empty squares with the unassigned cells. The Panel tab's theme
// reaches the editor's preview and the stored settings.
func TestChromeEditorTemplateContentsAndTheme(t *testing.T) {
	b, a, ana, id := editorTab(t, `{"name":"Full","rows":2,"cols":3,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"source","n":1,"caption":"Cam one"},
		{"r":1,"c":2,"rs":1,"cs":1,"kind":"destination","n":2,"caption":"Mon two"},
		{"r":2,"c":1,"rs":1,"cs":1,"kind":"take"}]}`)
	b.waitFor("empty squares counted", `document.getElementById('ed-todo').textContent === '3 unassigned squares'`)

	b.eval(`const s = document.getElementById('ed-set-theme'); s.value = 'dark'; s.dispatchEvent(new Event('change', { bubbles: true }))`)
	b.waitFor("preview follows the theme", `document.getElementById('ed-grid').style.colorScheme === 'dark'`)
	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	if body := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.String(); !strings.Contains(body, `"theme":"dark"`) {
		t.Fatalf("theme not stored: %s", body)
	}

	b.eval(`document.getElementById('ed-templates').click()`)
	b.waitFor("dialog", `!!document.getElementById('tpl-save')`)
	b.eval(`window.tplSave = (name, boxes) => {
		document.getElementById('tpl-name').value = name;
		for (const id of ['tpl-sources', 'tpl-dests', 'tpl-settings']) { const c = document.getElementById(id); if (c.checked !== boxes.includes(id)) c.click(); }
		document.getElementById('tpl-save').click();
	}; true`)
	b.eval(`tplSave('Src only', ['tpl-sources'])`)
	b.waitFor("first saved", `document.querySelector('.sode-tpl .pb-modal__footer').textContent.includes('Saved Src only to Me.')`)
	b.eval(`tplSave('Both', ['tpl-sources', 'tpl-dests', 'tpl-settings'])`)
	b.waitFor("second saved", `document.querySelector('.sode-tpl .pb-modal__footer').textContent.includes('Saved Both to Me.')`)
	if body := apiDo(a, "GET", "/api/v1/panel-templates", ana, nil).Body.String(); !strings.Contains(body, `"dests":false`) || !strings.Contains(body, `"dests":true`) {
		t.Fatalf("flags not stored: %s", body)
	}

	blank := makePanel(t, a, ana, `{"name":"Blank","rows":2,"cols":3}`)
	b.eval(`location.href = location.origin + '/panels/editor?panel=` + blank + `'`)
	b.waitFor("blank panel loaded", `document.getElementById('ed-name')?.value === 'Blank' && !!document.getElementById('ed-templates')`)
	b.waitFor("six empty squares", `document.getElementById('ed-todo').textContent === '6 unassigned squares'`)

	apply := func(name string) {
		b.eval(`document.getElementById('ed-templates').click()`)
		b.waitFor("dialog", `!!document.querySelector('[data-apply]')`)
		b.eval(`document.querySelector('[aria-label="Apply ` + name + `"]').click()`)
		b.waitFor("dialog closed", `!document.getElementById('tpl-save')`)
	}
	apply("Src only")
	b.waitFor("Sources came back, Destinations did not", `document.getElementById('ed-todo').textContent === '4 unassigned squares' && document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]').textContent.includes('Cam one') && !document.querySelector('.sode-ed__sq[data-r="1"][data-c="2"]').textContent.includes('Mon two')`)
	apply("Both")
	b.waitFor("both came back", `document.getElementById('ed-todo').textContent === '3 unassigned squares' && document.querySelector('.sode-ed__sq[data-r="1"][data-c="2"]').textContent.includes('Mon two')`)
	// The settings stored with it (theme dark) came with the cells.
	b.waitFor("settings applied with the contents", `document.getElementById('ed-set-theme').value === 'dark' && document.getElementById('ed-grid').style.colorScheme === 'dark'`)

	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("blank panel saved", `document.getElementById('ed-state').textContent === 'Saved'`)

	// Applying it back onto the first panel after a cell was removed restores the cell.
	b.eval(`location.href = location.origin + '/panels/editor?panel=` + id + `'`)
	b.waitFor("first panel loaded", `document.getElementById('ed-name')?.value === 'Full' && !!document.getElementById('ed-templates')`)
	b.eval(`const g = document.getElementById('ed-grid'); const k = (key) => g.dispatchEvent(new KeyboardEvent('keydown', { key, bubbles: true, cancelable: true })); k('ArrowDown'); k('ArrowDown'); k('Delete')`)
	apply("Both")
	b.waitFor("Sources and Destinations restored", `document.getElementById('ed-todo').textContent === '3 unassigned squares' && document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]').textContent.includes('Cam one') && document.querySelector('.sode-ed__sq[data-r="1"][data-c="2"]').textContent.includes('Mon two')`)

	b.eval(`document.getElementById('ed-save').click()`)
	b.waitFor("saved", `document.getElementById('ed-state').textContent === 'Saved'`)
	// The panel page sets data-theme from the stored panel, with no device settings code to undo it.
	b.eval(`location.href = location.origin + '/panels/` + id + `'`)
	b.waitFor("panel page runs in the stored theme", `document.documentElement.dataset.theme === 'dark' && !!document.querySelector('#panel-grid > *')`)
	b.check()
}
