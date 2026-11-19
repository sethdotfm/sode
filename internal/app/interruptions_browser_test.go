package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The review lists the Interruptions the server sends, says they do not come back, and the Save button names them.
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
