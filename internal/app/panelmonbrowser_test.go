package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// The panels page: Watch a panel, then a missed keepalive raises a toast and the badge, and the
// return a second toast. The clock jumps instead of waiting 45 seconds.
func TestChromePanelWatchToast(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	var skew atomic.Int64
	a.mon.now = func() time.Time { return time.Now().Add(time.Duration(skew.Load())) }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall panel"}`)

	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	b.waitFor("Watch checkbox", `!!document.getElementById('panels-watch')`)
	b.eval(`document.getElementById('panels-watch').click()`)
	for deadline := time.Now().Add(10 * time.Second); !a.mon.watching("Ana", id); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the checkbox did not set the watch")
		}
	}
	b.shot("watching")

	if c := keepalive(a, id, ana, "Lobby tablet"); c != 200 {
		t.Fatalf("keepalive %d", c)
	}
	b.waitFor("online mark", `document.querySelector('[data-online]')?.textContent === 'All Online'`)
	// Open now lists the instance with its own Watch box, which watches only that instance.
	b.waitFor("instance watch box", `!!document.querySelector('[data-watch-instance="Lobby tablet"]')`)
	b.eval(`document.querySelector('[data-watch-instance="Lobby tablet"]').click()`)
	for deadline := time.Now().Add(10 * time.Second); !a.mon.watchingInst("Ana", id, "Lobby tablet"); time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the instance checkbox did not set the instance watch")
		}
	}

	skew.Store(int64(time.Minute))
	b.waitFor("lost toast", `/Wall panel is offline/.test(document.querySelector('[data-panel-toast="lost"]')?.textContent || '') && /Lobby tablet/.test(document.querySelector('[data-panel-toast="lost"]').textContent)`)
	b.waitFor("badge", `document.querySelector('[data-panel-badge]')?.textContent === '1 panel offline' && !document.querySelector('[data-panel-badge]').hidden`)
	b.waitFor("toast title links to the panel", fmt.Sprintf(`document.querySelector('[data-panel-toast="lost"] a')?.getAttribute('href') === 'panels/%s'`, id))
	first, _ := b.eval(`document.querySelector('[data-panel-toast="lost"] .pb-toast__msg').textContent`).(string)
	b.waitFor("the silence counts up", fmt.Sprintf(`document.querySelector('[data-panel-toast="lost"] .pb-toast__msg').textContent !== %q`, first))
	b.waitFor("last seen wording", `/last seen/.test(document.getElementById('panels-open')?.textContent || '')`)
	b.shot("lost")

	skew.Store(0)
	if c := keepalive(a, id, ana, "Lobby tablet"); c != 200 {
		t.Fatalf("keepalive %d", c)
	}
	b.waitFor("return toast", `/Lobby tablet is back/.test(document.querySelector('[data-panel-toast="back"]')?.textContent || '')`)
	b.waitFor("badge gone", `document.querySelector('[data-panel-badge]').hidden`)
	b.shot("back")

	// Each instance has an uptime strip, here with a drop in its last slot, and a Hide that ends nothing.
	b.waitFor("uptime strip", `document.querySelectorAll('#panels-open .sode-up__bar').length === 96 && /1 drop/.test(document.getElementById('panels-open').textContent)`)
	b.eval(`document.querySelectorAll('.pb-toast').forEach((t) => t.remove()); document.getElementById('panels-open').scrollIntoView()`)
	b.shot("uptime")
	b.eval(`document.querySelector('[data-hide-instance="Lobby tablet"]').click()`)
	b.waitFor("instance hidden", `!document.querySelector('[data-hide-instance]') && /Hidden: Lobby tablet/.test(document.getElementById('panels-open').textContent)`)
	b.waitFor("nothing counted", `document.querySelector('[data-online]')?.textContent === 'All Offline'`)
	if !a.mon.watchingInst("Ana", id, "Lobby tablet") {
		t.Fatal("Hide ended the instance watch")
	}
	b.shot("hidden")
	b.eval(`document.querySelector('[data-show-hidden]').click()`)
	b.waitFor("shown again", `!!document.querySelector('[data-hide-instance="Lobby tablet"]') && document.querySelector('[data-online]').textContent === 'All Online'`)
	b.check()
}

// The Sode page: a watcher signed in there sees a banner for a panel that went offline, clears it
// for now, sees it again for the next outage, and stops monitoring from the same menu.
func TestChromePanelOutageBanner(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	var skew atomic.Int64
	a.mon.now = func() time.Time { return time.Now().Add(time.Duration(skew.Load())) }
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall panel"}`)
	watchPanel(t, a, id, ana)

	b.open(srv.URL + "/")
	// The Sode page is not locked here, so raise the sign-in pad the way the user chip does.
	// The offer waits for the server's lock state, so it is repeated until the pad is up.
	b.waitFor("PIN pad", `(import('/core.js').then((m) => m.emit('login')), !!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent)`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	lose := func() {
		keepalive(a, id, ana, "Lobby tablet")
		skew.Add(int64(time.Minute))
		a.monSweep()
	}
	lose()
	b.waitFor("banner", `/Wall panel is offline/.test(document.querySelector('[data-panel-outage]')?.textContent || '')`)
	b.waitFor("banner is drawn on the page", `document.querySelector('[data-panel-outage]').getBoundingClientRect().height > 0`)
	b.waitFor("no floating badge", `document.querySelector('[data-panel-badge]').hidden`)
	b.waitFor("pad closed", `!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.shot("banner")

	b.eval(`document.querySelector('[data-panel-outage] [data-panel-alarm="menu"]').click()`)
	b.waitFor("clear menu", `!!document.querySelector('[data-panel-alarm="clear"]') && /Stop monitoring this panel/.test(document.querySelector('[data-panel-alarm="stop"]').textContent)`)
	b.shot("menu")
	b.eval(`document.querySelector('[data-panel-alarm="clear"]').click()`)
	b.waitFor("banner cleared", `!document.querySelector('[data-panel-outage]').textContent`)
	if !a.mon.watching("Ana", id) {
		t.Fatal("Clear alarm for now ended the watch")
	}

	keepalive(a, id, ana, "Lobby tablet") // back
	lose()                                // lost again
	b.waitFor("banner again", `/Wall panel is offline/.test(document.querySelector('[data-panel-outage]')?.textContent || '')`)
	b.eval(`document.querySelector('[data-panel-alarm="menu"]').click()`)
	b.waitFor("stop button", `!!document.querySelector('[data-panel-alarm="stop"]')`)
	b.eval(`document.querySelector('[data-panel-alarm="stop"]').click()`)
	b.waitFor("banner gone", `!document.querySelector('[data-panel-outage]').textContent`)
	if a.mon.watching("Ana", id) {
		t.Fatal("Stop monitoring kept the watch")
	}
	b.check()
}
