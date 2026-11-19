package app

import (
	"fmt"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func TestChromeInboxButton(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	show := mePreset("Show", 1, 2)
	a := simApp(t, sim, func(c *config.Config) {
		bo := mkUser(t, "Bo", "5678")
		bo.Profile.Presets = []config.Preset{show}
		c.Users = append(c.Users, bo)
	})
	srv := seamServer(t, a)
	// A queue no tally fills, so the server keeps Bo's connection for the whole check.
	bo := &client{ip: "10.0.0.9", send: make(chan []byte, 1<<14)}
	a.mu.Lock()
	a.clients[bo] = struct{}{}
	a.mu.Unlock()
	if err := a.UserUnlock(bo, "5678"); err != nil {
		t.Fatal(err)
	}
	send := func() {
		t.Helper()
		if _, _, err := a.SendCopy(bo, inboxReq{To: []string{"Ana"}, Presets: []cardPresetReq{{Scope: scopeMe, ID: show.ID}}}); err != nil {
			t.Fatal(err)
		}
	}
	const btn = `[...document.querySelectorAll('[data-inbox-button]')].find((x) => x.offsetParent)`
	count := func(n int) string {
		if n == 0 {
			return fmt.Sprintf(`!!%s && %s.querySelector('.sode-inbox__count').hidden`, btn, btn)
		}
		return fmt.Sprintf(`!!%s && %s.querySelector('.sode-inbox__count').textContent === '%d' && %s.getAttribute('aria-label').startsWith('Inbox, %d')`, btn, btn, n, btn, n)
	}

	b.open(srv.URL + "/#settings")
	b.waitFor("Settings open", `document.documentElement.dataset.view === 'settings'`)
	if b.eval(`!!(`+btn+`)`) == true {
		t.Fatal("an Inbox button shows with nobody signed in")
	}
	signInSettings(b)
	b.waitFor("the button in Settings, no count", count(0))
	send()
	b.waitFor("the count goes up live", count(1))
	b.waitFor("one toast for the new card", `document.querySelectorAll('[data-inbox-toast]').length === 1 && document.querySelector('[data-inbox-toast]').textContent.includes('Bo sent you 1 preset.')`)

	b.open(srv.URL + "/")
	b.waitFor("the Main page shows the count", count(1))
	for _, page := range []string{"/log", "/panels", "/panels/editor"} {
		b.open(srv.URL + page)
		b.waitFor(page+" shows the count", count(1))
	}
	if n := b.eval(`document.querySelectorAll('[data-inbox-toast]').length`); n != float64(0) {
		t.Fatalf("a page load showed %v toasts for an old card", n)
	}

	b.open(srv.URL + "/log")
	b.waitFor("the Log shows the count", count(1))
	send()
	b.waitFor("a second card, with its toast", count(2)+` && document.querySelectorAll('[data-inbox-toast]').length === 1`)
	b.eval(`document.querySelector('[data-inbox-toast] .pb-toast__close').click(); true`)
	b.eval(btn + `.click(); true`)
	b.waitFor("the Inbox lists both cards with Accept and Decline", `document.querySelectorAll('#inbox-cards [data-card]').length === 2 && [...document.querySelectorAll('#inbox-cards button')].some((x) => x.textContent === 'Accept…')`)
	b.shot("inbox-log")
	b.eval(`document.querySelector('#inbox-cards [data-decline]').click(); true`)
	b.waitFor("Decline lowers the count with no toast", count(1)+` && document.querySelectorAll('[data-inbox-toast]').length === 0`)

	b.eval(`[...document.querySelectorAll('#inbox-cards button')].find((x) => x.textContent === 'Accept…').click(); true`)
	b.waitFor("the review, item ticked", `!!document.getElementById('inbox-accept').offsetParent && document.querySelector('.sode-inbox__review [data-key="item:presets:`+show.ID+`"]')?.checked === true`)
	b.eval(`document.getElementById('inbox-accept').click(); true`)
	b.waitFor("Accept clears the count", count(0)+` && document.getElementById('inbox-note').textContent === 'Accepted.'`)
	if p := a.r().cfg.Users[0].Profile.Presets; len(p) != 1 || p[0].Name != "Show" {
		t.Fatalf("Ana's presets after Accept: %+v", p)
	}
	send()
	b.waitFor("a third card", count(1))

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 390, "height": 844, "deviceScaleFactor": 1, "mobile": true})
	b.call("Emulation.setTouchEmulationEnabled", map[string]any{"enabled": true, "maxTouchPoints": 5})
	b.open(srv.URL + "/")
	b.waitFor("the phone layout", `document.documentElement.dataset.layout === 'phone'`)
	b.waitFor("the phone header shows the count", count(1))
	b.eval(btn + `.click(); true`)
	b.waitFor("Decline only on the phone", `(() => {
		const btns = [...document.querySelectorAll('#inbox-cards button')].map((x) => x.textContent);
		return btns.length === 1 && btns[0] === 'Decline' && document.querySelector('.sode-inbox .sode__desktop-only')?.textContent.includes('Open Sode on a computer to accept a copy or withdraw one.');
	})()`)
	b.shot("inbox-phone")
	b.eval(`document.querySelector('#inbox-cards [data-decline]').click(); true`)
	b.waitFor("the last Decline clears the count", count(0))
	declined := func(k inboxCard) bool { return k.State == cardDeclined }
	for deadline := time.Now().Add(5 * time.Second); len(a.inbox.cards(declined)) != 2; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("declined cards: %+v", a.inbox.cards(func(inboxCard) bool { return true }))
		}
	}

	id := makePanel(t, a, userKey(t, a, "Ana"), `{"name":"Desk panel"}`)
	b.open(srv.URL + "/panels/" + id)
	b.waitFor("the PIN pad or the panel", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent || document.getElementById('panel-title')?.textContent === 'Desk panel'`)
	if b.eval(`!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`) == true {
		signInOnPad(b)
	}
	b.waitFor("the panel runs", `document.getElementById('panel-title')?.textContent === 'Desk panel'`)
	send()
	time.Sleep(time.Second)
	if got := b.eval(`document.querySelectorAll('[data-inbox-button], [data-inbox-toast]').length`); got != float64(0) {
		t.Fatalf("a panel page shows %v Inbox buttons or toasts", got)
	}
	b.check()
}
