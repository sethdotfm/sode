package app

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/swp08"
	"github.com/sethdotfm/sode/web"
)

// logTablePage opens /log at 1920x1080 on a log with the rows that set the table's shape: router messages with a
// level chip, short command rows, a row with no reason, and one problem row with a long note.
func logTablePage(t *testing.T) *tab {
	b := startBrowser(t)
	sim := startSim(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	want := a.sent.Total() + 13
	for i := range 6 {
		a.txq <- swp08.TxEvent{Msg: swp08.ExtConnect(0, 0, i, 10+i), Why: swp08.Why{Reason: swp08.ReasonTake, By: "Ana"},
			At: time.Now(), Tries: 1, Result: swp08.TxAck}
		a.logEvent(event(kindCommand, "rereadNames", "Ana", "sent"))
	}
	long := event(kindAPI, "panelLost", "Sode", "error")
	long.Note = "panel studio-a (Studio A), instance Wall left: missed its keepalive for 8s; " + strings.Repeat("no device is online; ", 8)
	a.logEvent(long)
	for deadline := time.Now().Add(10 * time.Second); a.sent.Total() < want; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("log stuck at %d, want %d", a.sent.Total(), want)
		}
	}

	b.call("Emulation.setDeviceMetricsOverride", map[string]any{"width": 1920, "height": 1080, "deviceScaleFactor": 1, "mobile": false})
	b.open(srv.URL + "/log")
	b.waitFor("the seeded rows", `document.querySelectorAll('#log-rows tr').length >= 13 && !!document.querySelector('#log-rows .sode-log__lvl')`)
	b.eval(`document.fonts.ready.then(() => true)`)
	return b
}

// The Log table fits the page: the reason column wraps, so no column is cut and no sideways scroll shows.
func TestChromeLogTableLayout(t *testing.T) {
	b := logTablePage(t)
	b.shot("table")

	if fit := b.eval(`(() => {
		const s = document.querySelector('.sode-log__scroll');
		return s.scrollWidth <= s.clientWidth ? '' : 'table ' + s.scrollWidth + ' px in a ' + s.clientWidth + ' px scroller';
	})()`).(string); fit != "" {
		t.Errorf("the Log table is wider than the page: %s", fit)
	}
	b.noClip("Bytes column", "#log-rows td.sode-log__hex")
	b.check()
}
