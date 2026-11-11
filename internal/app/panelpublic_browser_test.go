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
// fragment and the panel page lock on. A take inside the client's Rights works; one outside is
// refused with the reason. The idle letter Lock still covers it, and a User or keyed API panel
// behind the same lock still shows the PIN pad (docs/panels-design.md).
const padShown = `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`

func TestChromePublicPanel(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim, func(c *config.Config) {
		c.Users[0].Trusted = true
		c.Locks = config.Locks{Panel: true}
		c.SettingsPin = mkUser(t, "", "2468").Pin
		c.AutoLockMinutes = 1
	})
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
	// .sode-lock exists once a lockState arrived, so the pad is not just still to come.
	b.waitFor("without a PIN pad or a sign-in", `document.querySelector('.sode-lock') && !document.documentElement.dataset.sodeLock && !`+padShown+` && document.getElementById('panels-signin').hidden`)
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
	// Idle past the auto-lock: the letter Lock, never the pad. The idle share's own clock stays real.
	b.eval(`window.realNow = Date.now; Date.now = () => window.realNow() + 2 * 60_000`)
	b.waitFor("idle letter Lock", `document.querySelector('.sode-lock__letter-line')?.offsetParent && !`+padShown)
	b.shot("public-idle")
	b.eval(`Date.now = window.realNow`)
	b.waitFor("Dismiss ready", `!document.querySelector('.sode-lock .pb-btn--ok')?.disabled`)
	b.eval(`document.querySelector('.sode-lock .pb-btn--ok').click()`)
	b.waitFor("letter Lock dismissed", `!document.documentElement.dataset.sodeLock`)

	desk := makePanel(t, a, ana, `{"name":"Desk","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	b.open(srv.URL + "/panels/" + desk)
	b.waitFor("a User panel still asks for the PIN", padShown)
	wall := makePanel(t, a, ana, `{"name":"Wall","acts":"api","cells":[{"r":1,"c":1,"kind":"take"}]}`)
	secret, _ := mintKey(t, a, ana, wall)
	b.open(srv.URL + "/panels/" + wall + "?instance=Lobby#key=" + strings.TrimPrefix(secret, keyPrefix))
	b.waitFor("the keyed API panel opened", `document.getElementById('panel-title')?.textContent === 'Wall'`)
	b.waitFor("a keyed API panel still asks for the PIN", padShown)

	// The stale key's one refusal is the only error the page may log.
	for _, e := range b.problems() {
		if !strings.Contains(e, "status of 401") {
			t.Errorf("page error: %s", e)
		}
	}
}
