package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// With its socket up, a panel page shows another caller's take from the tally frame, with no tally read.
func TestChromePanelShowsTakeFromSocket(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	count := &notModified{next: a.Handler(web.FS)}
	srv := httptest.NewServer(count)
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, fmt.Sprintf(`{"name":"Wall","rows":4,"cols":8,"cells":[
		{"r":1,"c":1,"kind":"destination","n":%d},
		{"r":1,"c":2,"kind":"source","n":%d}],"settings":{"mode":"take"}}`, simDst, simSrc))
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
	dst := `document.querySelector('.sode-panel__cell--destination')`
	b.waitFor("current source under the destination", dst+`?.querySelector('small')?.textContent === 'CG1 Fill'`)
	// The first reads settle; after that no tally poll runs while the socket is up.
	time.Sleep(3 * time.Second)
	before := count.reads.Load()
	time.Sleep(3 * time.Second)
	if n := count.reads.Load() - before; n != 0 {
		t.Errorf("%d tally reads in 3 s with the socket up", n)
	}

	start := time.Now()
	if rec := apiSend(a, "POST", "/api/v1/takes", ana, fmt.Sprintf(`{"routes":[{"src":%d,"dst":%d}]}`, simSrc, simDst)); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("the page shows the new source", dst+`?.querySelector('small')?.textContent === '`+simSrcName+`'`)
	t.Logf("take to screen: %v", time.Since(start))
	if n := count.reads.Load() - before; n != 0 {
		t.Errorf("the take showed after %d tally reads, not from the frame", n)
	}
	b.check()
	b.call("Page.close", nil)
}
