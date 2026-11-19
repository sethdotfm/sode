package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The two Request approval cards, with a socket peer on the other side. Ana (Trusted) asks Ada for an Administrator
// field: the Waiting for approval card counts down, and Ada's approval saves it. Then Bo (Standard) asks for a
// Trusted field, and Ana approves it on her card with her PIN.
func TestChromeRequestApprovalCards(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleTrusted, true
		c.Users = append(c.Users, mkUser(t, "Bo", "567890"))
		withAdministrator(c)
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	ada := approver(t, a, "Ada", "2468")

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.eval(`const i = document.getElementById('cfg-lockout'); i.value = '900'; i.dispatchEvent(new Event('input', { bubbles: true }));
		document.getElementById('settings-save').click(); true`)
	b.waitFor("review", `!!document.getElementById('review-save')?.offsetParent`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("Request approval offered", `!!document.getElementById('approval-request')?.offsetParent &&
		!document.getElementById('approval-request').disabled && document.getElementById('approval-request-note').hidden`)
	b.eval(`document.getElementById('approval-request').click(); true`)
	b.waitFor("Waiting for approval card with a countdown", `(() => {
		const c = document.getElementById('approval-waiting');
		return !!c?.offsetParent && c.textContent.includes('Waiting for approval') && /\d:\d\d left/.test(c.textContent) &&
			!document.getElementById('approval-pin').offsetParent;
	})()`)

	card := ada.until("approvalCard")
	if r := ada.answer(card["request"], true, "2468"); r["ok"] != true {
		t.Fatalf("approve: %v", r)
	}
	b.waitFor("approved and saved", `document.getElementById('approval-waiting').textContent.includes('Approved by Ada') &&
		!document.querySelector('[data-refused="confirmLockoutMs"]')`)
	if got := a.r().cfg.ConfirmLockoutMs; got != 900 {
		t.Fatalf("confirmLockoutMs = %d, want 900", got)
	}

	bo := dialPeer(t, a)
	bo.signIn("Bo", "567890")
	asked(t, bo.askSave(7, map[string]any{"brandText": "Studio B"}))
	b.waitFor("approver card", `(() => {
		const c = document.querySelector('[data-approval-card]');
		return !!c?.offsetParent && c.textContent.includes('Bo asks') && c.textContent.includes('Studio B') && /\d:\d\d left/.test(c.textContent);
	})()`)
	b.eval(`const c = document.querySelector('[data-approval-card]'); c.querySelector('input').value = '1234';
		c.querySelector('[data-approval-approve]').click(); true`)
	wantSaved(t, bo.until("settingsSaveResult"), "brandText")
	b.waitFor("card says who approved", `document.querySelector('[data-approval-card]').textContent.includes('Approved by Ana')`)
	b.check()
}
