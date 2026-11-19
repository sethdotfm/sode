package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The server refuses Clear History without a PIN recheck, so the page asks for the PIN.
func TestChromeClearHistoryAsksRecheck(t *testing.T) {
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
	b.eval(`document.getElementById('clear-history').click(); document.getElementById('clear-confirm').click(); true`)
	b.waitFor("PIN recheck asked", `!!document.getElementById('recheck-pin')?.offsetParent`)
	b.eval(`document.getElementById('recheck-pin').value = '0000'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("wrong PIN shown", `!!document.querySelector('#recheck-pin ~ .pb-note--error:not([hidden])')`)
	b.eval(`document.getElementById('recheck-pin').value = '1234'; document.getElementById('recheck-go').click(); true`)
	b.waitFor("History cleared", `document.body.textContent.includes('History cleared')`)
	if b.eval(`!!document.getElementById('recheck-pin').offsetParent`) == true {
		t.Fatal("the PIN recheck stayed open")
	}
	b.check()
}
