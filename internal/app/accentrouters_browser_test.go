package app

import (
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The accent with two routers, one of them Down (docs/color-roles.md, web/accent.js). The page border color gives the
// accent on the main page, /log and a panel page in both themes, as with one router. The red of the Down router stays
// the failure color, and its icon has the word "Router B" beside it.
func TestChromeAccentDownRouter(t *testing.T) {
	b := startBrowser(t)
	simA := startSim(t)
	simB, stopB := startSimStoppable(t)
	a := simRoutersApp(t, []string{simA, simB}, func(c *config.Config) { c.BrandBorder = "teal" })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	uids := routerUIDs(a)
	waitRouterRead(t, a, ana, uids[0], "DSM1")
	waitRouterRead(t, a, ana, uids[1], "DSM1")
	id := makePanel(t, a, ana, `{"name":"Desk grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	stopB()
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	// Canvas turns any color form into RGB. want is the accent that accent.js makes from teal for the theme.
	const kit = `const rgb = (c) => { const x = document.createElement('canvas').getContext('2d'); x.fillStyle = c; x.fillRect(0, 0, 1, 1); return [...x.getImageData(0, 0, 1, 1).data].join(','); };
		const probe = (cls, prop, style) => { const p = document.createElement('button'); p.className = cls; if (style) p.style.color = style; document.body.append(p); const c = getComputedStyle(p)[prop]; p.remove(); return rgb(c); };
		const m = await import('./accent.js');
		const dark = matchMedia('(prefers-color-scheme: dark)').matches;
		const want = rgb(m.accentFor('#5ccfc9')[dark ? 'dark' : 'light']);`
	usesTeal := `(async () => {
		` + kit + `
		const got = { accent: probe('', 'color', 'var(--pb-accent)'), primary: probe('pb-btn sode-btn--primary', 'backgroundColor') };
		const bad = Object.entries(got).filter(([, v]) => v !== want).map(([k, v]) => k + ' ' + v + ' not ' + want);
		return bad.join(' | ');
	})()`
	check := func(page, ready string) {
		t.Helper()
		b.open(srv.URL + page)
		b.waitFor(page+" ready", ready)
		b.waitFor(page+" brand", `document.querySelector('.sode__brandframe')?.dataset.brand === 'teal'`)
		for _, theme := range []string{"light", "dark"} {
			b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": theme}}})
			if got := b.eval(usesTeal); got != "" {
				t.Fatalf("%s %s: the accent is not teal: %v", page, theme, got)
			}
		}
	}

	check("/", `document.documentElement.dataset.conn === 'up'`)
	marks := `[...document.querySelectorAll('#conn-routers > .pb-status')]`
	b.waitFor("router B is Down: a red icon with its word", `(() => {
		const [ra, rb] = `+marks+`;
		if (!rb) return false;
		const red = getComputedStyle(document.documentElement).getPropertyValue('--pb-status-error').trim();
		const probe = document.createElement('i'); probe.style.color = red; document.body.append(probe);
		const want = getComputedStyle(probe).color; probe.remove();
		const dot = rb.querySelector('.sode__state');
		return ra.querySelector('.sode__state').dataset.state === 'ok' && dot.dataset.state === 'error' && getComputedStyle(dot).color === want
			&& rb.lastElementChild.textContent.trim() === 'Router B';
	})()`)
	check("/log", `document.querySelectorAll('.sode-up__bars[data-router]').length === 2`)
	check("/panels/"+id, `!!document.getElementById('panel-title')`)
	check("/panels", `!!document.getElementById('panels-new')`)

	for _, e := range b.problems() {
		if !strings.Contains(e, "net::ERR_CONNECTION_REFUSED") {
			t.Errorf("page error: %s", e)
		}
	}
}
