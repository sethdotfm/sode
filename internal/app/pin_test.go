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

// Wrong PINs checked at the same time are each counted once in their name's pool.
func TestParallelWrongPins(t *testing.T) {
	a := pinApp(t)
	u := mkUser(t, "Ana", "1111")
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		wrong int
	)
	for i := range 30 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := a.provePin(&client{ip: fmt.Sprintf("10.0.1.%d", i)}, nil, &u, "0000")
			if err != nil && strings.HasPrefix(err.Error(), "wrong PIN") {
				mu.Lock()
				wrong++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	a.mu.Lock()
	n := a.userPoolMu(&u).n
	a.mu.Unlock()
	if wrong == 0 || n != wrong {
		t.Fatalf("%d wrong PINs checked, %d counted", wrong, n)
	}
}

// The lock shows the wait as words, not as a Go duration such as "1m30s".
func TestPinWaitInWords(t *testing.T) {
	for d, want := range map[time.Duration]string{
		30 * time.Second:        "30 seconds",
		90 * time.Second:        "90 seconds",
		time.Second:             "1 second",
		1500 * time.Millisecond: "2 seconds",
		20 * time.Millisecond:   "1 second",
		5 * time.Minute:         "5 minutes",
		150 * time.Second:       "3 minutes",
	} {
		if got := inWords(d); got != want {
			t.Errorf("inWords(%v) = %q, want %q", d, got, want)
		}
	}
}
