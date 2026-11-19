package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func TestChromeReviewNamesInterruptions(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	makePanel(t, a, userKey(t, a, "Ana"), publicPanel+`}`)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Panels shown", `!!document.getElementById('cfg-public-panels')?.offsetParent`)
	b.eval(`(() => {
		const f = document.getElementById('cfg-public-panels');
		f.checked = false;
		f.dispatchEvent(new Event('change', { bubbles: true }));
		document.getElementById('settings-save').click();
		return true;
	})()`)
	b.waitFor("review lists the Interruptions", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const t = d.textContent;
		return t.includes('Interruptions') && t.includes('Public panel Lobby stops') && t.includes('do not come back, even after a restore')
			&& document.getElementById('review-save').textContent === 'Save and stop 1 panel';
	})()`)
	b.eval(`document.getElementById('review-back').click(); true`)
	b.waitFor("review closed", `!document.getElementById('review').offsetParent`)
	if !a.r().cfg.PublicPanels {
		t.Fatal("the review saved")
	}
	b.check()
}

func TestChromeReviewBackupOffer(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	makePanel(t, a, userKey(t, a, "Ana"), publicPanel+`}`)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Panels shown", `!!document.getElementById('cfg-public-panels')?.offsetParent`)
	b.eval(`(() => {
		window.sodeExports = [];
		const real = window.fetch;
		window.fetch = (url, init) => {
			if (String(url).endsWith('api/export')) window.sodeExports.push(init.body);
			return real(url, init);
		};
		const f = document.getElementById('cfg-public-panels');
		f.checked = false;
		f.dispatchEvent(new Event('change', { bubbles: true }));
		document.getElementById('settings-save').click();
		return true;
	})()`)
	b.waitFor("review offers a backup", `(() => {
		const d = document.getElementById('review');
		return !!d?.offsetParent && !!document.getElementById('review-backup')
			&& d.textContent.includes('Last backup: never in this browser');
	})()`)
	b.eval(`document.getElementById('review-backup').click(); true`)
	b.waitFor("backup saved", `document.getElementById('review-backup-note').textContent.startsWith('Saved ')
		&& document.getElementById('review').textContent.includes('Last backup: less than a minute ago')`)
	b.eval(`document.getElementById('review-export').click(); true`)
	b.waitFor("Export & Import open", `!document.getElementById('review').offsetParent && !!document.getElementById('xi-backup')?.offsetParent`)
	b.eval(`document.getElementById('xi-backup').click(); true`)
	b.waitFor("Back up everything sent", `window.sodeExports.length === 2`)
	if got := b.eval(`window.sodeExports[0] === window.sodeExports[1]`); got != true {
		t.Errorf("Download a backup and Back up everything differ: %v", b.eval(`JSON.stringify(window.sodeExports)`))
	}
	if !a.r().cfg.PublicPanels {
		t.Fatal("the review saved")
	}
	b.check()
}
