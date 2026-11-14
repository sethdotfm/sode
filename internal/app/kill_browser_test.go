package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Settings > Access shows Kill only with killEnabled on and the PIN entered on this
// connection; the dialog lists what a Kill does, a wrong PIN is refused there, and the right
// one reaches the exit (stubbed) with exit code 0.
func TestChromeSettingsKill(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.SettingsPin = mkUser(t, "", "2468").Pin })
	x := stubExit(t, a)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	const shown = `!document.getElementById('kill-row').hidden`
	unlock := func() {
		b.waitFor("Settings asks for the PIN", `document.getElementById('settings-view').dataset.locked !== undefined`)
		b.eval(`document.getElementById('lock-pin').value = '2468'; document.getElementById('lock-unlock').click()`)
		b.waitFor("Settings unlocked", `document.getElementById('settings-view').dataset.locked === undefined`)
	}

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	unlock()
	if b.eval(shown) == true {
		t.Fatal("Kill shown with killEnabled off")
	}

	killOn(t, a, true)
	b.waitFor("Kill shown with the setting on and the PIN entered", shown)

	// Reopening Settings drops this connection's unlock.
	b.eval(`document.getElementById('settings-exit').click()`)
	b.waitFor("Settings closed", `document.documentElement.dataset.view !== 'settings'`)
	b.eval(`document.getElementById('settings-open').click()`)
	b.waitFor("Settings open again", `document.documentElement.dataset.view === 'settings'`)
	if b.eval(shown) == true {
		t.Fatal("Kill shown with the PIN locked")
	}
	unlock()
	b.waitFor("Kill shown again", shown)
	b.shot("kill-row")

	b.eval(`document.getElementById('kill-open').click()`)
	b.waitFor("dialog open", `!!document.getElementById('kill-pin')?.offsetParent`)
	text, _ := b.eval(`document.getElementById('kill-warnings').textContent`).(string)
	for _, w := range []string{"router link drops", "not yet sent are lost", "may still land", "already accepted stay", "Lost link banner", "restart policy is no or on-failure"} {
		if !strings.Contains(text, w) {
			t.Errorf("Kill dialog lacks %q: %s", w, text)
		}
	}
	b.shot("kill-dialog")

	b.eval(`document.getElementById('kill-pin').value = '0000'; document.getElementById('kill-go').click()`)
	b.waitFor("wrong PIN refused in the dialog", `/wrong PIN/i.test(document.querySelector('#kill-pin ~ .pb-note--error')?.textContent || '')`)
	if x.n() != 0 {
		t.Fatal("a wrong PIN reached the exit")
	}

	b.eval(`document.getElementById('kill-pin').value = '2468'; document.getElementById('kill-go').click()`)
	for deadline := time.Now().Add(5 * time.Second); x.n() == 0; time.Sleep(20 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the right PIN never reached the exit")
		}
	}
	if x.codes[0] != 0 {
		t.Fatalf("exit code %d, want 0", x.codes[0])
	}
	if e := lastRow(t, a, "kill", 2); e.Result != "ok" { // the first is the wrong PIN's refusal
		t.Errorf("kill row %+v", e)
	}
	b.check()
}
