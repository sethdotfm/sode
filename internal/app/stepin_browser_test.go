package app

import (
	"context"
	"fmt"
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

// openStepIn presses Step in in Settings > My user and waits for the lock screen's pad in Step in mode.
func openStepIn(b *tab) {
	b.t.Helper()
	b.waitFor("Step in shown", `!document.getElementById('account-stepin').hidden`)
	// Focused first, as a person's click or Enter does, so Cancel can give the focus back.
	b.eval(`{ const s = document.getElementById('account-stepin'); s.focus(); s.click(); } true`)
	b.waitFor("Step in pad", `!!document.querySelector('.sode-lock.is-on') && !!document.querySelector('.sode-lock__mode:not([hidden])') &&
		!!document.querySelector('.sode-lock__cancel:not([hidden])') && document.activeElement?.id === 'lock-name'`)
}

// padDigits presses each digit on the lock screen's pad.
func padDigits(b *tab, pin string) {
	b.t.Helper()
	for _, d := range pin {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click(); true`, d))
	}
}

// stepInAs steps in on the lock screen's pad. The PIN is the user's full length, so the pad sends it.
func stepInAs(b *tab, name, pin string) {
	b.t.Helper()
	openStepIn(b)
	b.padName(name)
	padDigits(b, pin)
	b.waitFor("Step in pad gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
}

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

	stepInAs(b, "Bo", "567890")
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

	stepInAs(b, "Bo", "567890")
	b.waitFor("stepped in again", `!document.getElementById('account-stepout').hidden`)
	b.eval(`document.getElementById('account-stepout').click(); true`)
	b.waitFor("Ana after Step out", `document.getElementById('account-name').textContent.includes('Signed in as Ana') &&
		document.getElementById('account-stepout').hidden`)
	if got := b.eval(sessionKeysJS); got != before {
		t.Fatalf("session keys after Step out:\nbefore %v\nafter  %v", before, got)
	}
	b.check()
}

// While stepped in, the page border has the step-in colour, not the Branding one, and "Bo stepped in" shows with Step
// out. Bo's Compact view never fades the label, and it still shows after the layout turns to phone. The typed source
// stays across Step in and Step out.
func TestChromeStepInBorderAndLabel(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleAdministrator, true
		c.BrandBorder = "violet"
		bo := mkUser(t, "Bo", "567890")
		bo.Profile.Prefs = map[string]string{"compact": "on"}
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
	b.waitFor("no step-in border yet", `document.getElementById('stepbar').hidden && document.querySelector('.sode__stepframe').hidden`)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("Settings closed", `!document.documentElement.dataset.view`)
	b.eval(`const s = document.getElementById('src'); s.value = 'cam'; s.dispatchEvent(new Event('input', { bubbles: true })); true`)
	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("Settings open again", `document.documentElement.dataset.view === 'settings'`)

	stepInAs(b, "Bo", "567890")
	b.waitFor("stepped in as Bo", `!document.getElementById('account-stepout').hidden`)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("Settings closed in Compact view", `!document.documentElement.dataset.view && document.documentElement.hasAttribute('data-compact')`)

	frame := `(() => {
		const f = getComputedStyle(document.querySelector('.sode__stepframe'));
		const brand = getComputedStyle(document.querySelector('.sode__brandframe')).borderTopColor;
		return f.display === 'block' && f.position === 'fixed' && parseFloat(f.borderTopWidth) > 0 && f.borderTopColor !== brand && brand !== 'rgba(0, 0, 0, 0)';
	})()`
	// The label and everything round it are opaque, after Compact view's fade would have run.
	label := `(() => {
		const bar = document.getElementById('stepbar');
		const r = bar.getBoundingClientRect();
		if (bar.hidden || !r.height || bar.textContent !== 'Bo stepped inStep out') return false;
		for (let n = bar; n; n = n.parentElement) if (getComputedStyle(n).opacity !== '1' || getComputedStyle(n).visibility === 'hidden') return false;
		return true;
	})()`
	b.waitFor("step-in border, not the Branding colour", frame)
	// A real pointer at rest away from the faded chrome, as in TestChromeMainCompact: the footer fades, the label does not.
	b.call("Input.dispatchMouseEvent", map[string]any{"type": "mouseMoved", "x": 640, "y": 400})
	b.waitFor("Compact view faded the footer", `getComputedStyle(document.querySelector('#patch-view .sode__footer')).opacity === '0'`)
	b.waitFor("label shows in Compact view", label)
	b.shot("compact")
	if got := b.eval(`document.getElementById('src').value`); got != "cam" {
		t.Fatalf("typed source after Step in = %v, want cam", got)
	}

	b.eval(`document.getElementById('settings-open').click(); true`)
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	b.eval(`const l = document.getElementById('cfg-layout'); l.value = 'phone'; l.dispatchEvent(new Event('change', { bubbles: true })); true`)
	b.waitFor("phone layout", `document.documentElement.dataset.layout === 'phone'`)
	b.waitFor("label shows on the phone", label)
	b.waitFor("border shows on the phone", frame)
	b.shot("phone")

	b.eval(`document.getElementById('stepbar-out').click(); true`)
	b.waitFor("border and label gone after Step out", `document.getElementById('stepbar').hidden &&
		getComputedStyle(document.querySelector('.sode__stepframe')).display === 'none'`)
	if got := b.eval(`document.getElementById('src').value`); got != "cam" {
		t.Fatalf("typed source after Step out = %v, want cam", got)
	}
	b.check()
}

// Administrator Bo steps in at Trusted Ana's browser and stages a Branding and a Locks edit. After Step out, Ana's
// review names Bo for the Branding edit and shows the Locks one only as a count. Save sends Branding only; the Locks
// edit stays unsaved.
func TestChromeStepInReviewNamesAuthor(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleTrusted, true
		bo := mkUser(t, "Bo", "567890")
		bo.Role = config.RoleAdministrator
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
	stepInAs(b, "Bo", "567890")
	b.waitFor("stepped in as Bo, Locks shown", `!document.getElementById('stepbar').hidden &&
		!document.querySelector('fieldset[data-section="locks"]').hidden`)
	if got := b.eval(`document.getElementById('settings-path').textContent`); got == "Unsaved changes" {
		t.Fatalf("Step in alone made a change")
	}

	b.eval(`const t = document.getElementById('cfg-brandtext'); t.value = 'Truck 4'; t.dispatchEvent(new Event('input', { bubbles: true }));
		document.getElementById('cfg-lock-main').click(); true`)
	b.waitFor("two edits staged", `document.getElementById('settings-path').textContent === 'Unsaved changes'`)
	b.eval(`document.getElementById('stepbar-out').click(); true`)
	b.waitFor("Ana again, Locks hidden", `document.getElementById('stepbar').hidden &&
		document.getElementById('account-name').textContent.includes('Signed in as Ana') &&
		document.querySelector('fieldset[data-section="locks"]').hidden`)

	b.eval(`document.getElementById('settings-save').click(); true`)
	b.waitFor("review names Bo and counts the Locks edit", `(() => {
		const d = document.getElementById('review');
		if (!d?.offsetParent) return false;
		const head = [...d.querySelectorAll('thead th')].map((th) => th.textContent);
		const row = [...d.querySelectorAll('tbody tr')].find((r) => r.cells[0]?.textContent === 'Logo text and page title');
		return d.textContent.includes('1 change by Bo that your Role cannot see') && head.includes('Staged by')
			&& row?.cells[1].textContent === 'Bo' && !d.textContent.includes('Lock Main page')
			&& document.getElementById('review-save').textContent === 'Save 1 change';
	})()`)
	b.eval(`document.getElementById('review-save').click(); true`)
	b.waitFor("saved, the Locks edit still staged", `!document.getElementById('review').offsetParent && !document.getElementById('settings-save').disabled &&
		document.getElementById('settings-path').textContent === 'Unsaved changes'`)
	if cfg := a.r().cfg; cfg.BrandText != "Truck 4" || cfg.Locks.Main {
		t.Fatalf("after Ana's Save: brandText %q, main lock %v; want Truck 4 and no main lock", cfg.BrandText, cfg.Locks.Main)
	}
	b.check()
}

// Esc and Cancel each close the Step in pad and leave Ana's page as it was: Settings open, the staged edit kept. A
// wrong PIN keeps the pad in Step in mode. Shots in both themes at desktop width and at a phone's width.
func TestChromeStepInCancelKeepsPage(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleAdministrator, true
		c.Users = append(c.Users, mkUser(t, "Bo", "567890"), config.User{Name: "Cy"}) // Cy has no PIN
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
	b.eval(`const t = document.getElementById('cfg-brandtext'); t.value = 'Truck 4'; t.dispatchEvent(new Event('input', { bubbles: true })); true`)
	b.waitFor("edit staged", `document.getElementById('settings-path').textContent === 'Unsaved changes'`)
	kept := `document.documentElement.dataset.view === 'settings' && !document.documentElement.hasAttribute('data-sode-lock') &&
		document.getElementById('account-name').textContent.includes('Signed in as Ana') && document.getElementById('account-stepout').hidden &&
		document.getElementById('cfg-brandtext').value === 'Truck 4' && document.getElementById('settings-path').textContent === 'Unsaved changes'`

	openStepIn(b)
	// The list has only other users with a PIN.
	b.waitFor("Bo alone in the list", `[...document.querySelectorAll('#lock-name-list [role="option"]')].map((o) => o.textContent).join() === 'Bo'`)
	b.eval(`document.dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true })); true`)
	b.waitFor("Esc left the page as it was", kept)
	b.waitFor("focus back on Step in", `document.activeElement?.id === 'account-stepin'`)

	openStepIn(b)
	b.padName("Bo")
	padDigits(b, "111111")
	b.waitFor("wrong PIN told, still Step in", `document.querySelector('.sode-lock__dots').textContent === 'Incorrect' &&
		!!document.querySelector('.sode-lock__mode:not([hidden])') && document.getElementById('account-stepout').hidden`)
	b.waitFor("pad faded in", `getComputedStyle(document.querySelector('.sode-lock__body:not([hidden])')).opacity === '1'`)
	b.shot("light-desktop")
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": "dark"}}})
	b.shot("dark-desktop")
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": false})
	b.waitFor("Cancel fits a phone's width", `(() => {
		const r = document.querySelector('.sode-lock__cancel').getBoundingClientRect();
		return r.width > 0 && r.left >= 0 && r.right <= innerWidth;
	})()`)
	b.shot("dark-narrow")
	b.call("Emulation.setEmulatedMedia", map[string]any{"features": []map[string]string{{"name": "prefers-color-scheme", "value": "light"}}})
	b.shot("light-narrow")
	b.call("Emulation.clearDeviceMetricsOverride", nil)
	b.eval(`document.querySelector('.sode-lock__cancel').click(); true`)
	b.waitFor("Cancel left the page as it was", kept)

	stepInAs(b, "Bo", "567890")
	b.waitFor("stepped in as Bo, Settings still open", `!document.getElementById('account-stepout').hidden &&
		document.documentElement.dataset.view === 'settings'`)
	b.check()
}
