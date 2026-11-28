package app

import (
	"context"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The wizard at 1920x1080. The fields keep a readable width and the swatches sit on one pitch. The sections have one
// rhythm, and Router salvo moves no button. The controls show patchbay's own focus ring.
func TestChromeSetupWizardWidths(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	t.Setenv("SODE_ROUTER", sim) // for the Filled from hint under Address
	a := setupApp(t, func(c *config.Config) { c.Routers[0].Address = sim })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	type_ := func(id, text string) {
		b.eval(`(() => { const i = document.getElementById('` + id + `'); i.focus(); i.value = '` + text + `'; i.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	}
	next := `document.getElementById('setup-next')`
	width := func(id string) float64 { return b.boxes("#" + id)[0].Right - b.boxes("#" + id)[0].Left }
	near := func(x, y float64) bool { return x-y <= layoutTol && y-x <= layoutTol }

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	b.waitFor("Welcome", `document.getElementById('setup-title')?.textContent === 'Set up Sode'`)

	// One field width: half of a 48rem column, for the server Name here and the Router step's fields.
	half := (b.tokenPx("#setup-body", "48rem") - b.tokenPx("#setup-body", "--pb-space-6")) / 2
	if w := width("setup-brandText"); !near(w, half) {
		t.Errorf("server Name is %.1f px wide, want %.1f", w, half)
	}
	if w := b.boxes("#setup-body .sode-setup__row--3")[0]; w.Right-w.Left > b.tokenPx("#setup-body", "48rem")+layoutTol {
		t.Errorf("the first user's row is %.1f px wide, want 48rem at most", w.Right-w.Left)
	}

	// Each swatch box is on one pitch, Custom too, whatever its name's length.
	b.waitFor("the swatch boxes on one pitch", `(() => {
		const xs = [...document.querySelectorAll('.sode-setup__swatch-box')].map((n) => n.getBoundingClientRect().left);
		const step = xs[1] - xs[0];
		return xs.length === 6 && xs.every((x, i) => Math.abs(x - xs[0] - i * step) <= 0.5);
	})()`)
	b.noClip("each swatch name fits its item", ".sode-setup__swatch-name")

	// Keyboard focus shows patchbay's ring (2px accent, 1px out) on the step chip itself, and on a swatch's radio, which
	// fills the box.
	ring := func(what, sel string) {
		b.waitFor(what, `(() => {
			const n = document.querySelector('`+sel+`');
			n.focus();
			const s = getComputedStyle(n);
			const box = n.closest('.sode-setup__swatch-box');
			const r = n.getBoundingClientRect();
			const fills = !box || (Math.abs(r.width - box.clientWidth) <= 0.5 && Math.abs(r.height - box.clientHeight) <= 0.5);
			return n.matches(':focus-visible') && s.outlineStyle === 'solid' && s.outlineWidth === '2px' && s.outlineOffset === '1px'
				&& s.opacity === '1' && fills;
		})()`)
	}
	ring("a swatch shows patchbay's ring", `input[name="brandBorder"][value="blue"]`)

	// The caution is one line, and its place is kept, so the fields under it do not move.
	nameTop := b.boxes("#setup-name")[0].Top
	b.eval(`(() => { const c = document.querySelector('input[name="brandBorder"][type="color"]'); c.value = '#e05050'; c.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.waitFor("the caution shows", `getComputedStyle(document.getElementById('setup-brand-caution')).visibility === 'visible'`)
	b.waitFor("the caution is one line", `(() => {
		const r = document.createRange();
		r.selectNodeContents(document.querySelector('#setup-brand-caution span'));
		return new Set([...r.getClientRects()].map((q) => Math.round(q.top))).size === 1;
	})()`)
	if top := b.boxes("#setup-name")[0].Top; !near(top, nameTop) {
		t.Errorf("the caution moved the Name box from %.1f to %.1f", nameTop, top)
	}
	b.eval(`document.querySelector('input[name="brandBorder"][value="teal"]').click(); true`)
	b.waitFor("Teal hides the caution", `getComputedStyle(document.getElementById('setup-brand-caution')).visibility === 'hidden'`)

	// Step 1: section, rule, section on one rhythm.
	b.gapY("Style & User: sections and the rule 24px apart", "--pb-space-9", "#setup-body > .sode-setup__section, #setup-body > hr")

	type_("setup-name", "Ada")
	type_("setup-pin", "2468")
	type_("setup-pin2", "2468")
	b.waitFor("Next", `!`+next+`.disabled`)
	b.eval(next + `.click(); true`)
	b.waitFor("Router", `/^Connected/.test(document.getElementById('setup-router-status')?.textContent || '')`)
	ring("a step chip shows patchbay's ring", `#setup-steps [data-go]`)

	for _, id := range []string{"setup-routerTitle", "setup-router", "setup-levels"} {
		if w := width(id); !near(w, half) {
			t.Errorf("%s is %.1f px wide, want %.1f", id, w, half)
		}
	}
	b.gapY("Router: sections 24px apart", "--pb-space-9", "#setup-body > .sode-setup__section")
	// The status line is the Address field's last line, under its hint at the field's own gap.
	b.gapY("the status under the Address hint", "--pb-space-2", "#setup-router ~ .sode__hint", "#setup-router-status")

	// Router salvo adds its group under the choice. The button that the user pressed stays under the pointer.
	btn := func() string {
		x := b.boxes(`label[for="setup-fireMode-native"]`)[0]
		return strconv.FormatFloat(x.Top, 'f', 1, 64) + "," + strconv.FormatFloat(x.Left, 'f', 1, 64)
	}
	before := btn()
	b.eval(`document.getElementById('setup-fireMode-native').click(); true`)
	b.waitFor("Router salvo group", `!!document.getElementById('setup-group')`)
	if after := btn(); after != before {
		t.Errorf("Router salvo moved its button from %s to %s", before, after)
	}
	b.waitFor("the group is under the choice, with its rule as a hint", `(() => {
		const g = document.getElementById('setup-group').getBoundingClientRect();
		const c = document.querySelector('label[for="setup-fireMode-native"]').getBoundingClientRect();
		const f = document.getElementById('setup-group').closest('.sode__field');
		return g.top > c.bottom && f.querySelector('.sode__hint')?.textContent === 'Use a group that nothing else on the router uses.'
			&& !f.querySelector('label').hasAttribute('data-pb-tip');
	})()`)
	b.eval(`document.getElementById('setup-fireMode-sequential').click(); true`)
	b.eval(next + `.click(); true`)
	b.waitFor("Access", next+`.textContent === 'Finish'`)
	if w := b.boxes(".sode-setup__sheet")[0]; w.Right-w.Left > b.tokenPx("#setup-body", "64rem")+layoutTol {
		t.Errorf("the Access table is %.1f px wide, want 64rem at most", w.Right-w.Left)
	}
	b.check()
}
