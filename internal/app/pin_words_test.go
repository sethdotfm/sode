package app

import (
	"testing"
	"time"
)

// The lock shows the server's wait as it is, so the wait is words (seconds, then minutes), never a Go duration ("1m30s").
func TestPinWaitInWords(t *testing.T) {
	for d, want := range map[time.Duration]string{
		time.Second:              "1 second",
		400 * time.Millisecond:   "1 second",
		29500 * time.Millisecond: "30 seconds",
		30 * time.Second:         "30 seconds",
		60 * time.Second:         "60 seconds",
		61 * time.Second:         "1 minute 1 second",
		90 * time.Second:         "1 minute 30 seconds",
		120 * time.Second:        "2 minutes",
		5 * time.Minute:          "5 minutes",
	} {
		if got := waitWords(d); got != want {
			t.Errorf("waitWords(%v) = %q, want %q", d, got, want)
		}
	}
	a := userApp(t, true)
	c := &client{ip: "10.0.0.1"}
	var err error
	for range pinFreeTries + 1 {
		err = a.UserUnlockAs(c, "Ana", "0000")
	}
	if err == nil || err.Error() != "wrong PIN; try again in 30 seconds" {
		t.Fatalf("5th wrong PIN: %v", err)
	}
	if err := a.UserUnlockAs(c, "Ana", "0000"); err == nil || err.Error() != "too many wrong PINs; try again in 30 seconds" {
		t.Fatalf("a PIN during the wait: %v", err)
	}
}
