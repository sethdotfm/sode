package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

func seamServer(t *testing.T, a *App) *httptest.Server {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })
	return srv
}

func waitNamesAndTally(t *testing.T, a *App, key string) {
	t.Helper()
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", key, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", key, nil).Body.String(), `"routes":[{`) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}
}

// A keyless page assigned to a panel with fixed destinations patches them on the stored key.
// After a revoke of that key it shows the ended-key note and is keyless again.
func TestChromeSeamKeylessAssignRunsFixedDestsThenKeyEnds(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleTrusted })
	srv := seamServer(t, a)

	ana := userKey(t, a, "Ana")
	const other = 5
	from := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Studio A","acts":"api","slug":"studio-a","settings":{"mode":"take","fixedDests":[{"n":%d}]}}`, other))
	to := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Studio B","acts":"api","slug":"studio-b","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"source","n":%d},{"r":1,"c":2,"kind":"take"}],"settings":{"mode":"take","fixedDests":[{"n":%d},{"n":%d}]}}`, simSrc, simDst, other))
	secret := instanceKey(t, a, from, ana, "Lobby")
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+from+"/key?instance=Lobby", ana, ""); rec.Code/100 != 2 {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	beforeFrom, beforeTo := unwritten(t, a, from, ana), unwritten(t, a, to, ana)
	waitNamesAndTally(t, a, ana)
	until := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); !ok(); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}

	b.open(srv.URL + "/panels/studio-a?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix) + "&kiosk=1")
	b.waitFor("the ended-key note", `document.getElementById('panels-note')?.textContent.includes("Key rotated or revoked")`)
	fromPanel, _ := a.panels.get(from)
	var kid string
	until("the page announces itself keyless", func() bool {
		rows := a.mon.keylessOf(fromPanel)
		if len(rows) == 1 && rows[0].Device == "Lobby" && rows[0].SignIn == "ended" {
			kid = rows[0].Keyless
		}
		return kid != ""
	})
	if code, m := keylessAssign(a, to, ana, kid, ""); code != 200 {
		t.Fatalf("assign: %d %v", code, m)
	}

	b.waitFor("the page runs the target by its short name", `location.pathname === '/panels/studio-b' && document.getElementById('panel-title')?.textContent === 'Studio B' && !!document.querySelector('.sode-panel__cell--source')`)
	if got := b.eval(`location.search + '|' + location.hash`).(string); !strings.Contains(got, "instance=Lobby") || !strings.Contains(got, "kiosk=1") || strings.Contains(got, "key=") {
		t.Fatalf("address %s", got)
	}
	if b.eval(`!!localStorage.getItem('sode-panel-key-studio-b')`) != true {
		t.Fatal("the key is not stored under the short name")
	}
	until("the target holds the instance", func() bool { _, ok := a.mon.streamDevice(to, "Lobby"); return ok })
	if got := unwritten(t, a, from, ana); got != beforeFrom {
		t.Errorf("the source panel changed: %s -> %s", beforeFrom, got)
	}
	if got := unwritten(t, a, to, ana); got != beforeTo {
		t.Errorf("the target changed: %s -> %s", beforeTo, got)
	}

	b.waitFor("no Destination cell is drawn", `!document.querySelector('.sode-panel__cell--destination')`)
	// A press before the first poll answers is dropped; the router's name comes with that poll.
	b.waitFor("the first poll answered", `document.querySelector('.sode-panel__cell--source .sode-panel__text')?.textContent === 'Zoom 1'`)
	b.eval(`document.querySelector('.sode-panel__cell--source').click()`)
	b.waitFor("Take armed", `!document.querySelector('.sode-panel__take').disabled`)
	b.eval(`document.querySelector('.sode-panel__take').click()`)
	for _, d := range []int{simDst, other} {
		waitRouted(t, a, ana, d, simSrc)
	}

	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+to+"/key?instance=Lobby", ana, ""); rec.Code/100 != 2 {
		t.Fatalf("revoke the assigned key: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("the ended-key note on the target", `document.getElementById('panels-note')?.textContent.includes("Key rotated or revoked")`)
	toPanel, _ := a.panels.get(to)
	until("the target's page announces itself keyless", func() bool {
		rows := a.mon.keylessOf(toPanel)
		return len(rows) == 1 && rows[0].Device == "Lobby" && rows[0].SignIn == "ended" && rows[0].Stream
	})
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 401") {
			t.Errorf("page error: %s", e)
		}
	}
}

// Icon B stays the corner badge of a library icon and of a Default artwork cell after a Push and a Copy.
func TestChromeSeamIconBBadgeAfterPushAndCopy(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleTrusted })
	srv := seamServer(t, a)

	ana := userKey(t, a, "Ana")
	ref := "lib:" + libIcon(t, a, ana, "Logo")
	src := makePanel(t, a, ana, `{"name":"Art"}`)
	img := apiBody(t, apiSendRaw(a, "POST", "/api/v1/panels/"+src+"/artwork", ana, imgBytes("png", 64, 64)))["id"].(string)
	if rec := apiSend(a, "PUT", "/api/v1/default-artwork", ana, `{"kind":"source","n":2,"art":"`+img+`","artB":"A","panel":"`+src+`"}`); rec.Code != 200 {
		t.Fatalf("default: %d %s", rec.Code, rec.Body.String())
	}
	cells := `[{"r":1,"c":1,"kind":"source","n":1,"art":"` + ref + `","artB":"2"},{"r":1,"c":2,"kind":"source","n":2}]`
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+src, ana, `{"name":"Art","acts":"api","rows":4,"cols":8,"cells":`+cells+`}`); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	dst := makePanel(t, a, ana, `{"name":"Pushed","acts":"api"}`)
	if code, body := pushTo(a, ana, src, dst); code != 200 {
		t.Fatalf("push: %d %v", code, body)
	}
	// A copy starts in Session mode. API mode lets the page open on a key with no sign-in.
	copied := apiBody(t, apiSend(a, "POST", "/api/v1/panels/"+src+"/copy", ana, ""))
	cp := copied["id"].(string)
	copied["acts"] = "api"
	body, _ := json.Marshal(copied)
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+cp, ana, string(body)); rec.Code != 200 {
		t.Fatalf("copy to API mode: %d %s", rec.Code, rec.Body.String())
	}
	waitNamesAndTally(t, a, ana)

	for _, id := range []string{cp, dst} {
		key := instanceKey(t, a, id, ana, "Wall")
		b.open(srv.URL + "/panels/" + id + "?instance=Wall#key=" + strings.TrimPrefix(key, keyPrefix) + "&kiosk=1")
		for i, what := range []string{"the library icon", "the Default artwork"} {
			cell := fmt.Sprintf(`document.querySelectorAll('.sode-panel__cell--source')[%d]`, i)
			b.waitFor(fmt.Sprintf("%s on %s draws with Icon B as a badge", what, id),
				`!!`+cell+` && !!`+cell+`.querySelector('.sode-panel__art--fill .sode-panel__art-a:not([hidden])') && !!`+cell+`.querySelector('.sode-panel__art-b--badge')`)
		}
	}
	b.check()
}

// Seam probe: the Lost link banner of the shared header (panelsbar.js) on /log, /panels and the panel editor, with the
// Server link down. Each page's router node has the name that routerName gives for the router, as the main page has.
// On a phone, /log and the editor keep the banner to a quarter of the screen with no sideways scroll.
func TestChromeSeamLostLinkEveryPage(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Routers[0].Title = "Studio North" })
	srv := seamServer(t, a)
	ana := userKey(t, a, "Ana")
	waitNamesAndTally(t, a, ana)
	sizeTo(b, 1920, 1080)

	addr := strings.TrimPrefix(srv.URL, "http://")
	srv.Close()
	var live *httptest.Server
	up := func() {
		l, err := net.Listen("tcp", addr)
		if err != nil {
			t.Fatal(err)
		}
		live = &httptest.Server{Listener: l, Config: &http.Server{Handler: a.Handler(web.FS)}}
		live.Start()
	}
	down := func() {
		live.CloseClientConnections()
		live.Close()
		b.eval(`(window.sodeSockets || []).forEach((s) => s.close()); true`)
	}
	up()
	t.Cleanup(func() { live.CloseClientConnections(); live.Close() })
	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": keepSockets})
	base := "http://" + addr

	const banner = `!document.getElementById('conn-down').hidden && document.querySelector('#conn-down .sode__down-title')?.textContent === 'Lost link: the Server link is down'
		&& document.documentElement.dataset.conn === 'down'`
	const noBanner = `document.getElementById('conn-down').hidden && document.documentElement.dataset.conn === 'up'`
	// The router node's words without its address, and what routerName gives for the page's first router.
	const nodeName = `(() => { const n = document.querySelectorAll('#conn-down .sode__down-node')[2]; return n ? [...n.childNodes].filter((x) => x.nodeType === 3).map((x) => x.textContent).join('').trim() : ''; })()`
	const ruleName = `Promise.all([import('./core.js'), import('./routerlinkrules.js')]).then(([c, r]) => r.routerName(c.routerData('')))`

	name := func(page string) string {
		t.Helper()
		got, _ := b.eval(nodeName).(string)
		want, _ := b.eval(ruleName).(string)
		if got == "" || got != want {
			t.Errorf("%s: the router node says %q, routerName gives %q", page, got, want)
		}
		return got
	}

	b.open(base + "/")
	b.waitFor("the main page has the hello", `document.documentElement.dataset.conn === 'up' && `+ruleName+`.then((n) => n.includes('Studio North'))`)
	down()
	b.waitFor("the main page shows the banner", `!document.getElementById('conn-down').hidden && !!document.querySelectorAll('#conn-down .sode__down-node')[2]`)
	home := name("main page")
	if !strings.Contains(home, "Studio North") {
		t.Errorf("main page: the router node %q does not have the Router title", home)
	}
	up()

	// ready is an element that the page shows only once it is signed in.
	pages := []struct{ what, path, head, ready string }{
		{"/log", "/log", ".sode-log__head", "#log-rows"},
		{"/panels", "/panels", ".sode-panels__head", "#panels-new"},
		{"the panel editor", "/panels/editor?new", ".sode-panels__head", "#ed-save"},
	}
	for _, p := range pages {
		b.open(base + p.path)
		ready := `(!document.documentElement.dataset.sodeLock && !!document.querySelector('` + p.ready + `')?.offsetParent)`
		// /panels and the editor offer the PIN pad a moment after they load, so a page that shows is not yet signed in.
		if p.path != "/log" {
			signed := `(document.querySelector('.sode-lock__who')?.textContent || '').includes('Ana')`
			b.waitFor(p.what+": the PIN pad or Ana signed in", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent || (`+ready+` && `+signed+`)`)
			if b.eval(`!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`).(bool) {
				signInOnPad(b)
			}
		}
		// The banner names the router from the hello, so the link goes down only after it.
		b.waitFor(p.what+": signed in, with the hello", ready+` && `+noBanner+` && `+ruleName+`.then((n) => n.includes('Studio North'))`)
		down()
		b.waitFor(p.what+": the Lost link banner", banner)
		b.waitFor(p.what+": the header starts below the banner", `document.querySelector('`+p.head+`').getBoundingClientRect().top >= document.getElementById('conn-down').getBoundingClientRect().bottom - 0.5`)
		b.shot("lost" + strings.NewReplacer("/", "-", "?", "-").Replace(p.path))
		if got := name(p.what); got != home {
			t.Errorf("%s: the router node %q, the main page has %q", p.what, got, home)
		}
		up()
		b.waitFor(p.what+": the banner goes", noBanner)
	}

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 375, "height": 812, "deviceScaleFactor": 1, "mobile": true})
	const phone = `(() => {
		const h = document.getElementById('conn-down').getBoundingClientRect().height;
		return h > 0 && h <= innerHeight / 4 && document.documentElement.scrollWidth <= innerWidth;
	})()`
	for _, p := range []struct{ what, path string }{{"phone /log", "/log"}, {"phone editor", "/panels/editor?new"}} {
		b.open(base + p.path)
		b.waitFor(p.what+": no banner", `!document.documentElement.dataset.sodeLock && `+noBanner)
		down()
		b.waitFor(p.what+": the banner is at most a quarter of the screen, with no sideways scroll", banner+` && `+phone)
		b.shot("lost-phone" + strings.NewReplacer("/", "-", "?", "-").Replace(p.path))
		up()
	}

	for _, e := range b.problems() {
		if !strings.Contains(e, "ERR_CONNECTION_REFUSED") && !strings.Contains(e, "ERR_CONNECTION_RESET") && !strings.Contains(e, "WebSocket") && !strings.Contains(e, "/ws") {
			t.Errorf("page error: %s", e)
		}
	}
}

