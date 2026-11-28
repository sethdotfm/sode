package app

import (
	"context"
	"fmt"
	"math"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The rest of the Settings form at 1920x1080, signed out and then as an Administrator.
func TestChromeSettingsRest(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	view := func(sel string) {
		b.t.Helper()
		b.eval(`document.querySelector('` + sel + `').scrollIntoView({ block: 'start', behavior: 'instant' }); true`)
	}
	text := func(js string) string {
		b.t.Helper()
		s, _ := b.eval(js).(string)
		return s
	}

	// B2-30: a guest's My user says what a guest can do.
	b.waitFor("Sign in shown", `!document.getElementById('account-login').hidden`)
	view(`[data-section="my-user"]`)
	b.shot("guest")
	if got := text(`(() => { const n = document.getElementById('account-guest-note'); return n && !n.hidden ? n.textContent : ''; })()`); got != "You act as a guest, with Guest Rights." {
		t.Errorf("guest note %q", got)
	}

	// B2-11: the selected hit keeps the text on the others' edge, and "No match" does not move the sidebar.
	search := func(q string) {
		b.t.Helper()
		b.eval(fmt.Sprintf(`(() => { const f = document.getElementById('settings-find'); f.focus(); f.value = %q; f.dispatchEvent(new Event('input')); return true; })()`, q))
	}
	toc := func() float64 {
		return b.eval(`document.getElementById('settings-toc').getBoundingClientRect().top`).(float64)
	}
	view(`#topic-preferences`)
	before := toc()
	search("router")
	b.waitFor("hits listed", `document.querySelectorAll('#settings-found li').length > 2`)
	b.eval(`document.getElementById('settings-find').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowDown', bubbles: true })); true`)
	b.waitFor("second hit selected", `document.querySelector('#settings-found li:nth-child(2)')?.getAttribute('aria-selected') === 'true'`)
	b.shot("search")
	b.sameLeft("search hit names", "#settings-found li > span:first-child")
	b.sameLeft("search hit places", "#settings-found li > .sode__hint")
	search("zzqqxx")
	b.waitFor("No match shown", `!document.getElementById('settings-find-none').hidden`)
	b.shot("search-none")
	if after := toc(); math.Abs(after-before) > layoutTol {
		b.layoutFail(fmt.Sprintf("No match moves the sidebar from %.2f to %.2f", before, after))
	}
	b.sameLeft("No match on the search box edge", "#settings-find", "#settings-find-none")
	search("")
	b.eval(`document.getElementById('settings-find').blur(); true`)

	signInSettings(b)
	b.waitFor("Users shown", `!!document.getElementById('users-manage')?.offsetParent`)

	// C1-20: the foot link has the Communication Log's name.
	if got := text(`document.getElementById('open-log')?.textContent.trim() || ''`); got != "Communication Log" {
		t.Errorf("foot link %q, want Communication Log", got)
	}
	if got := text(`document.getElementById('open-log')?.dataset.label || ''`); got != "Communication Log" {
		t.Errorf("foot link search label %q", got)
	}

	// B2-14: the custom picker, the border and the app icon are one size on one line, with a hint that names them.
	b.eval(`(() => { const s = document.getElementById('cfg-brandborder'); s.value = 'custom'; s.dispatchEvent(new Event('change', { bubbles: true })); return true; })()`)
	b.waitFor("custom picker shown", `!document.getElementById('cfg-brandcustom-wrap').hidden`)
	view(`[data-section="branding"]`)
	b.shot("branding")
	b.sameHeight("theme color preview", "#cfg-brandcustom-wrap", "#brand-swatch", "#brand-icon")
	b.sameMiddle("theme color preview", "#cfg-brandcustom-wrap", "#brand-swatch", "#brand-icon")
	if got := text(`document.querySelector('#brand-preview + .sode__hint')?.textContent || ''`); got != "Page border and app icon" {
		t.Errorf("theme color hint %q", got)
	}

	// B2-22: key facts are hints under their control, not tips.
	for id, want := range map[string]string{
		"cfg-lockout":     "Delay before the Confirm letter and OK work.",
		"cfg-dismiss":     "0 keeps it open. A failure banner always stays.",
		"cfg-auto-lock":   "0 is off.",
		"api-rate":        "0 is off.",
		"cfg-locks-label": "Needs a user with a PIN.",
	} {
		got := text(`(() => {
			const c = document.getElementById('` + id + `');
			const f = c.closest('.sode__field');
			const tip = f.querySelector('[data-pb-tip]');
			const h = f.querySelector(':scope > .sode__hint');
			return (tip ? 'tip: ' + tip.dataset.pbTip + '; ' : '') + (h ? h.textContent : '');
		})()`)
		if got != want {
			t.Errorf("%s: %q, want the hint %q", id, got, want)
		}
	}

	// B2-30: Settings labels in sentence case, with the words the glossary uses.
	for id, want := range map[string]string{
		"cfg-new-panel-acts":  "Authentication mode for new panels",
		"cfg-search-filters":  "Search in History uses filters",
		"gcfg-search-filters": "Search in History uses filters",
		"cfg-flip":            "Destination before source",
		"gcfg-flip":           "Destination before source",
		"cfg-dismiss":         "Close a success banner after (s)",
	} {
		if got := text(`document.querySelector('label[for="` + id + `"]').textContent.trim()`); got != want {
			t.Errorf("label of %s %q, want %q", id, got, want)
		}
	}
	if got := text(`document.querySelector('#cfg-require-https').closest('.sode__field').querySelector('.sode__hint').textContent`); got != "Sode refuses a request that a trusted proxy did not mark X-Forwarded-Proto: https. The health endpoints stay open. Add a trusted proxy first, and save from an HTTPS address. To turn it off from the console: sode -reset-https." {
		t.Errorf("Require HTTPS hint %q", got)
	}

	// B2-23: a bad level color says so under the grid at once.
	view(`[data-section="level-colors"]`)
	if got := text(`[...document.querySelectorAll('[data-section="level-colors"] > .sode__hint')].map((h) => h.textContent).join(' | ')`); got == "" || !strings.Contains(got, "Grey text is the default color.") {
		t.Errorf("level colors hints %q", got)
	}
	b.eval(`(() => { const f = document.getElementById('cfg-color-1'); f.value = '#zzz'; f.dispatchEvent(new Event('input')); return true; })()`)
	b.shot("level-colors")
	if got := text(`(() => { const e = document.getElementById('cfg-colors-err'); return e && !e.hidden ? e.textContent : ''; })()`); got != "Position 1 color must be hex, like #3a7bd5." {
		t.Errorf("level color error %q", got)
	}
	b.eval(`(() => { const f = document.getElementById('cfg-color-1'); f.value = ''; f.dispatchEvent(new Event('input')); return true; })()`)
	if b.eval(`document.getElementById('cfg-colors-err')?.hidden !== false`) != true {
		t.Error("level color error stays after the color is cleared")
	}

	// B2-24: the my-default buttons are outline buttons with short words, and Clear mine shows once a default is mine.
	view(`[data-section="this-browser"]`)
	b.waitFor("my-default buttons", `!!document.getElementById('my-default-theme')?.offsetParent`)
	if got := text(`(() => { const s = document.getElementById('my-default-theme'); return [s.textContent, s.classList.contains('pb-btn--outline'), s.classList.contains('pb-btn--sm'), s.dataset.pbTip].join('|'); })()`); got != "Save for me|true|false|Save as my default in every browser" {
		t.Errorf("Save button %q", got)
	}
	b.eval(`document.getElementById('my-default-theme').click(); true`)
	b.waitFor("Clear mine shown", `!!document.getElementById('my-default-clear-theme')?.offsetParent`)
	b.shot("this-browser")
	if got := text(`(() => { const s = document.getElementById('my-default-clear-theme'); return [s.textContent, s.classList.contains('pb-btn--outline')].join('|'); })()`); got != "Clear mine|true" {
		t.Errorf("Clear button %q", got)
	}

	// B2-25: the Clear History confirm buttons sit in one row with the row gap.
	view(`[data-section="history"]`)
	b.eval(`document.getElementById('clear-history').click(); true`)
	b.waitFor("confirm step", `!document.getElementById('clear-step2').hidden`)
	b.shot("clear-history")
	b.gapX("Clear History confirm", "--pb-space-5", "#clear-step2 > .pb-btn")
	b.sameMiddle("Clear History confirm", "#clear-step2 > .pb-btn")
	b.eval(`document.getElementById('clear-cancel').click(); true`)

	// B2-01: an Access preset badge sits in its label: the Locks grid keeps two columns and the check group its checks.
	b.eval(`document.getElementById('cfg-apply-preset').click(); true`)
	b.waitFor("preset dialog", `!!document.querySelector('[aria-labelledby="preset-apply-title"]')?.offsetParent`)
	b.eval(`(() => {
		const d = document.querySelector('[aria-labelledby="preset-apply-title"]');
		d.querySelector('label[for="preset-apply-pick-secured"]').click();
		[...d.querySelectorAll('button')].find((x) => x.textContent === 'Stage changes').click();
		return true;
	})()`)
	b.waitFor("staged and marked", `document.querySelectorAll('[data-section="locks"] .sode__preset-mark').length > 0`)
	view(`[data-section="locks"]`)
	b.shot("locks-preset")
	if got := text(`(() => {
		const out = [];
		for (const m of document.querySelectorAll('.sode__preset-mark')) {
			const p = m.parentElement;
			if (p.matches('.sode__fields, [role="group"]')) out.push(m.textContent + ' in ' + (p.className || p.getAttribute('role')));
		}
		const g = document.querySelector('[data-section="locks"] .sode__fields');
		// auto-fit gives an empty track 0px.
		const cols = getComputedStyle(g).gridTemplateColumns.split(' ').filter((w) => parseFloat(w) > 0).length;
		if (cols !== 2) out.push('Locks grid has ' + cols + ' columns');
		return out.join('; ');
	})()`); got != "" {
		t.Errorf("preset badges: %s", got)
	}
	b.gapX("Locks grid columns", "--pb-space-5", `[data-section="locks"] .sode__fields > .sode__field`)
	b.sameMiddle("Main page badge on its check", "#cfg-lock-main + .pb-check__label", "#cfg-lock-main + .pb-check__label .sode__preset-mark")

	// The badge is not part of the setting's name: the Save review's Setting column and the search index leave it out.
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("the review", `!!document.getElementById('review')?.offsetParent`)
	if got := text(`[...document.querySelectorAll('#review tbody tr')].map((r) => r.cells[0]).filter((c) => c && c.tagName === 'TD')
		.map((c) => c.textContent).filter((t) => t.includes('Secured access')).join('|')`); got != "" {
		t.Errorf("review Setting cells hold the badge text: %q", got)
	}
	if got := text(`[...document.querySelectorAll('#review tbody tr')].map((r) => r.cells[0]).find((c) => c?.tagName === 'TD' && c.textContent.startsWith('Main page'))?.textContent || ''`); got != "Main page" {
		t.Errorf("review row for the Main page lock: %q, want %q", got, "Main page")
	}
	b.eval(`[...document.querySelectorAll('#review button')].find((x) => x.textContent === 'Keep editing').click(); true`)
	b.check()
}

// B2-28: env-only, My user says how to add a user, and offers no Lock that does nothing.
func TestChromeSettingsEnvOnlyMyUser(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	sim := startSim(t)
	cfg := config.Defaults()
	cfg.Routers[0].Address = sim
	cfg.Normalize()
	hist, _ := OpenHistory("")
	sl, err := OpenSentLog(filepath.Join(t.TempDir(), "sent.jsonl"), 200_000)
	if err != nil {
		t.Fatal(err)
	}
	a, err := New(cfg, "", hist, sl, false)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	b.eval(`document.querySelector('[data-section="my-user"]').scrollIntoView({ block: 'start', behavior: 'instant' }); true`)
	b.waitFor("no users line", `(() => { const n = document.getElementById('account-name'); return !n.hidden && n.textContent === 'No users yet. Add one in Access > Users.'; })()`)
	b.shot("envonly-my-user")
	if b.eval(`!!document.getElementById('lock-btn').offsetParent`) == true {
		t.Error("Lock shown with no users")
	}
	b.check()
}
