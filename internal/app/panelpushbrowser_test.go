package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// The Push modal on /panels: Ana pushes EVS 1 to the EVS group, saves a set, then restores a
// checkpoint. The router is the simulator, which the page never uses here.
func TestChromePushFlow(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	src := makePanel(t, a, ana, `{"name":"EVS 1","cells":[{"r":1,"c":1,"kind":"take"},{"r":1,"c":2,"kind":"clear"}]}`)
	e2 := makePanel(t, a, ana, `{"name":"EVS 2"}`)
	e3 := makePanel(t, a, ana, `{"name":"EVS 3"}`)
	b.open(srv.URL + "/panels")

	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(`document.querySelector('.sode-lock__key[data-key="` + string(d) + `"]').click()`)
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("Lock overlay gone", `!document.documentElement.hasAttribute('data-sode-lock')`)

	b.waitFor("Push button", `!!document.querySelector('[data-act="push"]')`)
	b.eval(`document.querySelector('[data-act="push"]').click()`)
	b.waitFor("the group box", `!!document.querySelector('.sode-ov__share input[data-group="EVS"]')`)
	b.waitFor("Push waits for a target", `document.querySelector('#panel-push .pb-btn--ok').disabled === true`)
	b.eval(`document.querySelector('.sode-ov__share input[data-group="EVS"]').click()`)
	b.waitFor("both targets ticked", `document.querySelectorAll('.sode-ov__indent input:checked').length === 2`)
	b.shot("push-group")
	b.eval(`document.querySelector('#panel-push .pb-btn--ok').click()`)
	b.waitFor("the answer", `document.querySelector('.sode-ov__share .pb-modal__footer .sode__hint').textContent.startsWith('Pushed to EVS 2 and EVS 3.')`)

	for _, id := range []string{e2, e3} {
		if p := readPanel(t, a, ana, id); cellCount(p) != 2 || len(checkpointsOf(t, a, ana, id)) != 1 {
			t.Fatalf("%s after Push: %v", id, p)
		}
	}

	// A set, then restore the checkpoint of EVS 2 from the Restore tab.
	b.eval(`document.querySelector('#tab-sets').click()`)
	b.waitFor("Save needs a name", `document.querySelector('#panel-sets .pb-btn--outline').disabled === true`)
	b.eval(`(() => { const i = document.querySelector('#panel-sets input.pb-input'); i.value = 'Show A'; i.dispatchEvent(new Event('input', {bubbles: true})); })()`)
	b.waitFor("Save enabled", `document.querySelector('#panel-sets .pb-btn--outline').disabled === false`)
	b.eval(`document.querySelector('#panel-sets .pb-btn--outline').click()`)
	b.waitFor("set saved", `document.querySelector('.sode-ov__share .pb-modal__footer .sode__hint').textContent === 'Saved the set Show A.'`)
	if n := len(apiBody(t, apiDo(a, "GET", "/api/v1/panel-sets", ana, nil))["sets"].([]any)); n != 1 {
		t.Fatalf("sets %d", n)
	}

	b.eval(`document.querySelector('#tab-restore').click()`)
	b.eval(`(() => { const s = document.querySelector('#panel-restore select'); s.value = '` + e2 + `'; s.dispatchEvent(new Event('change', {bubbles: true})); })()`)
	b.waitFor("a checkpoint row", `!!document.querySelector('#panel-restore [data-cp]')`)
	b.shot("push-restore")
	b.eval(`document.querySelector('#panel-restore [data-cp]').click()`)
	b.waitFor("restored", `document.querySelector('.sode-ov__share .pb-modal__footer .sode__hint').textContent.startsWith('Restored EVS 2')`)
	if p := readPanel(t, a, ana, e2); cellCount(p) != 0 || p["name"] != "EVS 2" {
		t.Fatalf("EVS 2 after restore: %v", p)
	}
	_ = src
	b.check()
}
