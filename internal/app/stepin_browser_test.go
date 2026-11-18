package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// sessionKeysJS reads the five session keys a step-in must not write.
const sessionKeysJS = `JSON.stringify([
	...['sode-session', 'sode-last-user', 'sode-profile-prefs'].map((k) => [k, localStorage.getItem(k)]),
	...['sode-tab-session', 'sode-held-user'].map((k) => [k, sessionStorage.getItem(k)]),
])`

// A step-in acts as Bo in this tab and writes none of the session keys. A reload is Ana again; so is Step out.
func TestChromeStepInWritesNoSessionKeys(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleAdministrator, true
		bo := mkUser(t, "Bo", "567890")
		bo.Profile.Prefs = map[string]string{"compact": "on"} // Bo's profile reaches the page, so a cache write would show
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
	b.waitFor("Ana's token kept", `!!localStorage.getItem('sode-session')`)
	before := b.eval(sessionKeysJS)

	b.waitFor("Step in shown", `!document.getElementById('account-stepin').hidden`)
	b.eval(`document.getElementById('account-stepin').click(); true`)
	b.waitFor("Step in popup", `!!document.getElementById('stepin-pin')?.offsetParent`)
	b.eval(`document.getElementById('stepin-name').value = 'Bo'; document.getElementById('stepin-pin').value = '567890';
		document.getElementById('stepin-go').click(); true`)
	b.waitFor("stepped in as Bo", `!document.getElementById('account-stepout').hidden &&
		document.getElementById('account-name').textContent.includes('Bo')`)
	if got := b.eval(sessionKeysJS); got != before {
		t.Fatalf("the step-in wrote session keys:\nbefore %v\nafter  %v", before, got)
	}

	b.eval(`location.reload(); true`)
	b.waitFor("Ana after a reload", `document.getElementById('account-name')?.textContent.includes('Ana') &&
		document.getElementById('account-stepout').hidden`)
	if got := b.eval(sessionKeysJS); got != before {
		t.Fatalf("session keys after the reload:\nbefore %v\nafter  %v", before, got)
	}

	b.waitFor("Step in shown", `!document.getElementById('account-stepin').hidden`)
	b.eval(`document.getElementById('account-stepin').click(); true`)
	b.waitFor("Step in popup", `!!document.getElementById('stepin-pin')?.offsetParent`)
	b.eval(`document.getElementById('stepin-name').value = 'Bo'; document.getElementById('stepin-pin').value = '567890';
		document.getElementById('stepin-go').click(); true`)
	b.waitFor("stepped in again", `!document.getElementById('account-stepout').hidden`)
	b.eval(`document.getElementById('account-stepout').click(); true`)
	b.waitFor("Ana after Step out", `document.getElementById('account-name').textContent.includes('Signed in as Ana') &&
		document.getElementById('account-stepout').hidden`)
	if got := b.eval(sessionKeysJS); got != before {
		t.Fatalf("session keys after Step out:\nbefore %v\nafter  %v", before, got)
	}
	b.check()
}
