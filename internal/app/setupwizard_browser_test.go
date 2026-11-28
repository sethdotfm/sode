package app

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// A fresh install with a config path opens the setup wizard. Finish leaves a signed-in Administrator with
// the router and Standard access on disk, and nothing is written before it.
func TestChromeSetupWizard(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	t.Setenv("SODE_ROUTER", sim) // cmd/sode puts it in the config, as the edit below does

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
	// Each step name shows on one line inside the steps row, and the dialog starts below the row.
	stepsFit := func(step string) {
		b.waitFor(step+": the step names fit the steps row", `(() => {
			const row = document.getElementById('setup-steps').getBoundingClientRect();
			const dialog = document.getElementById('setup-dialog').getBoundingClientRect();
			const labels = [...document.querySelectorAll('#setup-steps .sode-setup__label')];
			return labels.length === 4 && labels.every((l) => {
				const r = l.getBoundingClientRect();
				const line = parseFloat(getComputedStyle(l).lineHeight) || r.height;
				return r.left >= row.left - 0.5 && r.right <= row.right + 0.5 && r.top >= row.top - 0.5 && r.bottom <= row.bottom + 0.5
					&& l.getClientRects().length === 1 && r.height <= line * 1.5;
			}) && row.bottom < dialog.top;
		})()`)
	}

	b.open(srv.URL + "/")
	b.waitFor("the wizard", `!document.getElementById('setup').hidden && document.documentElement.hasAttribute('data-sode-setup')`)
	b.waitFor("the patch page is hidden", `getComputedStyle(document.getElementById('patch-view')).display === 'none'`)
	b.waitFor("Welcome", `document.getElementById('setup-title').textContent === 'Set up Sode'`)
	stepsFit("Style & User")

	type_("setup-brandText", "Studio B")
	b.waitFor("the name shows live", `document.querySelector('#setup .sode__mark').textContent === 'Studio B'`)
	b.waitFor("each color has its name, and no retired color shows", `[...document.querySelectorAll('.sode-setup__swatch-name')].map((n) => n.offsetParent && n.textContent).join(' ') === 'Off Blue Teal Sky Slate Violet Custom'`)
	b.eval(`(() => { const c = document.querySelector('input[name="brandBorder"][type="color"]'); c.value = '#e05050'; c.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.waitFor("a custom color near a state color shows the caution note", `!!document.getElementById('setup-brand-caution')?.offsetParent`)
	b.eval(`document.querySelector('input[name="brandBorder"][value="teal"]').click(); true`)
	b.waitFor("the border shows live", `document.querySelector('.sode__brandframe').dataset.brand === 'teal'`)
	b.waitFor("Teal shows no note", `!document.getElementById('setup-brand-caution').offsetParent`)
	type_("setup-name", "Ada")
	type_("setup-pin", "2468")
	type_("setup-pin2", "2469")
	b.waitFor("PINs that differ stop Next", next+`.disabled && `+next+`.textContent === 'Next'`)
	type_("setup-pin2", "2468")
	b.waitFor("Next", `!`+next+`.disabled`)
	b.waitFor("Restore from a backup sits beside it", `!document.getElementById('setup-restore').hidden`)
	b.eval(next + `.click(); true`)

	b.waitFor("Router, filled from the config", `document.getElementById('setup-router')?.value === '`+sim+`'`)
	stepsFit("Router")
	b.waitFor("Router title, with the default title of Add router", `document.getElementById('setup-routerTitle')?.value === 'Router A'
		&& document.querySelector('label[for="setup-routerTitle"]').textContent === 'Router title'`)
	b.waitFor("Level numbering shows the real ranges, as the detail view does", `(() => { const n = +document.getElementById('setup-levels').value;
		return [...document.querySelectorAll('label[for^="setup-levelNumbering-"]')].map((l) => l.textContent).join(' | ') === '1–' + n + ' | 0–' + (n - 1) + ' | V, 1–' + (n - 1); })()`)
	type_("setup-routerTitle", "Studio Ultrix")
	b.waitFor("the env hint under Address", `document.getElementById('setup-router')?.parentElement.querySelector('.sode__hint')?.textContent === 'Filled from SODE_ROUTER.'`)
	status := `(document.getElementById('setup-router-status')?.textContent || '')`
	found := `/^Connected: \d+ sources? and \d+ destinations? discovered\.$/.test(` + status + `)`
	b.waitFor("the router answers with its counts", found)
	type_("setup-router", "127.0.0.1:1")
	b.waitFor("a dead address", status+` === 'No answer yet. Sode keeps trying.'`)
	type_("setup-router", sim)
	b.waitFor("the router answers again", found)
	b.eval(`document.getElementById('setup-fireMode-native').click(); true`)
	b.waitFor("Router salvo shows group 127", `document.getElementById('setup-group')?.value === '127'`)
	b.eval(`document.getElementById('setup-fireMode-sequential').click(); true`)
	if _, err := os.Stat(a.cfgPath); !os.IsNotExist(err) {
		t.Fatalf("config written before Finish: %v", err)
	}
	b.eval(next + `.click(); true`)

	b.waitFor("Access, Standard preselected", `document.querySelector('.sode-setup__sheet thead th.is-picked')?.dataset.preset === 'standard'`)
	stepsFit("Access")
	b.waitFor("Finish", next+`.textContent === 'Finish'`)
	b.eval(`document.querySelector('[data-go="0"]').click(); true`)
	b.waitFor("a chip goes back and keeps the answers", `document.getElementById('setup-name')?.value === 'Ada'`)
	b.eval(`document.querySelector('[data-go="2"]').click(); true`)
	b.waitFor("and forward again", next+`.textContent === 'Finish'`)
	b.eval(next + `.click(); true`)

	b.waitFor("Done", `document.getElementById('setup-title').textContent === 'Sode is set up'`)
	b.waitFor("the paragraph names user, router and preset", `(() => { const t = document.getElementById('setup-done-text').textContent;
		return t.includes('signed in as Ada') && t.includes('connected to the router at `+sim+`') && t.includes('Standard access'); })()`)
	b.waitFor("earlier steps lock", `!document.querySelector('#setup-steps [data-go]')`)
	c := loadFile(t, a.cfgPath)
	if ada, ok := c.FindUserByName("Ada"); !ok || ada.Role != config.RoleAdministrator || c.Routers[0].Address != sim || c.Routers[0].Title != "Studio Ultrix" || !c.PinWaits || c.BrandBorder != "teal" {
		t.Fatalf("after Finish: %+v", c)
	}
	b.waitFor("signed in", `!!localStorage.getItem('sode-session')`)

	b.eval(`document.getElementById('setup-open').click(); true`)
	b.waitFor("Open Sode shows the patch page", `document.getElementById('setup').hidden && getComputedStyle(document.getElementById('patch-view')).display !== 'none'`)
	b.open(srv.URL + "/")
	b.waitFor("no wizard after a reload", `!!document.getElementById('src') && document.getElementById('setup').hidden`)
	b.check()
}
