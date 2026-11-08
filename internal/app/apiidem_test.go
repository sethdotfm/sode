package app

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/sethdotfm/sode/internal/config"
)

// A slot still in flight is never evicted, and a repeat that waited gets the committed answer
// even when the map dropped the slot meanwhile; otherwise the repeat runs the take again.
func TestIdemEvictionKeepsInFlightSlot(t *testing.T) {
	a := &App{}
	now := time.Now()
	first, owner, _ := a.apiS.idemBegin(context.Background(), "key:a", "k", "fp", now)
	if !owner {
		t.Fatal("first request must own the slot")
	}
	type res struct {
		slot  *idemSlot
		owner bool
	}
	got := make(chan res, 1)
	go func() {
		sl, own, _ := a.apiS.idemBegin(context.Background(), "key:a", "k", "fp", now)
		got <- res{sl, own}
	}()
	time.Sleep(20 * time.Millisecond)
	for i := 0; i < idemMax; i++ {
		sl, _, _ := a.apiS.idemBegin(context.Background(), "key:b", strconv.Itoa(i), "fp", now.Add(time.Second))
		a.apiS.idemCommit(sl, 200, []byte("x\n"))
	}
	a.apiS.mu.Lock()
	_, kept := a.apiS.idem["key:a\x00k"]
	a.apiS.mu.Unlock()
	if !kept {
		t.Fatal("an in-flight slot was evicted")
	}
	a.apiS.idemCommit(first, 200, []byte("first\n"))
	select {
	case r := <-got:
		if r.owner || r.slot != first {
			t.Fatalf("the repeat did not replay the first answer: owner=%v", r.owner)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the repeat never returned")
	}
}

func TestIdemWaiterGetsCommittedSlotEvenIfDropped(t *testing.T) {
	a := &App{}
	now := time.Now()
	first, _, _ := a.apiS.idemBegin(context.Background(), "key:a", "k", "fp", now)
	got := make(chan bool, 1)
	go func() {
		sl, own, _ := a.apiS.idemBegin(context.Background(), "key:a", "k", "fp", now)
		got <- !own && sl == first
	}()
	time.Sleep(20 * time.Millisecond)
	a.apiS.mu.Lock()
	delete(a.apiS.idem, "key:a\x00k")
	a.apiS.mu.Unlock()
	a.apiS.idemCommit(first, 200, []byte("first\n"))
	if !<-got {
		t.Fatal("the waiting repeat ran its own take")
	}
}

// Guests behind one NAT share an address, so a keyless call cannot hold an Idempotency-Key.
func TestAPIWGuestTakeRefusesIdempotencyKey(t *testing.T) {
	a, _, _ := takerApp(t, func(c *config.Config) {
		c.API.GuestEnabled = true
		c.GuestAPIScopes = append(c.GuestAPIScopes, config.ScopeTake)
	})
	rec := apiPost(a, "/api/v1/takes", "", `{"routes":[{"src":4,"dst":2,"levels":[1]}]}`, map[string]string{"Idempotency-Key": "k"})
	wantErr(t, rec, 400, "bad_request")
}
