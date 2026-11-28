package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Run setup again opens the wizard with the current values and no Add a first user. Finish stages only
// what changed into the one Save, which needs one recheck and deletes nothing.
func TestChromeRunSetupAgain(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Users = append(c.Users, mkUser(t, "Bo", "5678"))
		c.Users[1].Persistent = true
		c.BrandText = "Studio B"
		c.Routers[0].LevelNumbering = "va"
		standard, _ := config.AccessPresetByName("standard")
		standard.Apply(c)
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	next := `document.getElementById('setup-next')`
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Run setup again ready", `(() => { const x = document.getElementById('setup-again-open'); return !!x?.offsetParent && !x.disabled; })()`)
	b.eval(`document.getElementById('setup-again-open').click(); true`)

	b.waitFor("the wizard over Settings", `!document.getElementById('setup').hidden && document.documentElement.hasAttribute('data-sode-setup')`)
	b.waitFor("Welcome with the current name and no Add a first user", `document.getElementById('setup-brandText')?.value === 'Studio B' &&
		!document.getElementById('setup-name') && document.getElementById('setup-restore').hidden && !`+next+`.disabled &&
		document.querySelectorAll('#setup-body .sode-setup__section').length === 1 && !document.getElementById('setup-body').textContent.includes('null')`)
	b.eval(`(() => { const i = document.getElementById('setup-brandText'); i.value = 'Studio C'; i.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	b.eval(next + `.click(); true`)
	b.waitFor("Router with the current values", `document.getElementById('setup-router')?.value === '`+sim+`' && document.getElementById('setup-levelNumbering-va').checked`)
	b.waitFor("Connection, with no first router", `[...document.querySelectorAll('#setup-body .sode-setup__h')].some((h) => h.textContent === 'Connection')`)
	b.eval(next + `.click(); true`)
	b.waitFor("Access with the matching preset picked", `document.querySelector('.sode-setup__sheet thead th.is-picked')?.dataset.preset === 'standard' && `+next+`.textContent === 'Finish'`)
	b.eval(`document.querySelector('.sode-setup__sheet thead th[data-preset="secured"] input').click(); true`)
	b.waitFor("Secured picked", `document.querySelector('.sode-setup__sheet thead th.is-picked')?.dataset.preset === 'secured'`)
	b.eval(next + `.click(); true`)

	rows := `[...document.querySelectorAll('#review tr')].filter((r) => r.querySelector('.sode__review-was'))`
	b.waitFor("Finish opens the review over Settings", `document.getElementById('setup').hidden && !document.documentElement.hasAttribute('data-sode-setup') &&
		!!document.getElementById('review')?.offsetParent`)
	b.waitFor("only the changed fields: the name and the preset's", `(() => {
		const rs = `+rows+`;
		const plain = rs.filter((r) => !r.textContent.includes('Secured access'));
		return rs.length >= 4 && plain.length === 1 && plain[0].textContent.includes('Studio C') && !rs.some((r) => r.textContent.includes('`+sim+`'));
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("saved after one recheck", `!document.getElementById('settings-path').textContent.includes('Unsaved') && document.getElementById('settings-err').hidden`)

	cfg := a.r().cfg
	bo, _ := cfg.FindUserByName("Bo")
	if cfg.BrandText != "Studio C" || !cfg.Locks.Main || cfg.PublicPanels || cfg.Routers[0].Address != sim || cfg.Routers[0].LevelNumbering != "va" {
		t.Fatalf("after the save: brand %q, locks %+v, public panels %v, router %q, numbering %q", cfg.BrandText, cfg.Locks, cfg.PublicPanels, cfg.Routers[0].Address, cfg.Routers[0].LevelNumbering)
	}
	if len(cfg.Users) != 2 || !bo.Persistent {
		t.Fatalf("users %d, Bo persistent %v", len(cfg.Users), bo.Persistent)
	}
	b.check()
}

// A hello while Run setup again is open (here after a reconnect) keeps the name and Theme color of the answers.
// Cancel puts back the saved ones.
func TestChromeRunSetupAgainKeepsBrand(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.BrandText = "Studio B"
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Run setup again ready", `(() => { const x = document.getElementById('setup-again-open'); return !!x?.offsetParent && !x.disabled; })()`)
	b.eval(`document.getElementById('setup-again-open').click(); true`)
	b.waitFor("Welcome with the current name", `document.getElementById('setup-brandText')?.value === 'Studio B'`)
	b.eval(`document.querySelector('input[name="brandBorder"][value="teal"]').click(); true`)
	b.eval(`(() => { const i = document.getElementById('setup-brandText'); i.value = 'Studio C'; i.dispatchEvent(new Event('input', {bubbles: true})); return true; })()`)
	title := `document.querySelector('#home-bar .sode-panels__title').textContent.trim()`
	frame := `document.querySelector('.sode__brandframe').dataset.brand`
	b.waitFor("the answers show in the bar", title+` === 'Studio C' && `+frame+` === 'teal'`)
	b.eval(`import('./core.js').then((m) => { window.hellos = 0; m.on('hello', () => window.hellos++); m.S.ws?.close(); return true; })`)
	b.waitFor("the hello after the reconnect", `window.hellos > 0`)
	if got := b.eval(title + ` + ' ' + ` + frame); got != "Studio C teal" {
		t.Fatalf("after the hello the bar shows %q, want the answers", got)
	}
	b.waitFor("the wizard stays open", `!document.getElementById('setup').hidden`)
	b.eval(`document.getElementById('setup-back').click(); true`)
	b.waitFor("Cancel puts back the saved name and color", `document.getElementById('setup').hidden && `+title+` === 'Studio B' && document.querySelector('.sode__brandframe').hidden`)
}
