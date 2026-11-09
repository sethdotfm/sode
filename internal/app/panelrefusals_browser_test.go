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

// A kiosk panel draws only the grid, and a route outside the user's Rights is refused on the
// page before Take arms.
func TestChromePanelKioskAndRefusal(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	a.r().cfg.Users[0].Rights = config.Rights{Dsts: ints(simDst)}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(srv.Close)

	ana := userKey(t, a, "Ana")
	const other = 5
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Kiosk panel","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"destination","n":%d},
		{"r":1,"c":3,"kind":"source","n":%d},
		{"r":1,"c":4,"kind":"take"}],"settings":{"mode":"take","kiosk":true}}`, simDst, other, simSrc))

	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if strings.Contains(apiDo(a, "GET", "/api/v1/names", ana, nil).Body.String(), "DSM1") &&
			strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), `"routes":[{`) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the app never read names and tally from swp08sim")
		}
	}
	b.open(srv.URL + "/panels/" + id)
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	cell := func(kind string, i int) string {
		return fmt.Sprintf(`document.querySelectorAll('.sode-panel__cell--%s')[%d]`, kind, i)
	}
	b.waitFor("four cells", `document.querySelectorAll('.sode-panel__cell').length === 4`)
	b.waitFor("destination named", cell("destination", 0)+`?.querySelector('span')?.textContent === 'DSM1'`)
	b.waitFor("kiosk hides the header", `document.querySelector('.sode-panels__head').offsetParent === null`)
	b.waitFor("the grid is shown", `!!document.getElementById('panel-grid').offsetParent`)

	// Outside the Rights: refused with a message, and Take stays disabled.
	b.eval(cell("destination", 1) + `.click()`)
	b.eval(cell("source", 0) + `.click()`)
	b.waitFor("refusal shown", `document.getElementById('panel-status').textContent.includes("outside this panel's rights")`)
	b.waitFor("Take not armed", cell("take", 0)+`.disabled`)
	b.shot("refused")

	// Inside the Rights: arms, and the server takes it.
	b.eval(cell("destination", 0) + `.click()`)
	b.eval(cell("source", 0) + `.click()`)
	b.waitFor("Take armed", `!`+cell("take", 0)+`.disabled`)
	b.eval(cell("take", 0) + `.click()`)
	b.waitFor("Connected result", cell("destination", 0)+`?.dataset.result === 'connected'`)
	b.check()
}
