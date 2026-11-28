package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Settings > Routers: the Address field of a new router tries the typed address before Save. The address of a second
// simulator shows the wizard's Connected line with the counts of its names, and nothing is saved. A closed port shows
// "No answer yet" and why.
func TestChromeSettingsRouterAddressTry(t *testing.T) {
	b := startBrowser(t)
	simA, simB := startSim(t), startSim(t)
	a := simApp(t, simA, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("Add router shown", `!!document.getElementById('router-add')?.offsetParent`)
	b.eval(`document.getElementById('router-add').click(); true`)
	b.waitFor("the new router selected", `document.getElementById('cfg-rt-prefix')?.value === 'B'`)
	try := `document.getElementById('cfg-rt-address-try')`
	typeField(b, "cfg-rt-address", simB)
	b.waitFor("Connected before Save", try+`.textContent === 'Connected: 200 sources and 120 destinations discovered.' && !!`+try+`.offsetParent`)
	if n := len(a.r().cfg.Routers); n != 1 {
		t.Fatalf("the try saved a router: %d routers", n)
	}
	typeField(b, "cfg-rt-address", closedAddr(t))
	b.waitFor("the failure and why", try+`.textContent.startsWith('No answer yet. Sode keeps trying.') && `+try+`.textContent.includes('refused')`)
	if rc := a.r().cfg.DefaultRouter(); rc.Address != simA {
		t.Fatalf("router A moved to %q", rc.Address)
	}
	b.check()
}
