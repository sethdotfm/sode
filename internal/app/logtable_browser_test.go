package app

import (
	"context"
	"fmt"
	"math"
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
	// A level chip does not make its row taller. The problem row's long note wraps, so it is left out.
	b.sameHeight("one-line rows", "#log-rows tr:has(.sode-log__lvl)", "#log-rows tr:not(:has(.sode-log__lvl)):not(.sode-log__row--problem)")

	// A row with no reason starts its by or note text on the header's text edge.
	if off := b.eval(`(() => {
		const h = document.querySelector('.sode-log__table th[data-sort="why"]');
		const edge = h.getBoundingClientRect().left + parseFloat(getComputedStyle(h).paddingLeft);
		const first = (td) => [...td.childNodes].find((n) => n.nodeType === 1 || n.textContent.trim() !== '');
		const parts = [...document.querySelectorAll('#log-rows td.sode-log__why')].map(first).filter((n) => n?.classList?.contains('sode-log__why-part'));
		if (!parts.length) return 'no row without a reason';
		return parts.map((p) => p.getBoundingClientRect().left - edge).filter((d) => Math.abs(d) > 0.5).map((d) => d.toFixed(2) + ' px').join(', ');
	})()`).(string); off != "" {
		t.Errorf("reason text off the header's text edge: %s", off)
	}
	if !b.eval(`[...document.querySelectorAll('#log-rows .sode-log__why-text + .sode-log__why-part')].every((p) => parseFloat(getComputedStyle(p).marginInlineStart) > 0)`).(bool) {
		t.Error("the by or note text after a reason lost its space")
	}

	// The problem stripe has the cell padding of air before the time, and every row starts its time there.
	inset := b.tokenPx(".sode-log__table", "calc(var(--pb-accent-bar-w) + var(--pb-space-3))")
	if bad := b.eval(`[...document.querySelectorAll('.sode-log__table tr > :first-child')].map((c) => parseFloat(getComputedStyle(c).paddingLeft)).filter((p) => Math.abs(p - ` + fmt.Sprint(inset) + `) > 0.5).join(', ')`).(string); bad != "" {
		t.Errorf("first cells pad %s px, want %.1f px", bad, inset)
	}

	// Numbers use the number font; only the bytes use the code font.
	if bad := b.eval(`(() => {
		const font = (v) => { const p = document.createElement('span'); p.style.fontFamily = 'var(' + v + ')'; document.body.append(p); const f = getComputedStyle(p).fontFamily; p.remove(); return f; };
		const want = { '#log-count': '--pb-font-num', '#log-uptime-sum': '--pb-font-num', '#log-rows .sode-log__time': '--pb-font-num', '#log-rows .sode-log__hex': '--pb-font-mono' };
		return Object.entries(want).filter(([sel, v]) => getComputedStyle(document.querySelector(sel)).fontFamily !== font(v)).map(([sel, v]) => sel + ' is not ' + v).join(', ');
	})()`).(string); bad != "" {
		t.Errorf("fonts: %s", bad)
	}

	// The divider before Problems only is a drawn line as high as the chips, not a typed character.
	if bad := b.eval(`(() => {
		const sep = document.querySelector('#log-kinds .sode-log__sep');
		const w = parseFloat(getComputedStyle(sep).borderInlineStartWidth);
		return sep.textContent === '' && w > 0 ? '' : 'text ' + JSON.stringify(sep.textContent) + ', line ' + w + ' px';
	})()`).(string); bad != "" {
		t.Errorf("Show divider: %s", bad)
	}
	b.sameHeight("Show divider and chips", "#log-kinds .sode-log__sep", "#log-kinds button")

	// The reason column takes the spare width, so the other columns keep their width when the rows on show change.
	timeW := `document.querySelector('.sode-log__table th[data-sort="time"]').getBoundingClientRect().width`
	before := b.eval(timeW).(float64)
	b.eval(`const chip = (w) => [...document.querySelectorAll('#log-kinds button')].find((x) => x.textContent === w);
		chip('All').click(); chip('Commands').click(); true`)
	b.waitFor("only the command rows", `document.querySelectorAll('#log-rows tr').length === 6`)
	b.shot("short-rows")
	if after := b.eval(timeW).(float64); math.Abs(after-before) > layoutTol {
		t.Errorf("Time column %.1f px with every row, %.1f px with only short rows", before, after)
	}
	b.check()
}
