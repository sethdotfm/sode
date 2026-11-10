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

	setName := func(name string) {
		b.eval(`(() => { const n = document.getElementById('key-instance'); n.value = ` + strconv.Quote(name) + `; n.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	}
	keyOf := func() string {
		b.eval(`document.querySelector('.sode-ov__keylink .pb-check__input').click()`)
		b.waitFor("full link and QR", input+`.value.startsWith(location.origin + '/panels/`+id+`?instance=') && !`+input+`.value.includes('sode_') && !!document.querySelector('.sode-ov__keylink .sode__qr')`)
		link, _ := b.eval(input + `.value`).(string)
		return keyPrefix + strings.TrimPrefix(link[strings.Index(link, "#key="):], "#key=")
	}
	status := func(secret string) int { return apiSend(a, "GET", "/api/v1/tally", secret, "").Code }
	// A make that gives a key shows it in its own dialog, whole with its prefix; Done removes the dialog and the key.
	shownKey := func() string {
		b.waitFor("the key dialog", `document.querySelector('.sode-ov__keyshow .sode__secret-key')?.value.startsWith('`+keyPrefix+`') && !!document.querySelector('.sode-ov__keyshow .sode__qr')`)
		k, _ := b.eval(`document.querySelector('.sode-ov__keyshow .sode__secret-key').value`).(string)
		b.eval(`[...document.querySelectorAll('.sode-ov__keyshow button')].find(x => x.textContent.trim() === 'Done').click()`)
		b.waitFor("the key dialog is gone", `!document.querySelector('.sode-ov__keyshow')`)
		return k
	}

	// A session panel opens on People, and its Link with key tab is greyed but readable, with no key action.
	openTab(session)
	b.waitFor("People is the first tab of a session panel", `document.querySelector('.sode-ov__share #tab-people').getAttribute('aria-selected') === 'true'`)
	b.waitFor("the key tab is greyed", `document.querySelector('.sode-ov__share #tab-key').getAttribute('aria-disabled') === 'true'`)
	b.eval(`document.querySelector('.sode-ov__share #tab-key').click()`)
	b.waitFor("a session panel says it has no key", tabText+`.includes('has no key') && `+tabText+`.includes('asks for a sign-in')`)
	b.waitFor("no key action is offered", `!document.querySelector('.sode-ov__share #panel-key button') || ![...document.querySelectorAll('.sode-ov__share #panel-key button')].some(x => /Make|Rotate|Revoke/.test(x.textContent))`)
	closeModal()

	// An API panel opens on Link with key. A link needs a name.
	openTab(id)
	b.waitFor("Link with key is the first tab of a client panel", `document.querySelector('.sode-ov__share #tab-key').getAttribute('aria-selected') === 'true'`)
	b.waitFor("no instances yet", tabText+`.includes('This panel has no instances.')`)
	b.waitFor("the kinds are named for what they do", tabText+`.includes('Named instance link') && `+tabText+`.includes('One-time setup link') && !`+tabText+`.includes('Standing')`)
	b.waitFor("Make link waits for a name", `[...document.querySelectorAll('.sode-ov__share #panel-key button')].find(x => x.textContent.trim() === 'Make link').disabled`)
	setName("Lobby")
	click("Make link")
	shown := shownKey()
	b.waitFor("masked link", input+`?.value.includes('?instance=Lobby#key=') && !`+input+`.value.includes('sode_')`)
	b.waitFor("no QR while masked", `!document.querySelector('.sode-ov__keylink .sode__qr')`)
	lobby := keyOf()
	if shown != lobby {
		t.Fatal("the key dialog does not show the key in the link")
	}
	// The Kiosk switch puts kiosk=1 after the key, keeps ?instance=, and Copy link copies that link.
	b.eval(`window.__copied = ''; navigator.clipboard.writeText = async (t) => { window.__copied = t; }`)
	b.eval(`document.getElementById('key-view-kiosk').click()`)
	click("Copy link")
	b.waitFor("kiosk link copied", `/\?instance=Lobby#key=[^&]+&kiosk=1$/.test(window.__copied) && `+tabText+`.includes('Copied.')`)
	b.eval(`document.getElementById('key-view-regular').click()`)
	b.shot("share-key-link")
	if status(lobby) != 200 {
		t.Fatal("the made key does not work")
	}
	b.waitFor("the list names the instance", `!!document.querySelector('.sode-ov__keyitem') && document.querySelector('.sode-ov__keyitem').textContent.includes('Lobby') && document.querySelector('.sode-ov__keyitem').textContent.includes('Offline')`)

	// A second instance has its own key, and rotating or revoking one leaves the other.
	setName("Studio")
	click("Make link")
	shownKey()
	b.waitFor("second masked link", input+`?.value.includes('?instance=Studio#key=')`)
	studio := keyOf()
	if status(lobby) != 200 || status(studio) != 200 {
		t.Fatal("two instances do not both hold a key")
	}
	setName("lobby")
	b.waitFor("a held name rotates", `!![...document.querySelectorAll('.sode-ov__share #panel-key button')].find(x => x.textContent.trim() === 'Rotate this instance')`)
	b.eval(`document.querySelector('.sode-ov__share [aria-label="Rotate Lobby"]').click()`)
	b.waitFor("Confirm shown", `!!document.querySelector('.sode-ov__keyconfirm:not([hidden]) kbd')`)
	if status(lobby) != 200 {
		t.Fatal("rotating began before the Confirm")
	}
	confirm(false)
	shownKey()
	b.waitFor("rotated", tabText+`.includes('Lobby rotated')`)
	if status(lobby) != 401 || status(studio) != 200 {
		t.Fatalf("rotate Lobby: Lobby %d, Studio %d", status(lobby), status(studio))
	}

	b.eval(`document.querySelector('.sode-ov__share [aria-label="Revoke Studio"]').click()`)
	confirm(true)
	b.waitFor("revoked", tabText+`.includes('Revoked Studio')`)
	if status(studio) != 401 {
		t.Fatal("Studio's key works after Revoke")
	}
	if n := len(keyList(t, a, ana, id)); n != 1 {
		t.Fatalf("%d instances left, want 1", n)
	}

	b.eval(`(() => { const n = document.querySelector('.sode-ov__share #key-networks'); n.value = '192.0.2.0/24'; n.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	click("Save networks")
	b.waitFor("networks saved", tabText+`.includes('only from these networks')`)
	if c, _ := clientOf(t, a, panelClientName(id)); len(c.Networks) != 1 || c.Networks[0] != "192.0.2.0/24" {
		t.Fatalf("networks %v", c.Networks)
	}
	b.check()
}

// A plain key link: once the key works the page keeps it in the browser and the address loses it, keeping
// ?instance= and the rest of the fragment. A reload runs on the kept key, a rotate ends it, and with storage
// blocked the key stays in the address.
func TestChromePanelLinkKeyLeavesAddress(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Trusted = true })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"api"}`)
	secret, body := mintKey(t, a, ana, id)
	link, _ := body["link"].(string)
	kept := `localStorage.getItem('sode-panel-key-` + id + `')`
	opened := `document.getElementById('panel-title')?.textContent === 'Wall'`

	b.open(srv.URL + "/" + link + "&kiosk=1")
	b.waitFor("the panel opened", opened)
	b.waitFor("the key left the address", `location.hash === '#kiosk=1' && location.search === '?instance=Lobby'`)
	b.waitFor("the key kept in the browser", kept+` === '`+secret+`'`)

	b.eval(`window.__old = true; setTimeout(() => location.reload(), 0)`)
	b.waitFor("a reload runs on the kept key", `!window.__old && `+opened+` && document.getElementById('panels-note')?.hidden && document.documentElement.hasAttribute('data-sode-kiosk')`)

	mintKey(t, a, ana, id)
	b.waitFor("the stale page says its key was ended", `document.getElementById('panels-note')?.textContent.includes('rotated or revoked')`)
	b.waitFor("the ended key is forgotten", kept+` === null`)

	b.open("about:blank")
	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": `Storage.prototype.setItem = function () { throw new DOMException('full', 'QuotaExceededError'); };`})
	_, body = mintKey(t, a, ana, id)
	link, _ = body["link"].(string)
	b.open(srv.URL + "/" + link)
	b.waitFor("the panel opened without storage", opened)
	b.waitFor("the key stays in the address", `location.hash.startsWith('#key=') && `+kept+` === null`)
	// No b.check(): the 401 that ended the page is a console line by design.
}
