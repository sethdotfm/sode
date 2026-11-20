package app

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// paletteApp: Ana (1234) and Bo (5678), a Standard user whose Rights hold only the preset Studio In.
func paletteApp(t *testing.T, sim string) *App {
	return simApp(t, sim, func(c *config.Config) {
		bo := mkUser(t, "Bo", "5678")
		bo.Rights = config.Rights{Presets: &[]string{"p-in"}}
		c.Users = append(c.Users, bo)
		c.Presets = []config.Preset{
			{ID: "p-in", Name: "Studio In", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst)}},
			{ID: "p-out", Name: "Studio Out", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst+1)}},
		}
	})
}

const (
	paletteShown = `!document.getElementById('palette').hidden`
	ctrlShiftP   = `document.dispatchEvent(new KeyboardEvent('keydown', { key: 'P', code: 'KeyP', ctrlKey: true, shiftKey: true, bubbles: true, cancelable: true })); true`
)

// paletteFind types q into the open Palette and waits for the first row to be key.
func paletteFind(b *tab, q, key string) {
	b.t.Helper()
	b.eval(fmt.Sprintf(`(() => { const i = document.querySelector('.sode-palette__q'); i.value = %q; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`, q))
	b.waitFor("Palette row "+key, fmt.Sprintf(`document.querySelector('#palette-list [aria-selected="true"]')?.dataset.key === %q`, key))
}

func paletteKey(b *tab, key string, shift bool) {
	b.t.Helper()
	b.eval(fmt.Sprintf(`document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: %q, shiftKey: %v, bubbles: true, cancelable: true })); true`, key, shift))
}

func signInAs(b *tab, name, pin string) {
	b.t.Helper()
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName(name)
	for _, d := range pin {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]')?.offsetParent && document.querySelector('.sode-lock__key[data-key="enter"]').click(); true`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
}

// The Palette on Home. It checks the Palette character, the doubled character and Ctrl+Shift+P. It checks a preset
// through the Confirm and into the boxes, grey rows only as bookmarks with their offers, a category word and a
// setting. It checks the last picks of a Guest and of a user, and no Palette under the Lock.
func TestChromePaletteMain(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := paletteApp(t, sim)
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	b.open(srv.URL + "/")
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`(() => { const t = document.getElementById('src'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); t.focus(); return true; })()`)

	gt := func(id string) {
		b.eval(fmt.Sprintf(`document.getElementById(%q).dispatchEvent(new KeyboardEvent('keydown', { key: '>', shiftKey: true, bubbles: true, cancelable: true })); true`, id))
	}
	gt("src")
	b.waitFor("> in an empty Source box opens the Palette", paletteShown+` && document.activeElement.matches('.sode-palette__q')`)
	b.waitFor("the empty Palette lists the categories", `document.querySelectorAll('#palette-list .sode-palette__row--cat').length >= 5`)
	if b.eval(`!!document.querySelector('#palette-list [data-key="page:home"]')`) == true {
		t.Error("Home lists Home")
	}
	paletteKey(b, ">", true)
	b.waitFor(">> searches for >", `document.getElementById('palette').hidden && document.getElementById('src').value === '>' && document.activeElement.id === 'src'`)
	gt("src")
	if b.eval(paletteShown) == true {
		t.Error("> after other text opened the Palette")
	}
	b.eval(`(() => { const t = document.getElementById('src'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); document.activeElement.blur(); return true; })()`)

	b.eval(ctrlShiftP)
	b.waitFor("Ctrl+Shift+P opens the Palette", paletteShown)
	paletteFind(b, "Studio In", "preset:global:p-in")
	paletteKey(b, "Enter", true)
	b.waitFor("Shift+Enter fills the boxes", `document.getElementById('palette').hidden && document.getElementById('src').value.includes('`+simSrcName+`')`)
	if b.eval(`document.getElementById('confirm').hidden`) != true {
		t.Error("Shift+Enter opened the Confirm")
	}
	b.eval(`document.getElementById('src').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)

	b.eval(ctrlShiftP)
	paletteFind(b, "Studio In", "preset:global:p-in")
	paletteKey(b, "Enter", false)
	b.waitFor("Enter fires the preset through the Confirm", `!document.getElementById('confirm').hidden`)
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("a Guest's picks stay in This browser", `JSON.parse(localStorage.getItem('sode-palette-picks') || '[]')[0] === 'preset:global:p-in'`)

	b.eval(`document.getElementById('settings-open').click(); true`)
	b.eval(`document.getElementById('account-login').click(); true`)
	signInAs(b, "Bo", "5678")
	b.eval(`document.getElementById('settings-back').click(); true`)

	b.eval(ctrlShiftP)
	b.waitFor("the Palette opens for Bo", paletteShown)
	paletteFind(b, "Studio", "preset:global:p-in")
	if b.eval(`!!document.querySelector('#palette-list [data-key="preset:global:p-out"]')`) == true {
		t.Error("the Palette lists a preset outside Bo's Rights")
	}
	// A bookmark with Show in Palette keeps its grey row, with the reason and the offers.
	b.eval(`localStorage.setItem('sode-bookmarks', JSON.stringify([{ key: 'preset:global:p-out', label: 'Studio Out', newTab: false, inPalette: true }])); import('./core.js').then((m) => { m.emit('bookmarks'); return true })`)
	paletteFind(b, "Studio Out", "preset:global:p-out")
	b.waitFor("a bookmarked preset outside Bo's Rights is grey", `document.querySelector('#palette-list [data-key="preset:global:p-out"]').matches('.sode-palette__row--off') && document.querySelector('#palette-list [data-key="preset:global:p-out"]').textContent.includes('Outside your Rights')`)
	paletteKey(b, "Enter", false)
	b.waitFor("picking it says why and offers Step in", `!document.querySelector('.sode-palette__why').hidden && document.querySelector('.sode-palette__why').textContent.includes('Step in')`)
	if b.eval(`document.getElementById('confirm').hidden`) != true {
		t.Error("a grey preset opened the Confirm")
	}
	b.eval(`document.querySelector('.sode-palette__q').focus(); document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)
	b.eval(`localStorage.setItem('sode-bookmarks', JSON.stringify([{ key: 'setting:branding/cfg-brandtext', label: 'Logo text', newTab: false, inPalette: true }])); import('./core.js').then((m) => { m.emit('bookmarks'); return true })`)
	paletteFind(b, "Logo text", "setting:branding/cfg-brandtext")
	paletteKey(b, "Enter", false)
	b.waitFor("a bookmarked setting outside Bo's Role offers the Approval flow", `document.querySelector('.sode-palette__why').textContent.includes('Approval')`)
	b.eval(`document.querySelector('.sode-palette__q').focus(); document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); localStorage.removeItem('sode-bookmarks'); true`)

	// The first word picks a category: its rows come first, the other matches follow.
	paletteFind(b, "set theme", "setting:this-browser/cfg-theme")
	b.waitFor("the line above the list names the category", `document.querySelector('.sode-palette__where').textContent === 'Settings first'`)

	paletteFind(b, "Theme", "setting:this-browser/cfg-theme")
	paletteKey(b, "Enter", false)
	b.waitFor("a setting row opens Settings at it", `document.documentElement.dataset.view === 'settings' && document.activeElement.id === 'cfg-theme'`)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		u, _ := a.r().cfg.FindUserByName("Bo")
		if slices.Equal(u.Profile.PalettePicks, []string{"setting:this-browser/cfg-theme"}) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Bo's picks in Me: %v", u.Profile.PalettePicks)
		}
	}

	b.eval(`document.getElementById('lock-btn').click(); true`)
	b.waitFor("the Lock is up", `document.documentElement.hasAttribute('data-sode-lock')`)
	b.eval(ctrlShiftP)
	if b.eval(paletteShown) == true {
		t.Error("Ctrl+Shift+P opened the Palette under the Lock")
	}
	b.check()
}

