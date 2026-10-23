package app

import (
	"sync"
	"testing"
	"time"
)

func TestRunLimited(t *testing.T) {
	var mu sync.Mutex
	running, peak, done := 0, 0, map[int]bool{}
	runLimited(3, 20, func(i int) {
		mu.Lock()
		running++
		peak = max(peak, running)
		mu.Unlock()
		time.Sleep(5 * time.Millisecond)
		mu.Lock()
		running--
		done[i] = true
		mu.Unlock()
	})
	if peak > 3 {
		t.Fatalf("peak %d running at once, limit 3", peak)
	}
	if len(done) != 20 {
		t.Fatalf("ran %d of 20", len(done))
	}
}
