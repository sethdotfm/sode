package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

const (
	chipName  = `document.querySelector('.sode-lock__who-name')?.textContent`
	chipLock  = `.sode-lock__who-slot > .pb-btn`
	chipOpen  = `(!document.getElementById('lock-chip-menu').hidden && document.getElementById('lock-chip-more').getAttribute('aria-expanded') === 'true')`
	chipShut  = `document.getElementById('lock-chip-menu').hidden`
	chipItems = `[...document.querySelectorAll('#lock-chip-menu .pb-menu__item')].filter((b) => !!b.offsetParent).map((b) => b.querySelector('.sode-lock__menu-label').firstChild.textContent).join(',')`
)

// mouseTo moves the mouse to the middle of the element that sel finds, with no press.
func mouseTo(b *tab, sel string) {
	b.t.Helper()
	x, y := touchAt(b, `document.querySelector('`+sel+`')`)
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": x, "y": y})
}

// darkShot saves a light and a dark shot of the page and of its top left corner (twice the size), then goes back to
// light. Headless Chrome otherwise follows the OS.
func darkShot(b *tab, name string) {
	b.t.Helper()
	dir := os.Getenv("SODE_BROWSER_SHOTS")
	if dir == "" {
		return
	}
	for _, scheme := range []string{"light", "dark"} {
		b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": scheme}}})
		time.Sleep(450 * time.Millisecond)
		n := name + "-" + scheme
		b.shot(n)
		var r struct{ Data []byte }
		_ = json.Unmarshal(b.call("Page.captureScreenshot", map[string]any{"format": "png", "clip": map[string]any{"x": 0, "y": 0, "width": 480, "height": 200, "scale": 2}}), &r)
		_ = os.WriteFile(filepath.Join(dir, strings.ReplaceAll(b.t.Name(), "/", "_")+"-"+n+"-corner.png"), r.Data, 0o644)
	}
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": "light"}}})
}

func startChipPage(t *testing.T, edit func(*config.Config)) *tab {
	t.Helper()
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleAdministrator, true
		c.Users = append(c.Users, mkUser(t, "Bo", "567890"))
		if edit != nil {
			edit(c)
		}
	})
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1280, "height": 720, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/")
	return b
}

