package app

import (
	"context"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// With "Ask users to set an instance name" off the page never asks and shows no name in the header, yet it
// still beats under its address, and ?instance= or a stored name still apply. On, the question is back.
func TestChromePanelAskInstanceNameOff(t *testing.T) {
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
	id := makePanel(t, a, ana, `{"name":"Wall","acts":"api","cells":[{"r":1,"c":1,"kind":"instanceName"}],"settings":{"askInstanceName":false}}`)
	secret, _ := mintKey(t, a, ana, id)
	base := srv.URL + "/panels/" + id
	frag := "#key=" + strings.TrimPrefix(secret, keyPrefix)

	waitDevice := func(want string) {
		t.Helper()
		var body string
		for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
			body = apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Body.String()
			if strings.Contains(body, `"device":"`+want+`"`) {
				return
			}
		}
		t.Fatalf("no instance %q: %s", want, body)
	}
	quiet := func(cell string) {
		t.Helper()
		b.waitFor("the Instance Name cell reads "+cell, `document.getElementById('panel-grid')?.textContent.includes(`+strconv.Quote(cell)+`)`)
		if got := b.eval(`document.getElementById('panel-device').hidden && document.getElementById('panel-instance').hidden`); got != true {
			t.Fatalf("a name question or header field shows: %v", got)
		}
	}

	b.open(base + frag)
	b.waitFor("the panel opened", `document.getElementById('panel-title')?.textContent === 'Wall'`)
	waitDevice("127.0.0.1")
	quiet("Instance Name")

	b.eval(`localStorage.setItem('sode-panel-device', 'Kiosk 3')`)
	b.open(base + "?x=1" + frag)
	waitDevice("Kiosk 3")
	quiet("Kiosk 3")

	b.open(base + "?instance=Studio%20B" + frag)
	waitDevice("Studio B")
	quiet("Studio B")

	// On again: an unnamed browser is asked, as before the setting.
	apiSend(a, "PUT", "/api/v1/panels/"+id, ana, `{"name":"Wall","acts":"api","cells":[{"r":1,"c":1,"kind":"instanceName"}],"settings":{}}`)
	b.eval(`localStorage.removeItem('sode-panel-device')`)
	b.open(base + "?x=2" + frag)
	b.waitFor("the device question", `!document.getElementById('panel-device').hidden`)
}
