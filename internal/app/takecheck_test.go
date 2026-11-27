package app

import (
	"context"
	"slices"
	"testing"
	"time"
)

// Against swp08sim, the Communication Log shows a big take's reads before its connects.
func TestTakeBigWithSimulator(t *testing.T) {
	slowTest(t)
	sim, _ := simServer(t)
	a := simApp(t, sim)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	t.Cleanup(func() { cancel(); <-done })
	for end := time.Now().Add(10 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		a.mu.Lock()
		n := len(a.firstUnit().router.tally)
		a.mu.Unlock()
		if n >= 120 {
			break
		}
		if time.Now().After(end) {
			t.Fatal("the app never read the tally from swp08sim")
		}
	}
	// As if the start-up dump were older than checkFresh.
	a.mu.Lock()
	clear(a.firstUnit().router.seenAt)
	a.mu.Unlock()
	items := bigItems(30, 40, 41, 42, 43, 44, 45, 46, 47)
	for i := range items {
		items[i].Levels = []int{1}
	}
	if a.takeAllowed(&client{ip: "10.0.0.1"}, items) {
		t.Fatal("check did not finish")
	}
	var rows []Sent
	for end := time.Now().Add(5 * time.Second); ; time.Sleep(20 * time.Millisecond) {
		rows, _, _ = a.sent.Page(-1, 500)
		if slices.ContainsFunc(rows, func(s Sent) bool { return s.Why == "take" && s.Kind == "connect" }) {
			break
		}
		if time.Now().After(end) {
			t.Fatalf("no connect rows in %+v", rows)
		}
	}
	slices.SortFunc(rows, func(x, y Sent) int { return int(x.Seq - y.Seq) })
	first := slices.IndexFunc(rows, func(s Sent) bool { return s.Why == "take" && s.Kind == "connect" })
	var checks []string
	for _, s := range rows {
		if s.Why == "check" {
			checks = append(checks, s.Kind)
			if s.Seq > rows[first].Seq {
				t.Fatalf("check row %+v after the first connect %+v", s, rows[first])
			}
		}
	}
	// 120 destinations are 2 dump answers, fewer than the take's 8 destinations on level 1.
	if !slices.Equal(checks, []string{"tallyDump"}) {
		t.Fatalf("check rows %v, want one tally dump", checks)
	}
}
