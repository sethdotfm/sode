package app

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
	"github.com/sethdotfm/sode/internal/swp08"
)

// A History entry keeps the router title it was made under, so it stays readable after a rename.
func TestHistoryEntriesNameTheirRouter(t *testing.T) {
	a, f, key := takerApp(t, func(c *config.Config) { c.Routers[0].Title = "Studio Ultrix" })
	f.seed(2, 1, 0)
	uid := a.r().cfg.Routers[0].UID
	take := func() {
		t.Helper()
		if rec := apiPost(a, "/api/v1/takes", key, `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, nil); rec.Code != 200 {
			t.Fatalf("take: %d %s", rec.Code, rec.Body.String())
		}
	}
	take()
	cfg := a.r().cfg.Clone()
	cfg.Routers[0].Title = "Truck B"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	take()
	page := apiBody(t, apiDo(a, "GET", "/api/v1/history?n=5", key, nil))
	es := page["entries"].([]any)
	if len(es) != 2 {
		t.Fatalf("entries %v", es)
	}
	for i, want := range []string{"Truck B", "Studio Ultrix"} {
		e := es[i].(map[string]any)
		if e["router"] != uid || e["routerTitle"] != want {
			t.Errorf("entry %v: router %v title %v, want %s %q", e["id"], e["router"], e["routerTitle"], uid, want)
		}
	}
}

// Router messages and link events name the router; rows with no router behind them do not.
func TestLogRowsNameTheirRouter(t *testing.T) {
	a := newTestApp(t)
	cfg := a.r().cfg.Clone()
	cfg.Routers[0].Title = "Studio Ultrix"
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	stop := startLog(t, a)
	defer stop()
	a.onTx(swp08.TxEvent{Msg: swp08.ExtConnect(0, 0, 2, 4), Why: swp08.Why{Reason: swp08.ReasonTake}, At: time.Now(), Tries: 1, Result: "ack"})
	a.onUnitState(a.firstUnit(), swp08.StateUp, nil)
	a.logEvent(event(kindBrowser, "connect", "10.1.1.1", "connected"))
	rows := waitLogged(t, a, 3)
	uid := a.r().cfg.Routers[0].UID
	if got := results(rows, kindLink); got != "up" {
		t.Fatalf("link rows %q, want up", got)
	}
	for _, e := range rows {
		named := e.Kind != kindBrowser
		if named && (e.Router != uid || e.RouterTitle != "Studio Ultrix") {
			t.Errorf("%s row: router %q title %q, want %s Studio Ultrix", e.Kind, e.Router, e.RouterTitle, uid)
		}
		if !named && (e.Router != "" || e.RouterTitle != "") {
			t.Errorf("browser row names a router: %+v", e)
		}
	}
}

// Lines written before rows named their router still load.
func TestHistoryLineWithoutRouterLoads(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history.jsonl")
	line := `{"id":0,"time":"2026-11-01T10:00:00Z","client":"10.0.0.5","src":4,"srcName":"CAM 4","dst":2,"dstName":"MON 2","levels":[1],"prev":[[1,1]],"ok":true}` + "\n"
	if err := os.WriteFile(path, []byte(line), 0o644); err != nil {
		t.Fatal(err)
	}
	h, err := OpenHistory(path)
	if err != nil {
		t.Fatal(err)
	}
	es, _ := h.Page(-1, 5)
	if len(es) != 1 || es[0].Src != 4 || es[0].Router != "" || es[0].RouterTitle != "" {
		t.Fatalf("entries %+v", es)
	}
}
