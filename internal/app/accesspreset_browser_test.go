package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func TestChromeApplyAccessPreset(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleAdministrator, true
		bo := mkUser(t, "Bo", "5678")
		bo.Persistent = true
		c.Users = append(c.Users, bo)
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
	b.waitFor("Apply shown", `(() => { const x = document.getElementById('cfg-apply-preset'); return !!x?.offsetParent && !x.disabled; })()`)
	b.waitFor("API scopes loaded", `!document.querySelector('#api-scopes input').disabled`)

	b.eval(`document.getElementById('cfg-apply-preset').click(); true`)
	b.waitFor("dialog lists users, unticked, Ana marked", `(() => {
		const d = document.querySelector('[aria-labelledby="preset-apply-title"]');
		if (!d?.offsetParent) return false;
		const rows = [...d.querySelectorAll('.sode__preset-users .pb-check')];
		return rows.length === 2 && rows.every((r) => !r.querySelector('input').checked) && rows[0].textContent.includes('You');
	})()`)
	b.eval(`(() => {
		const d = document.querySelector('[aria-labelledby="preset-apply-title"]');
		d.querySelector('label[for="preset-apply-pick-secured"]').click();
		d.querySelector('.sode__preset-users input').click();
		[...d.querySelectorAll('button')].find((x) => x.textContent === 'Stage changes').click();
		return true;
	})()`)
	b.waitFor("staged and marked", `document.getElementById('cfg-lock-main').checked && !document.getElementById('cfg-public-panels').checked &&
		[...document.querySelectorAll('.sode__preset-mark')].some((m) => m.textContent === 'Secured access') &&
		document.getElementById('settings-path').textContent.includes('Unsaved changes')`)

	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review marks rows with the preset", `(() => {
		const d = document.getElementById('review');
		return !!d?.offsetParent && [...d.querySelectorAll('.pb-badge')].filter((x) => x.textContent === 'Secured access').length >= 5;
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("saved", `!document.getElementById('settings-path').textContent.includes('Unsaved') && document.getElementById('settings-err').hidden`)

	cfg := a.r().cfg
	ana, _ := cfg.FindUserByName("Ana")
	bo, _ := cfg.FindUserByName("Bo")
	if !cfg.Locks.Main || cfg.PublicPanels || cfg.API.GuestEnabled || ana.Persistent || !bo.Persistent {
		t.Fatalf("locks %+v, public panels %v, keyless %v, Ana persistent %v, Bo persistent %v", cfg.Locks, cfg.PublicPanels, cfg.API.GuestEnabled, ana.Persistent, bo.Persistent)
	}
	rows := waitSaves(t, a, 1)
	if got := rows[len(rows)-1].Save.Preset; got != "Secured access" {
		t.Fatalf("Settings saves row preset %q", got)
	}
	b.check()
}
