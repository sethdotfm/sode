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
	b.waitFor("applied with the cells unassigned", `!document.getElementById('tpl-save') && document.getElementById('ed-todo').textContent === '2 unassigned' && document.getElementById('ed-state').textContent === 'Unsaved changes'`)
	// The refused Everyone save is the one expected failed load.
	for _, e := range b.problems() {
		if !strings.Contains(e, "403") || !strings.Contains(e, "/api/v1/panel-templates") {
			t.Errorf("page error: %s", e)
		}
	}
}
