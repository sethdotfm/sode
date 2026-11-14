package app

import (
	"context"
	"strings"
	"testing"
	"time"
)

func serverRows(t *testing.T, a *App) []Sent {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { a.Run(ctx); close(done) }()
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run didn't return")
	}
	var out []Sent
	for _, e := range logged(t, a) {
		if e.Kind == kindServer {
			out = append(out, e)
		}
	}
	return out
}

func TestUnknownKeysLogged(t *testing.T) {
	a := futureApp(t, "futureKey: secret-value\nlocks:\n  futureLock: true\n")
	var errs []string
	for _, e := range serverRows(t, a) {
		if e.Err != "" {
			errs = append(errs, e.Err)
		}
	}
	want := "config.yaml has keys this release doesn't know, kept as is: futureKey, locks.futureLock"
	if len(errs) != 1 || errs[0] != want {
		t.Fatalf("server row errors = %q, want [%q]", errs, want)
	}
	if strings.Contains(errs[0], "secret-value") {
		t.Fatal("row names a value")
	}
}

func TestNoUnknownKeysNoProblemRow(t *testing.T) {
	a := futureApp(t, "")
	for _, e := range serverRows(t, a) {
		if e.Err != "" {
			t.Fatalf("unexpected problem row: %+v", e)
		}
	}
}
