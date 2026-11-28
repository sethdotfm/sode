package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// An approver's card on the main page at 1920x1080 keeps off the page's big scroll bar: the card stack ends
// --pb-space-5 left of the bar's room.
func TestChromeApprovalCardClearsPageBar(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Role, c.Users[0].Persistent = config.RoleTrusted, true
		c.Users = append(c.Users, mkUser(t, "Bo", "567890"))
		withAdministrator(c)
	})
	srv := seamServer(t, a)
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.eval(`document.getElementById('settings-exit').click(); true`)
	b.waitFor("main page", `document.documentElement.dataset.view !== 'settings'`)

	bo := dialPeer(t, a)
	bo.signIn("Bo", "567890")
	asked(t, bo.askSave(7, map[string]any{"brandText": "Studio B"}))
	b.waitFor("approver card", `!!document.querySelector('[data-approval-card]')?.offsetParent`)
	b.waitFor("the card is in place", `document.querySelector('[data-approval-card]').getAnimations().length === 0`)
	b.shot("card")
	got := b.eval(`document.querySelector('.sode-approvals').getBoundingClientRect().right`).(float64)
	bar := b.tokenPx("body", "--sode-page-bar")
	if bar <= 0 {
		t.Fatalf("the page bar has no room (%v px)", bar)
	}
	if want := b.eval(`innerWidth`).(float64) - bar - b.tokenPx("body", "--pb-space-5"); got > want+layoutTol {
		t.Errorf("the card stack ends at %.1f px, over the page bar; want at most %.1f px", got, want)
	}
	b.check()
}
