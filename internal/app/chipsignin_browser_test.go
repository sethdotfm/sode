package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// realClick clicks the middle of the element that sel finds, through the browser's input pipeline, so the focus moves
// as for a person.
func realClick(b *tab, sel string) {
	b.t.Helper()
	r := b.eval(fmt.Sprintf(`(() => { const r = document.querySelector(%q).getBoundingClientRect(); return [r.left + r.width / 2, r.top + r.height / 2] })()`, sel)).([]any)
	for _, ty := range []string{"mouseMoved", "mousePressed", "mouseReleased"} {
		b.call("Input.dispatchMouseEvent", map[string]any{"type": ty, "x": r[0], "y": r[1], "button": "left", "clickCount": 1})
	}
}

// typeNameThenPin types Ana's name where the focus is, as a person would, picks it with Enter and types her PIN.
func typeNameThenPin(b *tab) {
	b.t.Helper()
	b.call("Input.insertText", map[string]any{"text": "Ana"})
	b.waitFor("the name box holds the typed name", `document.getElementById('lock-name').value === 'Ana'`)
	realKey(b, "Enter", "Enter", 13)
	for _, d := range "1234" {
		realKey(b, string(d), "Digit"+string(d), int(d))
	}
	b.waitFor("signed in as Ana", `import('/lock.js').then((m) => m.lockState().user === 'Ana')`)
}

func startSignInPage(t *testing.T) *tab {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	b.open(srv.URL + "/")
	return b
}

// A guest on Home sees Sign in beside the lock button. It opens the same pad as Settings > My user > Sign in, with the
// focus in the Name box. After the sign-in the button goes and the chip says who is signed in.
func TestChromeChipSignInForGuest(t *testing.T) {
	b := startSignInPage(t)
	b.waitFor("Sign in beside the lock button", `(() => {
		const s = document.getElementById('lock-chip-signin'), l = document.querySelector('.sode-lock__who-slot > .pb-btn');
		if (!s?.offsetParent || s.textContent !== 'Sign in') return false;
		const a = s.getBoundingClientRect(), c = l.getBoundingClientRect();
		return a.left >= c.right && Math.abs((a.top + a.bottom) / 2 - (c.top + c.bottom) / 2) < 2;
	})()`)
	realClick(b, "#lock-chip-signin")
	b.waitFor("the pad is up with the focus in the Name box", `!!document.getElementById('lock-name')?.offsetParent && document.activeElement?.id === 'lock-name'`)
	typeNameThenPin(b)
	b.waitFor("Signed in as, and no Sign in", `document.querySelector('.sode-lock__who-name').textContent === 'Signed in as Ana' && !document.getElementById('lock-chip-signin').offsetParent`)
	b.check()
}

// The Lock with nobody signed in, then Sign in as…: the focus goes to the Name box, so the typed name is not lost.
func TestChromeSignInAsFromLockFocusesName(t *testing.T) {
	b := startSignInPage(t)
	b.waitFor("Home is open", `!!document.getElementById('lock-chip-signin')?.offsetParent`)
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyDown", "key": "l", "code": "KeyL", "windowsVirtualKeyCode": 76, "modifiers": 2})
	b.call("Input.dispatchKeyEvent", map[string]any{"type": "keyUp", "key": "l", "code": "KeyL", "windowsVirtualKeyCode": 76, "modifiers": 2})
	signInAs := `[...document.querySelectorAll('.sode-lock__foot .pb-btn')].find((x) => x.textContent === 'Sign in as…')`
	b.waitFor("the Lock with Sign in as…", `!!document.querySelector('.sode-lock.is-on') && !!(`+signInAs+`)?.offsetParent`)
	b.eval(`(` + signInAs + `).id = 'test-sign-in-as'`)
	realClick(b, "#test-sign-in-as")
	b.waitFor("the pad is up with the focus in the Name box", `!!document.getElementById('lock-name')?.offsetParent && document.activeElement?.id === 'lock-name'`)
	typeNameThenPin(b)
	b.check()
}
