package app

import (
	"context"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// footAt fails the test unless the button sel is on the inner edge of the footer of dialog dlg. With end false that
// is the left edge, with end true the right edge. patchbay puts Cancel left and the confirm right. A lone Close goes
// right.
func footAt(b *tab, what, dlg, sel string, end bool) {
	b.t.Helper()
	foot := b.boxes(dlg + " .pb-modal__footer")
	btn := b.boxes(dlg + " .pb-modal__footer " + sel)
	if len(foot) != 1 || len(btn) != 1 {
		b.t.Fatalf("%s: %d footers and %d buttons, want one of each", what, len(foot), len(btn))
	}
	pad := b.tokenPx(dlg+" .pb-modal__footer > *", "--pb-space-5")
	got, want := btn[0].Left-foot[0].Left, pad
	if end {
		got, want = foot[0].Right-btn[0].Right, pad
	}
	if math.Abs(got-want) > layoutTol {
		b.shot("foot")
		b.t.Errorf("%s: %s is %.1fpx from the footer's %s edge, want %.1fpx", what, sel, got, map[bool]string{false: "left", true: "right"}[end], want)
	}
}

// A lone Close is on the right and says Close, in Settings' Manage users and My API clients.
func TestChromeDialogFootSettings(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Manage users shown", `!!document.getElementById('users-manage')?.offsetParent`)
	b.eval(`document.getElementById('users-manage').click(); true`)
	b.waitFor("Users dialog open", `!!document.querySelector('.sode__users-dlg')?.offsetParent`)
	if got := b.eval(`[...document.querySelectorAll('.sode__users-dlg .pb-modal__footer button')].map((x) => x.textContent).join(',')`); got != "Close" {
		t.Errorf("Manage users footer buttons %q, want only Close", got)
	}
	footAt(b, "Manage users", ".sode__users-dlg", "button", true)
	b.eval(`document.querySelector('.sode__users-dlg .pb-modal__footer button').click(); true`)
	b.waitFor("Users dialog closed", `!document.querySelector('.sode__users-dlg')?.offsetParent`)

	b.waitFor("My API clients shown", `!!document.getElementById('account-clients')?.offsetParent`)
	b.eval(`document.getElementById('account-clients').click(); true`)
	b.waitFor("My API clients open", `!!document.querySelector('[aria-labelledby="myclients-title"]')?.offsetParent`)
	footAt(b, "My API clients", `[aria-labelledby="myclients-title"]`, "button", true)
}

// In a panel dialog Cancel is left and Save right (Share), and a lone Close is on the right (Push).
func TestChromeDialogFootPanels(t *testing.T) {
	b := startBrowser(t)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	makePanel(t, a, ana, `{"name":"EVS 1"}`)
	makePanel(t, a, ana, `{"name":"EVS 2"}`)
	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)

	b.waitFor("Share button", `!!document.querySelector('[data-act="share"]')`)
	b.eval(`document.querySelector('[data-act="share"]').click()`)
	b.waitFor("Share open", `!!document.querySelector('.sode-ov__share #tab-people')`)
	share := `[aria-labelledby="share-title"]`
	footAt(b, "Share", share, ".pb-btn--outline", false)
	footAt(b, "Share", share, ".sode-btn--primary", true)
	b.shot("share")
	b.eval(`document.querySelector('[aria-labelledby="share-title"] .pb-modal__footer .pb-btn--outline').click(); true`)
	b.waitFor("Share closed", `!document.querySelector('[aria-labelledby="share-title"]')`)

	b.waitFor("Push button", `!!document.querySelector('[data-act="push"]')`)
	b.eval(`document.querySelector('[data-act="push"]').click()`)
	b.waitFor("Push open", `!!document.querySelector('[aria-labelledby="push-title"]')?.offsetParent`)
	footAt(b, "Push", `[aria-labelledby="push-title"]`, ".pb-btn--outline", true)
}
