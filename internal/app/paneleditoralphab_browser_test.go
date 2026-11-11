package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// editorTab starts the app and a browser signed in as Ana (Trusted) on the editor for a new panel.
func editorTab(t *testing.T, body string) (*tab, *App, string, string) {
	return editorTabPrep(t, body, nil)
}

// editorTabPrep is editorTab with a hook that runs once the panel exists and before the page opens.
func editorTabPrep(t *testing.T, body string, prep func(b *tab, a *App, key, id string)) (*tab, *App, string, string) {
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
	id := makePanel(t, a, ana, body)
	if prep != nil {
		prep(b, a, ana, id)
	}
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value !== ''`)
	return b, a, ana, id
}

// The Link with key tab in the editor's Share modal must show something for both kinds of panel.
func TestChromeEditorKeyTab(t *testing.T) {
	for _, acts := range []string{"session", "api"} {
		t.Run(acts, func(t *testing.T) {
			b, _, _, id := editorTab(t, `{"name":"Wall","acts":"`+acts+`"}`)
			b.eval(`document.getElementById('tab-ed-access').click()`)
			b.waitFor("Share button", `!document.getElementById('ed-share').disabled`)
			b.eval(`document.getElementById('ed-share').click()`)
			b.waitFor("key tab", `!!document.querySelector('.sode-ov__share #tab-key')`)
			first := map[string]string{"session": "people", "api": "key"}[acts]
			b.waitFor("the modal opens on the tab that fits", `document.querySelector('.sode-ov__share #tab-`+first+`').getAttribute('aria-selected') === 'true'`)
			b.waitFor("only a session panel's key tab is greyed", `(document.querySelector('.sode-ov__share #tab-key').getAttribute('aria-disabled') === 'true') === `+fmt.Sprint(acts == "session"))
			b.eval(`document.querySelector('.sode-ov__share #tab-key').click()`)
			b.waitFor("key tab shows text", `document.querySelector('.sode-ov__share #panel-key').textContent.length > 20`)
			if acts == "session" {
				b.waitFor("key tab has the panel link", `document.querySelector('.sode-ov__share #panel-key .sode-linkrow__text')?.value.endsWith('/panels/`+id+`')`)
			}
			b.eval(`document.getElementById('tab-people').click()`)
			b.waitFor("People tab has the link, except for a client panel", `!!document.querySelector('.sode-ov__share #panel-people .sode-linkrow__text') === `+fmt.Sprint(acts == "session"))
			b.shot("keytab-" + acts)
			b.check()
		})
	}
}
