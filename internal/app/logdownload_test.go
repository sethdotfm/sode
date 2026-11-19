package app

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

func logDownload(t *testing.T, a *App, tok string, req any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(req)
	r := httptest.NewRequest("POST", "/api/log/download", bytes.NewReader(b))
	if tok != "" {
		r.Header.Set(sessionHeader, tok)
	}
	rec := httptest.NewRecorder()
	a.Handler(fstest.MapFS{}).ServeHTTP(rec, r)
	return rec
}

func readCSV(t *testing.T, b []byte) [][]string {
	t.Helper()
	rows, err := csv.NewReader(bytes.NewReader(b)).ReadAll()
	if err != nil {
		t.Fatalf("%v in %s", err, b)
	}
	return rows
}

// downloadRow: the command row the download queued (no log writer runs in these tests).
func downloadRow(t *testing.T, a *App) Sent {
	t.Helper()
	select {
	case e := <-a.evq:
		if e.Kind != kindCommand || e.Why != "download" {
			t.Fatalf("row = %+v", e)
		}
		return e
	case <-time.After(2 * time.Second):
		t.Fatal("no download row")
	}
	return Sent{}
}

// tokenFor signs name in on a connection and gives it a session token, as exportSession does.
func tokenFor(t *testing.T, a *App, name, pin string) string {
	t.Helper()
	c := &client{ip: "10.0.0.11", send: make(chan []byte, 4096)}
	a.mu.Lock()
	a.clients[c] = struct{}{}
	a.mu.Unlock()
	if err := a.UserUnlockAs(c, name, pin); err != nil {
		t.Fatal(err)
	}
	tok, err := a.ExportSession(c)
	if err != nil || tok == "" {
		t.Fatalf("no session token for %s: %v", name, err)
	}
	return tok
}

