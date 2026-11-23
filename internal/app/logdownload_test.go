package app

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

// logReply: the parts of a download reply.
type logReply struct {
	Log           []Sent        `json:"log"`
	UptimeStates  []UptimeTrans `json:"uptimeStates"`
	UptimeMinutes []struct {
		M    int64 `json:"m"`
		Bad  int64 `json:"bad"`
		Slow int64 `json:"slow"`
	} `json:"uptimeMinutes"`
}

func readReply(t *testing.T, rec *httptest.ResponseRecorder) logReply {
	t.Helper()
	if rec.Code != 200 || !strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("%d %s %s", rec.Code, rec.Header().Get("Content-Type"), rec.Body)
	}
	var out logReply
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("%v in %s", err, rec.Body)
	}
	return out
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

// The reply holds every row of both log files, oldest first, with the Settings saves row in place,
// and the uptime record.
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

	got := readReply(t, logDownload(t, a, "", logDownloadReq{Parts: []string{partLog, partUptimeStates, partUptimeMinutes}, For: "full"}))
	if n, want := len(got.Log), int(a.sent.Total())+1; n != want {
		t.Fatalf("%d rows, want %d", n, want)
	}
	var seqs []string
	for _, e := range got.Log {
		seqs = append(seqs, e.Kind+":"+strconv.FormatInt(e.Seq, 10))
	}
	if !strings.Contains(strings.Join(seqs, " "), "connect:4 settings:5 connect:5") || seqs[0] != "connect:0" {
		t.Fatalf("order %v", seqs)
	}
	n := len(got.UptimeStates)
	if n < 2 || got.UptimeStates[n-2].S != "up" || got.UptimeStates[n-1].S != "down" {
		t.Fatalf("states %+v", got.UptimeStates)
	}
	if len(got.UptimeMinutes) != 1 || got.UptimeMinutes[0].Bad != 1 || got.UptimeMinutes[0].Slow != 0 {
		t.Fatalf("minutes %+v", got.UptimeMinutes)
	}
	if e := downloadRow(t, a); !strings.Contains(e.Note, "full log, 16 Log rows") || e.Result != "ok" {
		t.Fatalf("download row %+v", e)
	}
}

// Filtered rows: the page gets every row and applies its filters itself, so the row says what was sent.
func TestLogDownloadForFilteredRowsSendsEveryRow(t *testing.T) {
	a := newTestApp(t)
	base := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	for i := range 700 {
		e := event(kindCommand, "take", "Ana", "ok")
		e.Time = base.Add(time.Duration(i) * time.Second)
		a.sent.Append(&e)
	}
	got := readReply(t, logDownload(t, a, "", logDownloadReq{Parts: []string{partLog}, For: "filtered"}))
	if len(got.Log) != 700 || got.Log[0].Seq != 0 || got.Log[699].Seq != 699 || got.UptimeStates != nil {
		t.Fatalf("%d rows", len(got.Log))
	}
	if e := downloadRow(t, a); e.Note != "filtered rows, 700 Log rows sent to the page, 2026-11-19 10:00:00 to 2026-11-19 10:11:39 UTC" {
		t.Fatalf("download row %q", e.Note)
	}
	for _, bad := range []logDownloadReq{{Parts: []string{"history"}, For: "full"}, {Parts: []string{partLog}, For: "some"}, {For: "full"}} {
		if rec := logDownload(t, a, "", bad); rec.Code != 400 {
			t.Fatalf("%+v: %d", bad, rec.Code)
		}
	}
}

// Each reader gets Settings saves values only for the topics their Role sees; secrets read changed.
func TestLogDownloadFollowsReaderRole(t *testing.T) {
	a := userApp(t, false)
	stop := startLog(t, a)
	ana := adminSave(t, a)
	waitSaves(t, a, 1)
	stop()
	req := logDownloadReq{Parts: []string{partLog}, For: "filtered"}
	save := func(tok string) string {
		for _, e := range readReply(t, logDownload(t, a, tok, req)).Log {
			if e.Save != nil {
				b, _ := json.Marshal(e.Save)
				return string(b)
			}
		}
		t.Fatal("no Settings saves row")
		return ""
	}
	bo, ada := save(tokenFor(t, a, "Bo", "567890")), save(tokenFor(t, a, "Ada", "2468"))
	for who, s := range map[string]string{"Bo": bo, "Ada": ada} {
		if !strings.Contains(s, `Studio B`) || holdsPin(s, ana, "9753") {
			t.Errorf("%s's row: %s", who, s)
		}
	}
	if strings.Contains(bo, "10.1.0.0/16") {
		t.Errorf("a Standard user sees the Server value: %s", bo)
	}
	if !strings.Contains(ada, "10.1.0.0/16") {
		t.Errorf("the Administrator does not see the value: %s", ada)
	}
}

// Whoever can open /log can download. A Guest under the log lock cannot.
func TestLogDownloadNeedsLogAccess(t *testing.T) {
	req := logDownloadReq{Parts: []string{partLog}, For: "full"}
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
