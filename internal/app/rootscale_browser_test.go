package app

import (
	"fmt"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// rootSizes are the desktop windows the big screen rule in base.css is checked at, with the root font size each gets.
// 3440 by 1440 is an ultrawide: it grows as a 16:9 screen of its height does.
var rootSizes = []struct {
	w, h int
	px   float64
}{
	{1920, 1080, 16},
	{2560, 1440, 16.89},
	{3440, 1440, 16.89},
	{3840, 2160, 24},
}

// sizeTo sets a desktop window of w by h CSS px with a device scale of 1, a big screen with no OS scale.
func sizeTo(b *tab, w, h int) {
	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": w, "height": h, "deviceScaleFactor": 1, "mobile": false})
}

// rootIs waits until the root font size is px, within 0.05 px.
func rootIs(b *tab, what string, px float64) {
	b.t.Helper()
	b.waitFor(fmt.Sprintf("%s: the root is %.2f px", what, px), fmt.Sprintf(`Math.abs(parseFloat(getComputedStyle(document.documentElement).fontSize) - %g) < 0.05`, px))
}

// On the lock screen, the main page and Settings, the root font size stays 16 px up to 2400 px wide. It grows to 24 px
// at 3840 by 2160, and the window height limits it. The lock pad has no size rule of its own and grows with the root.
func TestChromeRootScale(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Locks = config.Locks{Main: true} })
	srv := seamServer(t, a)
	sizeTo(b, 1920, 1080)
	b.open(srv.URL + "/")
	b.waitFor("the pad on load", `!!document.querySelector('.sode-lock.is-on') && !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)

	each := func(page string) {
		t.Helper()
		for _, s := range rootSizes {
			sizeTo(b, s.w, s.h)
			rootIs(b, fmt.Sprintf("%s at %dx%d", page, s.w, s.h), s.px)
			b.shot(fmt.Sprintf("%s-%dx%d", page, s.w, s.h))
		}
	}
	each("lock")
	b.waitFor("the pad fits a 3840 by 2160 window", `(() => {
		const r = document.querySelector('.sode-lock__key[data-key="enter"]').getBoundingClientRect();
		return r.bottom <= innerHeight && r.right <= innerWidth;
	})()`)

	sizeTo(b, 1920, 1080)
	signInOnPad(b)
	b.waitFor("signed in", `!document.documentElement.hasAttribute('data-sode-lock') && document.documentElement.dataset.conn === 'up'`)
	each("main")

	sizeTo(b, 1920, 1080)
	b.eval(`location.hash = 'settings'; true`)
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	each("settings")

	sizeTo(b, 2400, 1350)
	rootIs(b, "Settings at 2400x1350, no step", 16)
	b.check()
}