// Seam probe: no standalone button keeps pb-btn--sm in the Settings Routers head or on the main page (Bookmarks,
// Filters, Clear filters, Show all).
func TestChromeSeamNoSmallStandaloneButtons(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleAdministrator })
	srv := seamServer(t, a)
	waitNamesAndTally(t, a, userKey(t, a, "Ana"))
	sizeTo(b, 1920, 1080)

	b.open(srv.URL + "/")
	b.waitFor("the Bookmarks strip and Filters", `!!document.querySelector('[data-presets] .sode__presets-all')?.offsetParent && !!document.querySelector('[data-filters]')?.offsetParent`)
	small := func(what, js string) {
		t.Helper()
		if got, _ := b.eval(js).(string); got != "" {
			t.Errorf("%s keep pb-btn--sm: %s", what, got)
		}
	}
	const list = `.filter((x) => x.classList.contains('pb-btn--sm') && !x.classList.contains('pb-btn--icon')).map((x) => x.textContent.trim()).join(', ')`
	small("Bookmarks strip buttons", `[...document.querySelectorAll('[data-presets] .pb-btn')]`+list)
	small("Filters buttons", `[...document.querySelectorAll('[data-filters]')]`+list)
	small("Show all buttons", `[...document.querySelectorAll('#src-all .pb-btn, #dst-all .pb-btn')]`+list)
	// Clear filters shows under History when the filter sets hide every entry; emptyNote draws it.
	small("Clear filters", `Promise.all([import('./searchbox.js'), import('./core.js')]).then(([s, c]) => {
		c.S.hist.push({});
		try {
			const x = [...s.emptyNote([], false).querySelectorAll('.pb-btn')];
			if (!x.some((y) => y.textContent === 'Clear filters')) return 'no Clear filters button';
			return x`+list+`;
		} finally { c.S.hist.pop(); }
	})`)

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	signInSettings(b)
	b.waitFor("the Routers head", `[...document.querySelectorAll('#router-detail .sode__racts > .pb-btn')].some((x) => x.offsetParent)`)
	small("Settings Routers head buttons", `[...document.querySelectorAll('#router-detail .sode__racts .pb-btn')]`+list)
	b.sameMiddle("router name and its buttons", "#router-detail-name", "#router-detail .sode__racts > .pb-btn")
	b.check()
}

