package app

import (
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// Palette pins on Home. A Guest's Alt+P and the ⋯ menu pin to This browser, and the empty Palette lists Pinned in the
// saved order. Alt+arrows reorder them, and the order survives a reload. A pin whose preset is gone shows Missing and
// Alt+P unpins it. The 25th pin is refused. Alt+B toggles a bookmark in the person's own list and never changes
// Everyone's. Signed in, only Me pins show; after sign-out This browser's come back.
func TestChromePalettePins(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users = append(c.Users, mkUser(t, "Bo", "5678"))
		c.Presets = []config.Preset{
			{ID: "p-in", Name: "Studio In", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst)}},
		}
		c.Bookmarks = []config.Bookmark{{Key: "page:log", Label: "Log"}}
	})
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	const (
		pinned = `[...document.querySelectorAll('#palette-list [data-pin]')].map((n) => n.dataset.key).join(',')`
		note   = `document.querySelector('.sode-palette__note').textContent`
		heads  = `[...document.querySelectorAll('#palette-list .sode-palette__head')].map((n) => n.textContent).join(',')`
		local  = `localStorage.getItem('sode-palette-pins') || '[]'`
	)
	altKey := func(key, code string) {
		b.t.Helper()
		b.eval(fmt.Sprintf(`document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: %q, code: %q, altKey: true, bubbles: true, cancelable: true })); true`, key, code))
	}
	open := func() {
		b.t.Helper()
		b.eval(ctrlShiftP)
		b.waitFor("the Palette opens", paletteShown)
	}
	clearQuery := func() {
		b.t.Helper()
		b.eval(`(() => { const i = document.querySelector('.sode-palette__q'); i.value = ''; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	}
	esc := func() {
		b.t.Helper()
		paletteKey(b, "Escape", false)
		paletteKey(b, "Escape", false)
		b.waitFor("the Palette closes", `document.getElementById('palette').hidden`)
	}

	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`(() => { const t = document.getElementById('src'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); t.blur(); return true; })()`)
	open()
	paletteFind(b, "Studio In", "preset:global:p-in")
	altKey("p", "KeyP")
	b.waitFor("Alt+P pins to This browser for a Guest", note+` === 'Studio In: pinned in This browser.' && `+local+` === '["preset:global:p-in"]'`)

	// The ⋯ menu: Pin and Bookmark, each with an icon, its text and its key.
	paletteFind(b, "Communication Log", "page:log")
	b.eval(`document.querySelector('#palette-list [aria-selected="true"] [data-mark]').click(); true`)
	b.waitFor("the menu offers Pin and Bookmark", `(() => { const m = document.getElementById('palette-menu'); const it = [...m.querySelectorAll('.pb-menu__item')]; return !m.hidden && it.map((x) => x.querySelector('span').textContent).join() === 'Pin,Bookmark' && it.every((x) => x.querySelector('svg') && x.querySelector('.pb-menu__hint').textContent); })()`)
	b.eval(`document.querySelector('#palette-menu [data-act="pin"]').click(); true`)
	b.waitFor("the menu pins", note+` === 'Communication Log: pinned in This browser.'`)
	b.eval(`document.querySelector('#palette-list [aria-selected="true"] [data-mark]').click(); true`)
	b.waitFor("the menu now says Unpin", `document.querySelector('#palette-menu [data-act="pin"] span').textContent === 'Unpin'`)
	paletteKey(b, "Escape", false)
	b.waitFor("Esc closes the menu first", `document.getElementById('palette-menu').hidden && document.querySelector('.sode-palette__q').value === 'Communication Log'`)

	// Alt+B: the row is only in Everyone's list, so it adds to This browser and leaves Everyone's alone. Again removes it.
	altKey("b", "KeyB")
	b.waitFor("Alt+B bookmarks in This browser", note+` === 'Communication Log: bookmarked in This browser.' && JSON.parse(localStorage.getItem('sode-bookmarks'))[0].key === 'page:log'`)
	altKey("b", "KeyB")
	b.waitFor("Alt+B again removes it", note+` === 'Communication Log: bookmark removed from This browser.' && !localStorage.getItem('sode-bookmarks')`)
	if got := a.r().cfg.Bookmarks; len(got) != 1 || got[0].Key != "page:log" {
		t.Errorf("Everyone's bookmarks changed: %+v", got)
	}

	clearQuery()
	b.waitFor("Home, Pinned, Last picks, Categories, the pins in their order", heads+` === 'Pinned,Categories' && `+pinned+` === 'preset:global:p-in,page:log'`)
	// The empty Palette starts with no cursor: Down twice is the second pin. Alt+Up moves it first.
	paletteKey(b, "ArrowDown", false)
	paletteKey(b, "ArrowDown", false)
	b.waitFor("the cursor on the last pin", `document.querySelector('#palette-list [aria-selected="true"]')?.dataset.key === 'page:log'`)
	b.eval(`document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowUp', altKey: true, bubbles: true, cancelable: true })); true`)
	b.waitFor("Alt+Up moves the pin", pinned+` === 'page:log,preset:global:p-in' && document.querySelector('#palette-list [aria-selected="true"]')?.dataset.key === 'page:log'`)
	b.reload()
	b.waitFor("names again", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`(() => { const t = document.getElementById('src'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); t.blur(); return true; })()`)
	open()
	b.waitFor("the order survives a reload", pinned+` === 'page:log,preset:global:p-in'`)
	esc()

	// A pin whose preset is gone: grey with Missing, and Alt+P unpins it.
	b.eval(`localStorage.setItem('sode-palette-pins', JSON.stringify(['page:log', 'preset:global:gone'])); true`)
	open()
	b.waitFor("a gone preset shows Missing", `(() => { const n = document.querySelector('#palette-list [data-key="preset:global:gone"]'); return !!n && n.matches('.sode-palette__row--off') && n.textContent.includes('Missing'); })()`)
	paletteKey(b, "ArrowDown", false)
	paletteKey(b, "ArrowDown", false)
	b.waitFor("the cursor on the Missing pin", `document.querySelector('#palette-list [aria-selected="true"]')?.dataset.key === 'preset:global:gone'`)
	altKey("p", "KeyP")
	b.waitFor("Alt+P unpins it", local+` === '["page:log"]' && !document.querySelector('#palette-list [data-key="preset:global:gone"]')`)
	esc()

	// The 25th pin is refused with a message.
	b.eval(`localStorage.setItem('sode-palette-pins', JSON.stringify(Array.from({ length: 24 }, (_, i) => 'preset:global:x' + i))); true`)
	open()
	paletteFind(b, "Studio In", "preset:global:p-in")
	altKey("p", "KeyP")
	b.waitFor("the 25th pin is refused", note+` === 'A list holds at most 24 Palette pins.' && JSON.parse(`+local+`).length === 24`)
	esc()
	b.eval(`localStorage.setItem('sode-palette-pins', JSON.stringify(['page:log'])); true`)

	// Home is never a pin: Alt+P on it says why and saves nothing.
	open()
	paletteFind(b, "Home", "page:home")
	altKey("p", "KeyP")
	b.waitFor("Home is refused", note+` === 'Home is always first in the Palette, so it is not pinned.' && `+local+` === '["page:log"]'`)
	esc()

	// Signed in, only Me pins show. Bo's pin goes to Me.
	b.eval(`document.getElementById('settings-open').click(); document.getElementById('account-login').click(); true`)
	signInAs(b, "Bo", "5678")
	b.eval(`document.getElementById('settings-back').click(); true`)
	b.waitFor("Home", `document.documentElement.dataset.view !== 'settings'`)
	open()
	b.waitFor("This browser's pins are hidden for Bo", `document.querySelectorAll('#palette-list .sode-palette__row--cat').length > 0 && `+pinned+` === ''`)
	paletteFind(b, "Studio In", "preset:global:p-in")
	altKey("p", "KeyP")
	b.waitFor("Alt+P pins to Me", note+` === 'Studio In: pinned in Me.'`)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		u, _ := a.r().cfg.FindUserByName("Bo")
		if slices.Equal(u.Profile.PalettePins, []string{"preset:global:p-in"}) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Bo's pins in Me: %q", u.Profile.PalettePins)
		}
	}
	clearQuery()
	b.waitFor("only Me pins", pinned+` === 'preset:global:p-in'`)
	esc()

	b.eval(`import('./core.js').then((m) => { m.emit('logout'); return true; })`)
	b.waitFor("signed out", `!document.querySelector('.sode-lock__who-user')`)
	open()
	b.waitFor("This browser's pins come back after sign-out", pinned+` === 'page:log'`)
	b.check()
}
