package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The Open now list reloads and renames a keyless page from the panels page, and a closed one reads Offline with its
// controls off. Two browsers, since one tab runs the panel page and the other the panels page.
func TestChromeOpenNowReloadsAndRenamesAKeylessPage(t *testing.T) {
	page := startBrowser(t)
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) { c.Users[0].Role = config.RoleTrusted })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Studio A"}`)
	p, _ := a.panels.get(id)
	held := func(what, name string) {
		t.Helper()
		for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
			rows := a.mon.keylessOf(p)
			for _, r := range rows {
				if r.Device == name && r.Stream {
					return
				}
			}
			if time.Now().After(deadline) {
				t.Fatalf("timed out waiting for %s: %+v", what, rows)
			}
		}
	}
	// A Session-mode page with nobody signed in waits as a keyless page.
	page.open(srv.URL + "/panels/" + id + "?instance=Wall")
	held("the keyless page", "Wall")
	page.eval(`window.sodeMark = 1`)

	b.open(srv.URL + "/panels")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	b.padName("Ana")
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
	b.waitFor("row of the panel", `!!document.querySelector('[data-id="`+id+`"].sode-ov__item')`)
	b.eval(`document.querySelector('[data-id="` + id + `"].sode-ov__item').click()`)

	reload := `document.querySelector('#panels-open [data-reload-instance="Wall"]')`
	b.waitFor("Reload on the keyless row", `!!`+reload+` && !`+reload+`.disabled && !!document.querySelector('#panels-open [data-reload-all]')`)
	b.eval(reload + `.click()`)
	page.waitFor("the page reloaded", `!window.sodeMark && document.readyState === 'complete'`)
	held("the reloaded page", "Wall")

	renameBtn := `document.querySelector('#panels-open [data-rename-instance="Wall"]')`
	b.waitFor("the Rename icon on the keyless row", `!!`+renameBtn+` && !`+renameBtn+`.disabled`)
	b.eval(renameBtn + `.click()`)
	b.waitFor("the name field", `document.activeElement?.dataset.renameField === 'Wall'`)
	b.eval(`(() => { const f = document.activeElement; f.value = 'Porch'; f.dispatchEvent(new KeyboardEvent('keydown', { key: 'Enter', bubbles: true })); })()`)
	page.waitFor("the address carries the new name", `new URLSearchParams(location.search).get('instance') === 'Porch'`)
	held("the page under its new name", "Porch")

	page.open("about:blank")
	porch := `document.querySelector('#panels-open [data-reload-instance="Porch"]')`
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if rows := a.mon.keylessOf(p); len(rows) == 1 && rows[0].Device == "Porch" && !rows[0].Stream {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("the closed page still holds its stream: %+v", a.mon.keylessOf(p))
		}
	}
	b.waitFor("the closed page reads Offline", `!!`+porch+` && `+porch+`.disabled && `+porch+`.closest('.sode-ov__inst').textContent.includes('Offline')`)
	b.shot("open-now-offline")

	if row := lastRow(t, a, "reload"); !strings.Contains(row.By, "Ana") || !strings.Contains(row.Note, "Wall") {
		t.Fatalf("row %+v", row)
	}
	if row := lastRow(t, a, "rename"); !strings.Contains(row.Note, "Wall") || !strings.Contains(row.Note, "Porch") {
		t.Fatalf("row %+v", row)
	}
	b.check()
}
