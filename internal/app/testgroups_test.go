package app

import (
	"os"
	"strings"
	"testing"

	"github.com/sethdotfm/sode/internal/config"
)

// Test groups (AGENTS.md "Test groups"): fast is everything -short runs; browser is TestChrome*;
// slow is anything that calls slowTest. CI selects the groups with -short, -run and -skip, so the
// names and calls here are what keep its jobs from missing a test.

func TestMain(m *testing.M) {
	config.UseTestCosts()
	code := m.Run()
	if simBuild.dir != "" {
		_ = os.RemoveAll(simBuild.dir)
	}
	os.Exit(code)
}

// slowTest skips under -short. Use it for any non-browser test over about a second, and for leak checks.
func slowTest(t *testing.T) {
	t.Helper()
	if testing.Short() {
		t.Skip("slow test skipped under -short")
	}
}

// browserTest runs first in startBrowser and startSim, so -short skips a browser check before it builds anything.
func browserTest(t *testing.T) {
	t.Helper()
	if !strings.HasPrefix(t.Name(), "TestChrome") {
		t.Fatalf("%s starts a browser, so its name must start with TestChrome (CI selects browser checks by name)", t.Name())
	}
	if testing.Short() {
		t.Skip("browser check skipped under -short")
	}
}

// browserSkip skips a browser check that cannot run here, or fails it under SODE_REQUIRE_BROWSER
// (set in CI), so a job never passes having run no browser checks.
func browserSkip(t *testing.T, why string) {
	t.Helper()
	if os.Getenv("SODE_REQUIRE_BROWSER") != "" {
		t.Fatal(why + " (SODE_REQUIRE_BROWSER is set)")
	}
	t.Skip(why)
}
