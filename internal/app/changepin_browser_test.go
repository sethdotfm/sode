package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Change my PIN on a phone. Cy is PIN needed with no PIN: picking the name signs in, and Change my PIN opens by
// itself with no Current PIN. After the set, the dialog asks the current PIN and offers Remove my PIN.
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
