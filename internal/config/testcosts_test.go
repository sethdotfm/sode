package config

import (
	"strings"
	"testing"
)

// This package never calls UseTestCosts, so these are the shipped values.
func TestCostsDefaultToFull(t *testing.T) {
	if h, _ := HashPin("1234"); !strings.HasPrefix(h, "pbkdf2-sha256$600000$") {
		t.Fatalf("default PIN hash lost its round count: %s", h)
	}
	if !SyncWrites() {
		t.Fatal("saves do not fsync by default")
	}
}
