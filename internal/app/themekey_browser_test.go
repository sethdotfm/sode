package app

import (
	"testing"
	"time"
)

// The theme key (themekey.js) works on every page: /log here, and a panel whose theme is Auto. A panel with a stored
// theme keeps it, and a Kiosk panel takes no page keys.
func TestChromeThemeKey(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	auto := makePanel(t, a, ana, `{"name":"Auto wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}]}`)
	dark := makePanel(t, a, ana, `{"name":"Dark wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}],"settings":{"theme":"dark"}}`)
	kiosk := makePanel(t, a, ana, `{"name":"Kiosk wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}],"settings":{"kiosk":true}}`)
	themeKey := func() { realKeyMods(b, "T", "KeyT", 84, modAlt|modShift) }
	const isDark = `(document.documentElement.dataset.theme ? document.documentElement.dataset.theme === 'dark' : matchMedia('(prefers-color-scheme: dark)').matches)`

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)

	b.open(srv.URL + "/log")
	b.waitFor("the Log's Home link", `!!document.querySelector('.sode-bar__home')`)
	b.eval(`window.was = ` + isDark + `; true`)
	themeKey()
	b.waitFor("the key changes the theme on /log", isDark+` !== window.was`)
	themeKey()
	b.waitFor("a second press changes it back", isDark+` === window.was`)

	b.open(srv.URL + "/panels/" + auto)
	b.waitFor("the Auto panel shows", `document.title === 'Auto wall' && !document.documentElement.hasAttribute('data-sode-panel-theme')`)
	b.eval(`window.was = ` + isDark + `; true`)
	themeKey()
	b.waitFor("the key changes the theme on an Auto panel", isDark+` !== window.was`)
	themeKey()
	b.waitFor("and back", isDark+` === window.was`)

	b.open(srv.URL + "/panels/" + dark)
	b.waitFor("the panel runs in its stored theme", `document.title === 'Dark wall' && document.documentElement.dataset.theme === 'dark' && document.documentElement.hasAttribute('data-sode-panel-theme')`)
	themeKey()
	time.Sleep(500 * time.Millisecond)
	if b.eval(`document.documentElement.dataset.theme`) != "dark" {
		t.Error("the theme key changed a panel that stores its own theme")
	}

	b.open(srv.URL + "/panels/" + kiosk)
	b.waitFor("Kiosk mode", `document.documentElement.hasAttribute('data-sode-kiosk')`)
	b.eval(`window.was = ` + isDark + `; true`)
	themeKey()
	time.Sleep(500 * time.Millisecond)
	if b.eval(isDark+` === window.was`) != true {
		t.Error("the theme key changed a Kiosk panel")
	}
	b.check()
}