// "Signed in as <name>" is plain text with the name bold in the accent. The lock button's drop-down opens on keyboard
// focus and on hover. On a touch screen the chevron opens it, and a tap on the lock button still locks. Esc closes it,
// and the focus that stays does not open it again. Stepped in, it offers only Step out. Under the Lock it
// does not open. Ctrl+Shift+L still asks its Confirm; the drop-down's Sign out asks none.
func TestChromeChipMenu(t *testing.T) {
	b := startChipPage(t, nil)
	b.waitFor("Sign in on the chip", `!!document.getElementById('lock-chip-signin')?.offsetParent`)
	b.waitFor("no chevron for a guest", `!document.getElementById('lock-chip-more').offsetParent`)
	realClick(b, "#lock-chip-signin")
	b.waitFor("the pad", `document.activeElement?.id === 'lock-name'`)
	typeNameThenPin(b)
	b.waitFor("Signed in as Ana in text", chipName+` === 'Signed in as Ana' && !document.documentElement.hasAttribute('data-sode-lock') && !document.querySelector('.sode-lock__who button.sode-lock__who-name, #lock-chip-name, #lock-chip-pin')`)
	style := b.eval(`(() => {
		const u = document.querySelector('.sode-lock__who-name > strong'), p = document.createElement('span');
		p.style.color = 'var(--pb-accent)';
		document.body.append(p);
		const accent = getComputedStyle(p).color;
		p.remove();
		const s = getComputedStyle(u), n = getComputedStyle(u.parentElement);
		return [u.textContent, s.color === accent, +s.fontWeight > +n.fontWeight].join(',');
	})()`)
	if style != "Ana,true,true" {
		t.Fatalf("name, accent, bold = %v; want Ana,true,true", style)
	}
	darkShot(b, "chip")

	// Keyboard: a focus given by script does not open it; Shift+Tab onto the lock button does.
	b.eval(`document.getElementById('lock-chip-more').focus(); true`)
	time.Sleep(300 * time.Millisecond)
	if b.eval(chipShut) != true {
		t.Fatal("a focus that did not come by Tab opened the drop-down")
	}
	realKeyMods(b, "Tab", "Tab", 9, modShift)
	b.waitFor("open on Tab focus, the focus kept on Lock", chipOpen+` && document.activeElement === document.querySelector('`+chipLock+`')`)
	b.waitFor("Sign out and Step in…", chipItems+` === 'Sign out,Step in…'`)
	realKey(b, "ArrowDown", "ArrowDown", 40)
	b.waitFor("Sign out first", `document.activeElement?.id === 'lock-chip-signout'`)
	realKey(b, "ArrowDown", "ArrowDown", 40)
	b.waitFor("Step in… next", `document.activeElement?.id === 'lock-chip-stepin'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed, focus on Lock", chipShut+` && document.activeElement === document.querySelector('`+chipLock+`')`)
	time.Sleep(300 * time.Millisecond)
	if b.eval(chipShut) != true {
		t.Fatal("Esc closed the drop-down, but the focus on Lock opened it again")
	}
	realKey(b, "ArrowDown", "ArrowDown", 40)
	b.waitFor("ArrowDown on Lock opens it on Sign out", chipOpen+` && document.activeElement?.id === 'lock-chip-signout'`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("closed", chipShut)
	b.eval(`document.activeElement.blur(); true`)

	// Mouse: hover opens it; the pointer crosses the gap to an item and it stays; away, it closes.
	mouseTo(b, chipLock)
	b.waitFor("open on hover", chipOpen)
	mouseTo(b, "#lock-chip-stepin")
	time.Sleep(400 * time.Millisecond)
	if b.eval(chipOpen) != true {
		t.Fatal("the drop-down closed on the way to its item")
	}
	darkShot(b, "menu")
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 640, "y": 500})
	b.waitFor("closed when the mouse leaves", chipShut)
	// A person points at the chevron, it opens on hover, and the click that follows keeps it open.
	mouseTo(b, "#lock-chip-more")
	b.waitFor("open on hover over the chevron", chipOpen)
	x, y := touchAt(b, `document.getElementById('lock-chip-more')`)
	for _, ty := range []string{"mousePressed", "mouseReleased"} {
		b.call("Input.dispatchMouseEvent", map[string]any{"type": ty, "x": x, "y": y, "button": "left", "clickCount": 1})
	}
	time.Sleep(300 * time.Millisecond)
	if b.eval(chipOpen) != true {
		t.Fatal("a click on the chevron shut the drop-down that hover opened")
	}
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 640, "y": 500})
	b.waitFor("closed when the mouse leaves", chipShut)

	// Touch: a tap on the chevron opens it with no Lock; a tap on Step in… opens the pad.
	tap(b, `document.getElementById('lock-chip-more')`)
	b.waitFor("open on a chevron tap, no Lock", chipOpen+` && !document.documentElement.hasAttribute('data-sode-lock')`)
	darkShot(b, "touch")
	tap(b, `document.getElementById('lock-chip-stepin')`)
	b.waitFor("Step in pad", `!!document.querySelector('.sode-lock.is-on') && document.activeElement?.id === 'lock-name' && `+chipShut)
	b.padName("Bo")
	padDigits(b, "567890")
	b.waitFor("stepped in as Bo", chipName+` === 'Signed in as Bo' && !document.documentElement.hasAttribute('data-sode-lock')`)
	realClick(b, "#lock-chip-more")
	b.waitFor("only Step out", chipOpen+` && `+chipItems+` === 'Step out'`)
	darkShot(b, "stepped")
	realClick(b, "#lock-chip-stepin")
	b.waitFor("Ana again with no Confirm", chipName+` === 'Signed in as Ana' && !document.getElementById('signout-ask-ok')?.offsetParent && document.getElementById('account-stepout').hidden`)

	ctrlShiftL(b)
	b.waitFor("Sign out Confirm", `!!document.getElementById('signout-ask-ok')?.offsetParent`)
	realKey(b, "Escape", "Escape", 27)
	b.waitFor("Esc keeps Ana", `!document.getElementById('signout-ask-ok')?.offsetParent && `+chipName+` === 'Signed in as Ana'`)
	time.Sleep(300 * time.Millisecond)
	if b.eval(chipShut) != true {
		t.Fatal("the focus given back after the Confirm opened the drop-down")
	}

	// Under the Lock the drop-down does not open, and its chevron fades with the lock button but keeps its place.
	nameX := b.eval(`document.querySelector('.sode-lock__who-name').getBoundingClientRect().left - document.querySelector('.sode-lock__who-bar').getBoundingClientRect().left`)
	// Lock stays one tap on a touch screen.
	tap(b, `document.querySelector('`+chipLock+`')`)
	b.waitFor("the Lock in one tap, the chevron inert", `document.querySelector('.sode-lock__who').classList.contains('is-hand') && document.getElementById('lock-chip-more').inert && `+chipShut)
	b.waitFor("the name holds its place in the bar", `Math.abs(document.querySelector('.sode-lock__who-name').getBoundingClientRect().left - document.querySelector('.sode-lock__who-bar').getBoundingClientRect().left - `+fmt.Sprint(nameX)+`) < 1.5`)
	darkShot(b, "locked")
	mouseTo(b, ".sode-lock__who-slot")
	time.Sleep(400 * time.Millisecond)
	if b.eval(chipShut) != true {
		t.Fatal("the drop-down opened under the Lock")
	}
	padDigits(b, "1234")
	b.waitFor("Lock resumed", `!document.querySelector('.sode-lock__who').classList.contains('is-hand') && !document.documentElement.hasAttribute('data-sode-lock')`)

	ctrlShiftL(b)
	answerSignOutAsk(b, "Sign out")
	b.waitFor("signed out", `import('/lock.js').then((m) => !m.lockState().user && !!document.getElementById('lock-chip-signin')?.offsetParent)`)
	b.check()
}

// The drop-down's Sign out asks no Confirm. Where the page lock is off it signs out and leaves the page open, with Sign
// in back on the chip. Where it is on, the pad shows.
func TestChromeChipSignOut(t *testing.T) {
	for _, enforced := range []bool{false, true} {
		name := "open"
		if enforced {
			name = "enforced"
		}
		t.Run(name, func(t *testing.T) {
			b := startChipPage(t, func(c *config.Config) { c.Locks = config.Locks{Main: enforced} })
			if enforced {
				b.waitFor("the pad on load", `!!document.querySelector('.sode-lock.is-on') && !!document.getElementById('lock-name')?.offsetParent`)
				b.eval(`document.getElementById('lock-name').focus(); true`)
			} else {
				b.waitFor("Sign in on the chip", `!!document.getElementById('lock-chip-signin')?.offsetParent`)
				realClick(b, "#lock-chip-signin")
				b.waitFor("the pad", `document.activeElement?.id === 'lock-name'`)
			}
			typeNameThenPin(b)
			b.waitFor("signed in", chipName+` === 'Signed in as Ana' && !document.documentElement.hasAttribute('data-sode-lock')`)
			mouseTo(b, chipLock)
			b.waitFor("open on hover", chipOpen)
			darkShot(b, "before")
			b.eval(`window.sawAsk = false; new MutationObserver(() => { if (document.getElementById('signout-ask-ok')?.offsetParent) window.sawAsk = true }).observe(document.body, { subtree: true, childList: true, attributes: true }); true`)
			realClick(b, "#lock-chip-signout")
			if enforced {
				b.waitFor("the pad", `import('/lock.js').then((m) => !m.lockState().user && !!document.querySelector('.sode-lock.is-on') && !!document.getElementById('lock-name')?.offsetParent)`)
			} else {
				b.waitFor("signed out, the page open", `import('/lock.js').then((m) => !m.lockState().user && !document.querySelector('.sode-lock.is-on') && !!document.getElementById('lock-chip-signin')?.offsetParent)`)
			}
			if b.eval(`window.sawAsk`) != false {
				t.Fatal("the drop-down's Sign out asked a Confirm")
			}
			time.Sleep(400 * time.Millisecond)
			darkShot(b, "after")
			b.check()
		})
	}
}
