package app

import (
	"fmt"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Cy is PIN needed with no PIN, so Change my PIN opens by itself with no Current PIN.
func TestChromeChangeMyPinOnPhone(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role = config.RoleAdministrator
		c.Users = append(c.Users, config.User{Name: "Cy", PinNeeded: true})
	})
	srv := seamServer(t, a)

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/#settings")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	b.waitFor("Sign in shown", `document.documentElement.dataset.view === 'settings' && !document.getElementById('account-login').hidden`)
	b.eval(`document.getElementById('account-login').click()`)
	b.waitFor("the pad", `!!document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent`)
	b.padName("Cy")
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Change my PIN opens by itself, with no Current PIN", `(() => {
		const d = document.querySelector('.sode-lock__pin-dlg');
		return !!d?.offsetParent && !document.getElementById('sode-pin-old').offsetParent
			&& d.querySelector('button[type=submit]').textContent === 'Set PIN';
	})()`)
	b.shot("phone-set-pin")
	b.eval(`(() => {
		for (const id of ['sode-pin-new', 'sode-pin-again']) document.getElementById(id).value = '4826';
		document.querySelector('.sode-lock__pin-dlg button[type=submit]').click();
	})()`)
	b.waitFor("the dialog closes", `!document.querySelector('.sode-lock__pin-dlg')?.offsetParent`)
	if u, _ := a.r().cfg.FindUserByName("Cy"); u.Len != 4 || u.PinNeeded {
		t.Fatalf("Cy after set %+v", u)
	}

	b.waitFor("Change my PIN in My user", `!document.getElementById('account-pin').hidden`)
	b.eval(`document.getElementById('account-pin').click()`)
	b.waitFor("the dialog asks the current PIN and offers Remove my PIN", `(() => {
		const d = document.querySelector('.sode-lock__pin-dlg');
		return !!d?.offsetParent && !!document.getElementById('sode-pin-old').offsetParent
			&& [...d.querySelectorAll('button')].some((x) => x.textContent === 'Remove my PIN' && x.offsetParent);
	})()`)
	b.eval(`(() => {
		document.getElementById('sode-pin-old').value = '4826';
		[...document.querySelectorAll('.sode-lock__pin-dlg button')].find((x) => x.textContent === 'Remove my PIN').click();
	})()`)
	b.waitFor("the dialog closes", `!document.querySelector('.sode-lock__pin-dlg')?.offsetParent`)
	if u, _ := a.r().cfg.FindUserByName("Cy"); u.Pin != "" {
		t.Fatalf("Cy after remove %+v", u)
	}
	b.check()
}

// A Trusted user with Never sign out and No auto-lock comes back by the session token after a reload.
// They still give the current PIN to change it.
func TestChromeChangeMyPinAsksCurrentPin(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, withAdministrator, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent, c.Users[0].NoAutoLock = config.RoleTrusted, true, true
	})
	srv := seamServer(t, a)

	b.open(srv.URL + "/#settings")
	b.waitFor("Sign in shown", `document.documentElement.dataset.view === 'settings' && !document.getElementById('account-login').hidden`)
	b.eval(`document.getElementById('account-login').click()`)
	b.waitFor("the pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click(); true`)
	b.waitFor("the session token", `!!localStorage.getItem('sode-session')`)
	b.open(srv.URL + "/?again#settings")
	b.waitFor("Change my PIN after the reload", `!document.getElementById('account-pin').hidden`)
	// The dialog is built with the Lock (lock.js), a moment after My user shows the button.
	b.waitFor("the Change my PIN dialog", `!!document.querySelector('.sode-lock__pin')`)
	b.eval(`document.getElementById('account-pin').click()`)
	b.waitFor("the dialog asks the current PIN", `!!document.querySelector('.sode-lock__pin-dlg')?.offsetParent && !!document.getElementById('sode-pin-old').offsetParent`)
	submit := func(old string) {
		b.eval(fmt.Sprintf(`(() => {
			document.getElementById('sode-pin-old').value = %q;
			for (const id of ['sode-pin-new', 'sode-pin-again']) document.getElementById(id).value = '8642';
			document.querySelector('.sode-lock__pin-dlg button[type=submit]').click();
			return true;
		})()`, old))
	}
	note := `(document.querySelector('.sode-lock__pin-dlg .pb-note--error')?.textContent || '')`
	submit("")
	b.waitFor("no current PIN is refused", note+` === 'Enter your current PIN.'`)
	submit("0000")
	b.waitFor("a wrong current PIN is refused", `/wrong PIN/i.test(`+note+`)`)
	if n := userPinCount(a, "Ana"); n != 1 {
		t.Fatalf("wrong tries %d, want 1", n)
	}
	if !config.CheckPin(pinOf(a, "Ana"), "1234") {
		t.Fatal("the PIN changed")
	}
	submit("1234")
	b.waitFor("the dialog closes", `!document.querySelector('.sode-lock__pin-dlg')?.offsetParent`)
	if !config.CheckPin(pinOf(a, "Ana"), "8642") {
		t.Fatal("the PIN did not change")
	}
	b.check()
}
