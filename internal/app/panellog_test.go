package app

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/sethdotfm/sode/internal/swp08"
)

// Communication Log rows from a panel name the panel's short name and the instance (docs/agents/comm-log.md).

const slugPanel = `{"name":"Wall","acts":"api","slug":"wall","clientRights":{"sources":[1,2,3],"dsts":[2,4]}}`

// lastRow waits for the n-th row (from 1) with that why and returns it; the writer is a goroutine.
func lastRow(t *testing.T, a *App, why string, n ...int) Sent {
	t.Helper()
	want := 1
	if len(n) > 0 {
		want = n[0]
	}
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var seen []Sent
		for _, r := range logged(t, a) {
			if r.Why == why {
				seen = append(seen, r)
			}
		}
		if len(seen) >= want {
			return seen[want-1]
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("fewer than %d %s rows in %+v", want, why, logged(t, a))
	return Sent{}
}

func TestPanelClientTakeRowNamesPanelAndInstance(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, slugPanel)
	secret, _ := mintKey(t, a, cy, id)
	f.seed(2, 1, 0)

	// The page says which instance it is; the key proves the panel.
	rec := apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, map[string]string{"X-Sode-Instance": "Front desk"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	e := lastRow(t, a, "take")
	if !strings.Contains(e.Note, "panel wall (Wall), instance Front desk") || strings.Contains(e.Note, "unverified") {
		t.Fatalf("note %q", e.Note)
	}
	if strings.Contains(e.Note+e.By, secret) {
		t.Fatal("key in row")
	}

	// A refusal carries it too, and a call without the header falls back to the instance the key was made for.
	rec = apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":3,"dst":5,"levels":[1]}]}`, nil)
	wantErr(t, rec, 403, "outside_rights")
	e = lastRow(t, a, "take", 2)
	if e.Result != "refused" || !strings.Contains(e.Note, "panel wall (Wall), instance Lobby") {
		t.Fatalf("refused row %+v", e)
	}
}

// The router messages of a panel's take carry the panel tag too, and its History entry names the panel and the
// instance, with a panel key and with a Session-mode page's headers alike.
func TestPanelTakeWireRowsAndHistoryNameInstance(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, slugPanel)
	secret, _ := mintKey(t, a, cy, id)
	f.seed(2, 1, 0)

	// Every message the take sent (connect and interrogate) carries the tag, and the row shows it in Note.
	sentNote := func() string {
		t.Helper()
		f.mu.Lock()
		defer f.mu.Unlock()
		note := ""
		for _, w := range f.whys {
			if w.why.Reason != swp08.ReasonTake && w.why.Reason != swp08.ReasonCheck && w.why.Reason != swp08.ReasonConfirm {
				continue
			}
			if note != "" && w.why.Note != note {
				t.Fatalf("messages of one take disagree: %q, %q", note, w.why.Note)
			}
			note = w.why.Note
		}
		f.whys = nil
		return note
	}

	rec := apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, map[string]string{"X-Sode-Instance": "Front desk"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if n := sentNote(); n != "panel wall (Wall), instance Front desk" {
		t.Fatalf("router messages %q", n)
	}
	if row := a.describe(swp08.TxEvent{Msg: []byte{0x02}, Why: swp08.Why{Reason: swp08.ReasonTake, Note: "panel wall (Wall), instance Front desk"}}); row.Note != "panel wall (Wall), instance Front desk" {
		t.Fatalf("row note %q", row.Note)
	}
	es, _ := a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Panel != "Wall" || es[0].Instance != "Front desk" {
		t.Fatalf("history %+v", es)
	}
	// The history file keeps them across a restart.
	var back Entry
	if b, err := json.Marshal(es[0]); err != nil || json.Unmarshal(b, &back) != nil || back.Panel != "Wall" || back.Instance != "Front desk" {
		t.Fatalf("round trip %+v (%v)", back, err)
	}

	// Session mode: the page names itself; a user's take with no panel names none.
	other := makePanel(t, a, cy, `{"name":"Desk"}`)
	rec = apiPost(a, "/api/v1/takes", cy, `{"routes":[{"src":2,"dst":2,"levels":[1]}]}`, map[string]string{"X-Sode-Panel": other, "X-Sode-Instance": "Lobby"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if n := sentNote(); n != "unverified panel "+other+" (Desk), instance Lobby" {
		t.Fatalf("router messages %q", n)
	}
	es, _ = a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Panel != "Desk" || es[0].Instance != "Lobby" {
		t.Fatalf("history %+v", es)
	}
	rec = apiPost(a, "/api/v1/takes", cy, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	es, _ = a.hist.Page(-1, 1)
	if len(es) != 1 || es[0].Panel != "" || es[0].Instance != "" {
		t.Fatalf("history %+v", es)
	}
}

func TestSessionPanelTakeRowNamesPanelFromHeaders(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, `{"name":"Wall","slug":"wall"}`)
	f.seed(2, 1, 0)

	// A user's key proves nothing about the panel, so the page's word is marked.
	hdr := map[string]string{"X-Sode-Panel": id, "X-Sode-Instance": "Lobby"}
	rec := apiPost(a, "/api/v1/takes", cy, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, hdr)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	e := lastRow(t, a, "take")
	if !strings.Contains(e.Note, "unverified panel wall (Wall), instance Lobby") || e.By != "Cy (Desk Cy)" {
		t.Fatalf("row %+v", e)
	}

	// An id that is no panel names nothing, and a instance alone is not shown.
	rec = apiPost(a, "/api/v1/takes", cy, `{"routes":[{"src":2,"dst":2,"levels":[1]}]}`, map[string]string{"X-Sode-Panel": "p-00000000", "X-Sode-Instance": "Lobby"})
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	e = lastRow(t, a, "take", 2)
	if strings.Contains(e.Note, "panel") || strings.Contains(e.Note, "Lobby") {
		t.Fatalf("row %+v", e)
	}
}

func TestPanelWithoutASlugIsNamedByItsID(t *testing.T) {
	a, f := clientApp(t)
	shortTimers(a)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, apiPanel+`}`)
	secret, _ := mintKey(t, a, cy, id)
	f.seed(2, 1, 0)
	rec := apiPost(a, "/api/v1/takes", secret, `{"routes":[{"src":3,"dst":2,"levels":[1]}]}`, nil)
	if rec.Code != 200 {
		t.Fatalf("%d %s", rec.Code, rec.Body.String())
	}
	if e := lastRow(t, a, "take"); !strings.Contains(e.Note, "panel "+id+" (Wall), instance Lobby") {
		t.Fatalf("note %q", e.Note)
	}
}

func TestPanelStreamRowsNamePanelAndInstance(t *testing.T) {
	a, _ := clientApp(t)
	shortTimers(a)
	defer startLog(t, a)()
	cy := userKey(t, a, "Cy")
	id := makePanel(t, a, cy, slugPanel)
	secret, _ := mintKey(t, a, cy, id)
	c, _, ctx, cancel, err := strmDialRaw(strmServer(t, a), secret, "names,status")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	_, _, _ = c.Read(ctx) // hello
	if e := lastRow(t, a, "streamConnect"); !strings.Contains(e.Note, "panel wall (Wall), instance Lobby") {
		t.Fatalf("connect row %+v", e)
	}
	c.Close(websocket.StatusNormalClosure, "")
	if e := lastRow(t, a, "streamDisconnect"); !strings.Contains(e.Note, "panel wall (Wall), instance Lobby") {
		t.Fatalf("disconnect row %+v", e)
	}
}

func TestPanelLostAndBackRowsNameShortNameAndInstance(t *testing.T) {
	a, clk := monApp(t)
	defer startLog(t, a)()
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","slug":"wall"}`)
	watchPanel(t, a, id, ana)
	if keepalive(a, id, ana, "Lobby") != 200 {
		t.Fatal("keepalive")
	}
	clk.advance(50 * time.Second)
	a.monSweep()
	e := lastRow(t, a, "panelLost")
	if e.Kind != kindAPI || e.Note != "panel wall (Wall), instance Lobby: missed its keepalive for 50s; no device is online" {
		t.Fatalf("lost row %+v", e)
	}
	if keepalive(a, id, ana, "Lobby") != 200 {
		t.Fatal("keepalive")
	}
	e = lastRow(t, a, "panelBack")
	if e.Note != "panel wall (Wall), instance Lobby: is back after 50s" {
		t.Fatalf("back row %+v", e)
	}
}

func TestPanelAlarmRowNamesPanelAndInstance(t *testing.T) {
	a, clk := monApp(t)
	defer startLog(t, a)()
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","slug":"wall"}`)
	watchPanel(t, a, id, ana)
	watchInst(t, a, id, ana, "Lobby", 204)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	keepalive(a, id, ana, "Lobby")
	clk.advance(50 * time.Second)
	a.monSweep()
	a.serveAlarm(c, &inMsg{T: "panelAlarm", Panel: id, Instance: "lobby", Action: "clear"})
	if e := lastRow(t, a, "panelAlarm"); e.Note != "clear panel wall (Wall), instance lobby" {
		t.Fatalf("clear row %+v", e)
	}
	a.serveAlarm(c, &inMsg{T: "panelAlarm", Panel: id, Action: "stop"})
	if e := lastRow(t, a, "panelAlarm", 2); e.Note != "stop panel wall (Wall)" {
		t.Fatalf("stop row %+v", e)
	}
}