// On a panel page with a key, the Palette follows the browser's own sign-in. Bo's Rights hide a preset that the panel's
// client can fire, and the panels are Bo's. A preset fires on Home as Bo. On a Kiosk panel the Palette does nothing.
func TestChromePalettePanelPage(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := paletteApp(t, sim)
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	keyed := makePanel(t, a, ana, `{"name":"Hall grid","type":"grid","acts":"api","settings":{"mode":"take","levels":[1]}}`)
	wall := makePanel(t, a, ana, `{"name":"Wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}]}`)
	kiosk := makePanel(t, a, ana, `{"name":"Kiosk wall","share":{"mode":"everyone"},"cells":[{"kind":"source","n":1}],"settings":{"kiosk":true}}`)
	waitNamesAndTally(t, a, ana)
	secret := instanceKey(t, a, keyed, ana, "Hall")

	b.open(srv.URL + "/")
	b.waitFor("Sign in offered", `!document.getElementById('account-login').hidden`)
	b.eval(`document.getElementById('settings-open').click(); document.getElementById('account-login').click(); true`)
	signInAs(b, "Bo", "5678")

	b.open(srv.URL + "/panels/" + keyed + "?instance=Hall#key=" + strings.TrimPrefix(secret, keyPrefix))
	b.waitFor("the grid", gridXp(simSrc, simDst)+` !== null`)
	b.eval(ctrlShiftP)
	b.waitFor("Ctrl+Shift+P opens the Palette on a panel page", paletteShown)
	b.waitFor("Home is first", `document.querySelector('#palette-list [data-key]')?.dataset.key === 'page:home'`)
	paletteFind(b, "Wall", "panel:"+wall)
	b.eval(`(() => { const i = document.querySelector('.sode-palette__q'); i.value = 'Hall grid'; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	if b.eval(`!!document.querySelector('#palette-list [data-key="panel:`+keyed+`"]')`) == true {
		t.Error("the Palette lists a panel that only the panel's key reaches")
	}
	paletteFind(b, "Studio", "preset:global:p-in")
	if b.eval(`!!document.querySelector('#palette-list [data-key="preset:global:p-out"]')`) == true {
		t.Error("the Palette lists a preset outside Bo's Rights, which the panel's client can fire")
	}
	paletteFind(b, "Studio In", "preset:global:p-in")
	paletteKey(b, "Enter", false)
	b.waitFor("the preset opens its Confirm on Home", `location.pathname === '/' && !document.getElementById('confirm').hidden && !document.getElementById('confirm-ok').disabled`)
	b.eval(`document.getElementById('confirm-ok').click(); true`)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		body := apiDo(a, "GET", "/api/v1/history", ana, nil).Body.String()
		if strings.Contains(body, `"client":"Bo"`) && strings.Contains(body, `Studio In`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("no take by Bo: %s", body)
		}
	}

	b.open(srv.URL + "/panels/" + kiosk)
	b.waitFor("Kiosk mode", `document.documentElement.hasAttribute('data-sode-kiosk')`)
	b.eval(ctrlShiftP)
	if b.eval(paletteShown) == true {
		t.Error("Ctrl+Shift+P opened the Palette on a Kiosk panel")
	}
	b.check()
}
