package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// The Share modal on /panels: Ana names Bo, Bo can use the panel and not change it, then Ana
// goes back to Only me and Bo loses it. The router is the simulator, which the page never uses here.
func TestChromeShareModalPeople(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	a.cfgMu.Lock()
	err := a.applyConfig(func(cur *config.Config) error {
		cur.Users = append(cur.Users, mkUser(t, "Bo", "5678"))
		return nil
	})
	a.cfgMu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana, bo := userKey(t, a, "Ana"), userKey(t, a, "Bo")
	id := makePanel(t, a, ana, `{"name":"Shared wall"}`)
	b.open(srv.URL + "/panels")

	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)
	b.waitFor("Share button", `!!document.querySelector('[data-act="share"]')`)
	b.eval(`document.querySelector('[data-act="share"]').click()`)
	b.waitFor("modal with a People tab", `document.querySelector('.sode-ov__share #tab-people')?.getAttribute('aria-selected') === 'true'`)
	b.waitFor("Save waits for a change", `document.querySelector('.sode-ov__share .pb-modal__footer .pb-btn--ok').disabled === true`)
	b.eval(`document.querySelector('.sode-ov__share [data-mode="named"]').click()`)
	b.waitFor("Bo in the picker and the owner left out", `[...document.querySelectorAll('.sode-ov__people .pb-check__label')].map(x => x.textContent).join() === 'Bo'`)
	b.shot("share-named")
	b.eval(`document.querySelector('.sode-ov__people input').click()`)
	b.eval(`document.querySelector('.sode-ov__share .pb-modal__footer .pb-btn--ok').click()`)
	b.waitFor("modal closed", `!document.querySelector('.sode-ov__share')`)
	b.waitFor("detail shows who it is shared with", `[...document.querySelectorAll('.sode-ov__kv dd')].some(d => d.textContent === 'Bo')`)

	// The server, not the page, says what Bo may do.
	p := apiBody(t, apiDo(a, "GET", "/api/v1/panels/"+id, bo, nil))
	if p["canEdit"] != false || p["name"] != "Shared wall" {
		t.Fatalf("Bo's view %v", p)
	}
	wantErr(t, apiSend(a, "PUT", "/api/v1/panels/"+id, bo, `{"name":"mine"}`), 403, "not_owner")

	b.eval(`document.querySelector('[data-act="share"]').click()`)
	b.waitFor("modal reopened on Named users", `document.querySelector('.sode-ov__people input')?.checked === true`)
	b.eval(`document.querySelector('.sode-ov__share [data-mode="me"]').click()`)
	b.eval(`document.querySelector('.sode-ov__share .pb-modal__footer .pb-btn--ok').click()`)
	b.waitFor("shared with only the owner", `[...document.querySelectorAll('.sode-ov__kv dd')].some(d => d.textContent === 'Only the owner')`)
	if got := panelIDs(t, a, bo); got != "" {
		t.Fatalf("Bo still sees %q", got)
	}
	b.check()
}
