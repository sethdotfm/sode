package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// A panel page whose key was revoked shows the ended-key note, is listed as a Keyless instance with Key ended, and a
// Trusted user assigns it from the panels page: it switches with no prompt, runs the target and opens it again.
func TestChromeKeylessPageIsAssignedAndRuns(t *testing.T) {
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
	from := makePanel(t, a, ana, `{"name":"Studio A","acts":"api"}`)
	to := makePanel(t, a, ana, `{"name":"Studio B","acts":"api","slug":"studio-b"}`)
	secret := instanceKey(t, a, from, ana, "Lobby")
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+from+"/key?instance=Lobby", ana, ""); rec.Code/100 != 2 {
		t.Fatalf("revoke: %d %s", rec.Code, rec.Body.String())
	}
	until := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); !ok(); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	fromPanel, _ := a.panels.get(from)
	b.call("Target.createTarget", map[string]any{"url": srv.URL + "/panels/" + from + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix) + "&kiosk=1"})
	until("the page announces itself with its key ended", func() bool {
		rows := a.mon.keylessOf(fromPanel)
		return len(rows) == 1 && rows[0].Device == "Lobby" && rows[0].SignIn == "ended" && rows[0].Stream
	})

	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
	b.waitFor("row of the target", `!!document.querySelector('[data-id="`+to+`"].sode-ov__item')`)
	b.eval(`document.querySelector('[data-id="` + to + `"].sode-ov__item').click()`)
	b.waitFor("Assign to", `!!document.querySelector('[data-act="assign"]')`)
	b.eval(`document.querySelector('[data-act="assign"]').click()`)
	row := `document.querySelector('#assign-list [data-keyless][data-instance="Lobby"]')`
	b.waitFor("the keyless page in the list", `!!`+row+` && !`+row+`.disabled && `+row+`.closest('.sode-ov__cp').textContent.includes('Studio A') && `+row+`.closest('.sode-ov__cp').textContent.includes('127.0.0.1, Key ended, up ')`)
	b.eval(row + `.click()`)
	b.waitFor("the Assign was sent", `document.querySelector('.sode-ov__share [role="status"]').textContent.includes('Sent Lobby to Studio B')`)

	until("the page runs the target", func() bool { _, ok := a.mon.streamDevice(to, "Lobby"); return ok })
	if rows := a.mon.keylessOf(fromPanel); len(rows) != 0 {
		t.Fatalf("still keyless: %v", rows)
	}
	var targets struct{ TargetInfos []struct{ Type, URL string } }
	var addr string
	until("the page's new address", func() bool {
		_ = json.Unmarshal(b.call("Target.getTargets", nil), &targets)
		for _, x := range targets.TargetInfos {
			if x.Type == "page" && strings.Contains(x.URL, "/panels/studio-b") {
				addr = x.URL
			}
		}
		return addr != ""
	})
	if !strings.Contains(addr, "?instance=Lobby") || !strings.Contains(addr, "kiosk=1") || strings.Contains(addr, "key=") {
		t.Fatalf("address %s", addr)
	}

	// The ended-key note, seen on a second open of the old link in this tab; then the target's address again.
	b.open(srv.URL + "/panels/" + from + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix))
	b.waitFor("the ended-key note", `document.getElementById('panels-note')?.textContent.includes("This link's key no longer works")`)
	b.open(addr)
	b.waitFor("the target opens again from its address", `document.getElementById('panel-title')?.textContent === 'Studio B' && document.getElementById('panels-note')?.hidden`)
	// The ended key's refusal logs as a failed load; anything else is a real page error.
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 401") {
			t.Errorf("page error: %s", e)
		}
	}
}
