package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/web"
)

// A panel Undo press stages the reverse of this instance's newest take and Take sends it. The next press goes one take
// further back and skips another instance's takes. The cell greys when none are left.
func TestChromePanelUndoStepsBackThisInstance(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	const second, elsewhere = 5, 7 // routed from themselves at the start
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Undo panel","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"destination","n":%d},
		{"r":1,"c":3,"kind":"source","n":%d},
		{"r":1,"c":4,"kind":"take"},
		{"r":1,"c":5,"kind":"undo"}],"settings":{"mode":"take"}}`, simDst, second, simSrc))
	waitNamesAndTally(t, a, ana)
	b.open(srv.URL + "/panels/" + id + "?instance=Desk")
	b.waitFor("PIN pad", `!!document.querySelector('.sode-lock__key[data-key="1"]')?.offsetParent`)
	for _, d := range "1234" {
		b.eval(fmt.Sprintf(`document.querySelector('.sode-lock__key[data-key="%c"]').click()`, d))
	}
	b.eval(`document.querySelector('.sode-lock__key[data-key="enter"]').click()`)

	dsts := `document.querySelectorAll('.sode-panel__cell--destination')`
	src := `document.querySelector('.sode-panel__cell--source')`
	take := `document.querySelector('.sode-panel__take')`
	undo := `document.querySelector('.sode-panel__cell--undo')`
	status := `document.getElementById('panel-status')`
	b.waitFor("the first poll answered", src+`?.querySelector('.sode-panel__text')?.textContent === '`+simSrcName+`'`)
	b.waitFor("Undo greyed with nothing taken", undo+`.disabled && `+undo+`.textContent === 'Undo'`)

	for i, d := range []int{simDst, second} {
		b.eval(fmt.Sprintf(`%s[%d].click()`, dsts, i))
		b.eval(src + `.click()`)
		b.waitFor("Take armed", `!`+take+`.disabled`)
		b.eval(take + `.click()`)
		waitRouted(t, a, ana, d, simSrc)
	}
	// Another instance of this panel takes after both.
	other := map[string]string{"X-Sode-Panel": id, "X-Sode-Instance": "Lobby"}
	if rec := apiPost(a, "/api/v1/takes", ana, fmt.Sprintf(`{"routes":[{"src":%d,"dst":%d,"levels":[1]}]}`, simSrc, elsewhere), other); rec.Code != 200 {
		t.Fatalf("other instance's take: %d %s", rec.Code, rec.Body.String())
	}
	waitRouted(t, a, ana, elsewhere, simSrc)

	// Each press stages the reverse and arms Take; nothing moves until Take.
	for i, d := range []int{second, simDst} {
		b.waitFor("Undo ready", `!`+undo+`.disabled`)
		b.eval(undo + `.click()`)
		b.waitFor("the undo staged", fmt.Sprintf(`%s[%d].dataset.s === 'staged' && %s.dataset.s === 'pending' && !%s.disabled`, dsts, 1-i, undo, take))
		if i == 0 {
			b.shot("undo staged")
		}
		if !strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, d, simSrc)) {
			t.Fatalf("the Undo press sent the undo of %d before Take", d)
		}
		b.eval(take + `.click()`)
		waitRouted(t, a, ana, d, d)
		b.waitFor("the status names the undo", status+`.textContent.startsWith('Undone: ') && `+status+`.classList.contains('sode-panel__status--ok')`)
		b.waitFor("the plan is empty again", undo+`.dataset.s === 'idle' && `+take+`.disabled`)
	}
	b.waitFor("Undo greyed with this instance's takes undone", undo+`.disabled`)
	if !strings.Contains(apiDo(a, "GET", "/api/v1/tally", ana, nil).Body.String(), fmt.Sprintf(`"dst":%d,"level":1,"src":%d`, elsewhere, simSrc)) {
		t.Fatal("the other instance's take was undone")
	}
	b.check()
}
