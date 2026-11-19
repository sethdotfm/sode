package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func TestChromeApprovalSavesShownSection(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleStandard, true
		withAdministrator(c)
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
	b.waitFor("Confirm and results editable", `(() => {
		const f = document.querySelector('fieldset[data-section="confirm"]');
		return !f.hidden && !f.disabled && !f.querySelector('legend > .pb-badge--warn').hidden && f.querySelector('legend > .pb-badge--warn').dataset.pbTip === 'Save needs Approval from an Administrator';
	})()`)
	b.eval(`const i = document.getElementById('cfg-lockout'); i.value = '900'; i.dispatchEvent(new Event('input', { bubbles: true }));
		document.getElementById('settings-save').click(); true`)
	b.waitFor("review", `!!document.getElementById('review-save')?.offsetParent`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("Approval popup", `!!document.getElementById('approval-pin')?.offsetParent`)
	b.eval(`document.getElementById('approval-name').value = 'Ada'; document.getElementById('approval-pin').value = '1111';
		document.getElementById('approval-go').click(); true`)
	b.waitFor("wrong PIN shown in the popup", `!!document.getElementById('approval-pin')?.offsetParent &&
		document.querySelector('#approval-title').closest('.pb-modal').textContent.includes('Wrong PIN')`)
	b.eval(`document.getElementById('approval-name').value = 'Ada'; document.getElementById('approval-pin').value = '2468';
		document.getElementById('approval-go').click(); true`)
	b.waitFor("saved, popup closed", `!document.getElementById('approval-pin').offsetParent &&
		!document.querySelector('[data-refused="confirmLockoutMs"]') && document.getElementById('settings-path').textContent !== 'Unsaved changes'`)
	if got := a.r().cfg.ConfirmLockoutMs; got != 900 {
		t.Fatalf("confirmLockoutMs = %d, want 900", got)
	}
	b.check()
}
