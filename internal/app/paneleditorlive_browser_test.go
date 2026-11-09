package app

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

// Test it live mounts the saved panel inside the editor: no new tab, a take works from there, the editor
// session is the auth, no instance shows as online, and Back to editing returns to the grid. Share sits in the toolbar.
func TestChromeEditorLive(t *testing.T) {
	b, a, ana, id := editorTab(t, fmt.Sprintf(`{"name":"Wall","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"source","n":%d},
		{"r":1,"c":3,"kind":"take"}],"settings":{"mode":"take"}}`, simDst, simSrc))
	for deadline := time.Now().Add(15 * time.Second); !strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the app never read names from swp08sim")
		}
	}
	b.waitFor("Share in the toolbar", `!!document.getElementById('ed-share')?.closest('.sode-ed__bar')`)
	b.waitFor("no Share in the sidebar", `![...document.querySelectorAll('#ed-side .pb-btn')].some(x => x.textContent === 'Share')`)

	b.eval(`window.opened = 0; window.open = () => { window.opened++ }; document.getElementById('ed-live').click()`)
	b.waitFor("live view shown, editor body hidden", `!document.getElementById('ed-live-view').hidden && document.querySelector('.sode-ed__body').hidden`)
	b.waitFor("three cells in the editor page", `document.querySelectorAll('#ed-live-view .sode-panel__cell').length === 3`)
	cell := func(kind string) string {
		return fmt.Sprintf(`document.querySelector('#ed-live-view .sode-panel__cell--%s')`, kind)
	}
	b.waitFor("destination named", cell("destination")+`?.querySelector('span')?.textContent === 'DSM1'`)
	b.eval(cell("destination") + `.click()`)
	b.eval(cell("source") + `.click()`)
	b.waitFor("Take armed", `!`+cell("take")+`.disabled`)
	b.eval(cell("take") + `.click()`)
	b.waitFor("Connected result", cell("destination")+`?.dataset.result === 'connected'`)
	if rec := apiDo(a, "GET", "/api/v1/tally", ana, nil); !strings.Contains(rec.Body.String(), fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, simDst, simSrc)) {
		t.Errorf("tally after take: %s", rec.Body.String())
	}
	time.Sleep(500 * time.Millisecond)
	if n := b.eval(`window.opened`); n != float64(0) {
		t.Errorf("a window was opened: %v", n)
	}
	if got := a.mon.instances(id); len(got) != 0 {
		t.Errorf("a live test showed as an instance: %v", got)
	}

	b.eval(`document.getElementById('ed-live-back').click()`)
	b.waitFor("back to editing", `document.getElementById('ed-live-view').hidden && !document.querySelector('.sode-ed__body').hidden && document.querySelectorAll('#ed-live-view .sode-panel__cell').length === 0`)
	b.check()
}
