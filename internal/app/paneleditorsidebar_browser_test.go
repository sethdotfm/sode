package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The editor's sidebar: it opens on Panel, a selected square switches it to Cell, and what is set in the
// three tabs is what the panel stores.
func TestChromeEditorSidebar(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Side panel","rows":2,"cols":2,"cells":[{"r":1,"c":1,"rs":1,"cs":1,"kind":"take"}]}`)
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded and the sidebar built", `document.getElementById('ed-name').value === 'Side panel' && !!document.querySelector('#ed-side .pb-tab')`)

	tab := `document.querySelector('#ed-side .pb-tab[aria-selected="true"]')?.textContent`
	b.waitFor("opens on Panel", tab+` === 'Panel'`)

	b.eval(`document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]').dispatchEvent(new PointerEvent('pointerdown', { bubbles: true, button: 0, clientX: document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]').getBoundingClientRect().x + 4, clientY: document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]').getBoundingClientRect().y + 4 }))`)
	b.waitFor("a selected square switches to Cell", tab+` === 'Cell'`)
	b.eval(`const c = document.getElementById('ed-cell-caption'); c.value = 'Go live'; c.dispatchEvent(new Event('input', { bubbles: true })); true`)
	b.waitFor("the caption shows in the grid", `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-ed__text')?.textContent === 'Go live'`)
	b.eval(`const s = document.getElementById('ed-cell-size'); s.value = '150'; s.dispatchEvent(new Event('input', { bubbles: true })); true`)

	b.waitFor("Hide text is offered only once the cell has artwork", `[...document.querySelectorAll('#panel-ed-cell .pb-btn')].every((x) => x.textContent !== 'Hide text' || x.hidden)`)
	b.eval(`[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Choose icon').click(); document.querySelector('.sode-side__icons button[aria-label="tv"]').click(); true`)
	b.waitFor("Hide text appears with an icon", `[...document.querySelectorAll('#panel-ed-cell .pb-btn')].some((x) => x.textContent === 'Hide text' && !x.hidden)`)
	b.eval(`[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Hide text').click(); true`)
	b.waitFor("the button flips to Show text", `[...document.querySelectorAll('#panel-ed-cell .pb-btn')].some((x) => x.textContent === 'Show text' && !x.hidden)`)

	b.eval(`document.getElementById('tab-ed-panel').click(); document.getElementById('ed-set-titleBar').click(); true`)
	b.waitFor("Rows and Columns are in the Panel tab, not the toolbar", `!!document.querySelector('#panel-ed-panel #ed-rows') && !!document.querySelector('#panel-ed-panel #ed-cols') && !document.querySelector('.sode-ed__bar #ed-rows')`)
	b.eval(`const r = document.getElementById('ed-rows'); r.value = '3'; r.dispatchEvent(new Event('change', { bubbles: true })); document.getElementById('ed-cols-inc').click(); true`)
	b.waitFor("the grid took the new size", `document.getElementById('ed-rows').value === '3' && document.getElementById('ed-cols').value === '3'`)
	b.waitFor("Take button behavior follows Mode, and Updates is gone", `document.getElementById('ed-set-guard').closest('.sode__field').previousElementSibling.contains(document.getElementById('ed-set-mode')) && ![...document.querySelectorAll('#panel-ed-panel .sode-side__h')].some((h) => h.textContent === 'Updates')`)
	b.eval(`document.getElementById('ed-set-fullscreen').click(); true`)
	b.waitFor("with no subset stored every level box is ticked", `(() => { const l = [...document.querySelectorAll('#panel-ed-panel .sode-side__levels input')]; return l.length > 0 && l.every((x) => x.checked); })()`)
	b.eval(`const m = document.getElementById('ed-set-guard'); m.value = 'hold'; m.dispatchEvent(new Event('change', { bubbles: true })); true`)
	b.eval(`document.getElementById('tab-ed-access').click(); true`)
	b.waitFor("Access says who it acts as", `document.getElementById('panel-ed-access').textContent.includes('Whoever opens it') && !document.getElementById('ed-acts').disabled`)
	b.waitFor("Watch is offered to the owner", `!!document.getElementById('panels-watch')`)
	b.eval(`document.getElementById('panels-watch').click(); true`)
	for deadline := time.Now().Add(10 * time.Second); !a.mon.watching("Ana", id); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the editor checkbox did not set the watch")
		}
	}
	b.eval(`const a = document.getElementById('ed-acts'); a.value = 'api'; a.dispatchEvent(new Event('change', { bubbles: true })); true`)
	b.waitFor("Acts as saves at once", `document.getElementById('ed-state').textContent === 'Saved' && document.getElementById('panel-ed-access').textContent.includes('The panel now has its own API client')`)
	b.waitFor("Watch stays checked after the Access tab repaints", `document.getElementById('panels-watch')?.checked === true`)
	b.shot("access")

	b.waitFor("Watch stays checked after a save", `document.getElementById('panels-watch')?.checked === true`)
	rec := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil)
	body := rec.Body.String()
	for _, want := range []string{`"caption":"Go live"`, `"fontSize":150`, `"titleBar":false`, `"guard":"hold"`, `"fullscreen":true`, `"hideText":true`, `"art":"icon:tv"`, `"rows":3`, `"cols":3`, `"acts":"api"`, `"client":"Panel ` + id + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the saved panel lacks %s: %s", want, body)
		}
	}
	b.waitFor("Access names the client", `document.getElementById('panel-ed-access').textContent.includes('Client Panel `+id+`')`)
	b.check()
}
