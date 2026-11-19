package app

import (
	"context"
	"net/http/httptest"
	"regexp"
	"strconv"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// Download filtered rows sends the page's filters, and the server applies them to the whole log, not
// only to the rows the page loaded.
func TestChromeLogDownloadFilteredRows(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	// evq holds 256, so write in steps and wait for the file to catch up.
	for range 7 {
		want := a.sent.Total() + 100
		for range 100 {
			a.logEvent(event(kindCommand, "rereadNames", "Ana", "ok"))
		}
		for deadline := time.Now().Add(10 * time.Second); a.sent.Total() < want; time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("log stuck at %d, want %d", a.sent.Total(), want)
			}
		}
	}

	b.open(srv.URL + "/log")
	b.waitFor("the first rows", `document.querySelectorAll('#log-rows tr').length >= 100`)
	loaded := int(b.eval(`document.querySelectorAll('#log-rows tr').length`).(float64))
	b.eval(`document.getElementById('log-download-rows').click(); true`)

	count := regexp.MustCompile(`^filtered rows, from the start to now UTC, (\d+) Log rows$`)
	deadline := time.Now().Add(10 * time.Second)
	for {
		page, _, err := a.sent.Page(-1, 50)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range page {
			if e.Why != "download" {
				continue
			}
			m := count.FindStringSubmatch(e.Note)
			if m == nil || e.Result != "ok" {
				t.Fatalf("download row %+v", e)
			}
			if n, _ := strconv.Atoi(m[1]); n < 700 || n <= loaded {
				t.Fatalf("%d rows in the file, %d loaded on the page", n, loaded)
			}
			b.waitFor("the button back", `!document.getElementById('log-download-rows').disabled`)
			b.check()
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no download row")
		}
		time.Sleep(50 * time.Millisecond)
	}
}
