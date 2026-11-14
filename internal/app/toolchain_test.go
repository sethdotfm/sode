package app

import (
	"os"
	"regexp"
	"testing"
)

// CI reads the Go version from go.mod, so a mismatch with the Dockerfile
// means CI tests a different toolchain than the image ships.
func TestDockerfileGoMatchesGoMod(t *testing.T) {
	mod, err := os.ReadFile("../../go.mod")
	if err != nil {
		t.Fatalf("no go.mod to compare: %v", err)
	}
	docker, err := os.ReadFile("../../Dockerfile")
	if err != nil {
		t.Fatalf("no Dockerfile to compare: %v", err)
	}
	m := regexp.MustCompile(`(?m)^go (\d+\.\d+)(?:\.\d+)?\s*$`).FindSubmatch(mod)
	if m == nil {
		t.Fatal("go.mod has no go directive")
	}
	d := regexp.MustCompile(`(?m)^FROM\s.*golang:(\d+\.\d+)`).FindSubmatch(docker)
	if d == nil {
		t.Fatal("Dockerfile has no golang:X.Y base image")
	}
	if string(m[1]) != string(d[1]) {
		t.Errorf("go.mod says go %s but the Dockerfile builds with golang:%s", m[1], d[1])
	}
}
