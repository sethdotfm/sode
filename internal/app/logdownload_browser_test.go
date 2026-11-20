package app

import (
	"context"
	"encoding/base64"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/web"
)

// Download filtered rows: the page reads the whole log, not only the rows it loaded, and applies its
// own filters to it. Browser rows are not in the default filter, so the file has none.
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
	for i := range 7 {
		want := a.sent.Total() + 100
		for j := range 100 {
			if i == 3 && j%2 == 0 {
				a.logEvent(event(kindBrowser, "connect", "10.9.9.9", "connected"))
				continue
			}
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
	b.eval(`window.__files = [];
		const make = URL.createObjectURL;
		URL.createObjectURL = (blob) => { window.__files.push(blob); return make(blob); };
		document.getElementById('log-download-rows').click(); true`)
	b.waitFor("the file", `window.__files.length === 1 && !document.getElementById('log-download-rows').disabled`)
	text := b.eval(`window.__files[0].text()`).(string)
	lines := strings.Split(strings.TrimSuffix(text, "\r\n"), "\r\n")
	if !strings.HasPrefix(lines[0], "time,seq,message,cmd,") {
		t.Fatalf("header %q", lines[0])
	}
	rows := lines[1:]
	if len(rows) < 650 || len(rows) <= loaded || strings.Contains(text, "Browser") || !strings.Contains(text, ",Command,,,,,,,,Re-read names,Ana,ok,0,") {
		t.Fatalf("%d rows in the file, %d loaded on the page, first %q", len(rows), loaded, rows[0])
	}
	if !b.eval(`document.getElementById('log-download-err').hidden`).(bool) {
		t.Fatal("the page shows an error")
	}

	count := regexp.MustCompile(`^filtered rows, (\d+) Log rows sent to the page, `)
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
			if n, _ := strconv.Atoi(m[1]); n < 700 {
				t.Fatalf("%d rows sent", n)
			}
			b.check()
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("no download row")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// Download full log: the page makes the zip with the log and the uptime record.
func TestChromeLogDownloadFullLog(t *testing.T) {
	b := startBrowser(t)
	sim := startSim(t)

	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	srv := httptest.NewServer(a.Handler(web.FS))
	t.Cleanup(func() { srv.CloseClientConnections(); srv.Close() })

	b.open(srv.URL + "/log")
	b.waitFor("the first rows", `document.querySelectorAll('#log-rows tr').length > 0`)
	b.eval(`window.__files = [];
		const make = URL.createObjectURL;
		URL.createObjectURL = (blob) => { window.__files.push(blob); return make(blob); };
		document.getElementById('log-download-full').click(); true`)
	b.waitFor("the zip", `window.__files.length === 1 && !document.getElementById('log-download-full').disabled`)
	enc := b.eval(`window.__files[0].arrayBuffer().then((x) => btoa(String.fromCharCode(...new Uint8Array(x))))`).(string)
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatal(err)
	}
	files := unzipAll(t, raw)
	log := string(files["log.csv"])
	if !strings.HasPrefix(log, "time,seq,message,") || !strings.Contains(log, ",Router link,") {
		t.Fatalf("log.csv %q", log)
	}
	if !strings.HasPrefix(string(files["uptime-states.csv"]), "time,state,note\r\n") || !strings.HasPrefix(string(files["uptime-minutes.csv"]), "minute,bad,slow\r\n") {
		t.Fatalf("files %v", files)
	}
	b.check()
}
