package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The accent is the page border color (docs/color-roles.md, web/accent.js) on the main page, /log, /panels, a panel
// page and the panel editor, in both themes. A reload shows it before the hello. Settings offers no retired color, keeps a saved one, and shows the
// caution note for a color in the reserved zone.
func TestChromeBrandAccent(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.BrandBorder = "teal"
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	id := makePanel(t, a, userKey(t, a, "Ana"), `{"name":"Desk grid","type":"grid","settings":{"mode":"take","levels":[1]}}`)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})

	// Each page keeps the accent it showed when the first socket message came, before any page handler ran.
	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": `(() => {
		const W = window.WebSocket;
		window.WebSocket = class extends W {
			constructor(...a) {
				super(...a);
				this.addEventListener('message', () => {
					if (window.accentAtHello !== undefined) return;
					const p = document.createElement('i');
					p.style.color = 'var(--pb-accent)';
					document.body.append(p);
					window.accentAtHello = getComputedStyle(p).color;
					p.remove();
				});
			}
		};
	})()`})

	// Canvas turns any color form into RGB. want is the accent accent.js makes from teal for the theme.
	const kit = `const rgb = (c) => { const x = document.createElement('canvas').getContext('2d'); x.fillStyle = c; x.fillRect(0, 0, 1, 1); return [...x.getImageData(0, 0, 1, 1).data].join(','); };
		const probe = (cls, prop, style) => { const p = document.createElement('button'); p.className = cls; if (style) p.style.color = style; document.body.append(p); const c = getComputedStyle(p)[prop]; p.remove(); return rgb(c); };
		const m = await import('./accent.js');
		const dark = matchMedia('(prefers-color-scheme: dark)').matches;
		const want = rgb(m.accentFor('#5ccfc9')[dark ? 'dark' : 'light']);
		const periwinkle = rgb(dark ? '#8b93ff' : '#3a44c4');`
	usesTeal := `(async () => {
		` + kit + `
		const got = { accent: probe('', 'color', 'var(--pb-accent)'), primary: probe('pb-btn sode-btn--primary', 'backgroundColor'), selected: probe('pb-btn pb-btn--selected', 'backgroundColor') };
		const bad = Object.entries(got).filter(([, v]) => v !== want).map(([k, v]) => k + ' ' + v + ' not ' + want);
		return want !== periwinkle && bad.length === 0 ? '' : bad.join(' | ') || 'teal is periwinkle';
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
		// The first load stored the accent, so this reload shows it before the hello.
		b.open(srv.URL + page)
		b.waitFor(page+" accent before the hello", `window.accentAtHello !== undefined`)
		if got := b.eval(`(async () => { ` + kit + ` const pre = rgb(window.accentAtHello); return pre === want ? '' : 'before the hello ' + pre + ', want ' + want + (pre === periwinkle ? ' (periwinkle)' : ''); })()`); got != "" {
			t.Fatalf("%s: %v", page, got)
		}
	}
	check("/log", `document.querySelectorAll('.sode-log tbody tr, .sode-log__row').length > 0`)
	check("/panels", `!!document.getElementById('panels-new')`)
	// The hello comes before a sign-in, so the panel pages show the accent behind the lock screen too.
	check("/panels/"+id, `!!document.getElementById('panel-title')`)
	check("/panels/editor?panel="+id, `!!document.getElementById('ed-save')`)
	check("/", `document.documentElement.dataset.conn === 'up'`)

	// A saved retired color still loads and draws, and Settings keeps it with the caution note.
	if err := a.applyConfig(func(c *config.Config) error { c.BrandBorder = "amber"; return nil }); err != nil {
		t.Fatal(err)
	}
	// A new document, so Settings fills from the hello after the change.
	b.open(srv.URL + "/?after#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Branding shown", `!!document.getElementById('cfg-brandborder')?.offsetParent`)
	b.waitFor("amber kept with the caution note, no other retired color", `(() => {
		const s = document.getElementById('cfg-brandborder');
		const names = [...s.options].map((o) => o.value).join(' ');
		return s.value === 'amber' && names === ' blue teal violet lilac amber custom' && !!document.getElementById('brand-caution').offsetParent &&
			document.querySelector('.sode__brandframe').dataset.brand === 'amber';
	})()`)
	pick := func(value, custom string) {
		t.Helper()
		b.eval(`(() => {
			const s = document.getElementById('cfg-brandborder');
			s.value = '` + value + `';
			s.dispatchEvent(new Event('change', { bubbles: true }));
			const c = document.getElementById('cfg-brandcustom');
			if ('` + custom + `') { c.value = '` + custom + `'; c.dispatchEvent(new Event('input', { bubbles: true })); }
			return true;
		})()`)
	}
	pick("blue", "")
	b.waitFor("no note for Blue", `!document.getElementById('brand-caution').offsetParent`)
	pick("custom", "#e05050")
	b.waitFor("the note for a custom red", `!!document.getElementById('brand-caution').offsetParent && /near a state color/.test(document.getElementById('brand-caution').textContent)`)
	pick("custom", "#4060e0")
	b.waitFor("no note for a custom color in the free band", `!document.getElementById('brand-caution').offsetParent`)
	b.check()
}
