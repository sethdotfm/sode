package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// ctrlShiftL presses Ctrl+Shift+L as a person does.
func ctrlShiftL(b *tab) {
	b.t.Helper()
	realKeyMods(b, "L", "KeyL", 76, modCtrl|modShift)
}

// answerSignOutAsk waits for the Ctrl+Shift+L Confirm with the given verb, waits out the lockout and types its letter.
func answerSignOutAsk(b *tab, verb string) {
	b.t.Helper()
	b.waitFor(verb+" Confirm", `(() => { const ok = document.getElementById('signout-ask-ok'); return !!ok?.offsetParent && ok.textContent.startsWith('`+verb+` (') })()`)
	b.waitFor("lockout over", `!document.getElementById('signout-ask-ok').disabled`)
	letter := b.eval(`document.getElementById('signout-ask-ok').textContent.slice(-2, -1).toLowerCase()`).(string)
	realKey(b, letter, "Key"+strings.ToUpper(letter), int(strings.ToUpper(letter)[0]))
	b.waitFor("Confirm gone", `!document.getElementById('signout-ask-ok')?.offsetParent`)
}

const chipName = `document.querySelector('.sode-lock__who-name')?.textContent`

// "Signed in as" opens a menu from the keyboard and the pointer: Esc gives the focus back, Change PIN opens its form and
// Step in… its pad. Stepped in, the first item is Step out and Step in… is gone. Ctrl+Shift+L asks a Confirm: stepped
// in it steps out to Ana, Esc keeps her, and the letter signs her out. Under the Lock the name has no menu.
func TestChromeChipMenu(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleAdministrator, true
		c.Users = append(c.Users, mkUser(t, "Bo", "567890"))
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	b.open(srv.URL + "/")
	b.waitFor("Sign in on the chip", `!!document.getElementById('lock-chip-signin')?.offsetParent`)
	realClick(b, "#lock-chip-signin")
	b.waitFor("the pad", `document.activeElement?.id === 'lock-name'`)
	typeNameThenPin(b)
	b.waitFor("Signed in as Ana on the name button", chipName+` === 'Signed in as Ana' && !!document.getElementById('lock-chip-name')?.offsetParent && !document.documentElement.hasAttribute('data-sode-lock')`)

	b.shot("signed-in")
	menuOpen := `!document.getElementById('lock-chip-menu').hidden && document.getElementById('lock-chip-name').getAttribute('aria-expanded') === 'true'`
	b.eval(`document.getElementById('lock-chip-name').focus(); true`)
	realKeyMods(b, "Enter", "Enter", 13, 0)
	b.waitFor("menu open on Sign out", menuOpen+` && document.activeElement?.id === 'lock-chip-signout' && document.activeElement.textContent.startsWith('Sign out')`)
	realKey(b, "ArrowDown", "ArrowDown", 40)
	b.waitFor("Step in… next", `document.activeElement?.id === 'lock-chip-stepin' && document.activeElement.textContent === 'Step in…'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("menu closed, focus on the name", `document.getElementById('lock-chip-menu').hidden && document.activeElement?.id === 'lock-chip-name'`)

	realClick(b, "#lock-chip-name")
	b.waitFor("menu open", menuOpen)
	b.shot("menu")
	realClick(b, "#lock-chip-pin")
	b.waitFor("Change my PIN", `!!document.getElementById('sode-pin-form')?.offsetParent && document.getElementById('lock-chip-menu').hidden`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Change my PIN closed", `!document.getElementById('sode-pin-form')?.offsetParent`)

	realClick(b, "#lock-chip-name")
	b.waitFor("menu open", menuOpen)
	realClick(b, "#lock-chip-stepin")
	b.waitFor("Step in pad", `!!document.querySelector('.sode-lock.is-on') && document.activeElement?.id === 'lock-name'`)
	b.padName("Bo")
	padDigits(b, "567890")
	b.waitFor("stepped in as Bo", chipName+` === 'Signed in as Bo' && !document.documentElement.hasAttribute('data-sode-lock')`)
	realClick(b, "#lock-chip-name")
	b.waitFor("Step out first, no Step in…", menuOpen+` && document.getElementById('lock-chip-signout').textContent.startsWith('Step out') && !document.getElementById('lock-chip-stepin').offsetParent`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc on the clicked name closes the menu", `document.getElementById('lock-chip-menu').hidden`)

	ctrlShiftL(b)
	answerSignOutAsk(b, "Step out")
	b.waitFor("Ana again, still signed in", chipName+` === 'Signed in as Ana' && document.getElementById('account-stepout').hidden`)

	ctrlShiftL(b)
	b.waitFor("Sign out Confirm", `!!document.getElementById('signout-ask-ok')?.offsetParent`)
	b.shot("sign-out-confirm")
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc keeps Ana", `!document.getElementById('signout-ask-ok')?.offsetParent && `+chipName+` === 'Signed in as Ana'`)

	// Under the Lock the name is status only.
	realKeyMods(b, "l", "KeyL", 76, modCtrl)
	b.waitFor("the Lock, the name inert", `document.querySelector('.sode-lock__who').classList.contains('is-hand') && document.getElementById('lock-chip-name').inert`)
	padDigits(b, "1234")
	b.waitFor("Lock resumed", `!document.querySelector('.sode-lock__who').classList.contains('is-hand') && !document.documentElement.hasAttribute('data-sode-lock')`)

	ctrlShiftL(b)
	answerSignOutAsk(b, "Sign out")
	b.waitFor("signed out", `import('/lock.js').then((m) => !m.lockState().user && !!document.getElementById('lock-chip-signin')?.offsetParent)`)
	b.check()
}