// Seam probe: the lock screen shows a wrong PIN wait in words (waitWords in pin.go) on the message line.
// At 1366x768 the pad does not move.
func TestChromeSeamLockWaitInWords(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Locks = config.Locks{Main: true}
		c.PinWaits = true
	})
	srv := seamServer(t, a)
	sizeTo(b, 1366, 768)
	b.open(srv.URL + "/")
	b.waitFor("the pad", `!!document.querySelector('.sode-lock.is-on') && !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.waitFor("the pad has faded in", `getComputedStyle(document.querySelector('.sode-lock__body')).opacity === '1' && document.getAnimations().every((a) => a.playState !== 'running')`)
	b.padName("Ana")
	const keys = `JSON.stringify([...document.querySelectorAll('.sode-lock__key')].map((k) => { const r = k.getBoundingClientRect(); return [Math.round(r.left * 2), Math.round(r.top * 2)]; }))`
	before, _ := b.eval(keys).(string)

	const msg = `document.querySelector('.sode-lock__body .sode-lock__msg')`
	// Ana's PIN has 4 digits, so the pad sends the fourth digit on its own.
	const waits = `/^Try again in \d+ seconds$/.test(` + msg + `.textContent)`
	for i := 1; ; i++ {
		if i > pinFreeTries+1 {
			t.Fatalf("no wait after %d wrong PINs", i-1)
		}
		for _, d := range "9999" {
			b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
		}
		b.waitFor(fmt.Sprintf("try %d: Incorrect or the wait", i), `document.querySelector('.sode-lock').textContent.includes('Incorrect') || `+waits)
		if b.eval(waits).(bool) {
			break
		}
		b.waitFor(fmt.Sprintf("try %d: Incorrect goes", i), `!document.querySelector('.sode-lock').textContent.includes('Incorrect')`)
	}
	b.waitFor("the wait in words on the message line", waits+` && `+msg+`.classList.contains('pb-note--error')`)
	b.waitFor("no animation running", `document.getAnimations().every((a) => a.playState !== 'running')`)
	b.shot("lock-wait")
	if got, _ := b.eval(`document.querySelector('.sode-lock').textContent`).(string); regexp.MustCompile(`\d+m\d*s|\ds\b`).MatchString(got) {
		t.Errorf("the lock shows a Go duration: %q", got)
	}
	if after, _ := b.eval(keys).(string); after != before {
		t.Errorf("the pad keys moved with the wait shown: %s, before %s", after, before)
	}
	if d := b.eval(`document.documentElement.scrollHeight - innerHeight`).(float64); d > 0.5 {
		t.Errorf("the Lock scrolls by %.1f px with the wait shown", d)
	}
	b.check()
}
