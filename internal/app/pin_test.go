package app

import (
	"errors"
	"sync"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

func pinApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Defaults()
	cfg.Normalize()
	hist, _ := OpenHistory("")
	a, err := New(cfg, "", hist, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestSettingsPin(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	if a.locked(c) || a.Unlock(c, "") != nil {
		t.Fatal("no PIN set: should be open")
	}
	if err := a.SaveSettings(c, a.r().cfg, "12", false); err == nil {
		t.Fatal("short PIN accepted")
	}
	if err := a.SaveSettings(c, a.r().cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	// Unlocking (or just being open) before the PIN existed doesn't count.
	if !a.locked(c) {
		t.Fatal("PIN set: should be locked")
	}
	if err := a.SaveSettings(c, a.r().cfg, "", false); !errors.Is(err, errLocked) {
		t.Fatalf("locked save: %v", err)
	}
	if a.Unlock(c, "2468") != nil || a.locked(c) {
		t.Fatal("right PIN refused")
	}
	// A save from the browser (no hash in its JSON) keeps the PIN.
	if err := a.SaveSettings(c, a.r().cfg, "", false); err != nil || a.r().cfg.SettingsPin == "" || a.locked(c) {
		t.Fatalf("PIN lost on save: %v", err)
	}
	a.Lock(c)
	if !a.locked(c) {
		t.Fatal("lock didn't lock")
	}
	for range pinMaxFails {
		if a.Unlock(c, "0000") == nil {
			t.Fatal("wrong PIN accepted")
		}
	}
	if a.Unlock(c, "2468") == nil {
		t.Fatal("right PIN accepted during lockout")
	}
	other := &client{ip: "10.0.0.10"}
	if a.Unlock(other, "2468") != nil {
		t.Fatal("lockout should be per IP")
	}
	if err := a.SaveSettings(other, a.r().cfg, "", true); err != nil || a.locked(c) {
		t.Fatalf("clearing the PIN: %v", err)
	}
}

func TestPinChangeRelocks(t *testing.T) {
	a := pinApp(t)
	admin, c := &client{ip: "10.0.0.1"}, &client{ip: "10.0.0.2"}
	if err := a.SaveSettings(admin, a.r().cfg, "1111", false); err != nil {
		t.Fatal(err)
	}
	if a.Unlock(c, "1111") != nil || a.Unlock(admin, "1111") != nil {
		t.Fatal("unlock")
	}
	if err := a.SaveSettings(admin, a.r().cfg, "2222", false); err != nil {
		t.Fatal(err)
	}
	if !a.locked(c) {
		t.Fatal("unlock under the old PIN survived a change")
	}
	// Removing the PIN and setting the same one again still relocks.
	if a.Unlock(admin, "2222") != nil {
		t.Fatal("unlock with the new PIN")
	}
	if err := a.SaveSettings(admin, a.r().cfg, "", true); err != nil {
		t.Fatal(err)
	}
	if a.Unlock(c, "") != nil {
		t.Fatal("no PIN: unlock should pass")
	}
	if err := a.SaveSettings(admin, a.r().cfg, "2222", false); err != nil {
		t.Fatal(err)
	}
	if !a.locked(c) {
		t.Fatal("unlock with no PIN survived a PIN being set")
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
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.Unlock(&client{ip: "10.0.0.66"}, "0000"); err != nil && err.Error() == "wrong PIN" {
				mu.Lock()
				wrong++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	// Every try that was actually checked counts, however many arrive at once.
	if wrong >= pinMaxFails {
		if a.Unlock(&client{ip: "10.0.0.66"}, "1111") == nil {
			t.Fatalf("%d wrong PINs checked in parallel, no lockout", wrong)
		}
	} else if n := a.pinFails["10.0.0.66"].n; n != wrong {
		t.Fatalf("%d wrong PINs checked, %d counted", wrong, n)
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
