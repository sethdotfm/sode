package app

import (
	"context"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// Rename from the panels page: the running panel page, named by its link, takes the new name in its header and its
// address, and a reload keeps it. Two browsers, since one tab runs the panel page and the other the panels page.
func TestChromeRenameARunningPanelPageFromThePanelsPage(t *testing.T) {
	page := startBrowser(t)
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
	id := makePanel(t, a, ana, `{"name":"Studio A","acts":"api"}`)
	rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/key", ana, `{"instance":"Lobby"}`)
	if rec.Code != 201 {
		t.Fatalf("key: %d %s", rec.Code, rec.Body.String())
	}
	secret := apiBody(t, rec)["key"].(string)
	// Only a Trusted user makes a panel with a client; past that, Ana renames as its owner.
	cfg := a.r().cfg
	cfg.Users = slices.Clone(cfg.Users)
	cfg.Users[0].Trusted = false
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}

	page.open(srv.URL + "/panels/" + id + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix))
	page.waitFor("the panel page names itself", `document.getElementById('panel-instance')?.textContent.includes('Lobby')`)
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if _, ok := a.mon.streamDevice(id, "Lobby"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the page never held its stream")
		}
	}

	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
	b.waitFor("row of the panel", `!!document.querySelector('[data-id="`+id+`"].sode-ov__item')`)
	b.eval(`document.querySelector('[data-id="` + id + `"].sode-ov__item').click()`)
	pencil := `document.querySelector('#panels-open [data-rename-instance="Lobby"]')`
	b.waitFor("the pencil on the open instance", `!!`+pencil+` && !`+pencil+`.disabled`)
	b.eval(pencil + `.click()`)
	b.waitFor("the name field", `document.activeElement?.dataset.renameField === 'Lobby'`)
	b.eval(`(() => { const f = document.activeElement; f.value = 'Front desk'; f.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); })()`)

	page.waitFor("the header takes the new name", `document.getElementById('panel-instance').textContent.includes('Front desk')`)
	page.waitFor("the address carries it", `new URLSearchParams(location.search).get('instance') === 'Front desk' && !location.hash.includes('key=')`)
	b.waitFor("the Open now list follows", `!!document.querySelector('#panels-open [data-rename-instance="Front desk"]') && !document.querySelector('#panels-open [data-rename-instance="Lobby"]')`)
	if keys := apiSend(a, "GET", "/api/v1/panels/"+id+"/keys", ana, "").Body.String(); !strings.Contains(keys, `"name":"Front desk"`) {
		t.Fatalf("keys: %s", keys)
	}

	page.call("Page.reload", nil)
	page.waitFor("a reload keeps the name", `document.getElementById('panel-title')?.textContent === 'Studio A' && document.getElementById('panel-instance').textContent.includes('Front desk')`)
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if _, ok := a.mon.streamDevice(id, "Front desk"); ok {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the reloaded page never held its stream under the new name")
		}
	}
	if row := lastRow(t, a, "rename"); !strings.Contains(row.Note, "Lobby") || !strings.Contains(row.Note, "Front desk") {
		t.Fatalf("row %+v", row)
	}
	page.check()
	b.check()
}
