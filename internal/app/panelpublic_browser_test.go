package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/web"
)

// A Public panel's plain URL opens with no sign-in and no PIN pad, even with a stale key in the
// fragment. A take inside the client's Rights works; one outside is refused with the reason. The
// panel page lock still covers it, as it covers a keyed link (docs/panels-design.md).
func TestChromePublicPanel(t *testing.T) {
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
	const other = 5
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Lobby","acts":"public","clientRights":{"dsts":[%d]},"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"destination","n":%d},
		{"r":1,"c":3,"kind":"source","n":%d},
		{"r":1,"c":4,"kind":"take"}],"settings":{"mode":"take"}}`, simDst, simDst, other, simSrc))
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"routes":[{`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}

	stale := strings.TrimPrefix(newPublicSecret(), keyPrefix)
	b.open(srv.URL + "/panels/" + id + "?instance=Lobby#key=" + stale)
	cell := func(kind string, i int) string {
		return fmt.Sprintf(`document.querySelectorAll('.sode-panel__cell--%s')[%d]`, kind, i)
	}
	b.waitFor("the plain URL opened the panel", `document.getElementById('panel-title')?.textContent === 'Lobby' && document.querySelectorAll('.sode-panel__cell').length === 4`)
	b.waitFor("destination named", cell("destination", 0)+`?.querySelector('span')?.textContent === 'DSM1'`)
	b.waitFor("without a PIN pad or a sign-in", `!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent && document.getElementById('panels-signin').hidden`)
	b.waitFor("the stale key left the address", `!location.hash.includes('key=')`)
	b.shot("public")

	b.eval(cell("destination", 1) + `.click()`)
	b.eval(cell("source", 0) + `.click()`)
	b.waitFor("refusal shown", `document.getElementById('panel-status').textContent.includes("outside this panel's rights")`)
	b.waitFor("Take not armed", cell("take", 0)+`.disabled`)

	b.eval(cell("destination", 0) + `.click()`)
	b.eval(cell("source", 0) + `.click()`)
	b.waitFor("Take armed", `!`+cell("take", 0)+`.disabled`)
	b.eval(cell("take", 0) + `.click()`)
	b.waitFor("Connected result", cell("destination", 0)+`?.dataset.result === 'connected'`)
	rec := apiDo(a, "GET", "/api/v1/tally", ana, nil)
	if !strings.Contains(rec.Body.String(), fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, simDst, simSrc)) {
		t.Errorf("tally after take: %s", rec.Body.String())
	}
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Actor == nil || es[0].Actor.Name != "Panel "+id {
		t.Errorf("history names %+v, want the panel's client", es)
	}
	// The stale key's one refusal is the only error the page may log.
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 401") {
			t.Errorf("page error: %s", e)
		}
	}
}

// Public from the editor: the Access tab saves it at once with its note and warning, the Share
// modal mutes Link with key and shows the plain link, and that link opens with no sign-in.
func TestChromeEditorPublic(t *testing.T) {
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
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Foyer","acts":"session","cells":[{"r":1,"c":1,"kind":"destination","n":%d}]}`, simDst))
	b.open(srv.URL + "/panels/editor?panel=" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)
	b.waitFor("lock gone", `!document.documentElement.dataset.sodeLock`)
	b.waitFor("panel loaded", `document.getElementById('ed-name').value === 'Foyer' && !!document.querySelector('#ed-side .pb-tab')`)

	access := `document.getElementById('panel-ed-access').innerText`
	b.eval(`document.getElementById('tab-ed-access').click(); true`)
	b.waitFor("Public is offered", `!!document.getElementById('ed-acts-public') && !document.getElementById('ed-acts-public').disabled`)
	b.waitFor("no Public warning on a User panel", `!`+access+`.includes('Its sharing does not limit')`)
	b.eval(`document.getElementById('ed-acts-public').click(); true`)
	b.waitFor("Public saves at once with its note", `document.getElementById('ed-state').textContent === 'Saved' && `+access+`.includes('Anyone with the link can use the panel')`)
	b.waitFor("the Access tab warns about sharing", access+`.includes('Its sharing does not limit who may open it') && `+access+`.includes('Listed on the panels page for:') && !`+access+`.includes('Who may use it')`)
	b.waitFor("the toolbar offers Copy link", `!document.getElementById('ed-copylink').hidden`)
	if body := apiDo(a, "GET", "/api/v1/panels/"+id, ana, nil).Body.String(); !strings.Contains(body, `"acts":"public"`) || !strings.Contains(body, `"client":"Panel `+id+`"`) {
		t.Errorf("the saved panel is not Public with a client: %s", body)
	}

	b.eval(`document.getElementById('ed-share').click(); true`)
	keyTab := `[...document.querySelectorAll('.sode-ov__share .pb-tab')].find((x) => x.textContent === 'Link with key')`
	b.waitFor("Link with key is muted", keyTab+`?.getAttribute('aria-disabled') === 'true'`)
	b.eval(keyTab + `.click(); true`)
	b.waitFor("the tab shows the plain link", `(document.querySelector('.sode-ov__keytab .sode-linkrow__text')?.value || '').endsWith('/panels/`+id+`')`)
	link, _ := b.eval(`document.querySelector('.sode-ov__keytab .sode-linkrow__text').value`).(string)
	b.shot("editor-public")
	b.check()

	// Storage cleared, so the page has no session to fall back on.
	b.eval(`localStorage.clear(); sessionStorage.clear(); true`)
	b.open(link)
	b.waitFor("the plain link opened the panel", `document.getElementById('panel-title')?.textContent === 'Foyer' && document.querySelectorAll('.sode-panel__cell').length === 1`)
	b.waitFor("without a PIN pad or a sign-in", `!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent && document.getElementById('panels-signin').hidden`)
	b.check()
}
