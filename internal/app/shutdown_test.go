package app

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
	"github.com/sethdotfm/sode/internal/swp08"
)

// shutdownRig: an App running Run, a fake router and one browser socket.
type shutdownRig struct {
	a      *App
	f      *fakeRouter
	ws     *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc // Run's context: what SIGTERM cancels in main
	ran    chan struct{}
	held   sync.WaitGroup // held deliveries still sleeping
	// afterStop: a held message reached the router after the stop row, so the router link
	// had already been cut. sendMsg bypasses the swp08 client, so this row is the proof.
	afterStop atomic.Bool
	connects  atomic.Int32
}

func newShutdownRig(t *testing.T, hold time.Duration) *shutdownRig {
	t.Helper()
	a := newTestApp(t)
	r := &shutdownRig{a: a, f: newFakeRouter(t, a), ran: make(chan struct{})}
	a.sendMsg = func(m []byte, why swp08.Why) error {
		if m[0] == swp08.CmdExtConnect {
			r.connects.Add(1)
		}
		if hold == 0 {
			return r.f.deliver(m, why)
		}
		r.held.Add(1)
		go func() {
			defer r.held.Done()
			time.Sleep(hold)
			if stopLogged(a) {
				r.afterStop.Store(true)
			}
			r.f.deliver(m, why)
		}()
		return nil
	}
	t.Cleanup(r.held.Wait) // before the fake router's own cleanup closes its channel
	var run context.Context
	run, r.cancel = context.WithCancel(context.Background())
	go func() { a.Run(run); close(r.ran) }()
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	t.Cleanup(srv.Close)
	var done context.CancelFunc
	r.ctx, done = context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(done)
	ws, _, err := websocket.Dial(r.ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.CloseNow() })
	r.ws = ws
	return r
}

func stopLogged(a *App) bool {
	page, _, _ := a.sent.Page(-1, 500)
	for _, e := range page {
		if e.Kind == kindServer && e.Result == "stop" {
			return true
		}
	}
	return false
}

func (r *shutdownRig) send(t *testing.T, v any) {
	t.Helper()
	b, _ := json.Marshal(v)
	if err := r.ws.Write(r.ctx, websocket.MessageText, b); err != nil {
		t.Fatal(err)
	}
}

func (r *shutdownRig) take(t *testing.T, id, src, dst int) {
	r.send(t, map[string]any{"t": "batch", "items": []map[string]any{{"id": id, "src": src, "dst": dst, "levels": []int{1}}}})
}

// sentConnect waits until the take's connect has been sent, so it is in flight.
func (r *shutdownRig) sentConnect(t *testing.T) {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if r.connects.Load() > 0 {
			return
		}
	}
	t.Fatal("the take never reached the router")
}

func (r *shutdownRig) stop(t *testing.T, within time.Duration) {
	t.Helper()
	r.cancel()
	select {
	case <-r.ran:
	case <-time.After(within):
		t.Fatalf("Run didn't return within %v", within)
	}
}

func takeEntries(a *App) []Entry {
	es, _ := a.hist.Page(-1, 50)
	var out []Entry
	for _, e := range es {
		if !e.Ext {
			out = append(out, e)
		}
	}
	return out
}

// A take in flight at shutdown finishes with the router link still up, and its History
// entry is written before Run returns.
func TestShutdownWaitsForTakeInFlight(t *testing.T) {
	r := newShutdownRig(t, 300*time.Millisecond)
	r.take(t, 1, 4, 6)
	r.sentConnect(t)
	r.stop(t, 5*time.Second)
	es := takeEntries(r.a)
	if len(es) != 1 || es[0].Dst != 6 || es[0].Src != 4 || !es[0].OK {
		t.Fatalf("History when Run returned: %+v", es)
	}
	r.held.Wait()
	if r.afterStop.Load() {
		t.Fatal("the take reached the router after the stop row: the link was cut before it finished")
	}
	rows := logged(t, r.a)
	if last := rows[len(rows)-1]; last.Kind != kindServer || last.Result != "stop" {
		t.Fatalf("last row = %+v", last)
	}
}

// readResult reads the socket until the takeResult for id.
func (r *shutdownRig) readResult(t *testing.T, id int) routeResult {
	t.Helper()
	for {
		_, b, err := r.ws.Read(r.ctx)
		if err != nil {
			t.Fatalf("no takeResult %d: %v", id, err)
		}
		var res routeResult
		if json.Unmarshal(b, &res) == nil && res.T == "takeResult" && res.ID == id {
			return res
		}
	}
}

// A take still waiting at the limit doesn't hold shutdown, and the log names it.
func TestShutdownStopsAtTheLimit(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	defer log.SetOutput(os.Stderr)
	r := newShutdownRig(t, 0)
	r.a.drainLimit = 100 * time.Millisecond
	r.a.routeTimeout = 800 * time.Millisecond // the router never answers, so the take waits this long
	r.f.mu.Lock()
	r.f.ignore[6] = true
	r.f.mu.Unlock()
	r.take(t, 1, 4, 6)
	r.sentConnect(t)
	began := time.Now()
	r.stop(t, 5*time.Second)
	if d := time.Since(began); d > 600*time.Millisecond {
		t.Fatalf("Run took %v, past the limit", d)
	}
	if len(takeEntries(r.a)) != 0 {
		t.Fatal("the take finished before the limit, so this proves nothing")
	}
	// The take ends on its own timer; wait so it doesn't outlive the test.
	if res := r.readResult(t, 1); res.Err == "" {
		t.Fatalf("an unanswered take succeeded: %+v", res)
	}
	log.SetOutput(os.Stderr)
	if !strings.Contains(buf.String(), "writes still running: batch 1") {
		t.Fatalf("log doesn't name the take left running:\n%s", buf.String())
	}
}

// A take sent once shutdown began is refused with a failed result and never reaches the router.
func TestShutdownRefusesNewTakes(t *testing.T) {
	r := newShutdownRig(t, 0)
	r.stop(t, 5*time.Second)
	r.take(t, 9, 4, 6)
	if res := r.readResult(t, 9); res.Err != errShuttingDown.Error() {
		t.Fatalf("result %+v", res)
	}
	if n := r.connects.Load(); n != 0 {
		t.Fatalf("%d connects reached the router", n)
	}
	if len(takeEntries(r.a)) != 0 {
		t.Fatal("a refused take has a History entry")
	}
}

// A save that starts nothing (no config in it) still ends its count, so shutdown doesn't
// wait the limit for it.
func TestShutdownNotHeldByEmptySave(t *testing.T) {
	r := newShutdownRig(t, 0)
	r.a.drainLimit = 3 * time.Second
	r.send(t, map[string]any{"t": "config"})
	r.take(t, 2, 4, 6) // handled after the save, in order: its result means the save was too
	if res := r.readResult(t, 2); res.Err != "" {
		t.Fatalf("take %+v", res)
	}
	began := time.Now()
	r.stop(t, 5*time.Second)
	if d := time.Since(began); d > time.Second {
		t.Fatalf("Run waited %v for a save that never started", d)
	}
}
