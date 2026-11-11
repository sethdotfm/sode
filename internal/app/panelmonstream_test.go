package app

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"testing"
	"time"
)

// REST seam for the held connection of a panel page: a closed page is lost after monGrace, not
// monMiss. The clock is the fake one from panelmon_test.go; the connection is a real test server.

func monHold(t *testing.T, a *App, id, key, device string) context.CancelFunc {
	t.Helper()
	srv := strmServer(t, a)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, _ := http.NewRequestWithContext(ctx, "GET", srv.URL+"/api/v1/panels/"+id+"/instances/stream?device="+device, nil)
	req.Header.Set("Authorization", "Bearer "+key)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 200 {
		t.Fatalf("stream: %v %v", resp, err)
	}
	if _, err := bufio.NewReader(resp.Body).ReadString('\n'); err != nil {
		t.Fatalf("first line: %v", err)
	}
	return cancel
}

func monConns(a *App, id string) int {
	a.mon.mu.Lock()
	defer a.mon.mu.Unlock()
	n := 0
	for _, in := range a.mon.inst[id] {
		n += len(in.conns)
	}
	return n
}

func monWaitConns(t *testing.T, a *App, id string, want int) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); monConns(a, id) != want; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the server sees %d streams, want %d", monConns(a, id), want)
		}
	}
}

func TestClosedStreamIsLostAfterTheGraceNotTheMiss(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	_, next := signedIn(t, a, "10.0.0.1", "1234")
	next()

	cancel := monHold(t, a, id, ana, "Lobby")
	if panelOf(t, a, id, ana)["online"] != true {
		t.Fatal("not online with a stream open")
	}
	clk.advance(10 * time.Second)
	keepalive(a, id, ana, "Lobby") // a live page keeps beating
	clk.advance(10 * time.Second)
	a.monSweep()
	if n := notices(next(), "panelNotice"); len(n) != 0 {
		t.Fatalf("alert while the stream is open: %v", n)
	}

	cancel()
	monWaitConns(t, a, id, 0)
	clk.advance(monGrace - time.Second)
	a.monSweep()
	if n := notices(next(), "panelNotice"); len(n) != 0 || panelOf(t, a, id, ana)["online"] != true {
		t.Fatalf("lost inside the grace: %v", n)
	}
	clk.advance(2 * time.Second)
	a.monSweep()
	got := notices(next(), "panelNotice")
	if len(got) != 1 || got[0]["state"] != "lost" || got[0]["offline"] != true {
		t.Fatalf("want a lost notice after the grace, got %v", got)
	}
	if panelOf(t, a, id, ana)["online"] != false {
		t.Fatal("still online after the grace")
	}
}

func TestReloadInsideTheGraceRaisesNothing(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	_, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	cancel := monHold(t, a, id, ana, "Lobby")
	cancel()
	monWaitConns(t, a, id, 0)
	clk.advance(5 * time.Second)
	monHold(t, a, id, ana, "lobby")
	clk.advance(30 * time.Second)
	keepalive(a, id, ana, "Lobby")
	a.monSweep()
	if n := notices(next(), "panelNotice"); len(n) != 0 {
		t.Fatalf("a reload alerted: %v", n)
	}
}

func TestPageWithoutAStreamKeepsTheKeepaliveMiss(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	keepalive(a, id, ana, "Lobby")
	clk.advance(30 * time.Second)
	if panelOf(t, a, id, ana)["online"] != true {
		t.Fatal("a page that never held a stream is lost after the grace")
	}
}

func TestStreamEndsWhenItsPanelIsForgotten(t *testing.T) {
	a, _ := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	monHold(t, a, id, ana, "Lobby")
	a.mon.forget(id, false)
	// forget drops the instance, so its connection count reads zero at once; the handler must also return.
	a.mon.mu.Lock()
	n := len(a.mon.inst[id])
	a.mon.mu.Unlock()
	if n != 0 {
		t.Fatalf("instances left: %d", n)
	}
}

// An unnamed instance (askInstanceName off) holds its stream under its address.
func TestStreamWithNoDeviceHoldsUnderTheAddress(t *testing.T) {
	a, _ := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall","settings":{"askInstanceName":false}}`)
	monHold(t, a, id, ana, "")
	if n := monConns(a, id); n != 1 {
		t.Fatalf("held streams: %d", n)
	}
	if !strings.Contains(apiSend(a, "GET", "/api/v1/panels/"+id, ana, "").Body.String(), `"device":"127.0.0.1"`) {
		t.Fatal("the stream is not held under 127.0.0.1")
	}
}

func TestClearAlarmHidesTheOutageUntilTheNextNotice(t *testing.T) {
	a, clk := monApp(t)
	ana, cy := userKey(t, a, "Ana"), userKey(t, a, "Cy")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	watchPanel(t, a, id, cy)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	keepalive(a, id, ana, "Lobby")
	clk.advance(50 * time.Second)
	a.monSweep()
	if got := notices(next(), "panelsOffline"); len(got) != 1 || len(got[0]["offline"].([]any)) != 1 {
		t.Fatalf("badge state %v", got)
	}
	r := send(t, a, c, next, inMsg{T: "panelAlarm", Panel: id, Action: "clear"}, "panelsOffline")
	if len(r["offline"].([]any)) != 0 {
		t.Fatalf("badge after clear %v", r)
	}
	if rows := a.offlineFor("Cy"); len(rows) != 1 {
		t.Fatalf("clearing for Ana hid it from Cy: %v", rows)
	}
	if !a.mon.watching("Ana", id) {
		t.Fatal("clearing stopped the watch")
	}
	keepalive(a, id, ana, "Lobby") // back, then lost again
	clk.advance(50 * time.Second)
	a.monSweep()
	if rows := a.offlineFor("Ana"); len(rows) != 1 {
		t.Fatalf("a new outage stayed cleared: %v", rows)
	}
}

func TestStopMonitoringEndsTheWatch(t *testing.T) {
	a, _ := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	// Not watching: a clear changes nothing.
	a.serveAlarm(c, &inMsg{T: "panelAlarm", Panel: id, Action: "clear"})
	if a.mon.clear["ana"][id] {
		t.Fatal("cleared a panel the user does not watch")
	}
	watchPanel(t, a, id, ana)
	a.serveAlarm(c, &inMsg{T: "panelAlarm", Panel: id, Action: "bogus"})
	if !a.mon.watching("Ana", id) {
		t.Fatal("an unknown action stopped the watch")
	}
	rl := policy["panelAlarm"].forMsg(&inMsg{T: "panelAlarm"})
	rl.handle(a, c, &inMsg{T: "panelAlarm", Panel: id, Action: "stop"}, &connCtx{})
	if a.mon.watching("Ana", id) {
		t.Fatal("still watching after stop")
	}
}
