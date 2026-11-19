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

// While stepped in the page border takes the step-in colour, not the Branding one, and "Bo stepped in" shows with Step
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

	b.waitFor("Step in shown", `!document.getElementById('account-stepin').hidden`)
	b.eval(`document.getElementById('account-stepin').click(); true`)
	b.waitFor("Step in popup", `!!document.getElementById('stepin-pin')?.offsetParent`)
	b.eval(`document.getElementById('stepin-name').value = 'Bo'; document.getElementById('stepin-pin').value = '567890';
		document.getElementById('stepin-go').click(); true`)
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
	b.eval(`new Promise((r) => setTimeout(() => r(true), 1500))`)
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
