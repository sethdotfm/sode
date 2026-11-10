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

// Assign from the panels page: a running panel page in another tab switches to the target with no prompt, keeps its
// instance name and kiosk, drops the old key from its address, and its address and stored key open the target again.
func TestChromeAssignSwitchesARunningPanelPage(t *testing.T) {
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
	to := makePanel(t, a, ana, `{"name":"Studio B","acts":"api","slug":"studio-b","settings":{"fixedDests":[2]}}`)
	rec := apiSend(a, "POST", "/api/v1/panels/"+from+"/key", ana, `{"instance":"Lobby"}`)
	if rec.Code != 201 {
		t.Fatalf("key: %d %s", rec.Code, rec.Body.String())
	}
	secret := apiBody(t, rec)["key"].(string)
	until := func(what string, ok func() bool) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); !ok(); time.Sleep(50 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s", what)
			}
		}
	}
	held := func(id string) func() bool {
		return func() bool { _, ok := a.mon.streamDevice(id, "Lobby"); return ok }
	}

	b.call("Target.createTarget", map[string]any{"url": srv.URL + "/panels/" + from + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix) + "&kiosk=1"})
	until("the panel page holds its stream", held(from))

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
	row := `document.querySelector('#assign-list [data-instance="Lobby"][data-from="` + from + `"]')`
	b.waitFor("the open page in the list", `!!`+row+` && !`+row+`.disabled && `+row+`.closest('.sode-ov__cp').textContent.includes('Studio A') && `+row+`.closest('.sode-ov__cp').textContent.includes("Signed in as the panel's client, up ")`)
	b.eval(row + `.click()`)
	b.waitFor("the Assign was sent", `document.querySelector('.sode-ov__share [role="status"]').textContent.includes('Sent Lobby to Studio B')`)

	until("the page runs the target", held(to))
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
	if fd := panelOf(t, a, to, ana)["settings"].(map[string]any)["fixedDests"]; len(fd.([]any)) != 1 {
		t.Fatalf("fixed destinations %v", fd)
	}

	// Same browser profile, so this tab opens the address with the key the assigned page stored: a reload.
	b.open(addr)
	b.waitFor("the target opens again from its address", `document.getElementById('panel-title')?.textContent === 'Studio B' && document.getElementById('panels-note')?.hidden`)
	b.waitFor("the key is stored under the short name", `(localStorage.getItem('sode-panel-key-studio-b') || '').startsWith('sode_')`)
	b.check()
}

// Assign to a panel that acts as a user: the running page drops its key, opens the target and shows its own sign-in,
// so the person at the device types their own PIN.
func TestChromeAssignToAUserModePanelShowsTheDevicesSignIn(t *testing.T) {
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
	to := makePanel(t, a, ana, `{"name":"Desk","slug":"desk"}`)
	rec := apiSend(a, "POST", "/api/v1/panels/"+from+"/key", ana, `{"instance":"Lobby"}`)
	if rec.Code != 201 {
		t.Fatalf("key: %d %s", rec.Code, rec.Body.String())
	}
	secret := apiBody(t, rec)["key"].(string)

	b.open(srv.URL + "/panels/" + from + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix) + "&kiosk=1")
	b.waitFor("the keyed page runs", `document.getElementById('panel-title')?.textContent === 'Studio A'`)
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if _, ok := a.mon.streamDevice(from, "Lobby"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the page never held its stream")
		}
	}
	// A key stored earlier under the target's address must not open it as a client.
	b.eval(`localStorage.setItem('sode-panel-key-desk', 'sode_stale')`)

	if rec := apiSend(a, "POST", "/api/v1/panels/"+to+"/assign", ana, `{"from":"`+from+`","instance":"Lobby"}`); rec.Code != 200 {
		t.Fatalf("assign: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("the device shows its own sign-in", `location.pathname.endsWith('/panels/desk') && document.getElementById('panels-note')?.textContent.includes('Sign in with your PIN')`)
	b.waitFor("the address keeps its name and kiosk without a key", `location.search === '?instance=Lobby' && location.hash === '#kiosk=1'`)
	b.waitFor("no key is stored for the target", `localStorage.getItem('sode-panel-key-desk') === null`)
	b.check()
}
