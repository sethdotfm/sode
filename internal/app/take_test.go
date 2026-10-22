package app

import (
	"testing"

	"github.com/sethdotfm/sode/internal/swp08"
)

func TestWaiterAlreadyRouted(t *testing.T) {
	w := &waiter{dst: 9, src: 2, wire: map[int]bool{0: true, 1: true}, already: map[int]bool{}, done: make(chan struct{})}
	// Level 0 changes: announced with Connected.
	w.see(swp08.Crosspoint{Level: 0, Dst: 9, Src: 2, Cmd: swp08.CmdExtConnected})
	// Wrong source on level 1 doesn't count.
	w.see(swp08.Crosspoint{Level: 1, Dst: 9, Src: 3, Cmd: swp08.CmdExtTally})
	if w.closed {
		t.Fatal("finished early")
	}
	// Level 1 was already on src: confirmed by the interrogate's Tally.
	w.see(swp08.Crosspoint{Level: 1, Dst: 9, Src: 2, Cmd: swp08.CmdExtTally})
	if !w.closed {
		t.Fatal("not finished")
	}
	if w.already[0] || !w.already[1] {
		t.Fatalf("already = %v, want only level 1", w.already)
	}
}
