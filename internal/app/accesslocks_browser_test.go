package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Guest role in Locks and New user defaults in Users go through the one Save review with their page labels and a
// PIN recheck. A new row in Manage users then starts at the New user defaults.
func TestChromeLocksGuestRoleAndNewUserDefaults(t *testing.T) {
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
	b.waitFor("Locks shown", `!!document.getElementById('cfg-guest-role')?.offsetParent`)
	b.eval(`(() => {
		const g = document.getElementById('cfg-guest-role');
		g.value = 'trusted';
		g.dispatchEvent(new Event('change', { bubbles: true }));
		const p = document.getElementById('cfg-new-persistent');
		p.checked = true;
		p.dispatchEvent(new Event('change', { bubbles: true }));
		document.getElementById('settings-save').click();
		return true;
	})()`)
	b.waitFor("review names both", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const t = d.textContent;
		return t.includes('Locks') && t.includes('Guest role') && t.includes('Trusted user') && t.includes('Users')
			&& t.includes('New user defaults: Never sign out') && document.getElementById('review-save').textContent === 'Save 2 changes';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("saved", `!document.getElementById('review').offsetParent && document.getElementById('cfg-guest-role').value === 'trusted'`)
	// The review closes on settingsSaveResult, which follows the commit.
	if c := a.r().cfg; c.GuestRole != config.RoleTrusted || !c.NewUserDefaults.Persistent || c.NewUserDefaults.NoAutoLock {
		t.Fatalf("Guest role %q, New user defaults %+v", c.GuestRole, c.NewUserDefaults)
	}

	b.eval(`document.getElementById('users-manage').click(); true`)
	b.waitFor("Manage users open", `!!document.querySelector('.sode__users-list')?.offsetParent`)
	b.eval(`[...document.querySelectorAll('button')].find((x) => x.textContent === 'Add user').click(); true`)
	b.waitFor("the new row starts at the defaults", `(() => {
		const rows = document.querySelectorAll('.sode__users-list > *');
		const last = rows[rows.length - 1];
		return !!last && !last.dataset.uid && last.querySelector('input[data-flag="persistent"]').checked;
	})()`)
	b.check()
}
