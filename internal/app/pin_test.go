package app

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

func pinApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Defaults()
	cfg.Normalize()
	hist, _ := OpenHistory("")
	a, err := New(cfg, "", hist, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestPinWait(t *testing.T) {
	for n, want := range map[int]time.Duration{1: 0, 4: 0, 5: 30 * time.Second, 6: time.Minute, 7: 2 * time.Minute, 50: time.Hour} {
		if got := pinWait(n); got != want {
			t.Errorf("pinWait(%d) = %v, want %v", n, got, want)
		}
	}
}

// wrongPins sends n wrong PINs, skipping the waits between them.
func wrongPins(t *testing.T, a *App, c *client, n int) {
	t.Helper()
	for range n {
		a.mu.Lock()
		a.pin.until = time.Time{}
		a.mu.Unlock()
		if a.Unlock(c, "0000") == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
}

func TestPinLockout(t *testing.T) {
	a := pinApp(t)
	admin := &client{ip: "10.0.0.1"}
	cfg := a.r().cfg
	cfg.PinMaxTries = 6
	if err := a.SaveSettings(admin, cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	wrongPins(t, a, &client{ip: "10.0.0.7"}, pinFreeTries)
	wrongPins(t, a, &client{ip: "10.0.0.8"}, 1)
	if a.Unlock(admin, "2468") == nil {
		t.Fatal("right PIN accepted during the wait")
	}
	a.pin.until = time.Time{}
	if a.Unlock(admin, "2468") != nil || a.pin.n != 0 {
		t.Fatal("right PIN after the wait should unlock and reset the count")
	}
	wrongPins(t, a, admin, 6)
	a.pin.until = time.Time{}
	if a.Unlock(admin, "2468") == nil {
		t.Fatal("right PIN accepted after PinMaxTries wrong")
	}
	// 0 = never locked for good.
	a = pinApp(t)
	cfg = a.r().cfg
	cfg.PinMaxTries = 0
	if err := a.SaveSettings(admin, cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	wrongPins(t, a, admin, 12)
	a.pin.until = time.Time{}
	if a.Unlock(admin, "2468") != nil {
		t.Fatal("PinMaxTries 0 locked for good")
	}
}

func TestParallelWrongPins(t *testing.T) {
	a := pinApp(t)
	admin := &client{ip: "10.0.0.1"}
	if err := a.SaveSettings(admin, a.r().cfg, "1111", false); err != nil {
		t.Fatal(err)
	}
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		wrong int
	)
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.Unlock(&client{ip: fmt.Sprintf("10.0.1.%d", i)}, "0000"); err != nil && strings.HasPrefix(err.Error(), "wrong PIN") {
				mu.Lock()
				wrong++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if wrong == 0 || a.pin.n != wrong {
		t.Fatalf("%d wrong PINs checked, %d counted", wrong, a.pin.n)
	}
}

func TestConcurrentSavesKeepPin(t *testing.T) {
	a := pinApp(t)
	setter, other := &client{ip: "10.0.0.1"}, &client{ip: "10.0.0.2"}
	stale := a.r().cfg
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); a.SaveSettings(setter, stale, "1357", false) }()
	go func() { defer wg.Done(); a.SaveSettings(other, stale, "", false) }()
	wg.Wait()
	if a.r().cfg.SettingsPin == "" {
		t.Fatal("a concurrent save dropped the new PIN")
	}
}
