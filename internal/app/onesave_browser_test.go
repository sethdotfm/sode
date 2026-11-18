package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The one Save review. A router address change asks the PIN recheck, then Save and reconnect, and Back sends
// nothing. A partly refused save writes the rest and marks each refused field with its reason; a conflict shows the
// other person's value. The exit warning names each section with changes.
func TestChromeOneSaveReview(t *testing.T) {
	b := startBrowser(t)
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
	signInSettings(b)
	b.waitFor("Locks shown", `!!document.getElementById('cfg-lock-main')?.offsetParent`)

	set := func(id, js string) {
		b.t.Helper()
		b.eval(`(() => { const f = document.getElementById('` + id + `'); ` + js + `; f.dispatchEvent(new Event('input', { bubbles: true })); f.dispatchEvent(new Event('change', { bubbles: true })); return true; })()`)
	}
	set("cfg-brandtext", `f.value = 'Truck 4'`)
	set("cfg-router", `f.value = '127.0.0.1:1'`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review lists both changes by section", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const t = d.textContent;
		return t.includes('Look and feel') && t.includes('Branding') && t.includes('Truck 4') && t.includes('Router & matrix')
			&& t.includes('127.0.0.1:1') && t.includes('Reconnects the router')
			&& document.getElementById('review-save').textContent === 'Save 2 changes';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("Save and reconnect asked", `document.getElementById('review')?.offsetParent && document.getElementById('review-save').textContent === 'Save and reconnect' && document.getElementById('review').textContent.includes('127.0.0.1:1')`)
	b.eval(`document.getElementById('review-back').click(); true`)
	b.waitFor("back on the review", `document.getElementById('review-back').textContent === 'Keep editing'`)
	b.eval(`document.getElementById('review-back').click(); true`)
	b.waitFor("review closed", `!document.getElementById('review').offsetParent`)
	if cfg := a.r().cfg; cfg.BrandText == "Truck 4" || cfg.Address == "127.0.0.1:1" {
		t.Fatalf("Back sent the save: brand %q, router %q", cfg.BrandText, cfg.Address)
	}

	// Someone else saves the confirm lockout after this page opened Settings.
	set("cfg-router", `f.value = '`+sim+`'`)
	a.cfgMu.Lock()
	err := a.applyConfig(func(c *config.Config) error { c.ConfirmLockoutMs = 700; return nil })
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	set("cfg-lockout", `f.value = '900'`)
	// Over plain HTTP, Require HTTPS is refused as a missing prerequisite.
	set("cfg-require-https", `f.checked = true`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review of three", `document.getElementById('review')?.offsetParent && document.getElementById('review-save').textContent === 'Save 3 changes'`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("refused fields marked", `(() => {
		const m = (k) => document.querySelector('.sode__refused[data-refused="' + k + '"]')?.textContent || '';
		return m('confirmLockoutMs').includes('Someone else saved "700"') && m('requireHttps').startsWith('Not saved.')
			&& !m('brandText') && !document.getElementById('settings-err').hidden;
	})()`)
	if got := a.r().cfg; got.BrandText != "Truck 4" || got.ConfirmLockoutMs != 700 || got.RequireHTTPS {
		t.Fatalf("after the partial save: brand %q, lockout %d, requireHttps %v", got.BrandText, got.ConfirmLockoutMs, got.RequireHTTPS)
	}

	// A second Save shows their value as Was, then writes this page's value.
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review Was is their value", `(() => {
		const row = [...document.querySelectorAll('#review tbody tr')].find((r) => r.cells[0]?.textContent.startsWith('Confirm lockout'));
		return row?.cells[1].textContent === '700' && row.cells[2].textContent === '900';
	})()`)
	b.eval(`document.getElementById('review-back').click(); true`)
	b.waitFor("review closed", `!document.getElementById('review').offsetParent`)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("exit warning names the sections", `(() => {
		const n = document.getElementById('settings-unsaved');
		return !n.hidden && n.textContent.includes('Confirm and results') && n.textContent.includes('Proxy') && !n.textContent.includes('Branding');
	})()`)
	if txt, _ := b.eval(`document.getElementById('settings-unsaved').textContent`).(string); !strings.HasPrefix(txt, "Unsaved changes in ") {
		t.Fatalf("exit warning: %q", txt)
	}
	b.check()
}