// The full zip holds every row of both log files, oldest first, with the Settings saves row in
// place, and the uptime record.
func TestLogDownloadFullHasBothFilesAndUptime(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	sl, err := OpenSentLog(filepath.Join(dir, "sent.jsonl"), 4000)
	if err != nil {
		t.Fatal(err)
	}
	a.sent = sl
	base := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	for i := range 15 {
		e := Sent{Time: base.Add(time.Duration(i) * time.Second), Kind: "connect", Cmd: 2, Level: 1, Wire: 0, Dst: 3, Src: 4,
			SrcName: "CAM 5", DstName: "MON 4", Tries: 1, Result: "ack", Hex: "02 00", AckMs: 1.5}
		if i == 2 {
			e.DstName = "=SUM(A1)"
		}
		if err := a.sent.Append(&e); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := os.Stat(filepath.Join(dir, "sent.1.jsonl")); err != nil {
		t.Fatalf("the log did not rotate: %v", err)
	}
	if err := a.saves.Append(&Sent{Seq: 5, Time: base.Add(4500 * time.Millisecond), Kind: kindSettings, Why: "save", By: "Ada",
		Result: "ok", Cmd: -1, Wire: -1, Dst: -1, Src: -1, Save: &SettingsSave{Fields: []SavedField{
			{Topic: "Look and feel", Section: "Branding", SectionID: "branding", Key: "brandText", Label: "Logo text and page title", Old: json.RawMessage(`""`), New: json.RawMessage(`"Studio B"`)},
		}}}); err != nil {
		t.Fatal(err)
	}
	a.up.State(true)
	a.up.State(false)
	a.up.Note(Sent{Result: "nak", Tries: 3})

	rec := logDownload(t, a, "", logDownloadReq{Parts: []string{partLog, partUptimeStates, partUptimeMinutes}})
	if rec.Code != 200 || rec.Header().Get("Content-Type") != "application/zip" {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	files := unzipAll(t, rec.Body.Bytes())
	rows := readCSV(t, files["log.csv"])
	if strings.Join(rows[0], ",") != strings.Join(logCSVCols, ",") {
		t.Fatalf("header %v", rows[0])
	}
	if got, want := len(rows)-1, int(a.sent.Total())+1; got != want {
		t.Fatalf("%d rows, want %d", got, want)
	}
	var seqs []string
	for _, r := range rows[1:] {
		seqs = append(seqs, r[1]+":"+r[2])
	}
	if !strings.Contains(strings.Join(seqs, " "), "4:Connect 5:Settings save 5:Connect") || seqs[0] != "0:Connect" {
		t.Fatalf("order %v", seqs)
	}
	row2 := rows[3]
	if row2[7] != "'=SUM(A1)" || row2[3] != "0x02" || row2[6] != "4" || row2[16] != "1.5" || row2[4] == "" {
		t.Fatalf("row %v", row2)
	}
	for _, r := range rows[1:] {
		if r[2] == "Settings save" && !strings.Contains(r[15], `Branding: Logo text and page title: Blank → "Studio B"`) {
			t.Fatalf("save note %q", r[15])
		}
	}
	states := readCSV(t, files["uptime-states.csv"])
	n := len(states)
	if n < 3 || states[n-2][1] != "up" || states[n-1][1] != "down" || states[n-1][2] == "" {
		t.Fatalf("states %v", states)
	}
	mins := readCSV(t, files["uptime-minutes.csv"])
	if len(mins) != 2 || mins[1][1] != "1" || mins[1][2] != "0" {
		t.Fatalf("minutes %v", mins)
	}
	if e := downloadRow(t, a); !strings.Contains(e.Note, "full log, 16 Log rows") || e.Result != "ok" {
		t.Fatalf("download row %+v", e)
	}
}

// Filtered rows: the page's filters over the whole log, in the page's order, as one CSV.
func TestLogDownloadFilteredRows(t *testing.T) {
	a := newTestApp(t)
	base := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	for i := range 700 {
		e := event(kindCommand, "take", "Ana", "ok")
		if i%2 == 1 {
			e = Sent{Kind: "keepalive", Cmd: 1, Wire: -1, Dst: -1, Src: -1, Tries: 1, Result: "ack", Hex: "01"}
		}
		e.Time = base.Add(time.Duration(i) * time.Second)
		a.sent.Append(&e)
	}
	from := base.Add(10 * time.Second).UnixMilli()
	rec := logDownload(t, a, "", logDownloadReq{Parts: []string{partLog}, Filter: &logFilter{Kinds: []string{"command"}, From: &from},
		Sort: logSort{Col: "time", Dir: "asc"}})
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") {
		t.Fatalf("%d %s", rec.Code, rec.Body)
	}
	rows := readCSV(t, rec.Body.Bytes())[1:]
	if len(rows) != 345 || rows[0][1] != "10" || rows[344][1] != "698" {
		t.Fatalf("%d rows, first %v, last %v", len(rows), rows[0], rows[len(rows)-1])
	}
	if e := downloadRow(t, a); !strings.Contains(e.Note, "filtered rows, from 2026-11-19 10:00:10 to now UTC, 345 Log rows") {
		t.Fatalf("download row %q", e.Note)
	}
	if rec := logDownload(t, a, "", logDownloadReq{Parts: []string{"history"}}); rec.Code != 400 {
		t.Fatalf("an unknown part: %d", rec.Code)
	}
}

// Each reader gets Settings saves values only for the topics their Role sees; secrets read changed.
func TestLogDownloadFollowsReaderRole(t *testing.T) {
	a := userApp(t, false)
	stop := startLog(t, a)
	adminSave(t, a)
	waitSaves(t, a, 1)
	stop()
	req := logDownloadReq{Parts: []string{partLog}, Filter: &logFilter{Kinds: []string{"settings"}}}
	note := func(tok string) string {
		rec := logDownload(t, a, tok, req)
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body)
		}
		rows := readCSV(t, rec.Body.Bytes())
		if len(rows) != 2 {
			t.Fatalf("rows %v", rows)
		}
		return rows[1][15]
	}
	bo, ada := note(tokenFor(t, a, "Bo", "567890")), note(tokenFor(t, a, "Ada", "2468"))
	for who, n := range map[string]string{"Bo": bo, "Ada": ada} {
		if !strings.Contains(n, `Logo text and page title: Blank → "Studio B"`) || !strings.Contains(n, "User Ana: PIN: changed") || strings.Contains(n, "9753") {
			t.Errorf("%s's note: %q", who, n)
		}
	}
	if strings.Contains(bo, "10.1.0.0/16") || !strings.Contains(bo, "Trusted proxies: changed") {
		t.Errorf("a Standard user sees the Server value: %q", bo)
	}
	if !strings.Contains(ada, "Trusted proxies: added 10.1.0.0/16") {
		t.Errorf("the Administrator does not see the value: %q", ada)
	}
}

// Whoever can open /log can download. A Guest under the log lock cannot.
func TestLogDownloadNeedsLogAccess(t *testing.T) {
	req := logDownloadReq{Parts: []string{partLog}}
	locked := userApp(t, true)
	if rec := logDownload(t, locked, "", req); rec.Code != 401 {
		t.Fatalf("a Guest under the log lock: %d", rec.Code)
	}
	if rec := logDownload(t, locked, tokenFor(t, locked, "Ana", "1234"), req); rec.Code != 200 {
		t.Fatalf("a signed-in user: %d %s", rec.Code, rec.Body)
	}
	if e := downloadRow(t, locked); e.By != "Ana" {
		t.Fatalf("By = %q", e.By)
	}
	open := userApp(t, false)
	if rec := logDownload(t, open, "", req); rec.Code != 200 {
		t.Fatalf("a Guest with no log lock: %d", rec.Code)
	}
}
