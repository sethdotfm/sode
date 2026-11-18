package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// The /log page keeps at most 500 live rows (logpager.js LIVE_CAP). Older rows come back 100 at a time with
// Load older, and a live row after that does not drop them.
func TestChromeLogRowCap(t *testing.T) {
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
	emit := func(n int) {
		for n > 0 {
			k := min(n, 100)
			want := a.sent.Total() + int64(k)
			for i := 0; i < k; i++ {
				a.logEvent(event(kindCommand, "refreshNames", "Ana", "ok"))
			}
			for deadline := time.Now().Add(10 * time.Second); a.sent.Total() < want; time.Sleep(10 * time.Millisecond) {
				if time.Now().After(deadline) {
					t.Fatalf("log stuck at %d, want %d", a.sent.Total(), want)
				}
			}
			n -= k
		}
	}
	emit(300)

	b.open(srv.URL + "/log")
	rows := `document.querySelectorAll('#log-rows tr').length`
	b.waitFor("the first rows", rows+` >= 100`)
	time.Sleep(time.Second)
	emit(600)
	b.waitFor("rows capped at 500", rows+` === 500`)
	time.Sleep(time.Second)
	capped := 500
	b.eval(`document.getElementById('log-older').click(); true`)
	b.waitFor("Load older adds 100", fmt.Sprintf(`%s === %d`, rows, capped+100))
	emit(20)
	time.Sleep(time.Second)
	if got := int(b.eval(rows).(float64)); got != capped+100 {
		t.Fatalf("rows after 20 more live = %d, want %d", got, capped+100)
	}
	b.check()
}
