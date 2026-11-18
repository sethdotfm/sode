package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// Settings search skips a [data-find] button while it is hidden: signed out, it offers Sign in
// and not My API keys; signed in, the reverse. One sign-in shows both entries stay in the index.
func TestChromeSettingsFindSkipsHidden(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	// The search box reads the hit list as it types; offers polls only until Settings has built its index.
	finds := func(query, label string) string {
		return fmt.Sprintf(`(() => { const f = document.getElementById('settings-find'); f.value = %q; f.dispatchEvent(new Event('input')); const has = [...document.querySelectorAll('#settings-found li')].some((li) => li.firstChild.textContent === %q); f.value = ''; f.dispatchEvent(new Event('input')); return has; })()`, query, label)
	}

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	b.waitFor("Sign in shown", `!document.getElementById('account-login').hidden`)
	b.waitFor("search offers Sign in while it is shown", finds("sign in", "Sign in"))
	if b.eval(finds("my api keys", "My API keys")) == true {
		t.Fatal("search offers My API keys while it is hidden")
	}

	b.eval(`document.getElementById('account-login').click()`)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click(); true`)
	b.waitFor("signed in", `!document.getElementById('account-keys').hidden && document.getElementById('account-login').hidden`)
	b.waitFor("search offers My API keys while it is shown", finds("my api keys", "My API keys"))
	if b.eval(finds("sign in", "Sign in")) == true {
		t.Fatal("search offers Sign in while it is hidden")
	}
	b.check()
}
