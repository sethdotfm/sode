package app

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Secure mode on the panel page: a setup link opens the panel once, the page keeps its own key and
// drops the token from the address, a reload still works and the link does not. Rotating the key
// from elsewhere ends the open page, which says so.
func TestChromePanelSetupLink(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"api"}`)
	token, _ := setupLink(t, a, ana, id)

	b.open(srv.URL + "/panels/" + id + "#setup=" + token)
	b.waitFor("the panel opened", `document.getElementById('panel-title')?.textContent === 'Wall'`)
	b.waitFor("the token left the address", `location.hash === ''`)
	b.waitFor("the key kept in the browser", `(localStorage.getItem('sode-panel-key-`+id+`') || '').startsWith('sode_')`)
	wantErr(t, exchangeSetup(a, apiPeer, id, token), 401, "invalid_setup")

	b.open(srv.URL + "/panels/" + id)
	b.waitFor("a reload opens it again", `document.getElementById('panel-title')?.textContent === 'Wall' && document.getElementById('panels-note')?.hidden`)

	mintKey(t, a, ana, id)
	b.waitFor("the open page says its key is gone", `document.getElementById('panels-note')?.textContent.includes('rotated or revoked')`)
	// No b.check(): the 401 that ended the page is a console line by design.
}

// The Link with key tab of the Share modal: Ana (Trusted) makes a link that stays masked until
// shown, rotates it behind a Confirm, then revokes it with the letter key and limits the networks.
// The server is asked each time, since only it knows whether a key is alive.
func TestChromeShareModalKeyTab(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"api"}`)
	session := makePanel(t, a, ana, `{"name":"Mine"}`)
	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)

	tabText := `document.querySelector('.sode-ov__share #panel-key').textContent`
	openTab := func(panel string) {
		b.waitFor("row of "+panel, `!!document.querySelector('[data-id="`+panel+`"].sode-ov__item')`)
		b.eval(`document.querySelector('[data-id="` + panel + `"].sode-ov__item').click()`)
		b.waitFor("Share button", `!!document.querySelector('[data-act="share"]')`)
		b.eval(`document.querySelector('[data-act="share"]').click()`)
		b.waitFor("Link with key tab", `!!document.querySelector('.sode-ov__share #tab-key')`)
		b.eval(`document.querySelector('.sode-ov__share #tab-key').click()`)
	}
	closeModal := func() {
		b.eval(`document.querySelector('.sode-ov__share .pb-modal__footer .pb-btn--outline').click()`)
		b.waitFor("modal closed", `!document.querySelector('.sode-ov__share')`)
	}
	click := func(label string) {
		b.eval(`[...document.querySelectorAll('.sode-ov__share #panel-key button')].find(x => x.textContent.trim() === ` + strconv.Quote(label) + `).click()`)
	}
	confirm := func(viaKey bool) {
		b.waitFor("Confirm ready", `!!document.querySelector('.sode-ov__keyconfirm:not([hidden]) .pb-btn--ok:not([disabled])')`)
		if viaKey {
			b.eval(`(() => { const k = document.querySelector('.sode-ov__keyconfirm kbd'); k.dispatchEvent(new KeyboardEvent('keydown', {key: k.textContent.toLowerCase(), bubbles: true})); })()`)
		} else {
			b.eval(`document.querySelector('.sode-ov__keyconfirm .pb-btn--ok').click()`)
		}
	}
	input := `document.querySelector('.sode-ov__keylink input.pb-input')`

	openTab(session)
	b.waitFor("a session panel says it has no key", strings.ReplaceAll(tabText, "textContent", "textContent.includes('has no key')"))
	closeModal()

	openTab(id)
	b.waitFor("no key yet", tabText+`.includes('This panel has no key.')`)
	click("Make link")
	b.waitFor("masked link", input+`?.value.includes('#key=') && !`+input+`.value.includes('sode_')`)
	b.waitFor("no QR while masked", `!document.querySelector('.sode-ov__keylink .sode__qr')`)
	b.eval(`document.querySelector('.sode-ov__keylink .pb-check__input').click()`)
	b.waitFor("full link and QR", input+`.value.startsWith(location.origin + '/panels/`+id+`#key=') && !`+input+`.value.includes('sode_') && !!document.querySelector('.sode-ov__keylink .sode__qr')`)
	link, _ := b.eval(input + `.value`).(string)
	b.shot("share-key-link")
	old := keyPrefix + strings.TrimPrefix(link[strings.Index(link, "#key="):], "#key=")
	if rec := apiSend(a, "GET", "/api/v1/tally", old, ""); rec.Code != 200 {
		t.Fatalf("the made key: %d", rec.Code)
	}

	click("Rotate key")
	b.waitFor("Confirm shown", `!!document.querySelector('.sode-ov__keyconfirm:not([hidden]) kbd')`)
	if rec := apiSend(a, "GET", "/api/v1/tally", old, ""); rec.Code != 200 {
		t.Fatal("rotating began before the Confirm")
	}
	confirm(false)
	b.waitFor("rotated", tabText+`.includes('Rotated')`)
	wantErr(t, apiSend(a, "GET", "/api/v1/tally", old, ""), 401, "invalid_key")

	click("Revoke")
	confirm(true)
	b.waitFor("revoked", tabText+`.includes('Revoked')`)
	if a.keys.holds(keyOwner{Kind: actorClient, Name: panelClientName(id)}) {
		t.Fatal("a key is left after Revoke")
	}

	b.eval(`(() => { const n = document.querySelector('.sode-ov__share #key-networks'); n.value = '192.0.2.0/24'; n.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	click("Save networks")
	b.waitFor("networks saved", tabText+`.includes('works only from these networks')`)
	if c, _ := clientOf(t, a, panelClientName(id)); len(c.Networks) != 1 || c.Networks[0] != "192.0.2.0/24" {
		t.Fatalf("networks %v", c.Networks)
	}
	b.check()
}
