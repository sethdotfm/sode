package app

import (
	"strings"
	"testing"
)

// The Buttons panel editor at 1920x1080. The Pages column and the grid start on one line. The sidebar keeps one
// width between tabs, and its labels are one size. At 960 px an Info cell's text stays in its square.
func TestChromePanelEditorButtonsLayout(t *testing.T) {
	b, _, _, id := editorTab(t, `{"name":"Studio A","acts":"both","pages":[{"id":"p1","name":"Main","enabled":true,"cells":[
		{"r":1,"c":1,"rs":1,"cs":1,"kind":"info"},
		{"r":1,"c":2,"rs":1,"cs":1,"kind":"source","n":1}]}]}`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.waitFor("the sidebar", `!!document.querySelector('#ed-side .pb-tab') && document.getElementById('ed-name').value === 'Studio A' && !!document.querySelector('#ed-pages .sode-pages__row')`)
	b.eval(`document.activeElement.blur(); true`)
	text := func(js string) string {
		v, _ := b.eval(js).(string)
		return v
	}
	px := func(js string) float64 {
		v, _ := b.eval(js).(float64)
		return v
	}
	near := func(a, c float64) bool { return a-c < layoutTol && c-a < layoutTol }
	b.shot("editor")

	// C2-19: the first Page row and the grid start on one line.
	if r, g := px(`document.querySelector('#ed-pages .sode-pages__row').getBoundingClientRect().top`), px(`document.getElementById('ed-grid').getBoundingClientRect().top`); !near(r, g) {
		t.Errorf("the first Page row starts at %.1f, the grid at %.1f", r, g)
	}

	// C2-20: one sidebar width on every tab, and the same room above as at the sides.
	tabTo := func(id string) {
		b.eval(`document.getElementById('tab-` + id + `').click(); true`)
		b.waitFor(id, `document.getElementById('tab-`+id+`').getAttribute('aria-selected') === 'true'`)
	}
	widths := map[string]float64{}
	for _, tb := range []string{"ed-panel", "ed-cell", "ed-access"} {
		tabTo(tb)
		widths[tb] = px(`document.querySelector('#ed-side > .sode__tabs').getBoundingClientRect().width`)
	}
	if !near(widths["ed-panel"], widths["ed-cell"]) || !near(widths["ed-panel"], widths["ed-access"]) {
		t.Errorf("the tab bar widths differ between tabs: %v", widths)
	}
	if top, side := px(`parseFloat(getComputedStyle(document.getElementById('ed-side')).paddingTop)`), px(`parseFloat(getComputedStyle(document.getElementById('ed-side')).paddingLeft)`); !near(top, side) {
		t.Errorf("the sidebar has %.1f px above and %.1f px at the side", top, side)
	}

	// C2-21, C2-22: the Info cell's Cell tab.
	b.eval(`(() => { const sq = document.querySelector('#ed-grid .sode-ed__sq[data-r="1"][data-c="1"]'), r = sq.getBoundingClientRect();
		const at = { bubbles: true, button: 0, clientX: r.x + 4, clientY: r.y + 4 };
		sq.dispatchEvent(new PointerEvent('pointerdown', at)); document.dispatchEvent(new PointerEvent('pointerup', at)); })(); true`)
	b.waitFor("the Cell tab on the Info cell", `document.getElementById('tab-ed-cell').getAttribute('aria-selected') === 'true' && !!document.querySelector('#ed-cell-show > legend')?.offsetParent`)
	b.eval(`document.activeElement.blur(); true`)
	b.shotOf("cell", "#ed-side")
	b.sameHeight("Show and Text on the cell labels", "#ed-cell-show > legend", `label[for="ed-cell-caption"]`)
	if got := text(`document.getElementById('ed-cell-kind').textContent`); got != "Info, row 1, column 1" {
		t.Errorf("the Cell tab heading reads %q", got)
	}
	if !b.eval(`document.getElementById('ed-cell-kind').classList.contains('sode-side__h') && getComputedStyle(document.getElementById('ed-cell-kind')).textAlign !== 'center'`).(bool) {
		t.Error("the Cell tab heading is not a left-aligned tab heading")
	}
	b.sameLeft("the heading on the form's edge", "#ed-cell-kind", `label[for="ed-cell-caption"]`)
	if got := text(`document.querySelector('label[for="ed-cell-caption"]').textContent`); got != "Text on the cell" {
		t.Errorf("caption label %q", got)
	}

	// C2-23, C2-24: the Panel tab.
	tabTo("ed-panel")
	b.eval(`document.activeElement.blur(); true`)
	b.shotOf("panel", "#ed-side")
	if !b.eval(`(() => { const f = document.getElementById('ed-set-font').closest('.sode-side__inline'), c = document.getElementById('ed-set-titleBar').closest('.sode__field');
		return f.getBoundingClientRect().top >= c.getBoundingClientRect().bottom && parseFloat(getComputedStyle(f).borderLeftWidth) === 0; })()`).(bool) {
		t.Error("Font size shares a row with Show the title bar, or keeps its rule")
	}
	b.sameLeft("Font size under the check box", `#ed-set-titleBar`, `label[for="ed-set-font"]`)
	panelText := text(`document.getElementById('panel-ed-panel').textContent`)
	for _, want := range []string{"ID " + id, "Owner: Ana", "Short name for the link", "Letters, digits and hyphens. Empty uses the ID.", "Stage more than one destination", "Go to full screen at the first press", "Keep the page in landscape", "Ask users to set an instance name", "Take button behavior"} {
		if !strings.Contains(panelText, want) {
			t.Errorf("the Panel tab lacks %q", want)
		}
	}
	for _, gone := range []string{"Owned by", "Automatically enter fullscreen", "Enforce landscape orientation", "Stage several Destinations", "Take guard"} {
		if strings.Contains(panelText, gone) {
			t.Errorf("the Panel tab still says %q", gone)
		}
	}

	// C2-24, C2-25: the Access tab.
	tabTo("ed-access")
	b.waitFor("the client table", `!!document.querySelector('#panel-ed-access .sode-side__table th')`)
	b.eval(`document.activeElement.blur(); true`)
	b.shotOf("access", "#ed-side")
	accessText := text(`document.getElementById('panel-ed-access').textContent`)
	for _, want := range []string{"API client: Panel " + id + ". Its Rights:", "Limit the Rights of this panel", "Shared with: Only the owner"} {
		if !strings.Contains(accessText, want) {
			t.Errorf("the Access tab lacks %q in %q", want, accessText)
		}
	}
	if n := px(`document.querySelectorAll('#panel-ed-access .sode-side__people li').length`); n != 0 {
		t.Errorf("one sharing choice shows as a list of %v", n)
	}
	b.gapX("Rights label and value", "--pb-space-5", "#panel-ed-access .sode-side__table tr:first-child th", "#panel-ed-access .sode-side__table tr:first-child td")
	b.sameLeft("Rights labels on the text edge", "#panel-ed-access .sode-side__rights", "#panel-ed-access .sode-side__table th")
	if w := px(`parseFloat(getComputedStyle(document.querySelector('#panel-ed-access .sode-side__table td')).borderBottomWidth)`); w != 0 {
		t.Errorf("a Rights value has a %v px rule under it", w)
	}

	// C2-26: icons on the toolbar buttons, and Save stays at the right when the bar wraps.
	if !b.eval(`['ed-share', 'ed-copylink', 'ed-open', 'ed-live', 'ed-name-btn'].every((id) => !!document.getElementById(id).querySelector('svg'))`).(bool) {
		t.Error("a toolbar button has no icon")
	}

	// C2-27: at 960 px the Info cell's lines stay in its square, under its icon.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 960, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.waitFor("the narrow layout", `innerWidth === 960`)
	b.shot("narrow")
	if !b.eval(`(() => { const bar = document.querySelector('.sode-ed__bar'), save = document.getElementById('ed-save'), s = getComputedStyle(bar);
		return Math.abs(bar.getBoundingClientRect().right - parseFloat(s.paddingRight) - save.getBoundingClientRect().right) < 0.5; })()`).(bool) {
		t.Error("Save is not at the right of the wrapped toolbar")
	}
	if got := text(`document.querySelector('#ed-grid .sode-ed__sq--info .sode-ed__text').innerText`); got != "Panel Name\nMain\nInstance Name" {
		t.Errorf("the Info cell reads %q", got)
	}
	if !b.eval(`(() => { const sq = document.querySelector('#ed-grid .sode-ed__sq--info'), r = sq.getBoundingClientRect(), i = sq.querySelector('svg').getBoundingClientRect(), x = sq.querySelector('.sode-ed__text').getBoundingClientRect();
		return i.top >= r.top && x.bottom <= r.bottom && x.top >= i.bottom; })()`).(bool) {
		t.Error("the Info cell's icon or text is past its square")
	}

	// C2-28: the Live Editor's bar.
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.eval(`document.getElementById('ed-live').click(); true`)
	b.waitFor("the Live Editor", `!!document.getElementById('ed-live-bar')?.offsetParent`)
	b.shotOf("live", "#ed-live-bar")
	if got := text(`document.getElementById('ed-live-title').textContent`); got != "Live Editor" {
		t.Errorf("Live Editor title %q", got)
	}
	if got := text(`[...document.querySelectorAll('#ed-live-bar .pb-badge--warn')].map((x) => x.textContent).join('|')`); got != "Live: takes change the router" {
		t.Errorf("Live Editor badge %q", got)
	}
	if got := text(`document.getElementById('ed-live-back').textContent`); got != "Back to the editor" {
		t.Errorf("Back button %q", got)
	}
	b.check()
}
