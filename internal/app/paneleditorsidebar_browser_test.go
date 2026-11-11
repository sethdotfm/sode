package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"

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
	b.waitFor("an unset cell font size shows 100, so the arrows start there", `document.getElementById('ed-cell-size').value === '100'`)
	b.eval(`const s = document.getElementById('ed-cell-size'); s.value = '150'; s.dispatchEvent(new Event('input', { bubbles: true })); true`)

	b.waitFor("Hide text is offered only once the cell has artwork", `[...document.querySelectorAll('#panel-ed-cell .pb-btn')].every((x) => x.textContent !== 'Hide text' || x.hidden)`)
	b.eval(`[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Choose icon').click(); true`)
	b.waitFor("the picker offers the cell kind icons in Icon A", `!!document.querySelector('.sode-side__icons[aria-label="Icon A"] button[aria-label="arrow-right-from-line"]') && !!document.querySelector('.sode-side__icons[aria-label="Icon A"] button[aria-label="eraser"]')`)
	b.eval(`document.querySelector('.sode-side__icons[aria-label="Icon A"] button[aria-label="tv"]').click(); true`)
	b.waitFor("Hide text appears with an icon", `[...document.querySelectorAll('#panel-ed-cell .pb-btn')].some((x) => x.textContent === 'Hide text' && !x.hidden)`)
	b.waitFor("the square shows Icon A", `!!document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-panel__art-a')`)
	// Icon B: the glyph column beside Icon A's, both shown at once, each with its own None; the square shows "[tv] 2".
	b.waitFor("Icon A and Icon B sit side by side, each with None in its header", `(() => { const a = document.querySelector('.sode-side__icons[aria-label="Icon A"]'); const b = document.querySelector('.sode-side__icons[aria-label="Icon B"]'); const ra = a.getBoundingClientRect(); const rb = b.getBoundingClientRect(); return ra.width > 0 && rb.width > 0 && rb.left >= ra.right && Math.abs(rb.top - ra.top) < 2 && a.parentNode.querySelector('.sode-side__col-top').textContent.includes('None') && b.parentNode.querySelector('.sode-side__col-top').textContent.includes('None') && getComputedStyle(a).overflowY === 'scroll' && getComputedStyle(b).overflowY === 'scroll'; })()`)
	b.waitFor("Icon A and Icon B columns are the same width", `(() => { const w = (l) => document.querySelector('.sode-side__icons[aria-label="' + l + '"]').parentNode.getBoundingClientRect().width; return w('Icon A') > 0 && Math.abs(w('Icon A') - w('Icon B')) < 2; })()`)
	b.waitFor("the Icon B column lists digits and letters", `!!document.querySelector('.sode-side__icons[aria-label="Icon B"] button[aria-label="16"]') && !!document.querySelector('.sode-side__icons[aria-label="Icon B"] button[aria-label="Z"]')`)
	b.eval(`document.querySelector('.sode-side__icons[aria-label="Icon B"] button[aria-label="2"]').click(); true`)
	b.waitFor("the square shows Icon B beside Icon A", `(() => { const a = document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-panel__art-a'); const b = document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-panel__art-b'); return !!a && b?.textContent === '2' && b.getBoundingClientRect().left >= a.getBoundingClientRect().right; })()`)
	b.eval(`[...document.querySelectorAll('#panel-ed-cell .pb-btn')].find((x) => x.textContent === 'Hide text').click(); true`)
	b.waitFor("the button flips to Show text", `[...document.querySelectorAll('#panel-ed-cell .pb-btn')].some((x) => x.textContent === 'Show text' && !x.hidden)`)
	b.waitFor("the editor square hides its text as the panel will", `document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"]').dataset.hideText === '1' && document.querySelector('.sode-ed__sq[data-r="1"][data-c="1"] .sode-ed__text').getBoundingClientRect().width <= 1`)

	b.eval(`document.getElementById('tab-ed-panel').click(); document.getElementById('ed-set-titleBar').click(); true`)
	b.waitFor("Rows and Columns are in the Panel tab, not the toolbar", `!!document.querySelector('#panel-ed-panel #ed-rows') && !!document.querySelector('#panel-ed-panel #ed-cols') && !document.querySelector('.sode-ed__bar #ed-rows')`)
	b.waitFor("Rows and Columns share one line", `(() => { const q = (id) => document.getElementById(id).getBoundingClientRect(); return Math.abs(q('ed-rows').top - q('ed-cols').top) < 2 && q('ed-cols').left > q('ed-rows-inc').right; })()`)
	b.waitFor("Tab from Rows reaches Columns: the step buttons are out of the Tab order", `[...document.querySelectorAll('#panel-ed-panel .sode-side__step button')].every((x) => x.tabIndex === -1) && (() => { const f = [...document.querySelectorAll('#panel-ed-panel input, #panel-ed-panel button, #panel-ed-panel select')].filter((x) => x.tabIndex >= 0 && !x.disabled && x.offsetParent); return f[f.indexOf(document.getElementById('ed-rows')) + 1] === document.getElementById('ed-cols'); })()`)
	b.waitFor("an unset panel font size shows 100, so the arrows start there", `(() => { const f = document.getElementById('ed-set-font'); f.stepUp(); const up = f.value; f.stepDown(); return up === '101' && f.value === '100'; })()`)
	b.eval(`const r = document.getElementById('ed-rows'); r.value = '3'; r.dispatchEvent(new Event('change', { bubbles: true })); document.getElementById('ed-cols-inc').click(); true`)
	b.waitFor("the grid took the new size", `document.getElementById('ed-rows').value === '3' && document.getElementById('ed-cols').value === '3'`)
	b.waitFor("Take button behavior follows Mode, and Updates is gone", `document.getElementById('ed-set-guard').closest('.sode__field').previousElementSibling.contains(document.getElementById('ed-set-mode')) && ![...document.querySelectorAll('#panel-ed-panel .sode-side__h')].some((h) => h.textContent === 'Updates')`)
	b.eval(`document.getElementById('ed-set-fullscreen').click(); true`)
	b.waitFor("Ask users to set an instance name starts on", `document.getElementById('ed-set-askInstanceName').checked`)
	b.eval(`document.getElementById('ed-set-askInstanceName').click(); true`)
	b.waitFor("with no subset stored every level box is ticked", `(() => { const l = [...document.querySelectorAll('#panel-ed-panel .sode__picker .sode__pick')]; return l.length > 0 && l.every((x) => x.getAttribute('aria-pressed') === 'true'); })()`)
	b.eval(`document.querySelector('#panel-ed-panel .sode__picker .sode__pick').click(); true`)
	b.waitFor("the first level goes off and All is offered again", `(() => { const p = document.querySelector('#panel-ed-panel .sode__picker .sode__pick'); return p.getAttribute('aria-pressed') === 'false' && !document.querySelector('#panel-ed-panel .sode__all').disabled; })()`)
	b.eval(`document.getElementById('ed-set-guard-hold').click(); true`)
	b.eval(`document.getElementById('tab-ed-access').click(); true`)
	b.waitFor("Access says who it acts as", `document.getElementById('panel-ed-access').textContent.includes('Authentication mode') && !document.getElementById('ed-acts-api').disabled`)
	b.eval(`document.getElementById('ed-acts-api').click(); true`)
	b.waitFor("Acts as saves at once", `document.getElementById('ed-state').textContent === 'Saved' && document.getElementById('panel-ed-access').textContent.includes('The panel now has its own API client')`)
	b.shot("access")

	rec := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil)
	body := rec.Body.String()
	if !regexp.MustCompile(`"settings":\{[^}]*"levels":\[2,`).MatchString(body) {
		t.Errorf("the saved panel lacks the level subset without level 1: %s", body)
	}
	for _, want := range []string{`"caption":"Go live"`, `"fontSize":150`, `"titleBar":false`, `"guard":"hold"`, `"fullscreen":true`, `"askInstanceName":false`, `"hideText":true`, `"art":"icon:tv"`, `"artB":"2"`, `"rows":3`, `"cols":3`, `"acts":"api"`, `"client":"Panel ` + id + `"`} {
		if !strings.Contains(body, want) {
			t.Errorf("the saved panel lacks %s: %s", want, body)
		}
	}
	b.waitFor("Access names the client", `document.getElementById('panel-ed-access').textContent.includes('Client Panel `+id+`')`)
	b.check()
}
