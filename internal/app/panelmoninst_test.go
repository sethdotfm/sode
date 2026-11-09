package app

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// Per-instance watch: a user watches a whole panel, one named instance, or both. The names are
// the device names of panelmon.go; the clock is the fake one from panelmon_test.go.

func watchInst(t *testing.T, a *App, id, key, device string, want int) {
	t.Helper()
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/watch?instance="+url.QueryEscape(device), key, ""); rec.Code != want {
		t.Fatalf("watch %s: %d %s", device, rec.Code, rec.Body.String())
	}
}

func TestInstanceWatchIsToldOnlyAboutItsDevice(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchInst(t, a, id, ana, "Studio A", 204)
	_, next := signedIn(t, a, "10.0.0.1", "1234")
	next()

	keepalive(a, id, ana, "Studio A")
	keepalive(a, id, ana, "Studio B")
	clk.advance(40 * time.Second)
	keepalive(a, id, ana, "Studio A")
	clk.advance(10 * time.Second)
	a.monSweep() // Studio B is lost, and Ana does not watch it
	if n := notices(next(), "panelNotice"); len(n) != 0 {
		t.Fatalf("told about an instance not watched: %v", n)
	}
	if rows := a.offlineFor("Ana"); len(rows) != 0 {
		t.Fatalf("badge rows for an instance not watched: %v", rows)
	}

	clk.advance(50 * time.Second)
	keepalive(a, id, ana, "studio b") // keeps the panel online
	a.monSweep()
	got := notices(next(), "panelNotice")
	if len(got) != 1 || got[0]["device"] != "Studio A" || got[0]["state"] != "lost" || got[0]["offline"] != false {
		t.Fatalf("want one lost notice for Studio A that does not call the panel offline, got %v", got)
	}
	rows := a.offlineFor("Ana")
	if len(rows) != 1 || rows[0].Device != "Studio A" || rows[0].Instance != "studio a" {
		t.Fatalf("rows %v", rows)
	}

	keepalive(a, id, ana, "Studio A")
	back := notices(next(), "panelNotice")
	if len(back) != 1 || back[0]["state"] != "back" || back[0]["device"] != "Studio A" {
		t.Fatalf("return notice %v", back)
	}
	if rows := a.offlineFor("Ana"); len(rows) != 0 {
		t.Fatalf("still listed after returning: %v", rows)
	}
}

func TestPanelWatchStillSeesEveryDeviceAndBothKindsCoexist(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	watchInst(t, a, id, ana, "Lobby", 204)
	p := panelOf(t, a, id, ana)
	if p["watching"] != true {
		t.Fatalf("whole-panel watch lost: %v", p)
	}
	wi, _ := p["watchingInstances"].([]any)
	if len(wi) != 1 || wi[0] != "lobby" {
		t.Fatalf("watchingInstances %v", p["watchingInstances"])
	}
	_, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	keepalive(a, id, ana, "Lobby")
	keepalive(a, id, ana, "Stage")
	clk.advance(30 * time.Second)
	keepalive(a, id, ana, "Stage")
	clk.advance(20 * time.Second)
	a.monSweep()
	got := notices(next(), "panelNotice")
	if len(got) != 1 {
		t.Fatalf("one device lost must give one notice even with two watches: %v", got)
	}
}

func TestUnwatchInstanceKeepsThePanelWatch(t *testing.T) {
	a, _ := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	watchInst(t, a, id, ana, "Lobby", 204)
	if rec := apiSend(a, "DELETE", "/api/v1/panels/"+id+"/watch?instance=LOBBY", ana, ""); rec.Code != 204 {
		t.Fatalf("unwatch: %d", rec.Code)
	}
	p := panelOf(t, a, id, ana)
	if p["watching"] != true {
		t.Fatal("removing an instance ended the panel watch")
	}
	if wi, _ := p["watchingInstances"].([]any); len(wi) != 0 {
		t.Fatalf("instance still watched: %v", wi)
	}
	if rec := apiSend(a, "POST", "/api/v1/panels/"+id+"/watch?instance=%20%20", ana, ""); rec.Code != 400 {
		t.Fatalf("blank instance: %d", rec.Code)
	}
}

