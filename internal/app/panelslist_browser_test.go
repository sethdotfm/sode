package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// The panels page's keyboard: the Find box is fuzzy and Ctrl+J and K move the highlight, Enter hands
// focus to Launch, and Ctrl+E edits and Ctrl+Enter launches the highlighted panel from anywhere.
func TestChromePanelsListKeys(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	makePanel(t, a, ana, `{"name":"Alpha desk"}`)
	makePanel(t, a, ana, `{"name":"Studio B"}`)
	wall := makePanel(t, a, ana, `{"name":"Studio Wall"}`)

	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
	b.waitFor("rows", `document.querySelectorAll('.sode-ov__item').length === 3`)

	key := func(target, k, code string, ctrl bool) {
		c := "false"
		if ctrl {
			c = "true"
		}
		b.eval(`(` + target + `).dispatchEvent(new KeyboardEvent('keydown', {key: '` + k + `', code: '` + code + `', ctrlKey: ` + c + `, bubbles: true, cancelable: true}))`)
	}
	find := `document.getElementById('panels-find')`
	cur := `document.querySelector('.sode-ov__item[aria-current="true"]')?.dataset.id`

	// "stdw" is a subsequence of Studio Wall only, and the highlight lands on it.
	b.eval(`(() => { const f = ` + find + `; f.focus(); f.value = 'stdw'; f.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	b.waitFor("fuzzy match", `document.querySelectorAll('.sode-ov__item').length === 1 && `+cur+` === '`+wall+`'`)
	b.eval(`(() => { const f = ` + find + `; f.value = 'studio'; f.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	b.waitFor("two studios", `document.querySelectorAll('.sode-ov__item').length === 2`)
	first, _ := b.eval(cur).(string)
	// The highlight stayed on Studio Wall, the last row, so K moves up first and J back.
	key(find, "k", "KeyK", true)
	b.waitFor("Ctrl+K moves up", cur+` !== '`+first+`'`)
	key(find, "j", "KeyJ", true)
	b.waitFor("Ctrl+J moves down", cur+` === '`+first+`'`)
	key(find, "Enter", "Enter", false)
	b.waitFor("Enter focuses Launch", `document.activeElement?.textContent.includes('Launch')`)
	b.waitFor("hints on the buttons", `document.querySelector('#panels-detail a[href] kbd')?.textContent.endsWith('Enter') || document.querySelector('#panels-detail a[href] kbd')?.textContent === '⌘↵'`)

	// Anywhere: Ctrl+E edits, Ctrl+Enter launches, Ctrl+J and K step.
	b.eval(`document.activeElement.blur()`)
	key(`document.body`, "e", "KeyE", true)
	b.waitFor("Ctrl+E opens the editor", `location.pathname.startsWith('/panels/editor')`)
	b.open(srv.URL + "/panels")
	b.waitFor("rows again", `document.querySelectorAll('.sode-ov__item').length === 3`)
	b.eval(`document.querySelector('.sode-ov__item').click()`)
	b.waitFor("selected", `!!`+cur)
	want, _ := b.eval(cur).(string)
	key(`document.body`, "Enter", "Enter", true)
	b.waitFor("Ctrl+Enter launches", `location.pathname === '/panels/`+want+`'`)
}
