package app

import (
	"net"
	"strings"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// closedAddr gives an address with nothing listening on it.
func closedAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	l.Close()
	return addr
}

// silentAddr gives an address that accepts a connection and never answers, as a wrong port can.
func silentAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			t.Cleanup(func() { c.Close() })
		}
	}()
	return l.Addr().String()
}

func tryMsg(addr string) map[string]any {
	return map[string]any{"try": map[string]any{"address": addr, "matrix": 0, "nameRequests": "std", "nameLength": 12}}
}

// Only an Administrator can try an address: it dials from the server. A Standard user and a Trusted user are refused
// for the Role, not for a lock, so the page asks for no PIN.
func TestRouterTryNeedsAnAdministrator(t *testing.T) {
	a := userApp(t, false)
	p := dialPeer(t, a)
	p.signIn("Ana", "1234")
	refusedForRole(t, "routerTry as a Standard user", p.ask("routerTry", 1, tryMsg(closedAddr(t))), config.RoleAdministrator)

	q := dialPeer(t, a)
	q.signIn("Ada", "2468")
	r := q.ask("routerTry", 2, tryMsg(closedAddr(t)))
	if r["ok"] != true || r["state"] != tryFailed || !strings.Contains(r["err"].(string), "refused") {
		t.Fatalf("routerTry as an Administrator, closed port: %v", r)
	}
	if r := q.ask("routerTry", 3, tryMsg(" ")); r["ok"] != true || r["state"] != tryCancelled {
		t.Fatalf("a blank address only cancels: %v", r)
	}
}

// The reply states of a try. Against swp08sim it is connected, with the counts of the names. A closed port fails. A
// port that accepts and never answers fails with "no answer". The next try cancels the one that runs. Nothing moves
// the Router link of the router.
func TestRouterTryStates(t *testing.T) {
	slowTest(t)
	sim, _ := simServer(t)
	defer func(d time.Duration) { tryTimeout = d }(tryTimeout)
	tryTimeout = 1500 * time.Millisecond
	a := newTestApp(t)
	p := dialPeer(t, a)
	was := a.r().cfg.DefaultRouter().Address

	r := p.ask("routerTry", 1, tryMsg(sim))
	if r["state"] != tryConnected || r["sources"] != float64(200) || r["destinations"] != float64(120) {
		t.Fatalf("swp08sim: %v", r)
	}
	if r := p.ask("routerTry", 2, tryMsg(closedAddr(t))); r["state"] != tryFailed || !strings.Contains(r["err"].(string), "refused") {
		t.Fatalf("closed port: %v", r)
	}
	if r := p.ask("routerTry", 3, tryMsg(silentAddr(t))); r["state"] != tryFailed || r["err"] != errTryNoAnswer.Error() {
		t.Fatalf("silent port: %v", r)
	}
	ext := tryMsg(sim)
	ext["try"].(map[string]any)["matrix"] = 20
	if r := p.ask("routerTry", 4, ext); r["state"] != tryFailed || r["err"] != "the Matrix does not suit the Name requests" {
		t.Fatalf("matrix 20 with Standard name requests: %v", r)
	}

	// A try on the silent port, then the next key: the first answers cancelled, the second connected.
	p.send(map[string]any{"t": "routerTry", "id": 5, "try": tryMsg(silentAddr(t))["try"]})
	time.Sleep(100 * time.Millisecond)
	p.send(map[string]any{"t": "routerTry", "id": 6, "try": tryMsg(sim)["try"]})
	got := map[float64]any{}
	for len(got) < 2 {
		m := p.until("routerTryResult")
		got[m["id"].(float64)] = m["state"]
	}
	if got[5] != tryCancelled || got[6] != tryConnected {
		t.Fatalf("cancel by the next try: %v", got)
	}
	if now := a.r().cfg.DefaultRouter().Address; now != was {
		t.Fatalf("the router's address moved from %q to %q", was, now)
	}
}