func TestInstanceWatchIsStoredWithThePanelIDAndLoadsBack(t *testing.T) {
	a, _ := monApp(t)
	dir := t.TempDir()
	a.mon.path = filepath.Join(dir, watchFile)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	watchInst(t, a, id, ana, "Studio A", 204)
	b, err := os.ReadFile(a.mon.path)
	if err != nil {
		t.Fatal(err)
	}
	want := "ana:\n    - " + id + "\n    - " + id + "/studio a\n"
	if string(b) != want {
		t.Fatalf("file %q, want %q", b, want)
	}
	m := loadPanelMon(filepath.Join(dir, "config.yaml"))
	if !m.watching("Ana", id) || !m.watchingInst("Ana", id, "STUDIO A") {
		t.Fatalf("not loaded: %v", m.watch)
	}
	// A file from before instances held plain panel ids and still loads.
	if err := os.WriteFile(a.mon.path, []byte("ana:\n    - "+id+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if m := loadPanelMon(filepath.Join(dir, "config.yaml")); !m.watching("Ana", id) {
		t.Fatal("old file did not load")
	}
}

func TestAlarmClearAndStopFollowTheInstance(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchInst(t, a, id, ana, "Lobby", 204)
	c, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	keepalive(a, id, ana, "Lobby")
	clk.advance(50 * time.Second)
	a.monSweep()
	r := send(t, a, c, next, inMsg{T: "panelAlarm", Panel: id, Instance: "lobby", Action: "clear"}, "panelsOffline")
	if len(r["offline"].([]any)) != 0 {
		t.Fatalf("badge after clear %v", r)
	}
	if !a.mon.watchingInst("Ana", id, "lobby") {
		t.Fatal("clear ended the watch")
	}
	a.serveAlarm(c, &inMsg{T: "panelAlarm", Panel: id, Instance: "lobby", Action: "stop"})
	if a.mon.watchingInst("Ana", id, "lobby") {
		t.Fatal("stop kept the instance watch")
	}
}

func TestDeletingAPanelForgetsItsInstanceWatches(t *testing.T) {
	a, _ := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchInst(t, a, id, ana, "Lobby", 204)
	a.mon.forget(id, true)
	if a.mon.watchingInst("Ana", id, "Lobby") {
		t.Fatal("instance watch outlived the panel")
	}
}

// A killed browser resets the connection instead of closing it; the page still goes lost after the
// grace, well inside 15 seconds.
func TestResetStreamIsLostAfterTheGrace(t *testing.T) {
	a, clk := monApp(t)
	ana := userKey(t, a, "Ana")
	id := makePanel(t, a, ana, `{"name":"Wall"}`)
	watchPanel(t, a, id, ana)
	_, next := signedIn(t, a, "10.0.0.1", "1234")
	next()
	srv := strmServer(t, a)
	c, err := net.Dial("tcp", strings.TrimPrefix(srv.URL, "http://"))
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintf(c, "GET /api/v1/panels/%s/instances/stream?device=Lobby HTTP/1.1\r\nHost: x\r\nAuthorization: Bearer %s\r\n\r\n", id, ana)
	monWaitConns(t, a, id, 1)
	c.(*net.TCPConn).SetLinger(0)
	c.Close()
	monWaitConns(t, a, id, 0)
	clk.advance(monGrace + time.Second)
	a.monSweep()
	if got := notices(next(), "panelNotice"); len(got) != 1 || got[0]["state"] != "lost" {
		t.Fatalf("want a lost notice, got %v", got)
	}
	if monGrace+time.Second >= 15*time.Second {
		t.Fatal("detection is not inside 15 seconds")
	}
}

// A dead peer sends no FIN, so only TCP keepalive on the accepted connection ends its stream.
func TestAcceptedConnectionsGetTCPKeepalive(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	kl := KeepAliveListener(ln)
	go func() {
		c, err := net.Dial("tcp", ln.Addr().String())
		if err == nil {
			defer c.Close()
			time.Sleep(time.Second)
		}
	}()
	c, err := kl.Accept()
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	raw, err := c.(*net.TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var on int
	raw.Control(func(fd uintptr) { on, _ = syscall.GetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_KEEPALIVE) })
	if on == 0 {
		t.Fatal("SO_KEEPALIVE is off")
	}
}
