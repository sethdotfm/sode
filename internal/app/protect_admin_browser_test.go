package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// An Administrator without the Protect right clears another's Protect from the route view (protectOffer's admin).
// After it, no shield shows, since they cannot set one.
func TestChromeProtectAdminClearsWithoutTheRight(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Users[0].Rights = config.Rights{Protect: ptr(false)}
		c.ProtectDevice = 126
		c.APIClients = []config.APIClient{{Name: "Desk", Scopes: []string{config.ScopeProtect, config.ScopeProtected},
			Rights: config.Rights{Protect: ptr(true)}}}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	desk := apiTestKey(t, a, keyOwner{Kind: actorClient, Name: "Desk"}, "k", time.Time{})
	if rec := apiSend(a, "POST", fmt.Sprintf("/api/v1/protected/%d", simDst), desk, ""); rec.Code != 200 {
		t.Fatalf("API Protect: %d %s", rec.Code, rec.Body.String())
	}

	b.open(srv.URL + "/")
	b.waitFor("Settings button", `!!document.getElementById('settings-open')`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)
	b.waitFor("DSM1 typed", `(() => { const t = document.getElementById('dst'); t.focus(); t.value = 'DSM1'; t.dispatchEvent(new Event('input', {bubbles: true})); return !!document.getElementById('dst-opt-0'); })()`)
	b.eval(`document.getElementById('dst').dispatchEvent(new KeyboardEvent('keydown', {key: 'Tab', code: 'Tab', bubbles: true, cancelable: true})); true`)
	b.waitFor("Clear Protect offered", `!!document.querySelector('#dst-under [data-protect="clear"]')`)
	b.eval(`document.querySelector('#dst-under [data-protect="clear"]').click(); true`)
	b.waitFor("Confirm clear protect", `!document.getElementById('confirm').hidden && document.getElementById('confirm-title').textContent === 'Confirm clear protect'`)
	b.waitFor("Confirm unlocks", `!document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	b.waitFor("no hold and no shield", `!document.querySelector('#dst-under .pb-badge[data-owner]') && !document.querySelector('#dst-under [data-protect]')`)
	if body := apiDo(a, "GET", "/api/v1/protected", desk, nil).Body.String(); strings.Contains(body, `"sodeUser"`) {
		t.Fatalf("Sode's Protect left: %s", body)
	}
	b.check()
}
