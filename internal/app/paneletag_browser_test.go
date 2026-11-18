package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// notModified counts 304 answers by request path, and every tally read in reads.
type notModified struct {
	next              http.Handler
	tally, protected  atomic.Int32
	names, panelReads atomic.Int32
	reads             atomic.Int32
	panelPath         string
	noSocket          atomic.Bool // hold /ws unanswered, so a panel page polls as with its socket down
}

type statusWriter struct {
	http.ResponseWriter
	code int
}

func (s *statusWriter) WriteHeader(c int) { s.code = c; s.ResponseWriter.WriteHeader(c) }

// Unwrap keeps http.ResponseController (flush for the instance stream) working.
func (s *statusWriter) Unwrap() http.ResponseWriter { return s.ResponseWriter }

func (n *notModified) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// A refused handshake is a console error, which fails the check; one that never answers is not.
	if r.URL.Path == "/ws" && n.noSocket.Load() {
		<-r.Context().Done()
		return
	}
	if r.URL.Path == "/api/v1/tally" {
		n.reads.Add(1)
	}
	sw := &statusWriter{ResponseWriter: w, code: 200}
	n.next.ServeHTTP(sw, r)
	if sw.code != http.StatusNotModified {
		return
	}
	switch r.URL.Path {
	case "/api/v1/tally":
		n.tally.Add(1)
	case "/api/v1/protected":
		n.protected.Add(1)
	case "/api/v1/names":
		n.names.Add(1)
	case n.panelPath:
		n.panelReads.Add(1)
	}
}

// keepSockets lets a test close the page's WebSockets.
const keepSockets = `{ const W = window.WebSocket; window.sodeSockets = [];
  window.WebSocket = class extends W { constructor(...a) { super(...a); window.sodeSockets.push(this); } }; }`

// A panel page with its socket down polls. After 304 answers it still shows a route another caller changed, and still
// applies a saved edit.
func TestChromePanelStillUpdatesAfter304(t *testing.T) {
	b := startBrowser(t)
	b.call("Page.addScriptToEvaluateOnNewDocument", map[string]any{"source": keepSockets})
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
	body := func(name string) string {
		return fmt.Sprintf(`{"name":%q,"rows":4,"cols":8,"cells":[
			{"r":1,"c":1,"kind":"destination","n":%d},
			{"r":1,"c":2,"kind":"source","n":%d}],"settings":{"mode":"take"}}`, name, simDst, simSrc)
	}
	id := makePanel(t, a, ana, body("Before"))
	count.panelPath = "/api/v1/panels/" + id
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
	count.noSocket.Store(true)
	b.eval(`window.sodeSockets.forEach((w) => w.close())`)

	// Polls every 2 s and the update check every 5 s: wait until both have had a 304.
	for deadline := time.Now().Add(20 * time.Second); count.tally.Load() < 2 || count.protected.Load() < 2 || count.panelReads.Load() < 1; time.Sleep(100 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("304 answers: tally %d, protected %d, panel %d", count.tally.Load(), count.protected.Load(), count.panelReads.Load())
		}
	}
	b.waitFor("the page is still drawn", dst+`?.querySelector('small')?.textContent === 'CG1 Fill'`)

	// Another caller moves the route: a new tally ETag reaches the page.
	if rec := apiSend(a, "POST", "/api/v1/takes", ana, fmt.Sprintf(`{"routes":[{"src":%d,"dst":%d}]}`, simSrc, simDst)); rec.Code != 200 {
		t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("the page shows the new source", dst+`?.querySelector('small')?.textContent === '`+simSrcName+`'`)

	// A saved edit after the panel reads answered 304 still raises the banner and applies.
	if rec := apiSend(a, "PUT", "/api/v1/panels/"+id, ana, body("After")); rec.Code != 200 {
		t.Fatalf("save: %d %s", rec.Code, rec.Body.String())
	}
	b.waitFor("refresh banner", `!document.getElementById('panel-update').hidden`)
	b.eval(`document.getElementById('panel-apply').click()`)
	b.waitFor("new panel applied", `document.getElementById('panel-title').textContent === 'After'`)
	b.check()
	b.call("Page.close", nil)
}
