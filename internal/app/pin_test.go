package app

import (
	"errors"
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
	a, err := New(cfg, "", hist, false)
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestFilterSetsPin(t *testing.T) {
	a := pinApp(t)
	c := &client{ip: "10.0.0.9"}
	cfg := a.r().cfg
	cfg.LevelColors = []string{"#aabbcc"}
	if err := a.ApplyConfig(cfg); err != nil {
		t.Fatal(err)
	}
	before := a.r().cfg.LevelColors
	sets := []config.FilterSet{{Name: "No shader", Dst: "!MON", Sode: true, Ext: true}}
	if err := a.SaveFilterSets(c, sets); err != nil || len(a.r().cfg.FilterSets) != 1 {
		t.Fatalf("open save: %v", err)
	}
	// The new config has its own slices, not the live one's.
	if after := a.r().cfg.LevelColors; &after[0] == &before[0] {
		t.Fatal("level colors shared with the config being replaced")
	}
	if err := a.SaveFilterSets(c, []config.FilterSet{{Name: ""}}); err == nil {
		t.Fatal("unnamed set accepted")
	}
	if err := a.SaveSettings(c, a.r().cfg, "2468", false); err != nil {
		t.Fatal(err)
	}
	if err := a.SaveFilterSets(c, nil); !errors.Is(err, errLocked) {
		t.Fatalf("locked save: %v", err)
	}
	if a.Unlock(c, "2468") != nil {
		t.Fatal("unlock")
	}
	if err := a.SaveFilterSets(c, nil); err != nil || len(a.r().cfg.FilterSets) != 0 || a.r().cfg.SettingsPin == "" {
		t.Fatalf("unlocked save: %v, sets %v", err, a.r().cfg.FilterSets)
	}
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
	if a.Unlock(c, "0000") == nil {
		t.Fatal("wrong PIN accepted")
	}
	if err := a.SaveSettings(c, a.r().cfg, "", true); !errors.Is(err, errLocked) {
		t.Fatalf("locked clear: %v", err)
	}
	if a.Unlock(c, "2468") != nil {
		t.Fatal("right PIN refused")
	}
	if err := a.SaveSettings(c, a.r().cfg, "", true); err != nil || a.locked(c) {
		t.Fatalf("clearing the PIN: %v", err)
	}
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
	// Wrong PINs count for everyone, whatever the address.
	wrongPins(t, a, &client{ip: "10.0.0.7"}, pinFreeTries)
	wrongPins(t, a, &client{ip: "10.0.0.8"}, 1)
	if a.Unlock(admin, "2468") == nil {
		t.Fatal("right PIN accepted during the wait")
	}
	a.pin.until = time.Time{}
	if a.Unlock(admin, "2468") != nil || a.pin.n != 0 {
		t.Fatal("right PIN after the wait should unlock and reset the count")
	}
	// PinMaxTries in a row: locked until reset, waits or not.
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
	// One PIN is checked at a time and every checked one counts, however
	// many arrive at once and from however many addresses.
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
