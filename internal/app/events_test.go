package app

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// startLog runs the Communication Log writer. The returned func stops it and waits.
func startLog(t *testing.T, a *App) (stop func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.logSent(ctx); close(done) }()
	return func() {
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("logSent didn't return")
		}
	}
}

// Any socket can send these, so with nobody signed in they must not write rows.
func TestLockWithNobodySignedInWritesNoRow(t *testing.T) {
	a := userApp(t, false)
	defer startLog(t, a)()
	c := &client{ip: "10.0.0.9"}
	for range 3 {
		for _, typ := range []string{"userLock", "userSignOut"} {
			policy[typ].handle(a, c, &inMsg{T: typ})
		}
	}
	a.command(c, "marker", "ok", "", nil)
	for _, r := range waitLogged(t, a, 1) {
		if r.Why == "userLock" || r.Why == "userSignOut" {
			t.Fatalf("row %+v", r)
		}
	}
}

// logged is the Communication Log, oldest first.
func logged(t *testing.T, a *App) []Sent {
	t.Helper()
	page, _, err := a.sent.Page(-1, 500)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(page)-1; i < j; i, j = i+1, j-1 {
		page[i], page[j] = page[j], page[i]
	}
	return page
}

func waitLogged(t *testing.T, a *App, n int) []Sent {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if rows := logged(t, a); len(rows) >= n {
			return rows
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("log has %d rows, want %d: %+v", len(logged(t, a)), n, logged(t, a))
	return nil
}

// waitLoggedWhy waits for a row with this Why. A count can pass before the row under test is written.
func waitLoggedWhy(t *testing.T, a *App, why string) Sent {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		for _, e := range logged(t, a) {
			if e.Why == why {
				return e
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("no %s row in %+v", why, logged(t, a))
	return Sent{}
}

func results(rows []Sent, kind string) string {
	var out []string
	for _, e := range rows {
		if e.Kind == kind {
			out = append(out, e.Result)
		}
	}
	return strings.Join(out, ",")
}

// The retry cycle while the router is down is one row. A router change starts the cycle again, so Sode logs the next
// up.
func TestLinkLoggedOncePerOutage(t *testing.T) {
	a := newTestApp(t)
	stop := startLog(t, a)
	defer stop()
	boom := errors.New("connection refused")
	for _, s := range []struct {
		s swp08.State
		e error
	}{
		{swp08.StateConnecting, nil}, {swp08.StateDown, boom}, {swp08.StateConnecting, nil},
		{swp08.StateDown, boom}, {swp08.StateConnecting, nil}, {swp08.StateDown, errors.New("timeout")},
		{swp08.StateConnecting, nil}, {swp08.StateUp, nil},
	} {
		a.onUnitState(a.firstUnit(), s.s, s.e)
	}
	rows := waitLogged(t, a, 3)
	if got := results(rows, kindLink); got != "down,down,up" {
		t.Fatalf("link rows = %s, want down (once), down (new error), up", got)
	}
	if rows[0].Err != "connection refused" || rows[0].Note != a.r().cfg.Routers[0].Address {
		t.Fatalf("down row = %+v", rows[0])
	}

	cfg := a.r().cfg.Clone()
	cfg.Routers[0].Address = "10.9.9.9:8910"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	a.onUnitState(a.firstUnit(), swp08.StateUp, nil)
	rows = waitLogged(t, a, 5)
	if got := results(rows, kindLink); got != "down,down,up,changed,up" {
		t.Fatalf("after the change: %s", got)
	}
	if rows[3].Note != "10.9.9.9:8910" {
		t.Fatalf("changed row = %+v", rows[3])
	}
	var s []string
	for _, tr := range a.firstUnit().up.trans {
		s = append(s, tr.S)
	}
	if got := strings.Join(s, ","); got != "down,up,down,up" {
		t.Fatalf("uptime states = %s", got)
	}
}

func TestShutdownWritesStopAndOff(t *testing.T) {
	a := newTestApp(t)
	dir := t.TempDir()
	u, err := OpenUptime(dir + "/uptime.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	a.SetUptime(u)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	a.onTx(swp08.TxEvent{Msg: []byte{0x7F}, Why: swp08.Why{Reason: swp08.ReasonTake}, At: time.Now(), Tries: 3, Result: "noack"})
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run didn't return")
	}
	rows := logged(t, a)
	if len(rows) == 0 || rows[len(rows)-1].Kind != kindServer || rows[len(rows)-1].Result != "stop" {
		t.Fatalf("last row = %+v", rows)
	}
	if got := results(rows, kindServer); got != "start,stop" {
		t.Fatalf("server rows = %s", got)
	}
	v, err := OpenUptime(dir + "/uptime.jsonl")
	if err != nil {
		t.Fatal(err)
	}
	// down (first run's start), off (the stop), down (this start): off is there
	// because the stop wrote it, not because the reopen guessed.
	if n := len(v.trans); n != 3 || v.trans[1].S != upOff {
		t.Fatalf("trans = %+v", v.trans)
	}
	if len(u.mins) != 1 || u.mins[0].Bad != 1 {
		t.Fatalf("the unacked take wasn't counted: %+v", u.mins)
	}
}

// A failed write to the Communication Log still counts for the graph.
func TestUptimeCountsEvenIfNotSaved(t *testing.T) {
	a := newTestApp(t)
	stop := startLog(t, a)
	a.onTx(swp08.TxEvent{Msg: []byte{0x81}, At: time.Now(), Tries: 1, Result: "ack", Ack: 400 * time.Millisecond})
	waitLogged(t, a, 1)
	stop()
	a.firstUnit().up.mu.Lock()
	defer a.firstUnit().up.mu.Unlock()
	if a.firstUnit().up.cur.Slow != 1 && (len(a.firstUnit().up.mins) == 0 || a.firstUnit().up.mins[0].Slow != 1) {
		t.Fatalf("slow ACK not counted: cur=%+v mins=%+v", a.firstUnit().up.cur, a.firstUnit().up.mins)
	}
}

// Browsers that connect, drop and fire show in the log. `unlock` and `lock` are unknown types: nothing runs or logs.
func TestBrowserAndCommandRows(t *testing.T) {
	cfg := config.Defaults()
	cfg.Normalize()
	hist, _ := OpenHistory("")
	a, err := New(cfg, "", hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	stop := startLog(t, a)
	defer stop()
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	c, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	send := func(v any) {
		b, _ := json.Marshal(v)
		if err := c.Write(ctx, websocket.MessageText, b); err != nil {
			t.Fatal(err)
		}
	}
	send(map[string]any{"t": "unlock", "pin": "0000", "id": 1})
	send(map[string]any{"t": "unlock", "pin": "4711", "id": 2})
	send(map[string]any{"t": "lock"})
	send(map[string]any{"t": "clearHistory"})
	send(map[string]any{"t": "rereadNames"})
	send(map[string]any{"t": "take", "items": []map[string]any{{"id": 1, "src": 1, "dst": 2, "levels": []int{1}}, {"id": 2, "src": 1, "dst": 3, "levels": []int{1}}}})
	send(map[string]any{"t": "log", "before": -1}) // reads are not commands
	rows := waitLogged(t, a, 5)
	c.Close(websocket.StatusNormalClosure, "")
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		if rows = logged(t, a); results(rows, kindBrowser) == "connected,disconnected" {
			break
		}
	}

	var why, res []string
	for _, e := range rows {
		if e.Kind == kindCommand {
			why = append(why, e.Why)
			res = append(res, e.Result)
		}
	}
	if got := strings.Join(why, ","); got != "clearHistory,rereadNames,take" {
		t.Fatalf("commands = %s", got)
	}
	// No users: every browser acts as an Administrator, so clearHistory runs.
	if got := strings.Join(res, ","); got != "ok,sent,sent" {
		t.Fatalf("results = %s", got)
	}
	var take Sent
	for _, e := range rows {
		if e.Kind == kindCommand && e.Why == "take" {
			take = e
		}
	}
	if take.Note != "2 takes" || take.By == "" {
		t.Fatalf("take row = %+v", take)
	}
	if got := results(rows, kindBrowser); got != "connected,disconnected" {
		t.Fatalf("browser rows = %s", got)
	}
	for _, e := range rows {
		if e.Kind == kindBrowser && e.Result == "disconnected" && e.Err != "" {
			t.Fatalf("a normal close is not an error: %+v", e)
		}
		if e.Kind != "keepalive" && e.Kind != "interrogate" && (e.Cmd != -1 || e.Hex != "") && e.Kind == kindBrowser {
			t.Fatalf("event with wire fields: %+v", e)
		}
	}
}
