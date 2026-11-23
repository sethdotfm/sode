package app

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Dark cockpit (docs/color-roles.md). With both links up and nothing failing, the main page, /log and /panels show no
// green, red or blue state color. Amber shows only on the main page's Take. Take is amber with its word. A lost link is red with its words.
func TestChromeColorRoles(t *testing.T) {
	b := startBrowser(t)
	sim, stopSim := startSimStoppable(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1440, "height": 900, "deviceScaleFactor": 1, "mobile": false})
	// Canvas turns any color form into RGB; a probe reads a token in this theme.
	const kit = `const rgb = (c) => { const x = document.createElement('canvas').getContext('2d'); x.fillStyle = c; x.fillRect(0, 0, 1, 1); return [...x.getImageData(0, 0, 1, 1).data].join(','); };
		const token = (v) => { const p = document.createElement('i'); p.style.color = 'var(' + v + ')'; document.body.append(p); const c = getComputedStyle(p).color; p.remove(); return rgb(c); };`
	// One string, so a miss names each element and property. Polled, since a theme change runs the transitions.
	scan := `(() => {
		` + kit + `
		const state = { ok: token('--pb-status-ok'), error: token('--pb-status-error'), info: token('--pb-status-info'), warn: token('--pb-status-warn') };
		const take = document.getElementById('patch');
		const hits = [];
		for (const n of document.querySelectorAll('body *')) {
			// The uptime graph's key names each color in words, and its warn and error bars are past events, so they may show.
			if (!n.getClientRects().length || n.closest('#patch') || n.matches('.sode-up__key, .sode-up__bar--warn, .sode-up__bar--error')) continue;
			const s = getComputedStyle(n);
			if (s.visibility === 'hidden' || s.opacity === '0') continue;
			for (const p of ['color', 'backgroundColor', 'borderTopColor', 'borderLeftColor']) {
				if (p !== 'color' && p !== 'backgroundColor' && parseFloat(s[p === 'borderTopColor' ? 'borderTopWidth' : 'borderLeftWidth']) === 0) continue;
				const c = rgb(s[p]);
				if (c.endsWith(',0')) continue;
				for (const [k, v] of Object.entries(state)) if (c === v) hits.push(k + ':' + p + ':' + (n.id || n.getAttribute('class')));
			}
		}
		if (take && rgb(getComputedStyle(take).backgroundColor) !== token('--pb-commit')) hits.push('Take is not --pb-commit');
		return hits.slice(0, 6).join(' | ');
	})()`
	quiet := func(page, ready string) {
		t.Helper()
		b.open(srv.URL + page)
		b.waitFor(page+" ready", ready)
		for _, theme := range []string{"light", "dark"} {
			b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": theme}}})
			got := b.eval(scan)
			for end := time.Now().Add(3 * time.Second); got != "" && time.Now().Before(end); got = b.eval(scan) {
				time.Sleep(100 * time.Millisecond)
			}
			if got != "" {
				t.Fatalf("%s %s: a green, red or blue state color shows, or amber off Take: %v", page, theme, got)
			}
		}
	}
	quiet("/log", `document.querySelectorAll('.sode-log tbody tr, .sode-log__row').length > 0`)
	quiet("/panels", `!!document.getElementById('panels-new')`)
	quiet("/", `document.documentElement.dataset.conn === 'up' && document.getElementById('conn').classList.contains('sode__conn--ok')`)

	b.waitFor("Take is amber with its word, never the red button", `(() => {
		const t = document.getElementById('patch');
		return t.classList.contains('pb-btn--commit') && !t.classList.contains('pb-btn--danger') && /^Take/.test(t.textContent);
	})()`)
	b.waitFor("the Server and Router states have their glyph and spoken words", `[...document.querySelectorAll('#conn .sode__state')].every((d) => d.dataset.state === 'ok' && !!d.querySelector('.sode__state-badge use') && /connected/.test(d.getAttribute('aria-label')))`)

	stopSim()
	b.waitFor("a lost link is red, with its words and a glyph", `(() => {
		`+kit+`
		const box = document.getElementById('conn-down');
		return !box.hidden && rgb(getComputedStyle(box).backgroundColor) === token('--pb-status-error') && box.querySelector('.sode__down-title').textContent.trim() !== '' && !!box.querySelector('.sode__down-word use');
	})()`)
	b.check()
}
