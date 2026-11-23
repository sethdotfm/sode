package app

import (
	"fmt"
	"strings"
	"sync"
	"testing"

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
