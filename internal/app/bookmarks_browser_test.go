package app

import (
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

const bookmarkChips = `[...document.querySelectorAll('#patch-view [data-presets] [data-bookmark]')]`

func altDigit(b *tab, d string, shift bool) {
	b.t.Helper()
	b.eval(`document.dispatchEvent(new KeyboardEvent('keydown', { key: '` + d + `', code: 'Digit` + d + `', altKey: true, shiftKey: ` + map[bool]string{true: "true", false: "false"}[shift] + `, bubbles: true, cancelable: true })); true`)
}

// The Bookmarks strip on Home, with Everyone's list: Alt+1 fires a preset through the Confirm and Alt+Shift+1 fills
// the boxes. A deleted preset shows Missing. Signed in as Bo, a Settings button bookmark presses the button. A preset
// outside his Rights shows No access, and a click opens the Palette's why. The Palette's Alt+B adds to Me, so the strip shows Me. The editor refuses a javascript:
// address, and a This browser list wins over Me.
func TestChromeBookmarks(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		bo := mkUser(t, "Bo", "5678")
		bo.Rights = config.Rights{Presets: &[]string{"p-in"}}
		c.GuestRights = config.Rights{Presets: &[]string{"p-in"}} // as Bo, else Bo has the guest's presets too (ActsWithGuest)
		c.Users = append(c.Users, bo)
		c.Presets = []config.Preset{
			{ID: "p-in", Name: "Studio In", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst)}},
			{ID: "p-out", Name: "Studio Out", Items: []config.PresetItem{presetItem(simSrc, []int{1}, simDst+1)}},
		}
		c.Bookmarks = []config.Bookmark{
			{Key: "preset:global:p-in", Label: "Old name", InPalette: true},
			{Key: "preset:global:p-out", Label: "Studio Out"},
			{Key: "preset:global:gone", Label: "Gone show"},
			{URL: "https://example.com/rundown", Label: "Rundown", NewTab: true},
			{Key: "setting:my-user/account-profile", Label: "My profile"},
		}
	})
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))

	b.open(srv.URL + "/")
	b.waitFor("Everyone's five bookmarks, the preset under its live name", bookmarkChips+`.length === 5 && `+bookmarkChips+`[0].textContent.startsWith('Studio In')`)
	b.waitFor("a deleted preset shows Missing", bookmarkChips+`[2].dataset.state === 'missing' && `+bookmarkChips+`[2].textContent.includes('Missing')`)
	b.waitFor("names", `(() => { const t = document.getElementById('src'); t.value = 'Zoom'; t.dispatchEvent(new Event('input', { bubbles: true })); return !!document.getElementById('src-opt-0'); })()`)
	b.eval(`(() => { const t = document.getElementById('src'); t.value = ''; t.dispatchEvent(new Event('input', { bubbles: true })); t.blur(); return true; })()`)

	altDigit(b, "1", false)
	b.waitFor("Alt+1 fires the preset through the Confirm", `!document.getElementById('confirm').hidden`)
	b.eval(`document.getElementById('confirm-cancel').click(); true`)
	b.waitFor("confirm closed", `document.getElementById('confirm').hidden`)
	altDigit(b, "1", true)
	b.waitFor("Alt+Shift+1 fills the boxes", `document.getElementById('src').value.includes('`+simSrcName+`')`)
	if b.eval(`document.getElementById('confirm').hidden`) != true {
		t.Error("Alt+Shift+1 opened the Confirm")
	}
	b.eval(`document.getElementById('src').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)

	b.eval(`document.getElementById('settings-open').click(); document.getElementById('account-login').click(); true`)
	signInAs(b, "Bo", "5678")
	b.eval(`document.getElementById('settings-back').click(); true`)
	b.waitFor("outside Bo's Rights: grey with No access", bookmarkChips+`[1].dataset.state === 'noAccess' && `+bookmarkChips+`[1].textContent.includes('No access')`)
	b.eval(bookmarkChips + `[4].click(); true`)
	b.waitFor("a Settings button bookmark presses the button", `!!document.getElementById('profiles-title')?.offsetParent`)
	b.eval(`document.querySelector('#profiles-title').closest('.pb-modal').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)
	b.waitFor("the profile dialog closed", `!document.getElementById('profiles-title')?.offsetParent`)
	b.eval(`document.getElementById('settings-back').click(); true`)
	b.eval(bookmarkChips + `[1].click(); true`)
	b.waitFor("a click shows the Palette's why", paletteShown+` && !document.querySelector('.sode-palette__why').hidden && document.querySelector('.sode-palette__why').textContent.includes('Outside your Rights')`)
	if b.eval(`document.getElementById('confirm').hidden`) != true {
		t.Error("a No access bookmark opened the Confirm")
	}
	b.eval(`document.querySelector('.sode-palette__why button:last-child').click(); true`)
	b.eval(`(() => { const i = document.querySelector('.sode-palette__q'); i.value = ''; i.dispatchEvent(new Event('input', { bubbles: true })); return true; })()`)
	b.waitFor("Show in Palette lists the bookmark first", `document.querySelector('#palette-list .sode-palette__head')?.textContent === 'Bookmarks' && document.querySelector('#palette-list [data-key="preset:global:p-in"]') !== null`)

	paletteFind(b, "Communication Log", "page:log")
	b.eval(`document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: 'b', code: 'KeyB', altKey: true, bubbles: true, cancelable: true })); true`)
	for deadline := time.Now().Add(10 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		u, _ := a.r().cfg.FindUserByName("Bo")
		if len(u.Profile.Bookmarks) == 1 && u.Profile.Bookmarks[0].Key == "page:log" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Bo's bookmarks in Me: %+v", u.Profile.Bookmarks)
		}
	}
	b.eval(`document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)
	b.eval(`document.querySelector('.sode-palette__q').dispatchEvent(new KeyboardEvent('keydown', { key: 'Escape', bubbles: true, cancelable: true })); true`)
	b.waitFor("the strip shows Me, not Everyone", bookmarkChips+`.length === 1 && `+bookmarkChips+`[0].textContent.startsWith('Communication Log')`)

	b.eval(`document.querySelector('#patch-view [data-presets] .sode__chipbtn').click(); true`)
	b.waitFor("the editor", `!!document.getElementById('bm-dialog')?.offsetParent`)
	b.eval(`(() => { const s = document.getElementById('bm-scope'); s.value = 'local'; s.dispatchEvent(new Event('change')); return true; })()`)
	b.eval(`document.getElementById('bm-url').value = 'javascript:alert(1)'; document.getElementById('bm-add-url').click(); true`)
	b.waitFor("a javascript: address is refused", `document.getElementById('bm-note').textContent.includes('http://') && document.querySelectorAll('#bm-list [data-i]').length === 0`)
	b.eval(`document.getElementById('bm-url').value = 'https://example.com/x'; document.getElementById('bm-url-name').value = 'Example'; document.getElementById('bm-add-url').click(); true`)
	b.waitFor("the address is a row", `document.querySelectorAll('#bm-list [data-i]').length === 1`)
	if b.eval(`document.querySelector('#bm-list [data-opt="newTab"]').checked`) != true {
		t.Error("a web address does not open in a new tab by default")
	}
	b.eval(`document.getElementById('bm-save').click(); true`)
	b.waitFor("This browser's list wins over Me", `document.getElementById('bm-dialog').closest('[hidden]') !== null && `+bookmarkChips+`.length === 1 && `+bookmarkChips+`[0].textContent.startsWith('Example')`)
	if got := b.eval(`JSON.parse(localStorage.getItem('sode-bookmarks'))[0].url`); got != "https://example.com/x" {
		t.Errorf("sode-bookmarks holds %v", got)
	}
	b.check()
}
