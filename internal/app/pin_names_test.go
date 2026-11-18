package app

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"testing/fstest"
	"time"

	"github.com/coder/websocket"
)

// Waits after wrong PINs count per user name (docs/agents/users-locks.md).

// namedApp is userApp with PIN waits short enough to run out in a test.
func namedApp(t *testing.T) *App {
	t.Helper()
	a := userApp(t, false)
	shortTimers(a)
	return a
}

func waits(err error) bool { return err != nil && strings.Contains(err.Error(), "try again in") }

// wrongAs sends n wrong PINs for name and returns the last answer.
func wrongAs(t *testing.T, a *App, name string, n int) error {
	t.Helper()
	var err error
	for range n {
		if err = a.UserUnlockAs(&client{ip: "10.0.0.9"}, name, "0000"); err == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	return err
}

func TestWrongPinsForOneNameDoNotDelayAnother(t *testing.T) {
	a := namedApp(t)
	if err := wrongAs(t, a, "Ana", pinFreeTries+1); !waits(err) {
		t.Fatalf("Ana's 5th wrong PIN: %v", err)
	}
	if err := a.UserUnlockAs(&client{ip: "10.0.0.1"}, "Ana", "1234"); !waits(err) {
		t.Fatalf("Ana's right PIN during her wait: %v", err)
	}
	if err := a.UserUnlockAs(&client{ip: "10.0.0.2"}, "Bo", "9999"); err == nil || waits(err) {
		t.Fatalf("Bo's first wrong PIN: %v", err)
	}
	c := &client{ip: "10.0.0.2"}
	if err := a.UserUnlockAs(c, "Bo", "567890"); err != nil || c.who() != "Bo" {
		t.Fatalf("Bo held up by Ana's wrong PINs: %v", err)
	}
}

// The same over the socket, where the name comes from the sign-in message.
func TestSocketWaitsPerName(t *testing.T) {
	a := namedApp(t)
	srv := httptest.NewServer(a.Handler(fstest.MapFS{}))
	defer srv.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	ws, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(srv.URL, "http")+"/ws", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.CloseNow()
	ws.SetReadLimit(1 << 24)
	signIn := func(id int, user, pin string) userResult {
		b, _ := json.Marshal(map[string]any{"t": "userUnlock", "id": id, "user": user, "pin": pin})
		ws.Write(ctx, websocket.MessageText, b)
		for {
			_, b, err := ws.Read(ctx)
			if err != nil {
				t.Fatalf("no userUnlockResult: %v", err)
			}
			var r userResult
			if json.Unmarshal(b, &r) == nil && r.T == "userUnlockResult" && r.ID == id {
				return r
			}
		}
	}
	for i := range pinFreeTries + 1 {
		if r := signIn(i+1, "Ana", "0000"); r.OK {
			t.Fatal("wrong PIN accepted")
		}
	}
	if r := signIn(99, "Bo", "567890"); !r.OK {
		t.Fatalf("Bo refused after Ana's wrong PINs: %s", r.Err)
	}
}

func TestRightPinResetsThatNamesWait(t *testing.T) {
	a := namedApp(t)
	if err := wrongAs(t, a, "Ana", pinFreeTries+1); !waits(err) {
		t.Fatalf("no wait: %v", err)
	}
	time.Sleep(pinWaitFor(a, pinFreeTries+1) + 10*time.Millisecond)
	if err := a.UserUnlockAs(&client{ip: "10.0.0.1"}, "Ana", "1234"); err != nil {
		t.Fatalf("right PIN after the wait: %v", err)
	}
	if err := wrongAs(t, a, "Ana", 1); waits(err) {
		t.Fatalf("count not reset by the right PIN: %v", err)
	}
}

// pinWaitFor is the wait after n wrong PINs for a name that waits.
func pinWaitFor(a *App, n int) time.Duration { return userPinWait(a.pinFirstWait, n) }

func TestWaitSwitchOffSparesAllButAdministrators(t *testing.T) {
	a := namedApp(t)
	cfg := a.r().cfg
	cfg.PinWaits = false
	if err := a.SaveSettings(adminClient(t, a), cfg, "", false); err != nil {
		t.Fatal(err)
	}
	if err := wrongAs(t, a, "Ana", 3*pinFreeTries); waits(err) {
		t.Fatalf("a Standard user's name waits with the switch off: %v", err)
	}
	if err := a.UserUnlockAs(&client{ip: "10.0.0.1"}, "Ana", "1234"); err != nil {
		t.Fatalf("Ana: %v", err)
	}
	if err := wrongAs(t, a, "Ada", pinFreeTries+1); !waits(err) {
		t.Fatalf("an Administrator's name does not wait: %v", err)
	}
}

// One name's checks run one at a time, so every wrong PIN sent at once still counts.
func TestParallelWrongPinsForOneName(t *testing.T) {
	a := namedApp(t)
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		wrong int
	)
	for range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := a.UserUnlockAs(&client{ip: "10.0.0.5"}, "Bo", "000000")
			if err != nil && strings.HasPrefix(err.Error(), "wrong PIN") {
				mu.Lock()
				wrong++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if n := userPinCount(a, "Bo"); wrong == 0 || n != wrong {
		t.Fatalf("%d wrong PINs checked, %d counted", wrong, n)
	}
}

// userPinCount is the wrong-PIN count for a name; -1 for a name with none.
func userPinCount(a *App, name string) int {
	a.mu.Lock()
	defer a.mu.Unlock()
	u, ok := a.r().cfg.FindUserByName(name)
	if !ok {
		return -1
	}
	if p := a.userPins[u.ID]; p != nil {
		return p.n
	}
	return 0
}
