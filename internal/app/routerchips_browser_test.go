package app

import (
	"strconv"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// The level chips of the router detail view have one width, from the widest label. Level 1 and level 15 are the same
// width. With 64 levels (the most chips), level 1 and level 64 are too. Each label fits in its chip.
func TestChromeSettingsLevelChipsOneWidth(t *testing.T) {
	b := startBrowser(t)
	a := simApp(t, startSim(t), func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	same := func(n, i, j int) string {
		return `(() => {
			const c = [...document.querySelectorAll('#router-chips > .sode__slot')];
			if (c.length !== ` + strconv.Itoa(n) + ` || !c[0].offsetParent) return false;
			const w = (x) => x.getBoundingClientRect().width;
			if (w(c[` + strconv.Itoa(i) + `]) !== w(c[` + strconv.Itoa(j) + `])) throw new Error('chip widths ' + w(c[` + strconv.Itoa(i) + `]) + ' and ' + w(c[` + strconv.Itoa(j) + `]));
			return c.every((x) => x.scrollWidth <= x.clientWidth);
		})()`
	}
	b.waitFor("levels 1 and 15 the same width", same(17, 0, 14))
	typeField(b, "cfg-rt-levels", "64")
	b.waitFor("levels 1 and 64 the same width", same(64, 0, 63))
	b.check()
}
