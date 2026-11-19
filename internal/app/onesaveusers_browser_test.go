package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Manage users has no Save: a new user stays staged after Done and saves with an API change through the one Save.
func TestChromeOneSaveUsersAndAPI(t *testing.T) {
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
	b.waitFor("Manage users shown", `!!document.getElementById('users-manage')?.offsetParent`)
	b.waitFor("API scopes loaded", `!document.querySelector('#api-scopes input').disabled`)

	b.eval(`document.getElementById('users-manage').click(); true`)
	b.waitFor("Users dialog open with Ana", `(() => {
		const d = document.querySelector('.sode__users-dlg');
		return !!d?.offsetParent && [...d.querySelectorAll('input[data-field="name"]')].some((i) => i.value === 'Ana');
	})()`)
	if got := b.eval(`[...document.querySelectorAll('.sode__users-dlg .pb-modal__footer button')].map((x) => x.textContent).join(',')`); got != "Done" {
		t.Fatalf("dialog footer buttons %q, want only Done", got)
	}
	b.eval(`(() => {
		const d = document.querySelector('.sode__users-dlg');
		[...d.querySelectorAll('button')].find((x) => x.textContent === 'Add user').click();
		const rows = d.querySelectorAll('.sode__users-row');
		const row = rows[rows.length - 1];
		const set = (sel, v) => { const f = row.querySelector(sel); f.value = v; f.dispatchEvent(new Event('input', { bubbles: true })); };
		set('input[data-field="name"]', 'Cy');
		set('input[data-field="pin"]', '5678');
		[...d.querySelectorAll('button')].find((x) => x.textContent === 'Done').click();
		return true;
	})()`)
	b.waitFor("dialog closed, change kept", `!document.querySelector('.sode__users-dlg')?.offsetParent && document.getElementById('settings-path').textContent.includes('Unsaved changes')`)

	b.eval(`(() => { const f = document.getElementById('api-rate'); f.value = '77'; f.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review lists the new user and the API change", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const t = d.textContent;
		return t.includes('New user Cy') && t.includes('Rate limit') && t.includes('77') && document.getElementById('review-save').textContent === 'Save 2 changes';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("saved, nothing left unsaved", `!document.getElementById('settings-path').textContent.includes('Unsaved') && document.getElementById('settings-err').hidden`)
	cfg := a.r().cfg
	if cy, ok := cfg.FindUserByName("Cy"); !ok || !cy.HasPin() || cfg.API.RateLimit != 77 {
		t.Fatalf("Cy %+v (found %v), rate limit %d", cy, ok, cfg.API.RateLimit)
	}
	b.check()
}
