package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// One sign-in: the name box finds a user by fuzzy match or from the list of every user, then
// takes that user's PIN. Ana and Dana share a PIN, so only the name says who signs in. The
// browser remembers the last name.
func TestChromeSignInByNameThenPin(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users = append(c.Users, mkUser(t, "Bo", "567890"), mkUser(t, "Dana", "1234"))
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	listed := `[...document.querySelectorAll('#lock-name-list [role="option"]')].map((li) => li.textContent).join(',')`
	typeName := func(v string) {
		b.eval(fmt.Sprintf(`(() => { const i = document.getElementById('lock-name'); i.value = %q; i.dispatchEvent(new Event('input', { bubbles: true })); })()`, v))
	}
	key := func(k string) {
		b.eval(fmt.Sprintf(`document.activeElement.dispatchEvent(new KeyboardEvent('keydown', { key: %q, bubbles: true, cancelable: true }))`, k))
	}
	signedInAs := `import('/lock.js').then((m) => m.lockState().user)`
	signIn := func() {
		b.eval(`document.getElementById('account-login').click()`)
		b.waitFor("the name box", `!!document.getElementById('lock-name')?.offsetParent`)
	}

	b.open(srv.URL + "/#settings")
	b.waitFor("Sign in shown", `!document.getElementById('account-login').hidden`)
	signIn()
	b.waitFor("the empty box has focus and lists every user", `document.activeElement?.id === 'lock-name' && `+listed+` === 'Ana,Bo,Dana'`)

	// Fuzzy find, Enter picks, and the pad takes the PIN at Dana's own length.
	typeName("da")
	if got := b.eval(listed); got != "Dana" {
		t.Fatalf("typed da, the list is %v", got)
	}
	key("Enter")
	b.waitFor("Dana picked and the list closed", `document.getElementById('lock-name').value === 'Dana' && document.getElementById('lock-name-list').hidden && document.activeElement?.id !== 'lock-name'`)
	for _, d := range "1234" {
		key(string(d))
	}
	b.waitFor("signed in as Dana, not Ana", signedInAs+`.then((u) => u === 'Dana')`)

	// The name is remembered: the next sign-in starts with it, and the pad has the keys.
	b.eval(`import('/core.js').then((m) => m.emit('logout'))`)
	b.waitFor("signed out", `!document.getElementById('account-login').hidden`)
	signIn()
	b.waitFor("the remembered name", `document.getElementById('lock-name').value === 'Dana' && document.activeElement?.id !== 'lock-name'`)

	// The list by keys: an empty box, ArrowDown to the second user, Enter.
	b.eval(`document.getElementById('lock-name').focus()`)
	typeName("")
	key("ArrowDown")
	key("Enter")
	b.waitFor("Bo picked", `document.getElementById('lock-name').value === 'Bo'`)
	for _, d := range "567890" {
		key(string(d))
	}
	b.waitFor("signed in as Bo", signedInAs+`.then((u) => u === 'Bo')`)
	b.check()
}
